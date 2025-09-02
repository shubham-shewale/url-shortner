# URL Shortener API - Code Flow Documentation

This document explains how the URL Shortener API server works, designed for programmers coming from other languages.

## 🏗️ **API Architecture Overview**

```
┌─────────────────┐    HTTP     ┌─────────────────┐    DB Query    ┌─────────────────┐
│   CLI/Web       │ ──────────► │   HTTP Handler  │ ─────────────► │   Service Layer │
│   Client        │             │                 │                │                 │
│ • JSON Requests │             │ • Route Parsing │                │ • Business      │
│ • Auth Headers  │             │ • Input         │                │   Logic         │
│ • Response      │ ◄────────── │   Validation    │ ◄───────────── │ • Validation    │
│   Parsing       │             │ • JSON Response│                │ • Error         │
└─────────────────┘             └─────────────────┘                │   Handling     │
                                                                   └─────────────────┘
                                                                                   │
                                                                                   ▼
┌─────────────────┐    SQL      ┌─────────────────┐    Cache     ┌─────────────────┐
│   Storage       │ ◄────────── │   Database     │ ◄─────────── │   Redis Cache   │
│   Interface     │             │   (PostgreSQL) │                │                 │
│ • CRUD          │             │ • Persistence  │                │ • Fast Lookup  │
│   Operations    │             │ • Transactions │                │ • TTL          │
└─────────────────┘             └─────────────────┘                └─────────────────┘
```

## 🚀 **Request Flow Example**

### User runs: `shorten https://example.com --alias mylink`

1. **CLI sends HTTP request:**
   ```http
   POST /v1/links
   Content-Type: application/json

   {
     "long_url": "https://example.com",
     "alias": "mylink"
   }
   ```

2. **API Server Processing:**
   - **Router** (`chi`) matches `POST /v1/links` to `CreateLink` handler
   - **Handler** parses JSON request into `CreateLinkRequest` struct
   - **Service** validates URL, checks alias availability, generates code
   - **Storage** saves to PostgreSQL database
   - **Cache** stores in Redis for fast future lookups
   - **Response** returns JSON with short URL

## 📋 **Main Components**

### 1. **HTTP Handlers (`pkg/http/handlers.go`)**

```go
func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
    // 1. Parse JSON request
    var req service.CreateLinkRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // 2. Call service layer
    resp, err := h.linkService.CreateLink(r.Context(), &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 3. Return JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

**Key Concepts:**
- **HTTP Handlers**: Functions that process HTTP requests/responses
- **Context**: `r.Context()` for request-scoped values and cancellation
- **JSON Encoding/Decoding**: Converting between Go structs and JSON
- **Error Handling**: HTTP status codes and error messages

### 2. **Service Layer (`pkg/service/link_service.go`)**

```go
func (s *LinkService) CreateLink(ctx context.Context, req *CreateLinkRequest) (*CreateLinkResponse, error) {
    // 1. Validate URL format
    if _, err := url.ParseRequestURI(req.LongURL); err != nil {
        return nil, errors.New("invalid URL")
    }

    // 2. Check alias availability
    if req.Alias != nil {
        existing, err := s.storage.GetByCode(ctx, *req.Alias)
        if err != nil {
            return nil, err
        }
        if existing != nil {
            return nil, errors.New("alias already exists")
        }
    }

    // 3. Generate unique code
    code, err := GenerateCode(ctx, s.pool)
    if err != nil {
        return nil, err
    }

    // 4. Hash password if provided
    var passwordHash *string
    if req.Password != nil {
        hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
        if err != nil {
            return nil, err
        }
        hashStr := string(hash)
        passwordHash = &hashStr
    }

    // 5. Create link object
    link := &storage.Link{
        Code:         code,
        LongURL:      req.LongURL,
        Alias:        req.Alias,
        PasswordHash: passwordHash,
        ExpiresAt:    req.ExpiresAt,
        MaxClicks:    req.MaxClicks,
        ClickCount:   0,
        CreatedAt:    time.Now(),
    }

    // 6. Save to database
    err = s.storage.Create(ctx, link)
    if err != nil {
        return nil, err
    }

    // 7. Cache the result
    cachedLink := &cache.CachedLink{
        LongURL:     link.LongURL,
        HasPassword: link.PasswordHash != nil,
        ExpiresAt:   link.ExpiresAt,
        MaxClicks:   link.MaxClicks,
    }
    s.cache.Set(ctx, code, cachedLink, 24*time.Hour)

    // 8. Return response
    response := &CreateLinkResponse{
        Code:     code,
        ShortURL: "http://localhost:8080/r/" + code,
        Metadata: map[string]interface{}{
            "has_password": passwordHash != nil,
            "expires_at":   req.ExpiresAt,
            "max_clicks":   req.MaxClicks,
        },
    }
    return response, nil
}
```

**Service Layer Responsibilities:**
- **Business Logic**: URL validation, alias checking, password hashing
- **Data Transformation**: Converting between request/response types
- **Error Handling**: Meaningful error messages
- **Caching Strategy**: Store frequently accessed data in Redis
- **Transaction Management**: Database consistency

### 3. **Storage Layer (`pkg/storage/postgres.go`)**

```go
func (p *PostgresLinkStorage) Create(ctx context.Context, link *Link) error {
    query := `
        INSERT INTO links (code, long_url, alias, password_hash, expires_at, max_clicks, click_count, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `

    _, err := p.pool.Exec(ctx, query,
        link.Code,
        link.LongURL,
        link.Alias,
        link.PasswordHash,
        link.ExpiresAt,
        link.MaxClicks,
        link.ClickCount,
        link.CreatedAt,
    )
    return err
}
```

**Database Operations:**
- **Prepared Statements**: SQL queries with parameter placeholders (`$1`, `$2`, etc.)
- **Connection Pooling**: `pgxpool.Pool` for efficient database connections
- **Context Support**: Cancellation and timeout handling
- **Error Handling**: Database-specific error codes

### 4. **Cache Layer (`pkg/cache/redis.go`)**

```go
func (r *RedisLinkCache) Get(ctx context.Context, code string) (*CachedLink, error) {
    key := "link:" + code
    val, err := r.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return nil, nil // Cache miss
    }
    if err != nil {
        return nil, err
    }

    var cached CachedLink
    if err := json.Unmarshal([]byte(val), &cached); err != nil {
        return nil, err
    }

    return &cached, nil
}
```

**Caching Strategy:**
- **Fast Lookup**: Redis for sub-millisecond response times
- **TTL (Time To Live)**: Automatic expiration of cached data
- **JSON Serialization**: Store complex objects as JSON strings
- **Cache Invalidation**: Remove stale data when links are updated

## 🔄 **Complete Data Flow**

### Create Link Flow

```
CLI Request → HTTP Handler → Service Layer → Storage Layer → Database
     ↓              ↓              ↓              ↓              ↓
   JSON        Parse JSON    Validate Data   SQL Insert    PostgreSQL
     ↓              ↓              ↓              ↓              ↓
Response ←  JSON Response ← Format Response ← Return Data ←  Success
```

### Redirect Flow

```
User clicks short URL → HTTP Handler → Service Layer → Cache Check
         ↓                        ↓              ↓              ↓
    GET /r/{code}            Parse Code    Get Link Data   Redis Lookup
         ↓                        ↓              ↓              ↓
    Database Query ← Cache Miss ← Validate Link ← Check Expiry
         ↓                        ↓              ↓              ↓
    302 Redirect ← Update Clicks ← Valid Link ← Increment Counter
```

## 🛠️ **Key Go Patterns Used**

### 1. **Dependency Injection**

```go
// Constructor with dependencies
func NewLinkService(storage storage.LinkStorage, cache cache.LinkCacheInterface, pool *pgxpool.Pool) *LinkService {
    return &LinkService{
        storage: storage,
        cache:   cache,
        pool:    pool,
    }
}
```

**Benefits:**
- **Testability**: Easy to inject mock dependencies
- **Flexibility**: Swap implementations (different databases, caches)
- **Separation of Concerns**: Each layer focuses on one responsibility

### 2. **Interface-Based Design**

```go
type LinkStorage interface {
    Create(ctx context.Context, link *Link) error
    GetByCode(ctx context.Context, code string) (*Link, error)
    Update(ctx context.Context, link *Link) error
    Delete(ctx context.Context, code string) error
    IncrementClickCount(ctx context.Context, code string) error
}
```

**Advantages:**
- **Abstraction**: Implementation details hidden behind interface
- **Mocking**: Easy to create test doubles
- **Multiple Implementations**: Could support MongoDB, MySQL, etc.

### 3. **Context for Request Lifecycle**

```go
func (s *LinkService) CreateLink(ctx context.Context, req *CreateLinkRequest) (*CreateLinkResponse, error) {
    // Context carries request-scoped values
    // Can be cancelled by client or timeout
    link, err := s.storage.GetByCode(ctx, code)
    // ...
}
```

**Context Benefits:**
- **Cancellation**: Stop long-running operations
- **Timeout**: Automatic request termination
- **Request Tracing**: Pass request ID through call stack
- **Value Storage**: Request-scoped key-value storage

### 4. **Error Handling Strategy**

```go
func (s *LinkService) CreateLink(ctx context.Context, req *CreateLinkRequest) (*CreateLinkResponse, error) {
    // Validate URL
    if _, err := url.ParseRequestURI(req.LongURL); err != nil {
        return nil, errors.New("invalid URL")
    }

    // Check alias availability
    if req.Alias != nil {
        existing, err := s.storage.GetByCode(ctx, *req.Alias)
        if err != nil {
            return nil, err
        }
        if existing != nil {
            return nil, errors.New("alias already exists")
        }
    }
    // ... more validation
}
```

**Error Handling Patterns:**
- **Early Returns**: Fail fast on validation errors
- **Error Wrapping**: Add context to errors
- **Consistent Messages**: User-friendly error descriptions

## 🧪 **Testing Strategy**

### Unit Tests with Mocks

```go
func TestCreateLink(t *testing.T) {
    // Setup mocks
    mockStorage := newMockLinkStorage()
    mockCache := &mockLinkCache{}
    service := NewLinkService(mockStorage, mockCache, nil)

    // Test data
    req := &CreateLinkRequest{
        LongURL: "https://example.com",
        Alias:   stringPtr("test"),
    }

    // Execute
    resp, err := service.CreateLink(context.Background(), req)

    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Code)
    assert.Contains(t, resp.ShortURL, resp.Code)
}
```

### Integration Tests

```go
func TestCreateLinkEndpoint(t *testing.T) {
    // Setup test database
    mockStorage := newMockLinkStorage()
    mockCache := &mockLinkCache{}
    service := NewLinkService(mockStorage, mockCache, nil)
    handler := NewHandler(service)

    // Setup router
    r := chi.NewRouter()
    SetupRoutes(r, handler)

    // Test HTTP request
    reqBody := map[string]interface{}{
        "long_url": "https://example.com",
    }
    jsonData, _ := json.Marshal(reqBody)

    req := httptest.NewRequest("POST", "/v1/links", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)
}
```

## 🚀 **Performance Optimizations**

### 1. **Redis Caching Strategy**

```go
func (s *LinkService) GetLink(ctx context.Context, code string) (*storage.Link, error) {
    // 1. Try cache first (fast)
    cached, err := s.cache.Get(ctx, code)
    if err == nil && cached != nil {
        return convertCachedToLink(cached), nil
    }

    // 2. Cache miss - query database (slower)
    link, err := s.storage.GetByCode(ctx, code)
    if err != nil {
        return nil, err
    }

    // 3. Cache result for future requests
    if link != nil {
        s.cache.Set(ctx, code, convertToCached(link), 24*time.Hour)
    }

    return link, nil
}
```

### 2. **Connection Pooling**

```go
// Database connection pool
pool, err := pgxpool.New(context.Background(), dbURL)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

### 3. **Prepared Statements**

```go
// Reuse compiled SQL statements
stmt, err := pool.Prepare(ctx, "create_link", `
    INSERT INTO links (code, long_url, alias, password_hash, expires_at, max_clicks, click_count, created_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`)
```

## 🔒 **Security Features**

### Password Protection

```go
// Hash passwords using bcrypt
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
if err != nil {
    return err
}

// Verify passwords
err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
return err == nil
```

### Input Validation

```go
// URL validation
if _, err := url.ParseRequestURI(req.LongURL); err != nil {
    return nil, errors.New("invalid URL")
}

// Alias validation
if req.Alias != nil && !ValidateAlias(*req.Alias) {
    return nil, errors.New("invalid alias")
}
```

## 📊 **Database Schema**

```sql
CREATE TABLE links (
    code VARCHAR(255) PRIMARY KEY,
    long_url TEXT NOT NULL,
    alias VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255),
    expires_at TIMESTAMP,
    max_clicks INTEGER,
    click_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 🎯 **API Endpoints Summary**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/links` | Create short link |
| GET | `/v1/links/{code}` | Get link metadata |
| PATCH | `/v1/links/{code}` | Update link |
| DELETE | `/v1/links/{code}` | Delete link |
| POST | `/v1/links/{code}/verify` | Verify password |
| GET | `/r/{code}` | Redirect to URL |
| GET | `/health` | Health check |

This architecture provides a scalable, maintainable, and well-tested API server that follows Go best practices and can handle production workloads efficiently.
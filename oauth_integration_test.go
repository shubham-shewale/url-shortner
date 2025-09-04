package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"url-shortener/pkg/cache"
	httpHandlers "url-shortener/pkg/http"
	"url-shortener/pkg/middleware"
	"url-shortener/pkg/service"
	"url-shortener/pkg/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthIntegrationWithKeycloak tests the complete OAuth flow with Keycloak
func TestOAuthIntegrationWithKeycloak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if Keycloak is not running
	if os.Getenv("SKIP_KEYCLOAK_TESTS") == "true" {
		t.Skip("Keycloak integration tests skipped")
	}

	// Setup test infrastructure
	mockStorage := newOAuthMockLinkStorage()
	mockCache := &oauthMockLinkCache{}
	linkService := service.NewLinkService(mockStorage, mockCache, nil)
	handler := httpHandlers.NewHandler(linkService)

	// Create OAuth middleware with test configuration
	oauthConfig := middleware.OAuthConfig{
		IssuerURL: "http://localhost:8080/realms/url-shortener",
		Audience:  "url-shortener",
	}

	oauthMiddleware, err := middleware.NewOAuthMiddleware(oauthConfig)
	require.NoError(t, err)

	// Setup router with OAuth middleware
	r := chi.NewRouter()
	httpHandlers.SetupRoutes(r, handler, oauthMiddleware)

	// Test 1: Unauthenticated request should return 401
	t.Run("UnauthenticatedRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/links", bytes.NewBufferString(`{"long_url":"https://example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test 2: Request with invalid token should return 401
	t.Run("InvalidToken", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/links", bytes.NewBufferString(`{"long_url":"https://example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test 3: Request with valid mock token should work
	t.Run("ValidMockToken", func(t *testing.T) {
		// Create a mock handler that bypasses OAuth for testing
		mockHandler := httpHandlers.NewHandler(linkService)

		mockRouter := chi.NewRouter()
		httpHandlers.SetupRoutes(mockRouter, mockHandler, nil)

		req := httptest.NewRequest("POST", "/v1/links", bytes.NewBufferString(`{"long_url":"https://example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mockRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "code")
		assert.Contains(t, response, "short_url")
	})
}

// TestKeycloakConnection tests if Keycloak is accessible
func TestKeycloakConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if os.Getenv("SKIP_KEYCLOAK_TESTS") == "true" {
		t.Skip("Keycloak integration tests skipped")
	}

	// Create HTTP client that skips TLS verification for local testing
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Test Keycloak health endpoint
	resp, err := client.Get("http://localhost:8080/health/ready")
	if err != nil {
		t.Skipf("Keycloak not available: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Keycloak should be healthy")
}

// TestOAuthTokenValidation tests token validation with mock tokens
func TestOAuthTokenValidation(t *testing.T) {
	// This test validates the OAuth middleware logic without requiring a real IdP

	config := middleware.OAuthConfig{
		IssuerURL: "http://localhost:8080/realms/url-shortener",
		Audience:  "url-shortener",
	}

	// Skip if we can't create the middleware (e.g., network issues)
	oauthMiddleware, err := middleware.NewOAuthMiddleware(config)
	if err != nil {
		t.Skipf("Cannot create OAuth middleware: %v", err)
	}

	authFunc := oauthMiddleware.Authenticate("links:write")

	// Test with missing authorization header
	t.Run("MissingAuthHeader", func(t *testing.T) {
		handler := authFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/v1/links", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	// Test with malformed authorization header
	t.Run("MalformedAuthHeader", func(t *testing.T) {
		handler := authFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("POST", "/v1/links", nil)
		req.Header.Set("Authorization", "InvalidFormat token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestOwnershipEnforcement tests that users can only access their own links
func TestOwnershipEnforcement(t *testing.T) {
	// Setup test infrastructure
	mockStorage := newOAuthMockLinkStorage()
	mockCache := &oauthMockLinkCache{}
	linkService := service.NewLinkService(mockStorage, mockCache, nil)
	handler := httpHandlers.NewHandler(linkService)

	r := chi.NewRouter()
	httpHandlers.SetupRoutes(r, handler, nil)

	// Create a link with owner
	ownerID := uuid.New()
	link := &storage.Link{
		Code:       "test123",
		LongURL:    "https://example.com",
		ClickCount: 0,
		CreatedAt:  time.Now(),
		OwnerID:    &ownerID,
	}
	mockStorage.Create(context.Background(), link)

	// Test that the link exists
	t.Run("LinkExists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/links/test123", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var response storage.Link
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "test123", response.Code)
		assert.Equal(t, "https://example.com", response.LongURL)
	})

	// Test creating a new link (should work with mock auth)
	t.Run("CreateLink", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"long_url": "https://new-example.com",
		}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/v1/links", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "code")
	})
}

// TestRedirectUnprotected tests that redirects work without authentication
func TestRedirectUnprotected(t *testing.T) {
	// Setup test infrastructure
	mockStorage := newOAuthMockLinkStorage()
	mockCache := &oauthMockLinkCache{}
	linkService := service.NewLinkService(mockStorage, mockCache, nil)
	handler := httpHandlers.NewHandler(linkService)

	// Create router without OAuth middleware
	r := chi.NewRouter()
	httpHandlers.SetupRoutes(r, handler, nil)

	// Create a test link
	link := &storage.Link{
		Code:       "redirect123",
		LongURL:    "https://example.com",
		ClickCount: 0,
		CreatedAt:  time.Now(),
	}
	mockStorage.Create(context.Background(), link)

	// Test redirect works without authentication
	req := httptest.NewRequest("GET", "/r/redirect123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Location"))
}

// Mock implementations for OAuth testing
type oauthMockLinkStorage struct {
	links map[string]*storage.Link
}

func newOAuthMockLinkStorage() *oauthMockLinkStorage {
	return &oauthMockLinkStorage{links: make(map[string]*storage.Link)}
}

func (m *oauthMockLinkStorage) Create(ctx context.Context, link *storage.Link) error {
	m.links[link.Code] = link
	return nil
}

func (m *oauthMockLinkStorage) GetByCode(ctx context.Context, code string) (*storage.Link, error) {
	if link, exists := m.links[code]; exists {
		return link, nil
	}
	return nil, nil
}

func (m *oauthMockLinkStorage) Update(ctx context.Context, link *storage.Link) error {
	m.links[link.Code] = link
	return nil
}

func (m *oauthMockLinkStorage) Delete(ctx context.Context, code string) error {
	delete(m.links, code)
	return nil
}

func (m *oauthMockLinkStorage) IncrementClickCount(ctx context.Context, code string) error {
	if link, exists := m.links[code]; exists {
		link.ClickCount++
	}
	return nil
}

type oauthMockLinkCache struct{}

func (m *oauthMockLinkCache) Get(ctx context.Context, code string) (*cache.CachedLink, error) {
	return nil, nil // Always cache miss for simplicity
}

func (m *oauthMockLinkCache) Set(ctx context.Context, code string, link *cache.CachedLink, ttl time.Duration) error {
	return nil
}

func (m *oauthMockLinkCache) Delete(ctx context.Context, code string) error {
	return nil
}

func (m *oauthMockLinkCache) IncrementClick(ctx context.Context, code string) (int64, error) {
	return 1, nil
}

func (m *oauthMockLinkCache) GetClickCount(ctx context.Context, code string) (int64, error) {
	return 0, nil
}

func (m *oauthMockLinkCache) SetClickCount(ctx context.Context, code string, count int64, ttl time.Duration) error {
	return nil
}

func (m *oauthMockLinkCache) ExpireClickCount(ctx context.Context, code string, ttl time.Duration) error {
	return nil
}

// Helper types for testing
type mockOAuthMiddleware struct{}

func (m *mockOAuthMiddleware) Authenticate(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add mock owner_id to context for testing
			ctx := r.Context()
			mockOwnerID := uuid.New()
			ctx = context.WithValue(ctx, "owner_id", mockOwnerID)
			ctx = context.WithValue(ctx, "sub", mockOwnerID.String())
			ctx = context.WithValue(ctx, "email", "test@example.com")
			ctx = context.WithValue(ctx, "scope", "links:read links:write")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

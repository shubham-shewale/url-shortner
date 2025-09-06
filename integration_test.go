package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"url-shortener/pkg/cache"
	httpHandlers "url-shortener/pkg/http"
	"url-shortener/pkg/logging"
	"url-shortener/pkg/security"
	"url-shortener/pkg/service"
	"url-shortener/pkg/storage"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// Mock implementations for testing
type mockLinkStorage struct {
	links map[string]*storage.Link
}

func newMockLinkStorage() *mockLinkStorage {
	return &mockLinkStorage{links: make(map[string]*storage.Link)}
}

func (m *mockLinkStorage) CreateTx(ctx context.Context, tx pgx.Tx, link *storage.Link) error {
	m.links[link.Code] = link
	return nil
}

func (m *mockLinkStorage) Create(ctx context.Context, link *storage.Link) error {
	return m.CreateTx(ctx, nil, link)
}

func (m *mockLinkStorage) GetByCodeTx(ctx context.Context, tx pgx.Tx, code string) (*storage.Link, error) {
	if link, exists := m.links[code]; exists {
		return link, nil
	}
	return nil, nil
}

func (m *mockLinkStorage) GetByCode(ctx context.Context, code string) (*storage.Link, error) {
	return m.GetByCodeTx(ctx, nil, code)
}

func (m *mockLinkStorage) Update(ctx context.Context, link *storage.Link) error {
	m.links[link.Code] = link
	return nil
}

func (m *mockLinkStorage) Delete(ctx context.Context, code string) error {
	delete(m.links, code)
	return nil
}

func (m *mockLinkStorage) IncrementClickCount(ctx context.Context, code string) error {
	if link, exists := m.links[code]; exists {
		link.ClickCount++
	}
	return nil
}

type mockLinkCache struct{}

func (m *mockLinkCache) Get(ctx context.Context, code string) (*cache.CachedLink, error) {
	return nil, nil // Always cache miss for simplicity
}

func (m *mockLinkCache) Set(ctx context.Context, code string, link *cache.CachedLink, ttl time.Duration) error {
	return nil
}

func (m *mockLinkCache) Delete(ctx context.Context, code string) error {
	return nil
}

func (m *mockLinkCache) IncrementClick(ctx context.Context, code string) (int64, error) {
	return 1, nil
}

func (m *mockLinkCache) GetClickCount(ctx context.Context, code string) (int64, error) {
	return 0, nil
}

func (m *mockLinkCache) SetClickCount(ctx context.Context, code string, count int64, ttl time.Duration) error {
	return nil
}

func (m *mockLinkCache) ExpireClickCount(ctx context.Context, code string, ttl time.Duration) error {
	return nil
}

func TestCreateLinkEndpoint(t *testing.T) {
	// Setup
	mockStorage := newMockLinkStorage()
	mockCache := &mockLinkCache{}
	logger := logging.NewLogger(logging.LevelInfo)
	linkService := service.NewLinkService(mockStorage, mockCache, nil, logger) // pool not needed for this test
	csrfManager := security.NewCSRFTokenManager()
	handler := httpHandlers.NewHandler(linkService, csrfManager)

	r := chi.NewRouter()
	noopCSRF := func(next http.Handler) http.Handler { return next } // No CSRF for tests
	httpHandlers.SetupRoutes(r, handler, nil, noopCSRF)

	// Test data
	reqBody := map[string]interface{}{
		"long_url": "https://example.com",
	}
	jsonData, _ := json.Marshal(reqBody)

	// Create request
	req := httptest.NewRequest("POST", "/v1/links", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Contains(t, response, "code")
	assert.Contains(t, response, "short_url")
	assert.Contains(t, response, "metadata")
}

func TestHealthCheck(t *testing.T) {
	mockStorage := newMockLinkStorage()
	mockCache := &mockLinkCache{}
	logger := logging.NewLogger(logging.LevelInfo)
	linkService := service.NewLinkService(mockStorage, mockCache, nil, logger)
	csrfManager := security.NewCSRFTokenManager()
	handler := httpHandlers.NewHandler(linkService, csrfManager)

	r := chi.NewRouter()
	noopCSRF := func(next http.Handler) http.Handler { return next }
	httpHandlers.SetupRoutes(r, handler, nil, noopCSRF)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestGetLinkEndpoint(t *testing.T) {
	// Setup
	mockStorage := newMockLinkStorage()
	mockCache := &mockLinkCache{}

	// Pre-populate with a link
	link := &storage.Link{
		Code:       "test123",
		LongURL:    "https://example.com",
		ClickCount: 5,
		CreatedAt:  time.Now(),
	}
	mockStorage.Create(context.Background(), link)

	logger := logging.NewLogger(logging.LevelInfo)
	linkService := service.NewLinkService(mockStorage, mockCache, nil, logger)
	csrfManager := security.NewCSRFTokenManager()
	handler := httpHandlers.NewHandler(linkService, csrfManager)

	r := chi.NewRouter()
	noopCSRF := func(next http.Handler) http.Handler { return next }
	httpHandlers.SetupRoutes(r, handler, nil, noopCSRF)

	// Test GET request
	req := httptest.NewRequest("GET", "/v1/links/test123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response storage.Link
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.Equal(t, "test123", response.Code)
	assert.Equal(t, "https://example.com", response.LongURL)
	assert.Equal(t, 5, response.ClickCount)
}

func TestDeleteLinkEndpoint(t *testing.T) {
	// Setup
	mockStorage := newMockLinkStorage()
	mockCache := &mockLinkCache{}

	// Pre-populate with a link
	link := &storage.Link{
		Code:       "test123",
		LongURL:    "https://example.com",
		ClickCount: 5,
		CreatedAt:  time.Now(),
	}
	mockStorage.Create(context.Background(), link)

	logger := logging.NewLogger(logging.LevelInfo)
	linkService := service.NewLinkService(mockStorage, mockCache, nil, logger)
	csrfManager := security.NewCSRFTokenManager()
	handler := httpHandlers.NewHandler(linkService, csrfManager)

	r := chi.NewRouter()
	noopCSRF := func(next http.Handler) http.Handler { return next }
	httpHandlers.SetupRoutes(r, handler, nil, noopCSRF)

	// Test DELETE request
	req := httptest.NewRequest("DELETE", "/v1/links/test123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify link is deleted
	req2 := httptest.NewRequest("GET", "/v1/links/test123", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestInvalidURLError(t *testing.T) {
	mockStorage := newMockLinkStorage()
	mockCache := &mockLinkCache{}
	logger := logging.NewLogger(logging.LevelInfo)
	linkService := service.NewLinkService(mockStorage, mockCache, nil, logger)
	csrfManager := security.NewCSRFTokenManager()
	handler := httpHandlers.NewHandler(linkService, csrfManager)

	r := chi.NewRouter()
	noopCSRF := func(next http.Handler) http.Handler { return next }
	httpHandlers.SetupRoutes(r, handler, nil, noopCSRF)

	// Test with invalid URL
	reqBody := map[string]interface{}{
		"long_url": "not-a-valid-url",
	}
	jsonData, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/links", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthMiddleware_ValidToken(t *testing.T) {
	// Skip this test in CI environments or when network is not available
	t.Skip("Skipping test that requires network access to OIDC provider")

	config := OAuthConfig{
		IssuerURL: "https://test-issuer.com",
		Audience:  "test-audience",
	}

	middleware, err := NewOAuthMiddleware(config)
	assert.NoError(t, err)
	assert.NotNil(t, middleware)
}

func TestOAuthMiddleware_InvalidToken(t *testing.T) {
	t.Skip("Skipping test that requires network access to OIDC provider")

	config := OAuthConfig{
		IssuerURL: "https://test-issuer.com",
		Audience:  "test-audience",
	}

	middleware, err := NewOAuthMiddleware(config)
	assert.NoError(t, err)

	authFunc := middleware.Authenticate("links:read")

	handler := authFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 401 for invalid token
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOAuthMiddleware_MissingAuthHeader(t *testing.T) {
	t.Skip("Skipping test that requires network access to OIDC provider")

	config := OAuthConfig{
		IssuerURL: "https://test-issuer.com",
		Audience:  "test-audience",
	}

	middleware, err := NewOAuthMiddleware(config)
	assert.NoError(t, err)

	authFunc := middleware.Authenticate("links:read")

	handler := authFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 401 for missing auth header
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOAuthMiddleware_InvalidAuthHeaderFormat(t *testing.T) {
	t.Skip("Skipping test that requires network access to OIDC provider")

	config := OAuthConfig{
		IssuerURL: "https://test-issuer.com",
		Audience:  "test-audience",
	}

	middleware, err := NewOAuthMiddleware(config)
	assert.NoError(t, err)

	authFunc := middleware.Authenticate("links:read")

	handler := authFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 401 for invalid auth header format
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

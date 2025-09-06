package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type CSRFTokenManager struct {
	tokens map[string]csrfToken
}

type csrfToken struct {
	value     string
	createdAt time.Time
	expires   time.Time
}

func NewCSRFTokenManager() *CSRFTokenManager {
	return &CSRFTokenManager{
		tokens: make(map[string]csrfToken),
	}
}

func (c *CSRFTokenManager) GenerateToken(sessionID string) (string, error) {
	// Generate cryptographically secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store with expiration
	c.tokens[sessionID] = csrfToken{
		value:     token,
		createdAt: time.Now(),
		expires:   time.Now().Add(15 * time.Minute),
	}

	// Cleanup expired tokens
	go c.cleanupExpired()

	return token, nil
}

func (c *CSRFTokenManager) ValidateToken(sessionID, providedToken string) bool {
	storedToken, exists := c.tokens[sessionID]
	if !exists {
		return false
	}

	// Check expiration
	if time.Now().After(storedToken.expires) {
		delete(c.tokens, sessionID)
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(storedToken.value), []byte(providedToken)) == 1
}

func (c *CSRFTokenManager) InvalidateToken(sessionID string) {
	delete(c.tokens, sessionID)
}

func (c *CSRFTokenManager) cleanupExpired() {
	now := time.Now()
	for sessionID, token := range c.tokens {
		if now.After(token.expires) {
			delete(c.tokens, sessionID)
		}
	}
}

// CSRF Middleware
func CSRFMiddleware(tokenManager *CSRFTokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check CSRF for state-changing methods
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
				sessionID := getOrCreateSessionID(w, r)

				// Get CSRF token from header or form
				token := r.Header.Get("X-CSRF-Token")
				if token == "" {
					token = r.FormValue("csrf_token")
				}

				if !tokenManager.ValidateToken(sessionID, token) {
					http.Error(w, "Invalid CSRF token", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getOrCreateSessionID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil || cookie.Value == "" {
		// Create new session ID
		sessionID := uuid.New().String()
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400, // 24 hours
		})
		return sessionID
	}
	return cookie.Value
}

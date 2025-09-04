package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
)

type OAuthConfig struct {
	IssuerURL string
	Audience  string
}

type OAuthMiddleware struct {
	verifier *oidc.IDTokenVerifier
}

type AuthClaims struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Scope  string   `json:"scope"`
	Groups []string `json:"groups,omitempty"`
}

func NewOAuthMiddleware(config OAuthConfig) (*OAuthMiddleware, error) {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.Audience,
	})

	return &OAuthMiddleware{
		verifier: verifier,
	}, nil
}

func (m *OAuthMiddleware) Authenticate(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			// Parse and validate the JWT
			token, err := m.parseAndValidateToken(tokenString)
			if err != nil {
				log.Printf("OAuth middleware error: %v", err)
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Extract claims
			claims, err := m.extractClaims(token)
			if err != nil {
				http.Error(w, "failed to extract claims", http.StatusUnauthorized)
				return
			}

			// Check audience
			if !m.checkAudience(token, "url-shortener") {
				http.Error(w, "invalid audience", http.StatusUnauthorized)
				return
			}

			// Check scopes if required
			if len(requiredScopes) > 0 {
				if !m.checkScopes(claims.Scope, requiredScopes) {
					http.Error(w, "insufficient scope", http.StatusForbidden)
					return
				}
			}

			// Add claims to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, "sub", claims.Sub)
			ctx = context.WithValue(ctx, "email", claims.Email)
			ctx = context.WithValue(ctx, "scope", claims.Scope)

			// Convert sub to UUID for owner_id
			if subUUID, err := uuid.Parse(claims.Sub); err == nil {
				ctx = context.WithValue(ctx, "owner_id", subUUID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *OAuthMiddleware) parseAndValidateToken(tokenString string) (*oidc.IDToken, error) {
	return m.verifier.Verify(context.Background(), tokenString)
}

func (m *OAuthMiddleware) extractClaims(token *oidc.IDToken) (*AuthClaims, error) {
	var claims AuthClaims
	if err := token.Claims(&claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

func (m *OAuthMiddleware) checkAudience(token *oidc.IDToken, expectedAudience string) bool {
	var claims map[string]interface{}
	if err := token.Claims(&claims); err != nil {
		return false
	}

	aud, ok := claims["aud"]
	if !ok {
		return false
	}

	switch v := aud.(type) {
	case string:
		return v == expectedAudience
	case []interface{}:
		for _, a := range v {
			if str, ok := a.(string); ok && str == expectedAudience {
				return true
			}
		}
	}
	return false
}

func (m *OAuthMiddleware) checkScopes(tokenScopes string, requiredScopes []string) bool {
	scopes := strings.Fields(tokenScopes)
	scopeMap := make(map[string]bool)
	for _, s := range scopes {
		scopeMap[s] = true
	}

	for _, required := range requiredScopes {
		if !scopeMap[required] {
			return false
		}
	}
	return true
}

// Helper functions to extract values from context
func GetSubFromContext(ctx context.Context) string {
	if sub, ok := ctx.Value("sub").(string); ok {
		return sub
	}
	return ""
}

func GetEmailFromContext(ctx context.Context) string {
	if email, ok := ctx.Value("email").(string); ok {
		return email
	}
	return ""
}

func GetScopeFromContext(ctx context.Context) string {
	if scope, ok := ctx.Value("scope").(string); ok {
		return scope
	}
	return ""
}

func GetOwnerIDFromContext(ctx context.Context) uuid.UUID {
	if ownerID, ok := ctx.Value("owner_id").(uuid.UUID); ok {
		return ownerID
	}
	return uuid.Nil
}

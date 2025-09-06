package main

import (
	"context"
	"log"
	stdhttp "net/http"
	"os"

	"url-shortener/pkg/cache"
	"url-shortener/pkg/http"
	"url-shortener/pkg/logging"
	"url-shortener/pkg/middleware"
	"url-shortener/pkg/security"
	"url-shortener/pkg/service"
	"url-shortener/pkg/storage"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Initialize logger
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // Default to info in production
	}
	logger := logging.NewLogger(logging.LogLevel(logLevel))

	// DB connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/urlshortener?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Redis connection
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal(err)
	}

	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	// Cache
	linkCache := cache.NewLinkCache(redisClient)

	// Storage
	linkStorage := storage.NewPostgresLinkStorage(pool)

	// Service
	linkService := service.NewLinkService(linkStorage, linkCache, pool, logger)

	// OAuth Middleware
	oauthConfig := middleware.OAuthConfig{
		IssuerURL: os.Getenv("OIDC_ISSUER"),
		Audience:  os.Getenv("OIDC_AUDIENCE"),
	}
	if oauthConfig.IssuerURL == "" {
		oauthConfig.IssuerURL = "https://dev-123456.okta.com" // Default for development
	}
	if oauthConfig.Audience == "" {
		oauthConfig.Audience = "url-shortener"
	}

	oauthMiddleware, err := middleware.NewOAuthMiddleware(oauthConfig)
	if err != nil {
		log.Fatal("Failed to create OAuth middleware:", err)
	}

	// CSRF Protection
	csrfManager := security.NewCSRFTokenManager()
	csrfMiddleware := security.CSRFMiddleware(csrfManager)

	// Handler
	handler := http.NewHandler(linkService, csrfManager)

	// Router
	r := chi.NewRouter()
	http.SetupRoutes(r, handler, oauthMiddleware, csrfMiddleware)

	// Server
	log.Println("Starting API server on :8080")
	log.Fatal(stdhttp.ListenAndServe(":8080", r))
}

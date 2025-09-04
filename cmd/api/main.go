package main

import (
	"context"
	"log"
	stdhttp "net/http"
	"os"

	"url-shortener/pkg/cache"
	"url-shortener/pkg/http"
	"url-shortener/pkg/middleware"
	"url-shortener/pkg/service"
	"url-shortener/pkg/storage"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
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
	linkService := service.NewLinkService(linkStorage, linkCache, pool)

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

	// Handler
	handler := http.NewHandler(linkService)

	// Router
	r := chi.NewRouter()
	http.SetupRoutes(r, handler, oauthMiddleware)

	// Server
	log.Println("Starting API server on :8080")
	log.Fatal(stdhttp.ListenAndServe(":8080", r))
}

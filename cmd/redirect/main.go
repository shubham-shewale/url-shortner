package main

import (
	"context"
	"log"
	stdhttp "net/http"
	"os"

	"url-shortener/pkg/cache"
	httphandler "url-shortener/pkg/http"
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

	// Handler
	handler := httphandler.NewHandler(linkService)

	// Router
	r := chi.NewRouter()
	r.Get("/r/{code}", handler.Redirect)

	// Server
	log.Println("Starting redirect server on :8081")
	log.Fatal(stdhttp.ListenAndServe(":8081", r))
}

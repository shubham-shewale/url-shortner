# URL Shortener

A production-ready URL shortener service built in Go with PostgreSQL and Redis.

## Features

- Shorten URLs with optional custom aliases
- Password protection for links
- Time-based and click-based expiry
- Redis caching for performance
- RESTful API

## Endpoints

- `POST /v1/links` - Create a short link
- `GET /r/{code}` - Redirect to original URL
- `POST /v1/links/{code}/verify` - Verify password for protected links
- `GET /v1/links/{code}` - Get link metadata
- `DELETE /v1/links/{code}` - Delete link

## Running

1. Start services: `docker-compose up -d`
2. Build: `make build`
3. Run API: `./api`
4. Run Redirector: `./redirect`

## API Documentation

The complete API specification is available in `openapi.yaml` following OpenAPI 3.0.3 standard.

## Testing

- Unit tests: `make test`
- With race detector: `make test-race`
- Coverage: `make coverage`

## Password Protection Caveats

Password-protected links limit access to the redirect, not the destination resource. The destination URL is not protected by the password; only the redirect is gated.

## Environment Variables

- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string
# Keycloak Integration Tests

This document describes how to run integration tests with Keycloak for OAuth 2.0 authentication.

## Prerequisites

- Docker and Docker Compose
- Go 1.23+
- Make sure no other service is running on port 8080

## Setup Keycloak

1. Start Keycloak using Docker Compose:

```bash
cd url-shortener
docker-compose -f docker-compose.test.yml up -d
```

2. Wait for Keycloak to be ready (this may take a few minutes):

```bash
# Check if Keycloak is healthy
curl -f http://localhost:8080/health/ready
```

3. Access Keycloak admin console at http://localhost:8080
   - Username: `admin`
   - Password: `admin`

4. Create a realm called `url-shortener`

5. Create a client:
   - Client ID: `url-shortener`
   - Client Type: `OpenID Connect`
   - Access Type: `confidential`
   - Valid Redirect URIs: `http://localhost:53682/callback`

6. Create a user for testing

## Running Tests

### Run OAuth Integration Tests

```bash
# Run all OAuth integration tests
go test -v ./oauth_integration_test.go

# Run specific test
go test -v -run TestOAuthIntegrationWithKeycloak ./oauth_integration_test.go

# Run tests with verbose output
go test -v -run TestKeycloakConnection ./oauth_integration_test.go
```

### Skip Keycloak Tests

If Keycloak is not available, you can skip the integration tests:

```bash
# Set environment variable to skip Keycloak tests
export SKIP_KEYCLOAK_TESTS=true
go test -v ./oauth_integration_test.go
```

### Run All Tests

```bash
# Run all tests including integration tests
go test -v ./...

# Run tests in short mode (skips integration tests)
go test -short -v ./...
```

## Test Configuration

The integration tests are configured to:

- Connect to Keycloak at `http://localhost:8080/realms/url-shortener`
- Use audience `url-shortener`
- Test OAuth middleware functionality
- Validate token authentication
- Test ownership enforcement
- Verify unprotected redirect endpoints

## Troubleshooting

### Keycloak Not Starting

If Keycloak fails to start, check:

```bash
# Check Keycloak logs
docker-compose -f docker-compose.test.yml logs keycloak

# Check if port 8080 is available
netstat -an | grep 8080
```

### Tests Failing

Common issues:

1. **Keycloak not ready**: Wait longer for Keycloak to fully initialize
2. **Port conflicts**: Ensure no other service uses port 8080
3. **Network issues**: Check Docker network connectivity
4. **Realm/client misconfiguration**: Verify Keycloak setup matches test expectations

### Clean Up

```bash
# Stop and remove containers
docker-compose -f docker-compose.test.yml down

# Remove volumes
docker-compose -f docker-compose.test.yml down -v
```

## Test Coverage

The OAuth integration tests cover:

- ✅ OAuth middleware initialization
- ✅ Token validation with real Keycloak
- ✅ Authentication flow
- ✅ Authorization with scopes
- ✅ Ownership enforcement
- ✅ Unprotected endpoints (redirects)
- ✅ Error handling for invalid tokens
- ✅ Keycloak connectivity checks
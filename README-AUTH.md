# OAuth 2.0 Authentication

This document describes the OAuth 2.0 Bearer token authentication implementation for the URL Shortener API.

## Overview

The URL Shortener now supports OAuth 2.0 Bearer token authentication for its management and analytics endpoints. Redirects (`GET /r/{code}`) remain open and do not require authentication.

## Supported Endpoints

### Protected Endpoints (Require Bearer Token)
- `POST /v1/links` - Create new short links
- `GET /v1/links/{code}` - Get link metadata
- `PATCH /v1/links/{code}` - Update link properties
- `DELETE /v1/links/{code}` - Delete links

### Unprotected Endpoints
- `GET /health` - Health check
- `GET /r/{code}` - URL redirects
- `POST /v1/links/{code}/verify` - Password verification

## Authentication Flow

### 1. Obtain Access Token
The application supports Authorization Code + PKCE flow and Client Credentials flow.

#### Authorization Code + PKCE (Interactive)
1. Redirect user to IdP's `/authorize` endpoint
2. User authenticates and consents
3. Receive authorization code
4. Exchange code for access token at `/token` endpoint

#### Client Credentials (Service-to-Service)
1. Send client credentials to `/token` endpoint
2. Receive access token

### 2. Use Access Token
Include the access token in the `Authorization` header:

```
Authorization: Bearer <access_token>
```

## Configuration

Set the following environment variables:

```bash
OIDC_ISSUER=https://your-idp.com
OIDC_AUDIENCE=url-shortener
```

### Supported IdPs
- Auth0
- Google Identity Platform
- Keycloak
- Any OIDC-compliant provider

## Token Requirements

### Claims
- `sub` - Subject (user ID, converted to UUID for owner_id)
- `aud` - Audience (must include "url-shortener")
- `exp` - Expiration time
- `scope` - Space-separated list of scopes

### Required Scopes
- `links:read` - Required for GET operations
- `links:write` - Required for POST, PATCH, DELETE operations
- `admin:*` - Administrative operations (future use)

## Database Changes

The `links` table now includes an `owner_id` column (UUID, NOT NULL) that associates each link with its creator. The system enforces ownership:

- Users can only view/update/delete links they own
- The `sub` claim from the JWT is used as the owner identifier

## Migration

Run the database migration to update the schema:

```sql
ALTER TABLE links ALTER COLUMN owner_id TYPE UUID USING owner_id::UUID;
ALTER TABLE links ALTER COLUMN owner_id SET NOT NULL;
```

## Testing

### Unit Tests
```bash
go test ./pkg/middleware
```

### Integration Tests
```bash
go test -run TestOAuthFlow
```

### With Keycloak
1. Start Keycloak in Docker:
```bash
docker run -p 8080:8080 -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin quay.io/keycloak/keycloak:latest start-dev
```

2. Configure realm and client
3. Set environment variables
4. Run tests

## Error Responses

### 401 Unauthorized
```json
{
  "error": "missing authorization header"
}
```

### 403 Forbidden
```json
{
  "error": "insufficient scope"
}
```

### 403 Forbidden (Ownership)
```json
{
  "error": "access denied: not the owner of this link"
}
```

## Security Considerations

1. **Token Validation**: All tokens are validated against the configured OIDC issuer
2. **Audience Check**: Tokens must include "url-shortener" in the audience
3. **Scope Enforcement**: Operations require appropriate scopes
4. **Ownership Enforcement**: Users can only access their own resources
5. **HTTPS Only**: Always use HTTPS in production
6. **Token Expiration**: Expired tokens are automatically rejected

## CLI Integration

The CLI tool supports OAuth login:

```bash
shorten login --provider <idp>
```

This will:
1. Launch browser to IdP's authorization endpoint
2. Handle the redirect with a local server
3. Store tokens in `~/.shorten/tokens.json`
4. Automatically include Bearer tokens in API requests

## API Documentation

Updated OpenAPI specification includes:
- `bearerAuth` security scheme
- Security requirements for protected endpoints
- Authentication examples

View the complete API documentation in `openapi.yaml`.
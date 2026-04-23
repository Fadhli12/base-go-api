# HTTP API Contracts: API Key Authentication

**Feature**: API Key Authentication  
**Version**: 1.0 | **Date**: 2026-04-23 | **Status**: Phase 1 Design

---

## Overview

All endpoints follow the existing API Base response envelope format. Authentication via X-API-Key header (alternative to JWT Bearer token).

**Base URL**: `/api/v1`

**Authentication**:
- JWT Bearer token in `Authorization` header (existing)
- API Key in `X-API-Key` header (new)

**Response Envelope** (same as existing):
```json
{
  "data": { /* payload */ },
  "error": null,
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

---

## API Key Management Endpoints

### POST /api-keys

**Purpose**: Create a new API key for the authenticated user.

**Authentication**: JWT Bearer token (required)

**Request**:
```json
{
  "name": "Production Integration",
  "scopes": ["invoices:read", "invoices:write", "news:read"],
  "expires_at": "2027-04-23T00:00:00Z"  // Optional
}
```

**Validation**:
- `name`: Required, max 255 characters, unique per user
- `scopes`: Required, array of valid scope strings
- `scopes`: Must be subset of user's effective permissions
- `expires_at`: Optional, must be future timestamp

**Response** (201 Created):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Production Integration",
    "prefix": "ak_live_a1b2",
    "scopes": ["invoices:read", "invoices:write", "news:read"],
    "expires_at": "2027-04-23T00:00:00Z",
    "created_at": "2026-04-23T10:30:00Z",
    "key": "ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
  },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440001",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

**Important**: The `key` field is ONLY returned in this response. It cannot be retrieved later.

**Error Responses**:
- `400 INVALID_REQUEST`: Invalid scope format or validation failure
- `401 UNAUTHORIZED`: Missing or invalid JWT token
- `403 PERMISSION_DENIED`: User lacks required permissions for requested scopes
- `409 CONFLICT`: Key name already exists for this user

**Audit**: Creates `api_key:create` audit log entry.

---

### GET /api-keys

**Purpose**: List all API keys for the authenticated user.

**Authentication**: JWT Bearer token (required)

**Query Parameters**:
- `limit`: Number of results (default: 20, max: 100)
- `offset`: Pagination offset (default: 0)

**Response** (200 OK):
```json
{
  "data": {
    "api_keys": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "Production Integration",
        "prefix": "ak_live_a1b2",
        "scopes": ["invoices:read", "invoices:write", "news:read"],
        "expires_at": "2027-04-23T00:00:00Z",
        "last_used_at": "2026-04-22T15:30:00Z",
        "is_revoked": false,
        "created_at": "2026-04-20T10:00:00Z",
        "updated_at": "2026-04-22T15:30:00Z"
      },
      {
        "id": "660e8400-e29b-41d4-a716-446655440000",
        "name": "Test Integration",
        "prefix": "ak_test_x9y8",
        "scopes": ["invoices:read"],
        "expires_at": null,
        "last_used_at": null,
        "is_revoked": true,
        "created_at": "2026-04-21T14:00:00Z",
        "updated_at": "2026-04-21T16:30:00Z"
      }
    ],
    "total": 2,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440002",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

**Note**: Full key values are NOT returned, only the prefix for identification.

**Error Responses**:
- `401 UNAUTHORIZED`: Missing or invalid JWT token

---

### GET /api-keys/:id

**Purpose**: Get details of a specific API key.

**Authentication**: JWT Bearer token (required)

**Path Parameters**:
- `id`: API key UUID

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Production Integration",
    "prefix": "ak_live_a1b2",
    "scopes": ["invoices:read", "invoices:write", "news:read"],
    "expires_at": "2027-04-23T00:00:00Z",
    "last_used_at": "2026-04-22T15:30:00Z",
    "is_revoked": false,
    "created_at": "2026-04-20T10:00:00Z",
    "updated_at": "2026-04-22T15:30:00Z"
  },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440003",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

**Error Responses**:
- `401 UNAUTHORIZED`: Missing or invalid JWT token
- `403 PERMISSION_DENIED`: API key belongs to another user
- `404 NOT_FOUND`: API key not found

---

### DELETE /api-keys/:id

**Purpose**: Revoke an API key (soft deletion).

**Authentication**: JWT Bearer token (required)

**Path Parameters**:
- `id`: API key UUID

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "message": "API key revoked successfully"
  },
  "error": null,
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440004",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

**Behavior**:
- Sets `revoked_at` timestamp (soft revocation)
- Key remains in database for audit trail
- Subsequent authentication requests with this key return 401

**Error Responses**:
- `401 UNAUTHORIZED`: Missing or invalid JWT token
- `403 PERMISSION_DENIED`: API key belongs to another user
- `404 NOT_FOUND`: API key not found
- `409 CONFLICT`: API key already revoked

**Audit**: Creates `api_key:revoke` audit log entry.

---

## Authentication with API Key

### Using API Key in Requests

All existing API endpoints can be authenticated using X-API-Key header instead of Bearer token.

**Request**:
```http
GET /api/v1/invoices HTTP/1.1
Host: api.example.com
X-API-Key: ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

**Authentication Flow**:
1. Middleware checks for `Authorization: Bearer` header
2. If not found, checks for `X-API-Key` header
3. If API key found, validates:
   - Key exists in database
   - Key not revoked (`revoked_at IS NULL`)
   - Key not expired (`expires_at IS NULL OR expires_at > NOW()`)
   - Key's user not deleted (`users.deleted_at IS NULL`)
4. If valid, sets user context and scopes
5. Permission middleware checks scopes allow requested action

**Success Response**: Same as JWT authentication

**Error Responses**:
```json
{
  "data": null,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid API key",
    "details": {
      "reason": "revoked|expired|not_found|user_deleted"
    }
  },
  "meta": {
    "request_id": "550e8400-e29b-41d4-a716-446655440005",
    "timestamp": "2026-04-23T10:30:00Z"
  }
}
```

---

## Scope Validation

### Scope to Permission Mapping

| Scope | Permissions Granted |
|-------|---------------------|
| `invoices:read` | `invoices:read` |
| `invoices:write` | `invoices:create`, `invoices:update`, `invoices:delete` |
| `invoices:manage` | `invoices:create`, `invoices:read`, `invoices:update`, `invoices:delete` |
| `news:read` | `news:read` |
| `news:write` | `news:create`, `news:update`, `news:delete` |
| `*` | All permissions (admin only) |

### Permission Check Example

```go
// Middleware checks: Does user have permission for this action?
// If API key auth, check scopes instead of full permissions

func (m *PermissionMiddleware) CheckPermission(resource, action string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            scopes := c.Get("scopes").([]string)
            
            // If no scopes (JWT auth), use full permission check
            if len(scopes) == 0 {
                return m.enforcer.Enforce(userID, resource, action)
            }
            
            // If API key auth, check scopes
            for _, scope := range scopes {
                if m.scopeAllows(scope, resource, action) {
                    return next(c)
                }
            }
            
            return echo.NewHTTPError(403, "Permission denied")
        }
    }
}
```

---

## Rate Limiting

API Keys have separate rate limits from user sessions.

**Rate Limit Headers**:
```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 55
X-RateLimit-Reset: 1619097600
Retry-After: 60
```

**Default Limits**:
- User session: 100 requests/minute
- API key: 60 requests/minute (configurable)

**Rate Limit Identifier**:
- User session: `user:{user_id}`
- API key: `api_key:{key_id}`

---

## Scope Management

### Valid Scopes

The following scopes are supported:

**Read-only scopes**:
- `users:read` - View user profiles
- `roles:read` - View roles
- `permissions:read` - View permissions
- `invoices:read` - View invoices
- `news:read` - View news articles
- `audit_logs:read` - View audit logs

**Write scopes**:
- `invoices:write` - Create, update, delete invoices
- `news:write` - Create, update, delete news
- `media:write` - Create, update, delete media

**Management scopes**:
- `users:manage` - Full user management
- `roles:manage` - Full role management
- `permissions:manage` - Full permission management

**Wildcard**:
- `*` - All permissions (admin only)

### Scope Validation Request

```bash
POST /api/v1/api-keys
{
  "name": "Invoice Reader",
  "scopes": ["invoices:read", "invoices:write"]
}
```

**Validation**:
1. Parse user's effective permissions from Casbin
2. Check each scope is valid format
3. Check each scope is subset of user's permissions
4. If any scope invalid or not allowed, return 403

---

## Error Codes

| Code | HTTP | Meaning | Details |
|------|------|---------|---------|
| `INVALID_SCOPE` | 400 | Invalid scope format | `{scope: "invalid:format"}` |
| `SCOPE_NOT_ALLOWED` | 403 | Scope exceeds user permissions | `{scope: "invoices:write", reason: "user lacks permission"}` |
| `KEY_NAME_EXISTS` | 409 | Key name already used | `{name: "Production Integration"}` |
| `KEY_EXPIRED` | 401 | API key has expired | `{expires_at: "..."}` |
| `KEY_REVOKED` | 401 | API key has been revoked | `{revoked_at: "..."}` |
| `KEY_NOT_FOUND` | 404 | API key not found | `{id: "..."}` |

---

## Examples

### Creating an API Key

```bash
# Login first to get JWT token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "Password123!"}'
  
# Use JWT to create API key
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Integration",
    "scopes": ["invoices:read", "invoices:write"]
  }'
  
# Response includes full key (only shown once!)
# Save: ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

### Using API Key

```bash
# Use API key to list invoices
curl -X GET http://localhost:8080/api/v1/invoices \
  -H "X-API-Key: ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
  
# Response same as JWT auth
```

### Revoking API Key

```bash
# Revoke using JWT
curl -X DELETE http://localhost:8080/api/v1/api-keys/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
  
# Subsequent requests with revoked key return 401
```

---

## Security Considerations

1. **Key Exposure**: Key value only shown once on creation
2. **Key Storage**: Key stored as bcrypt hash, not plaintext
3. **Key Transmission**: Use HTTPS for all API requests
4. **Key Rotation**: Create new key before revoking old one
5. **Scope Limitation**: Keys cannot exceed user's own permissions
6. **Rate Limiting**: API keys have lower rate limits than sessions
7. **Audit Trail**: All key operations logged with actor, action, timestamp

---

**Version History**:
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-04-23 | Sisyphus | Initial specification |
# Quickstart: API Key Authentication

**Last Updated**: 2026-04-23 | **Feature**: API Key Auth

Get started with API key authentication in 5 minutes.

---

## Prerequisites

- Go API Base running (see `specs/go-api-base/quickstart.md`)
- User account with JWT token
- Understanding of scopes (resource:action)

---

## Step 1: Create API Key

### 1.1 Login to Get JWT Token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "password": "SecurePass123!"
  }'
```

**Response**:
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    ...
  }
}
```

**Save tokens**:
```bash
export ACCESS_TOKEN="<access_token_from_response>"
export USER_ID="<user_id_from_response>"
```

### 1.2 Create API Key

```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Integration Key",
    "scopes": ["invoices:read", "invoices:write"]
  }'
```

**Response** (201):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "My Integration Key",
    "prefix": "ak_live_a1b2",
    "scopes": ["invoices:read", "invoices:write"],
    "created_at": "2026-04-23T10:30:00Z",
    "key": "ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
  }
}
```

⚠️ **Important**: Copy the `key` value immediately! It will not be shown again.

**Save API key**:
```bash
export API_KEY="ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
```

---

## Step 2: Use API Key

### 2.1 List Invoices (Read Scope)

```bash
curl -X GET http://localhost:8080/api/v1/invoices \
  -H "X-API-Key: $API_KEY"
```

**Response** (200):
```json
{
  "data": {
    "invoices": [
      {
        "id": "770e8400-e29b-41d4-a716-446655440000",
        "user_id": "550e8400-e29b-41d4-a716-446655440000",
        "amount": 15000,
        "status": "draft",
        "created_at": "2026-04-20T10:00:00Z"
      }
    ],
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

### 2.2 Create Invoice (Write Scope)

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 25000,
    "customer": "Acme Corp"
  }'
```

**Response** (201):
```json
{
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440000",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount": 25000,
    "customer": "Acme Corp",
    "status": "draft",
    "created_at": "2026-04-23T10:35:00Z"
  }
}
```

### 2.3 Permission Denied Example

Attempt to access news (outside scope):

```bash
curl -X GET http://localhost:8080/api/v1/news \
  -H "X-API-Key: $API_KEY"
```

**Response** (403):
```json
{
  "data": null,
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "Permission denied",
    "details": {
      "required_permission": "news:read",
      "scopes": ["invoices:read", "invoices:write"]
    }
  }
}
```

---

## Step 3: Manage API Keys

### 3.1 List All API Keys

```bash
curl -X GET http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "api_keys": [
      {
        "id": "660e8400-e29b-41d4-a716-446655440000",
        "name": "My Integration Key",
        "prefix": "ak_live_a1b2",
        "scopes": ["invoices:read", "invoices:write"],
        "expires_at": null,
        "last_used_at": "2026-04-23T10:35:00Z",
        "is_revoked": false,
        "created_at": "2026-04-23T10:30:00Z"
      }
    ],
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

**Note**: Full key values are NOT returned, only the prefix for identification.

### 3.2 Get Single API Key

```bash
curl -X GET http://localhost:8080/api/v1/api-keys/660e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200): Same as list item

### 3.3 Revoke API Key

```bash
curl -X DELETE http://localhost:8080/api/v1/api-keys/660e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440000",
    "message": "API key revoked successfully"
  }
}
```

After revocation, authentication attempts with this key return 401:

```bash
curl -X GET http://localhost:8080/api/v1/invoices \
  -H "X-API-Key: $API_KEY"
```

**Response** (401):
```json
{
  "data": null,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid API key",
    "details": {
      "reason": "revoked"
    }
  }
}
```

---

## Step 4: Create Key with Expiration

### 4.1 Expiring API Key

```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Temporary Integration",
    "scopes": ["invoices:read"],
    "expires_at": "2026-05-23T00:00:00Z"
  }'
```

This key will work until May 23, 2026, then return 401 on authentication:

```json
{
  "data": null,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "API key expired",
    "details": {
      "reason": "expired",
      "expires_at": "2026-05-23T00:00:00Z"
    }
  }
}
```

---

## Valid Scopes

### Read-Only Scopes

| Scope | Permissions Granted |
|-------|---------------------|
| `users:read` | View user profiles |
| `roles:read` | View roles |
| `permissions:read` | View permissions |
| `invoices:read` | View invoices |
| `news:read` | View news |
| `media:read` | View media |
| `audit_logs:read` | View audit logs |

### Write Scopes

| Scope | Permissions Granted |
|-------|---------------------|
| `invoices:write` | Create, update, delete invoices |
| `news:write` | Create, update, delete news |
| `media:write` | Create, update, delete media |

### Management Scopes

| Scope | Permissions Granted |
|-------|---------------------|
| `users:manage` | Full user CRUD |
| `roles:manage` | Full role CRUD |
| `permissions:manage` | Full permission CRUD |

### Wildcard

| Scope | Permissions Granted |
|-------|---------------------|
| `*` | All permissions (admin only) |

---

## Scope Validation

When creating an API key, scopes are validated against your permissions:

**Example**: User with only `invoices:read` permission:

```bash
# This will FAIL - user doesn't have invoices:write
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid Key",
    "scopes": ["invoices:write"]
  }'
```

**Response** (403):
```json
{
  "data": null,
  "error": {
    "code": "SCOPE_NOT_ALLOWED",
    "message": "Scope exceeds user permissions",
    "details": {
      "scope": "invoices:write",
      "reason": "user lacks invoices:write permission"
    }
  }
}
```

---

## Rate Limiting

API keys have separate rate limits from user sessions.

**Default limits**:
- User session: 100 requests/minute
- API key: 60 requests/minute

**Rate limit headers** (included in every response):
```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 55
X-RateLimit-Reset: 1619097600
```

When rate limited (429):
```json
{
  "data": null,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Please retry after 60 seconds.",
    "details": {
      "retry_after": 60,
      "limit": 60
    }
  }
}
```

---

## Testing with Postman

### Environment Variables

```json
{
  "baseUrl": "http://localhost:8080",
  "accessToken": "{{from_login}}",
  "apiKey": "{{from_create_key}}"
}
```

### Collection Structure

```
📁 API Keys
  ├── POST   /api-keys          Create API Key
  ├── GET    /api-keys          List API Keys
  ├── GET    /api-keys/:id      Get API Key
  └── DELETE /api-keys/:id      Revoke API Key

📁 Invoices (with API Key)
  ├── GET    /invoices          List Invoices
  ├── POST   /invoices          Create Invoice
  ├── GET    /invoices/:id      Get Invoice
  └── PUT    /invoices/:id      Update Invoice
```

### Authorization

**For API Key Management** (POST/GET/DELETE `/api-keys`):
- Type: Bearer Token
- Token: `{{accessToken}}`

**For API Key Usage** (all other endpoints):
- Type: API Key
- Key: `{{apiKey}}`

---

## Security Best Practices

1. **Store API keys securely**: Use environment variables, never in code
2. **Use scoped keys**: Limit scopes to minimum required permissions
3. **Rotate keys regularly**: Create new key, update integrations, revoke old key
4. **Set expiration**: Use `expires_at` for temporary integrations
5. **Monitor usage**: Check `last_used_at` for suspicious activity
6. **Revoke immediately**: If key is compromised, revoke immediately
7. **Use test keys**: Create keys with `ak_test_*` prefix for development

---

## Troubleshooting

### "Invalid API key" Error

Possible causes:
1. Key is revoked (`revoked_at` is set)
2. Key has expired (`expires_at < NOW()`)
3. Key not found in database
4. User is soft-deleted
5. Key format is invalid

**Solution**: Create a new API key

### "Permission denied" Error

Possible causes:
1. Key's scopes don't include the required permission
2. Trying to access resource outside of key's scopes

**Solution**: Create key with additional scopes, or use JWT auth

### "Scope not allowed" Error

Possible causes:
1. Trying to create key with scopes you don't have permission for

**Solution**: Check your effective permissions first:
```bash
curl -X GET http://localhost:8080/api/v1/users/$USER_ID/effective-permissions \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

---

## Directory Structure

```
internal/
├── domain/
│   └── api_key.go           # APIKey entity
├── repository/
│   └── api_key.go           # APIKeyRepository interface
├── service/
│   └── api_key.go           # APIKeyService business logic
├── http/
│   └── handler/
│       └── api_key.go       # API key endpoints
│   └── middleware/
│       └── api_key.go       # API key authentication middleware
migrations/
└── 000003_api_keys.up.sql   # Database migration
```

---

## Next Steps

1. **Read spec.md**: Full feature specification
2. **Read data-model.md**: Database schema details
3. **Read contracts/api-v1.md**: API endpoint documentation
4. **Run integration tests**: `make test-integration`
5. **Create API key in production**: Use test key first

---

**Questions?** See `specs/api-key-auth/spec.md` for full specification.
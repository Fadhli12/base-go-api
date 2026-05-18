# API Contract: OAuth 2.0 Social Login v1

**Date**: 2026-05-18 | **Branch**: `021-oauth-social-login` | **Spec**: [spec.md](../spec.md)

## Base URL

```
/api/v1
```

## Authentication

- **Public endpoints**: No JWT required
- **Authenticated endpoints**: JWT Bearer token in `Authorization` header
- **Admin endpoints**: JWT + `oauth:manage` permission (via Casbin Enforce)

## Common Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Authenticated | `Bearer {access_token}` |
| `X-Organization-ID` | Optional | Organization context for org-scoped resources |
| `X-Request-ID` | Optional | Request correlation ID (auto-generated if missing) |
| `Content-Type` | POST/PUT | `application/json` |

---

## Public Endpoints (No Authentication)

### Initiate OAuth Login

```
GET /auth/oauth/:provider
```

Initiates an OAuth login flow by generating a PKCE challenge, storing state in Redis, and redirecting to the provider's authorization URL.

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| provider | string | Required. One of: `google`, `github`, `microsoft` |

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| callback_url | string | Optional. Frontend callback URL. Must match allowlist. Defaults to `OAUTH_FRONTEND_CALLBACK_URL`. |

**Response**: `302 Redirect` to provider authorization URL

**Error Responses**:
| Status | Code | Message |
|--------|------|---------|
| 400 | INVALID_PROVIDER | Provider not supported or not configured |
| 400 | PROVIDER_DISABLED | Provider is currently disabled |
| 400 | INVALID_CALLBACK_URL | Callback URL not in allowlist |

---

### OAuth Callback (Shared: Login + Link)

```
GET /auth/oauth/:provider/callback
```

Handles OAuth provider callback. Validates state parameter, exchanges authorization code for tokens (with PKCE), normalizes provider profile, and redirects to frontend with result in URL fragment.

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| provider | string | Required. One of: `google`, `github`, `microsoft` |

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| code | string | Authorization code from provider |
| state | string | State parameter from Redis |

**Response**: `302 Redirect` to `callback_url#${fragment}`

**Success Fragment (Login)**:
```
#access_token={jwt}&refresh_token={hex}&token_type=Bearer&expires_in=900
```

**Success Fragment (Link)**:
```
#status=linked&provider=google&message=Provider+linked+successfully
```

**Error Fragment**:
```
#error={error_code}&message={human_readable_message}
```

**Error Codes**:
| Code | Description |
|------|-------------|
| invalid_state | State parameter expired, invalid, or not found |
| access_denied | User denied authorization on provider |
| provider_error | Provider returned an error |
| email_already_exists | Email matches existing user but not linked |
| email_not_verified | Provider returned unverified email |
| provider_already_linked | Provider already linked to this user |
| account_already_linked | Provider account linked to different user |
| provider_disabled | Provider was disabled after flow started |
| internal_error | Unexpected server error |

---

### List Enabled Providers

```
GET /auth/oauth/providers
```

Lists all enabled OAuth providers (global + org-scoped). Returns only public information (no secrets).

**Response**: `200 OK`
```json
{
  "data": [
    {
      "name": "google",
      "display_name": "Google",
      "scopes": ["openid", "email", "profile"]
    },
    {
      "name": "github",
      "display_name": "GitHub",
      "scopes": ["user:email", "read:user"]
    }
  ]
}
```

---

## Authenticated Endpoints (JWT Required)

### Initiate Provider Link

```
POST /auth/oauth/:provider/link
```

Initiates an OAuth link flow for the authenticated user. Returns a redirect URL (NOT a redirect). The frontend should redirect the user to this URL.

**Permission**: `oauth:link`

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| provider | string | Required. One of: `google`, `github`, `microsoft` |

**Request Body**: None

**Response**: `200 OK`
```json
{
  "data": {
    "redirect_url": "https://accounts.google.com/o/oauth2/v2/auth?..."
  }
}
```

**Error Responses**:
| Status | Code | Message |
|--------|------|---------|
| 400 | INVALID_PROVIDER | Provider not supported |
| 400 | PROVIDER_DISABLED | Provider is currently disabled |
| 401 | UNAUTHORIZED | Missing or invalid JWT |
| 403 | FORBIDDEN | Missing `oauth:link` permission |
| 409 | PROVIDER_ALREADY_LINKED | User already has this provider linked |

---

### Unlink Provider

```
DELETE /auth/oauth/:provider/unlink
```

Unlinks an OAuth provider from the authenticated user's account. Fails if this would leave the user with no authentication method.

**Permission**: `oauth:link`

**Path Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| provider | string | Required. One of: `google`, `github`, `microsoft` |

**Request Body**: None

**Response**: `200 OK`
```json
{
  "data": {
    "message": "Provider unlinked successfully"
  }
}
```

**Error Responses**:
| Status | Code | Message |
|--------|------|---------|
| 400 | INVALID_PROVIDER | Provider not supported |
| 401 | UNAUTHORIZED | Missing or invalid JWT |
| 403 | FORBIDDEN | Missing `oauth:link` permission |
| 404 | PROVIDER_NOT_LINKED | User does not have this provider linked |
| 409 | LAST_AUTH_METHOD | Cannot unlink. User has no password and no other linked providers. |

---

### List Linked Accounts

```
GET /auth/oauth/accounts
```

Lists all OAuth accounts linked to the authenticated user.

**Permission**: `oauth:view`

**Response**: `200 OK`
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "provider": "google",
      "email": "user@gmail.com",
      "display_name": "John Doe",
      "avatar_url": "https://lh3.googleusercontent.com/...",
      "linked_at": "2026-05-18T10:00:00Z"
    }
  ]
}
```

---

## Admin Endpoints (`oauth:manage` Permission Required)

### Create Provider

```
POST /oauth-providers
```

Creates a new OAuth provider configuration. Client secret is encrypted before storage.

**Request Body**:
```json
{
  "name": "google",
  "display_name": "Google",
  "client_id": "1234567890.apps.googleusercontent.com",
  "client_secret": "GOCSPX-xxxxxxxxxxxxxxxx",
  "redirect_url": "https://app.example.com/auth/callback",
  "additional_scopes": ["https://www.googleapis.com/auth/calendar"],
  "config": {},
  "organization_id": null
}
```

**Response**: `201 Created`
```json
{
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "google",
    "display_name": "Google",
    "client_id": "1234567890.apps.googleusercontent.com",
    "redirect_url": "https://app.example.com/auth/callback",
    "scopes": ["openid", "email", "profile", "https://www.googleapis.com/auth/calendar"],
    "additional_scopes": ["https://www.googleapis.com/auth/calendar"],
    "config": {},
    "is_enabled": true,
    "is_system": false,
    "organization_id": null,
    "created_at": "2026-05-18T10:00:00Z",
    "updated_at": "2026-05-18T10:00:00Z"
  }
}
```

**Note**: `scopes` in response = default scopes for provider + `additional_scopes`. `client_secret` is never returned in responses.

---

### List Providers

```
GET /oauth-providers
```

Lists all OAuth providers (including disabled). Supports pagination and org filtering.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| page | int | Page number (default: 1) |
| per_page | int | Items per page (default: 20, max: 100) |
| organization_id | UUID | Filter by organization (omit for global) |

**Permission**: `oauth:manage`

**Response**: `200 OK`
```json
{
  "data": [...],
  "meta": {
    "total": 3,
    "page": 1,
    "per_page": 20
  }
}
```

---

### Get Provider

```
GET /oauth-providers/:id
```

**Response**: `200 OK` — same shape as Create Provider response

---

### Update Provider

```
PUT /oauth-providers/:id
```

All fields optional (partial update). `client_secret` updates trigger re-encryption.

**Request Body** (example — update display name and add scope):
```json
{
  "display_name": "Google Workspace",
  "additional_scopes": ["https://www.googleapis.com/auth/calendar", "https://www.googleapis.com/auth/drive.readonly"]
}
```

**Response**: `200 OK` — same shape as Create Provider response

---

### Delete Provider

```
DELETE /oauth-providers/:id
```

Soft-deletes the provider. Linked OAuth accounts continue to work for authentication but new login/link flows are blocked.

**Response**: `200 OK`
```json
{
  "data": {
    "message": "Provider deleted"
  }
}
```

**Error Responses**:
| Status | Code | Message |
|--------|------|---------|
| 403 | FORBIDDEN | Missing `oauth:manage` permission |
| 404 | NOT_FOUND | Provider not found |
| 403 | SYSTEM_PROVIDER | Cannot delete system provider (is_system=true) |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OAUTH_ENCRYPTION_KEY` | (derived from JWT_SECRET) | Override key for client secret encryption |
| `OAUTH_ALLOW_HTTP` | `false` | Allow HTTP URLs for redirect_url (dev only) |
| `OAUTH_FRONTEND_CALLBACK_URL` | `http://localhost:3000/auth/callback` | Default frontend callback URL |
| `OAUTH_STATE_TTL` | `600` | State parameter TTL in seconds |

## Provider Default Scopes

| Provider | Default Scopes |
|----------|---------------|
| google | `openid`, `email`, `profile` |
| github | `user:email`, `read:user` |
| microsoft | `openid`, `email`, `profile` |

## Microsoft Tenant ID Configuration

Microsoft providers require `tenant_id` in the `config` JSONB column:

```json
{
  "tenant_id": "common"
}
```

- `common` — Multi-tenant (default)
- `organizations` — Azure AD orgs only
- `consumers` — Personal accounts only
- Specific GUID — Single Azure AD tenant
# HTTP API Contracts: Go API Base v1

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Phase 1 Design

---

## Overview

All endpoints follow RESTful conventions. Responses use standardized envelopes. Authentication via JWT Bearer token in Authorization header.

**Base URL**: `/api/v1`

**Response Envelope**:
```json
{
  "data": { /* payload */ },
  "error": null,
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T10:30:00Z"
  }
}
```

**Error Envelope**:
```json
{
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": { /* optional context */ }
  },
  "meta": {
    "request_id": "uuid",
    "timestamp": "2026-04-22T10:30:00Z"
  }
}
```

---

## Error Codes

| Code | HTTP | Meaning | Details |
|------|------|---------|---------|
| `INVALID_REQUEST` | 400 | Malformed request (validation error) | `{field: "error"}` |
| `UNAUTHORIZED` | 401 | Missing or invalid JWT token | `{reason: "token_expired" \| "invalid" \| "missing"}` |
| `PERMISSION_DENIED` | 403 | User lacks required permission | `{required_permission: "resource:action"}` |
| `NOT_FOUND` | 404 | Resource not found | `{resource: "invoice", id: "..."}` |
| `CONFLICT` | 409 | Resource already exists | `{field: "email", value: "..."}` |
| `INTERNAL_ERROR` | 500 | Server error | `{error_id: "uuid"}` (for support) |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests | `{retry_after: 60}` |

### Rate Limit Error Response (429)

```json
{
  "data": null,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Please retry after 60 seconds.",
    "details": {
      "retry_after": 60,
      "limit": 100,
      "window": "60s"
    }
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

Rate limit headers are always included:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1619097600
Retry-After: 60
```

---

## Authentication Endpoints

### POST /auth/register

**Purpose**: Create new user account.

**Request**:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Validation**:
- email: valid format, globally unique
- password: min 8 chars, must include uppercase, lowercase, digit, symbol

**Response** (201 Created):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "created_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 400: Invalid email format, password weak, email already exists

---

### POST /auth/login

**Purpose**: Authenticate user, issue access + refresh tokens.

**Request**:
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "access_token_expires_in": 900,
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token_expires_in": 2592000
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Token Details**:
- `access_token`: 15 minutes (900 seconds), stateless JWT signed with HS256
- `access_token_expires_in`: 900 (seconds, fixed)
- `refresh_token`: 30 days (2592000 seconds), SHA256 hash stored in DB
- `refresh_token_expires_in`: 2592000 (seconds, fixed)

**Errors**:
- 401: Invalid email or password
- 404: User not found

---

### POST /auth/refresh

**Purpose**: Issue new access token using valid refresh token.

**Request**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response** (200 OK):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "access_token_expires_in": 900,
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token_expires_in": 2592000
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
- Validates refresh_token hash against DB (SHA256 lookup)
- Checks `revoked_at IS NULL` and `expires_at > NOW()`
- **Rotates refresh token**: Issues NEW access_token + NEW refresh_token
- Marks old refresh_token as revoked (sets `revoked_at = NOW()` for audit trail)
- Creates audit log entry with action=token_refresh, resource=auth

**Errors**:
- 401: Invalid, expired, or revoked token

---

### POST /auth/logout

**Purpose**: Revoke all user refresh tokens.

**Request**:
```
Authorization: Bearer <access_token>
```

**Request Body** (optional):
```json
{}
```

**Response** (200 OK):
```json
{
  "data": {
    "message": "Successfully logged out"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
- Extract user_id from access_token claims
- Mark ALL user's refresh_tokens as revoked (revoked_at = NOW())
- Client discards access_token

**Errors**:
- 401: Invalid or missing token

---

## Permission Management Endpoints

### POST /permissions

**Purpose**: Create custom permission (admin only).

**Authentication**: Required, permission: `permission:create`

**Request**:
```json
{
  "resource": "invoice",
  "action": "approve",
  "scope": "team",
  "description": "Approve team member's invoices"
}
```

**Response** (201 Created):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "invoice:approve",
    "resource": "invoice",
    "action": "approve",
    "scope": "team",
    "description": "Approve team member's invoices",
    "is_system": false,
    "created_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
- Generate `name` as `{resource}:{action}`
- Store in DB
- Trigger permission cache invalidation (Redis pub/sub)

**Errors**:
- 400: Missing fields, invalid resource/action format
- 403: User lacks permission:create
- 409: Permission name already exists

---

### GET /permissions

**Purpose**: List all permissions (admin only).

**Authentication**: Required, permission: `permission:read`

**Query Params**:
- `limit`: Results per page (default 20, max 100)
- `offset`: Pagination offset (default 0)
- `resource`: Filter by resource (optional)

**Response** (200 OK):
```json
{
  "data": {
    "permissions": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "invoice:create",
        "resource": "invoice",
        "action": "create",
        "scope": "all",
        "description": "Create new invoice",
        "is_system": true,
        "created_at": "2026-04-22T10:30:00Z"
      }
    ],
    "total": 42,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### POST /roles

**Purpose**: Create new role (admin only).

**Authentication**: Required, permission: `role:create`

**Request**:
```json
{
  "name": "Manager",
  "description": "Team manager with approval authority"
}
```

**Response** (201 Created):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "name": "Manager",
    "description": "Team manager with approval authority",
    "is_system": false,
    "created_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### PUT /roles/:id

**Purpose**: Update role (admin only).

**Authentication**: Required, permission: `role:update`

**Request**:
```json
{
  "description": "Updated description"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "name": "Manager",
    "description": "Updated description",
    "is_system": false,
    "updated_at": "2026-04-22T10:35:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: Role not found
- 403: Cannot update system role

---

### POST /roles/:id/permissions

**Purpose**: Attach permission to role (admin only).

**Authentication**: Required, permission: `role:update`

**Request**:
```json
{
  "permission_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "role_id": "550e8400-e29b-41d4-a716-446655440002",
    "permission_id": "550e8400-e29b-41d4-a716-446655440000"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
- Insert into role_permissions table
- Trigger cache invalidation

**Errors**:
- 404: Role or permission not found
- 409: Permission already attached to role

---

### DELETE /roles/:id/permissions/:pid

**Purpose**: Remove permission from role (admin only).

**Authentication**: Required, permission: `role:update`

**Response** (204 No Content):
```
(empty body)
```

**Behavior**:
- Delete from role_permissions table
- Trigger cache invalidation

---

## User Permission Endpoints

### POST /users/:id/roles

**Purpose**: Assign role to user (admin only).

**Authentication**: Required, permission: `user:update`

**Request**:
```json
{
  "role_id": "550e8400-e29b-41d4-a716-446655440002"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "role_id": "550e8400-e29b-41d4-a716-446655440002"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: User or role not found
- 409: User already has role

---

### POST /users/:id/permissions

**Purpose**: Grant or deny permission directly to user, overriding roles (admin only).

**Authentication**: Required, permission: `user:update`

**Request**:
```json
{
  "permission_id": "550e8400-e29b-41d4-a716-446655440000",
  "effect": "allow"
}
```

**Effect Values**:
- `allow` — User has this permission (override deny from role)
- `deny` — User lacks this permission (override allow from role)

**Response** (200 OK):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "permission_id": "550e8400-e29b-41d4-a716-446655440000",
    "effect": "allow"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### GET /users/:id/effective-permissions

**Purpose**: Compute union of user's permissions (from roles + direct overrides).

**Authentication**: Required

**Response** (200 OK):
```json
{
  "data": {
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "permissions": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "invoice:create",
        "resource": "invoice",
        "action": "create",
        "scope": "all",
        "effect": "allow"
      },
      {
        "id": "550e8400-e29b-41d4-a716-446655440001",
        "name": "invoice:delete",
        "resource": "invoice",
        "action": "delete",
        "scope": "own",
        "effect": "deny"
      }
    ]
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
1. Fetch user's roles
2. Fetch permissions for each role
3. Fetch user's direct permission overrides
4. Merge: overrides take precedence
5. Return effective permission set

---

### GET /users/:id

**Purpose**: Retrieve user profile by ID (admin only).

**Authentication**: Required, permission: `user:read`

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "created_at": "2026-04-22T10:30:00Z",
    "updated_at": "2026-04-22T10:30:00Z",
    "deleted_at": null,
    "roles": [
      {"id": "550e8400-e29b-41d4-a716-446655440002", "name": "Manager"}
    ]
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: User not found

---

### DELETE /users/:id

**Purpose**: Soft-delete user (admin only). User cannot login after deletion.

**Authentication**: Required, permission: `user:delete`

**Response** (204 No Content):
```
(empty body)
```

**Behavior**:
- Sets `deleted_at` on user record
- User's refresh tokens revoked
- User cannot login (email marked as deleted)

**Errors**:
- 404: User not found
- 403: Cannot delete user with existing invoices (ON DELETE RESTRICT on invoices.user_id)

---

### GET /users

**Purpose**: List users with pagination (admin only).

**Authentication**: Required, permission: `user:read`

**Query Params**:
- `limit`: Results per page (default 20, max 100)
- `offset`: Pagination offset
- `email`: Filter by email (partial match)

**Response** (200 OK):
```json
{
  "data": {
    "users": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "email": "user@example.com",
        "created_at": "2026-04-22T10:30:00Z"
      }
    ],
    "total": 42,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### GET /me

**Purpose**: Get current authenticated user's profile.

**Authentication**: Required

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "created_at": "2026-04-22T10:30:00Z",
    "roles": [
      {"id": "550e8400-e29b-41d4-a716-446655440002", "name": "Manager"}
    ]
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### DELETE /users/:id/roles/:roleId

**Purpose**: Remove role from user (admin only).

**Authentication**: Required, permission: `user:update`

**Response** (204 No Content):
```
(empty body)
```

**Errors**:
- 404: User or role assignment not found

---

### DELETE /users/:id/permissions/:permissionId

**Purpose**: Remove direct permission override from user (admin only).

**Authentication**: Required, permission: `user:update`

**Response** (204 No Content):
```
(empty body)
```

**Errors**:
- 404: User or permission override not found

---

### GET /roles

**Purpose**: List all roles (admin only).

**Authentication**: Required, permission: `role:read`

**Query Params**:
- `limit`: Results per page (default 20, max 100)
- `offset`: Pagination offset

**Response** (200 OK):
```json
{
  "data": {
    "roles": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440002",
        "name": "Manager",
        "description": "Team manager",
        "is_system": false,
        "created_at": "2026-04-22T10:30:00Z",
        "permission_count": 12
      }
    ],
    "total": 5,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

### DELETE /roles/:id

**Purpose**: Soft-delete non-system role (admin only).

**Authentication**: Required, permission: `role:delete`

**Response** (204 No Content):
```
(empty body)
```

**Behavior**:
- Sets `deleted_at` on role record
- Role is removed from all users (user_roles cascade)
- Role permissions removed (role_permissions cascade)

**Errors**:
- 404: Role not found
- 403: Cannot delete system role (is_system = true)

---

## Session Management

### GET /me/sessions

**Purpose**: List current user's active sessions (devices).

**Authentication**: Required

**Response** (200 OK):
```json
{
  "data": {
    "sessions": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440010",
        "device_name": "Chrome on Windows",
        "ip_address": "192.168.1.100",
        "created_at": "2026-04-20T10:30:00Z",
        "expires_at": "2026-05-20T10:30:00Z",
        "is_current": false
      },
      {
        "id": "550e8400-e29b-41d4-a716-446655440011",
        "device_name": "Safari on iPhone",
        "ip_address": "192.168.1.50",
        "created_at": "2026-04-22T08:00:00Z",
        "expires_at": "2026-05-22T08:00:00Z",
        "is_current": true
      }
    ],
    "total": 2
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Note**: `is_current` indicates which session is making the request.

---

### DELETE /me/sessions/:sessionId

**Purpose**: Revoke specific session (logout from device).

**Authentication**: Required

**Response** (204 No Content):
```
(empty body)
```

**Behavior**:
- Marks refresh_token as revoked
- Device cannot refresh access token after this
- Access token still valid until it expires (max 15 min)

**Errors**:
- 404: Session not found or not owned by user
- 403: Cannot revoke current session (use DELETE /me/sessions/current instead)

---

### DELETE /me/sessions/current

**Purpose**: Logout current session (logout from this device).

**Authentication**: Required

**Response** (204 No Content):
```
(empty body)
```

**Behavior**:
- Marks current refresh_token as revoked
- Returns new refresh_token in response (optional "logout and relogin" flow)

---

## Audit Logs

### GET /audit-logs

**Purpose**: Query audit logs for compliance and security review (admin only).

**Authentication**: Required, permission: `audit_log:read`

**Query Params**:
- `limit`: Results per page (default 50, max 100)
- `offset`: Pagination offset
- `actor_id`: Filter by user who performed action
- `resource`: Filter by resource type (invoice, user, role, permission, etc.)
- `action`: Filter by action type (create, update, delete, login, logout, etc.)
- `from`: Filter from date (ISO 8601)
- `to`: Filter to date (ISO 8601)

**Response** (200 OK):
```json
{
  "data": {
    "audit_logs": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440100",
        "actor_id": "550e8400-e29b-41d4-a716-446655440000",
        "actor_email": "alice@example.com",
        "action": "update",
        "resource": "invoice",
        "resource_id": "550e8400-e29b-41d4-a716-446655440003",
        "before": {"amount": 10000, "status": "draft"},
        "after": {"amount": 15000, "status": "sent"},
        "ip_address": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "created_at": "2026-04-22T10:30:00Z"
      }
    ],
    "total": 1500,
    "limit": 50,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Audit**: Reading audit logs is itself logged (audit_log:read action).

---

## Example Domain: Invoices

### GET /invoices

**Purpose**: List user's invoices with permission check.

**Authentication**: Required, permission: `invoice:read`

**Query Params**:
- `limit`: Results per page (default 20)
- `offset`: Pagination offset
- `status`: Filter by status (draft, sent, paid, cancelled)

**Response** (200 OK):
```json
{
  "data": {
    "invoices": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440003",
        "user_id": "550e8400-e29b-41d4-a716-446655440000",
        "amount": 15000,
        "status": "draft",
        "created_at": "2026-04-22T10:30:00Z",
        "updated_at": "2026-04-22T10:30:00Z"
      }
    ],
    "total": 5,
    "limit": 20,
    "offset": 0
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Behavior**:
1. Coarse check: user has `invoice:read` permission (via Casbin)
2. Fine check: filter to user's invoices or allow `invoice:read:all` for admins
3. Return paginated list

---

### POST /invoices

**Purpose**: Create new invoice.

**Authentication**: Required, permission: `invoice:create`

**Request**:
```json
{
  "amount": 15000
}
```

**Response** (201 Created):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440003",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount": 15000,
    "status": "draft",
    "created_at": "2026-04-22T10:30:00Z",
    "updated_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Audit**: Creates audit_log entry with action=create, resource=invoice, after=new invoice data.

---

### GET /invoices/:id

**Purpose**: Retrieve specific invoice.

**Authentication**: Required, permission: `invoice:read`

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440003",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount": 15000,
    "status": "draft",
    "created_at": "2026-04-22T10:30:00Z",
    "updated_at": "2026-04-22T10:30:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Authorization**:
1. Coarse: user has `invoice:read`?
2. Fine: user owns invoice OR has `invoice:read:all`?

---

### PUT /invoices/:id

**Purpose**: Update invoice.

**Authentication**: Required, permission: `invoice:update`

**Request**:
```json
{
  "amount": 20000,
  "status": "sent"
}
```

**Response** (200 OK):
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440003",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount": 20000,
    "status": "sent",
    "updated_at": "2026-04-22T10:35:00Z"
  },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Audit**: Creates audit_log entry with action=update, before=old invoice, after=new invoice.

---

### DELETE /invoices/:id

**Purpose**: Soft-delete invoice.

**Authentication**: Required, permission: `invoice:delete`

**Response** (204 No Content):
```
(empty body)
```

**Audit**: Creates audit_log entry with action=delete, before=invoice data, after=null.

---

## Health Checks

### GET /healthz

**Purpose**: Liveness probe (K8s, load balancer).

**Authentication**: None required

**Response** (200 OK):
```json
{
  "status": "healthy"
}
```

**Behavior**: Returns 200 if API process running. Does NOT check DB or Redis.

---

### GET /readyz

**Purpose**: Readiness probe (K8s, load balancer).

**Authentication**: None required

**Response** (200 OK):
```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

**Behavior**: Returns 200 only if DB + Redis accessible. Returns 503 Service Unavailable if unhealthy.

---

## Request Headers

| Header | Purpose | Example |
|--------|---------|---------|
| `Authorization` | Bearer JWT token | `Bearer eyJhbGciOiJIUzI1NiIs...` |
| `X-Request-ID` | Optional correlation ID | `550e8400-e29b-41d4-a716-446655440000` |
| `User-Agent` | Client identifier | `Mozilla/5.0...` |

---

## Response Headers

| Header | Purpose | Example |
|--------|---------|---------|
| `X-Request-ID` | Correlation ID (generated if not provided) | `550e8400-e29b-41d4-a716-446655440000` |
| `Content-Type` | Always JSON | `application/json; charset=utf-8` |

---

## Pagination

All list endpoints support:
- `limit`: Max results per page (default 20, max 100)
- `offset`: Pagination offset (default 0)

Response includes:
```json
{
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

---

## Rate Limiting

All endpoints except `/healthz`, `/readyz` subject to rate limiting:
- **Per-user**: 100 requests / minute (identified by user_id)
- **Per-IP**: 1000 requests / minute (for unauthenticated endpoints)

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1619097600
```

Exceeding limit returns 429 Too Many Requests.

---

**Version**: 1.0 | **Created**: 2026-04-22 | **Status**: Phase 1 Design

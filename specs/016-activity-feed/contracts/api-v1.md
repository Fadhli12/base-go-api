# HTTP API Contracts: Activity Feed v1

**Version**: 1.0 | **Date**: 2026-05-12 | **Status**: Phase 1 Design

---

## Overview

All activity feed endpoints follow existing API conventions: JWT authentication, standardized envelope responses, audit middleware on mutations.

**Base URL**: `/api/v1/activities`

**Authentication**: JWT Bearer token required for all endpoints.

**Permissions**: `activity:view` for read operations, `activity:manage` for soft-delete.

**Organization Context**: All activity endpoints respect the `X-Organization-ID` header. The existing organization middleware extracts this header and injects the organization ID into the request context. If the header is absent, activities are scoped globally (`organization_id = NULL`). Global activities are visible to all users; org-scoped activities are visible only to members of that organization.

---

## Endpoints

### GET /activities

**Purpose**: List activities for the authenticated user's organization (or globally if no org context). Supports filtering, pagination, and read/unread status.

**Authentication**: Required. Permission: `activity:view`

**Query Params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |
| `resource_type` | string | - | Filter by resource type (invoice, news, user, comment) |
| `resource_id` | string | - | Filter by resource ID |
| `action_type` | string | - | Filter by action type (created, updated, deleted, published, paid) |
| `unread` | bool | - | Filter to unread activities only (true/false) |
| `following` | bool | - | Filter to followed resources only (true/false) |
| `since` | string | - | ISO 8601 timestamp — include archived activities (those with `archived_at` set) from this time forward. Without this param, only non-archived activities are returned. |

**Response** (200 OK):
```json
{
  "data": {
    "activities": [
      {
        "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
        "actor_id": "user-uuid-...",
        "actor_name": "John Doe",
        "action_type": "created",
        "resource_type": "invoice",
        "resource_id": "inv-uuid-...",
        "organization_id": "org-uuid-...",
        "metadata": {
          "description": "John Doe created invoice INV-001",
          "invoice_number": "INV-001",
          "amount": 15000
        },
        "is_read": false,
        "is_following": true,
        "created_at": "2026-05-12T10:00:00Z"
      }
    ],
    "total": 42,
    "unread_count": 15,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Notes**:
- `is_read` and `is_following` are computed per-user via batch queries (NOT LEFT JOIN), then merged in Go.
- `unread_count` is the total unread for the user's scope (not just this page).
- Activities are sorted by `created_at DESC` (newest first).
- Activities with `archived_at IS NOT NULL` (auto-archived by the 90-day reaper) are excluded from default queries. Use `?since=<timestamp>` to include archived activities.
- Activities with `deleted_at IS NOT NULL` (explicitly deleted by an admin) are always excluded (GORM soft-delete filter).
- The `?since` parameter uses `Unscoped()` to bypass the `archived_at` filter but still respects `deleted_at`.
- When `?since` is provided, the query returns: `WHERE created_at >= ? AND archived_at IS NOT NULL AND deleted_at IS NULL` (plus the normal org-scope and user filters).
- When `?since` is NOT provided, the query returns: `WHERE archived_at IS NULL AND deleted_at IS NULL` (default, non-archived only).
- `actor_name` is resolved from the User relation at query time.

**Errors**:
- 403: User lacks `activity:view`

---

### GET /activities/count-unread

**Purpose**: Get the count of unread activities for the authenticated user in their current scope.

**Authentication**: Required. Permission: `activity:view`

**Response** (200 OK):
```json
{
  "data": {
    "unread_count": 15
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 403: User lacks `activity:view`

---

### PUT /activities/read-all

**Purpose**: Mark all visible activities as read for the authenticated user in their current scope (global or org-scoped).

**Authentication**: Required. Permission: `activity:view`

**Response** (200 OK):
```json
{
  "data": {
    "marked_count": 15
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Notes**:
- Creates `ActivityRead` records for all activities visible to the user that don't already have one.
- Uses batch INSERT with ON CONFLICT DO NOTHING for idempotency.
- Audit logged (action: `activity_read_all`, resource: `activity`).

**Errors**:
- 403: User lacks `activity:view`

---

### PUT /activities/:id/read

**Purpose**: Mark a single activity as read for the authenticated user.

**Authentication**: Required. Permission: `activity:view`

**Response** (200 OK):
```json
{
  "data": {
    "activity_id": "a1b2c3d4-...",
    "read_at": "2026-05-12T10:30:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Notes**:
- Idempotent — if already read, returns the existing read_at timestamp.
- Audit logged (action: `activity_read`, resource: `activity`, resource_id: activity_id).

**Errors**:
- 404: Activity not found
- 403: User lacks `activity:view`

---

### POST /activities/follow

**Purpose**: Follow a resource to see its activities in the "following" feed.

**Authentication**: Required. Permission: `activity:view`

**Request**:
```json
{
  "resource_type": "invoice",
  "resource_id": "inv-uuid-..."
}
```

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `resource_type` | string | yes | Must be a valid resource type (invoice, news, comment, user) |
| `resource_id` | string | yes | Must be a valid UUID or identifier |

**Response** (201 Created):
```json
{
  "data": {
    "id": "follow-uuid-...",
    "resource_type": "invoice",
    "resource_id": "inv-uuid-...",
    "created_at": "2026-05-12T10:30:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Notes**:
- Idempotent — if already following, returns existing follow.
- Audit logged (action: `activity_follow`, resource: resource_type, resource_id: resource_id).

**Errors**:
- 400: Invalid resource_type or resource_id
- 409: Already following this resource (returns existing follow)
- 403: User lacks `activity:view`

---

### DELETE /activities/follow/:id

**Purpose**: Unfollow a resource.

**Authentication**: Required. Permission: `activity:view`

**Response** (204 No Content)

**Notes**:
- Audit logged (action: `activity_unfollow`, resource: activity_follow, resource_id: follow_id).

**Errors**:
- 404: Follow not found
- 403: User lacks `activity:view`

---

### GET /activities/follows

**Purpose**: List the authenticated user's followed resources.

**Authentication**: Required. Permission: `activity:view`

**Query Params**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |

**Response** (200 OK):
```json
{
  "data": {
    "follows": [
      {
        "id": "follow-uuid-...",
        "resource_type": "invoice",
        "resource_id": "inv-uuid-...",
        "created_at": "2026-05-12T10:30:00Z"
      }
    ],
    "total": 3,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 403: User lacks `activity:view`

---

### DELETE /activities/:id

**Purpose**: Soft-delete an activity (admin only). Sets `deleted_at` timestamp. Activity is no longer visible in default queries but accessible via `?since` parameter.

**Authentication**: Required. Permission: `activity:manage`

**Response** (204 No Content)

**Notes**:
- Audit logged (action: `activity_delete`, resource: `activity`, resource_id: activity_id).
- Only users with `activity:manage` permission can delete activities.
- Soft delete — activity record is retained in database.

**Errors**:
- 404: Activity not found
- 403: User lacks `activity:manage`

---

## Common Error Responses

### 401 Unauthorized
```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid authentication"
  }
}
```

### 403 Forbidden
```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "User lacks required permission"
  }
}
```

### 404 Not Found
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found"
  }
}
```

### 500 Internal Server Error
```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred"
  }
}
```

---

## Read Status Computation

**Critical**: `is_read` is NOT computed via LEFT JOIN in the main query. Instead:

1. Query activities with pagination and filters (no JOIN on `activity_reads`)
2. Extract activity IDs from the result set
3. Batch-fetch read status: `SELECT activity_id, read_at FROM activity_reads WHERE user_id = ? AND activity_id IN (?)`
4. Merge `is_read` boolean in Go code by checking if each activity_id exists in the read set

This avoids catastrophic performance degradation at scale from LEFT JOIN.

## Follow Status Computation

**Same pattern as `is_read`**: The `is_following` boolean in ActivityListResponse is computed separately from the main query:

1. Query activities with pagination and filters (no JOIN on `activity_follows`)
2. Extract activity IDs from the result set
3. Batch-fetch follow status: `SELECT resource_type, resource_id FROM activity_follows WHERE user_id = ? AND (resource_type, resource_id) IN ((?,?),(?,?),...) GROUP BY resource_type, resource_id`
4. Merge `is_following` boolean in Go code by checking if each activity's (resource_type, resource_id) pair exists in the follow set

This avoids JOIN degradation for users with many follows.

---

## Pagination Convention

All list endpoints follow the standard pagination convention:
- `limit`: Number of results (default: 20, max: 100)
- `offset`: Offset from the start (default: 0)
- Response includes `total` count of all matching results

---

**Version**: 1.0 | **Created**: 2026-05-12 | **Status**: Phase 1 Design
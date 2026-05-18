# Idempotency Keys — Quickstart Guide

**Branch**: `019-idempotency-keys` | **Date**: 2026-05-18

---

## Overview

The Idempotency Key system prevents duplicate processing of API requests by caching responses in Redis and replaying them for subsequent identical requests. Clients include an `Idempotency-Key` header on mutating requests (POST, PUT, PATCH). The middleware intercepts the request, checks Redis for a prior completion, and either replays the cached response or processes the request while holding a distributed lock.

Key design points: Redis-first with async goroutine DB writes, fail-open on Redis errors, 4KB cache threshold with DB fallback for larger responses, background reaper for soft-deleting expired records.

---

## Prerequisites

- PostgreSQL database with migrations applied (migration `000027_create_idempotency_keys`)
- Redis running (required for cache and distributed locking)
- Casbin enforcer with `idempotency:view` and `idempotency:manage` permissions seeded
- EventBus initialized (if using other event-driven features)

---

## Quick Start

### 1. Run Migrations

```bash
make migrate
# Applies migrations/000027_create_idempotency_keys.up.sql
# Creates: idempotency_keys table
```

### 2. Seed Permissions

```bash
make seed
# or: go run ./cmd/api permission:sync
# Adds: idempotency:view, idempotency:manage
```

### 3. Start the Server

```bash
make serve
```

The IdempotencyService will:
- Register idempotency middleware in the request pipeline (after JWT)
- Start the background reaper goroutine (configurable interval, default 60s)
- Register management routes under `/api/v1/idempotency/`

---

## API Usage

### Making an Idempotent Request

Include the `Idempotency-Key` header on any POST, PUT, or PATCH request:

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer <token>" \
  -H "Idempotency-Key: create-invoice-2026-05-18-001" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100.00, "description": "Monthly subscription"}'
```

First request processes normally. Response includes:

```
HTTP/1.1 201 Created
X-Idempotency-Key: create-invoice-2026-05-18-001
X-Idempotency-Replayed: false
Content-Type: application/json
```

Subsequent requests with the same key return the cached response:

```
HTTP/1.1 201 Created
X-Idempotency-Key: create-invoice-2026-05-18-001
X-Idempotency-Replayed: true
Content-Type: application/json
```

The `X-Idempotency-Replayed` header tells you whether the response came from cache (`true`) or was processed fresh (`false`).

### 409 Conflict Handling

If a second request arrives while the first is still processing, the middleware returns:

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer <token>" \
  -H "Idempotency-Key: create-invoice-2026-05-18-001" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100.00}'
```

Response:

```
HTTP/1.1 409 Conflict
Retry-After: 5
Content-Type: application/json

{
  "error": {
    "code": "IDEMPOTENCY_PROCESSING",
    "message": "A request with this idempotency key is currently being processed"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

The `Retry-After` header indicates how many seconds to wait before retrying.

### List Keys

```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/idempotency/keys?page=1&per_page=20"
```

Response:

```json
{
  "data": {
    "records": [
      {
        "id": "...",
        "idempotency_key": "create-invoice-2026-05-18-001",
        "user_id": "...",
        "organization_id": "...",
        "http_method": "post",
        "request_path": "/api/v1/invoices",
        "status": "completed",
        "response_status_code": 201,
        "response_body_size": 256,
        "expires_at": "2026-05-19T10:00:00Z",
        "created_at": "2026-05-18T10:00:00Z"
      }
    ]
  },
  "meta": { "total": 1, "page": 1, "per_page": 20 }
}
```

### Get Key by ID

```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/idempotency/keys/<id>"
```

### Delete Key

```bash
curl -X DELETE -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/idempotency/keys/<id>"
# Soft-deletes the record (sets deleted_at)
# Requires idempotency:manage permission
```

### Trigger Cleanup

```bash
curl -X POST -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/idempotency/cleanup"
# Returns 202 Accepted (runs synchronously but response is async-friendly)

# Override retention days via query parameter
curl -X POST -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/idempotency/cleanup?retention_days=14"
```

Response:

```json
{
  "data": {
    "status": "accepted",
    "cleaned_count": 42,
    "retention_days": 7
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  Client Request (POST/PUT/PATCH with Idempotency-Key)   │
└──────────────────────────┬───────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │  IdempotencyMiddleware │
              │  (skip GET/DELETE/etc) │
              └───────────┬────────────┘
                          │
              ┌───────────┴───────────┐
              │                       │
        Redis GET :lock         No lock found
              │                       │
     ┌────────┴────────┐            │
     │                  │            ▼
  Lock exists      Lock miss    SETNX :lock
     │                  │       (5min TTL)
     │                  │            │
     ▼                  ▼        Acquired?
 Replay :record    :record     ┌────┴────┐
 from Redis        miss?       │         │
     │              │       Acquired  Conflict
     │              │          │      (409 + Retry-After)
     │              ▼          │
     │        SETNX :lock      ▼
     │        acquired?   Process Request
     │              │       Capture Status + Body
     │              │          │
     │              │          ▼
     │              │     ┌────────────────────┐
     │              │     │  Async goroutine:   │
     │              │     │  1. Update :lock    │
     │              │     │  2. Cache :record   │
     │              │     │   (if ≤4KB)        │
     │              │     │  3. Write to DB     │
     │              │     └────────────────────┘
     │              │
     ▼              ▼
  Return cached   Fall back to
  response          handler
```

**Redis key structure:**

```
idem:{orgSegment}:{userID}:{method}:{path}:{key}:lock    → 5min TTL (in-flight guard)
idem:{orgSegment}:{userID}:{method}:{path}:{key}:record  → 24h TTL (cached response)
```

- `orgSegment` is the organization ID, or `"default"` if no org context
- Lock stores `{"status": "processing"}` or `{"status": "completed"}`
- Record stores the full cached response (status code, body, replay headers) if under 4KB
- Responses over 4KB store a reference only; the middleware falls back to a DB lookup

---

## Key Format Requirements

| Constraint | Value |
|-----------|-------|
| Pattern | `^[a-zA-Z0-9_-]+$` |
| Max length | 128 characters |
| Applicable methods | POST, PUT, PATCH only |

GET, DELETE, OPTIONS, and HEAD requests pass through without idempotency processing.

Invalid keys return:

```json
{
  "error": {
    "code": "INVALID_IDEMPOTENCY_KEY",
    "message": "Key must match pattern ^[a-zA-Z0-9_-]+$ and be at most 128 characters"
  }
}
```

---

## Fail-Open Behavior

When Redis is unavailable, the middleware does **not** block requests. Instead, it logs a warning and passes the request through to the handler without idempotency guarantees. This means:

- No 409 Conflict errors due to Redis failures
- No cached response replays (each request is processed fresh)
- No distributed locking (concurrent duplicates may occur)
- A `warn`-level log entry is written for each Redis error

```
idempotency cache read error on lock key, fail-open  key=idem:... error="connection refused"
idempotency SetNX error, fail-open  key=idem:... error="connection refused"
```

---

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `IDEMPOTENCY_ENABLED` | true | Enable/disable idempotency middleware |
| `IDEMPOTENCY_LOCK_TTL` | 300s | Lock TTL for in-flight requests (5 minutes) |
| `IDEMPOTENCY_RECORD_TTL` | 86400s | Record TTL for cached responses (24 hours) |
| `IDEMPOTENCY_REAPER_INTERVAL` | 60s | Interval for the background cleanup reaper |
| `IDEMPOTENCY_REAPER_RETENTION_DAYS` | 7 | Days before soft-deleting expired records |
| `IDEMPOTENCY_MAX_KEY_LENGTH` | 128 | Maximum characters for an idempotency key |

Additional config fields (set via config file or struct):

| Field | Default | Description |
|-------|---------|-------------|
| `MaxCachedResponseSize` | 4096 (4KB) | Maximum response body size stored in Redis |
| `DefaultPageSize` | 20 | Default pagination page size |
| `MaxPageSize` | 100 | Maximum pagination page size |
| `KeyPattern` | `^[a-zA-Z0-9_-]+$` | Regex pattern for key validation |

---

## Permissions

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `idempotency:view` | idempotency | view | List and view keys |
| `idempotency:manage` | idempotency | manage | Delete keys; trigger cleanup |

Assign via role:

```bash
# Via API
POST /api/v1/roles/<role-id>/permissions
{ "permission_id": "<idempotency:view-permission-id>" }

# Or sync all permissions
go run ./cmd/api permission:sync
```

---

## Testing

```bash
# Unit tests — domain entities and DTOs
go test -v -race ./internal/domain/... -run Idempotency

# Unit tests — middleware logic
go test -v -race ./internal/http/middleware/... -run Idempotency

# Unit tests — service layer
go test -v -race ./tests/unit/... -run Idempotency

# Integration tests (requires Docker)
go test -v -tags=integration ./tests/integration/... -run Idempotency -timeout 5m
```

---

## Background Reaper

The IdempotencyService runs an embedded reaper goroutine that soft-deletes expired records:

1. Runs on a configurable interval (default: every 60 seconds)
2. Queries records where `expires_at < NOW() - INTERVAL '<retention_days> days'` AND `deleted_at IS NULL`
3. Sets `deleted_at = NOW()` via GORM soft-delete
4. Logs the count of cleaned records at `info` level

**Important distinction**:
- `expires_at` — the TTL expiration time for the idempotency guarantee. The record may still exist in the DB after expiration, but Redis keys have already expired.
- `deleted_at` — set by the reaper during cleanup, or by explicit DELETE from the API. Triggers GORM soft-delete filtering.

**Startup wiring** (in `cmd/api/main.go`):

```go
idempotencyService.StartReaper(ctx)

// Graceful shutdown order:
// server → eventBus → activityReaper → analyticsReaper → idempotencyService → workers → enforcer → db → redis

// During shutdown:
idempotencyService.StopReaper()
```

---

## Response Replay Headers

When a cached response is replayed, these headers are preserved:

| Header | Included in Replay | Purpose |
|--------|-------------------|---------|
| `Content-Type` | Yes | Original content type |
| `Content-Encoding` | Yes | Original encoding |
| `Content-Length` | Yes | Original body length |
| `Location` | Yes | Redirect target (for 201 responses) |
| `X-Request-Id` | Yes | Original request ID |
| `Retry-After` | Yes | For 409 Conflict responses |

Additional headers set on all idempotency responses:

| Header | Value |
|--------|-------|
| `X-Idempotency-Key` | The original idempotency key |
| `X-Idempotency-Replayed` | `"true"` if replayed from cache, `"false"` if newly processed |

---

**Version**: 1.0 | **Created**: 2026-05-18
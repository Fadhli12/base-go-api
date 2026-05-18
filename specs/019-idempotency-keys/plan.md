# Implementation Plan: Idempotency Keys

**Branch**: `019-idempotency-keys` | **Date**: 2026-05-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/019-idempotency-keys/spec.md`

## Summary

Implement RFC-draft-compliant idempotency key support for POST, PUT, and PATCH endpoints using the `Idempotency-Key` HTTP header. Redis-backed fast lookup with PostgreSQL durability layer, Echo middleware pattern (following rate limiter architecture), fail-open on Redis unavailability, per-user key scoping, 4KB response cache threshold with DB re-fetch for larger responses, and a background reaper for TTL-based cleanup. The middleware returns cached responses for duplicate requests, 409 Conflict for in-flight duplicates, and 409 Conflict for key reuse with different payloads.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, go-redis/v9, Casbin, golang-migrate
**Storage**: PostgreSQL (durability layer), Redis (fast lookup cache)
**Testing**: Go testing, testify, testcontainers-go (integration), mock logger
**Target Platform**: Linux server (Docker)
**Performance Goals**: ≤10ms cache lookup for replayed requests, ≤5ms pass-through overhead
**Constraints**: Fail-open on Redis errors; 4KB max cached response in Redis; 24h default TTL
**Scale/Scope**: Multi-instance deployment, organization-scoped RBAC

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ Pass | Structured logging, graceful degradation, edge cases covered |
| II. RBAC Mandatory | ✅ Pass | New permissions: `idempotency:view`, `idempotency:manage` (admin) |
| III. Soft Deletes | ✅ Pass | DB records use `deleted_at` for admin cleanup; Redis uses TTL |
| IV. Stateless JWT | ✅ Pass | User ID from JWT claims for per-user key scoping |
| V. Versioned Migrations | ✅ Pass | Migration `000027_create_idempotency_keys.up.sql` + `.down.sql` |
| VI. Multi-Instance Consistency | ✅ Pass | Redis provides distributed state; DB for durability |
| VII. Audit Logging | ✅ Pass | Idempotency events (created, replayed, conflict) audit-logged |
| VIII. Org-Scoped Multi-Tenancy | ✅ Pass | Keys scoped per user within org context |
| IX. Event-Driven Architecture | ⬜ N/A | No EventBus integration needed; middleware is synchronous |
| X. Background Job Processing | ✅ Pass | Reaper goroutine follows established pattern |

**Result**: All principles satisfied. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/019-idempotency-keys/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output (RFC analysis, Stripe patterns)
├── data-model.md        # Phase 1 output (entity diagram)
├── quickstart.md        # Phase 1 output (usage guide)
├── contracts/           # Phase 1 output (interface contracts)
│   └── idempotency.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── idempotency.go                        # IdempotencyConfig struct
├── domain/
│   └── idempotency.go                        # IdempotencyRecord entity, DTOs
├── repository/
│   └── idempotency.go                        # IdempotencyRepository interface + GORM impl
├── service/
│   └── idempotency.go                        # IdempotencyService (reaper, DB operations)
├── http/
│   ├── middleware/
│   │   └── idempotency.go                    # Echo middleware
│   ├── handler/
│   │   └── idempotency.go                    # HTTP handler (optional admin endpoints)
│   └── response/
│       └── envelope.go                        # (existing - no changes needed)
├── cache/
│   ├── cache.go                              # (existing Driver interface - may extend)
│   └── redis.go                              # (existing Redis impl - no changes needed)
migrations/
├── 000027_create_idempotency_keys.up.sql     # DB migration
└── 000027_create_idempotency_keys.down.sql   # Rollback
cmd/api/
│   └── main.go                               # (modify - wire config, reaper, permissions)
internal/http/
│   └── server.go                             # (modify - add IdempotencyService, middleware wire-up)
```

**Structure Decision**: Follows established project pattern exactly. Domain entities in `internal/domain/`, repository in `internal/repository/`, service in `internal/service/`, middleware in `internal/http/middleware/`, config in `internal/config/`, migration in `migrations/`.

## Research

### RFC and Industry Patterns

**IETF Draft** (`draft-ietf-httpapi-idempotency-key-header-07`):
- Header name: `Idempotency-Key` (case-insensitive)
- Key format: String, RECOMMENDED UUID v4
- Server MUST publish expiration policy
- Server SHOULD generate fingerprint from request payload
- Error codes: 400 (invalid key), 409 (concurrent request), 422 (payload mismatch)

**Stripe Implementation**:
- 24-hour TTL on keys
- Caches full response (status + body + headers)
- 95% of write requests use idempotency keys
- Even 500 errors are cached and replayed

**Our Design Decisions** (from spec clarifications):
- In-flight duplicates → 409 Conflict with Retry-After header (no polling)
- Large responses (≥4KB) → Redis stores status + reference ID only; DB stores full record for replay
- Fail-open on Redis errors (matches rate limiter pattern)

### Existing Codebase Patterns Used

| Pattern | Source File | How It's Used |
|---------|-------------|---------------|
| `cache.Driver` interface | `internal/cache/cache.go` | Redis operations via abstraction (Get, Set, Delete, Exists) |
| `RateLimiter` middleware struct | `internal/http/middleware/rate_limit.go` | Idempotency middleware follows same struct pattern |
| `responseWriter` wrapper | `internal/http/middleware/audit.go:48` | Response body capture for caching |
| `ActivityReaper` goroutine | `internal/service/activity_reaper.go` | Reaper Start/Stop pattern |
| `WebhookConfig` struct | `internal/config/webhook.go` | Config struct with mapstructure tags + defaults |
| `DefaultAuditSkipper()` | `internal/http/middleware/audit.go:27` | Skipper pattern for path filtering |
| Route-level middleware | `server.go:874-879` | JWT → Permission → Audit per-route-group |
| Permission manifest | `cmd/api/main.go` | `PermissionEntry{Name, Description, Resource, Action}` |
| Setter injection | `server.go:342-350` | `SetXxxService()` pattern for late wiring |

### Key Technical Decisions

1. **Custom middleware** (not third-party library): The project's constitution favors minimal dependencies and our requirements (4KB threshold, DB re-fetch, per-user scoping, fail-open, organization context) are specific enough that a custom implementation is cleaner and more maintainable.

2. **Redis-first, DB-second**: For in-flight requests, Redis SETNX provides distributed locking. The DB record is written after the handler completes (best-effort durability). This ordering is critical: the in-flight guard must be fast and distributed before the slower DB write.

3. **Separate lock key vs. record key**: Redis stores two keys per idempotency request:
   - `idem:{orgID}:{userID}:{key}:lock` — in-flight guard with 5-minute TTL, stores request hash
   - `idem:{orgID}:{userID}:{key}:record` — complete response record with configurable TTL (default 24h)

4. **Middleware after JWT, before handler**: The middleware needs `user_id` from JWT claims for key scoping. It must be in the per-route-group middleware chain (not global) because it only applies to authenticated, mutation endpoints.

5. **Response capture via ResponseWriter wrapper**: Follows the `audit.go` pattern — wrap `http.ResponseWriter` to capture status code and body. Skip WebSocket upgrade requests.

## Design

### Data Model

```sql
-- 000027_create_idempotency_keys.up.sql

CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(128) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    org_id UUID REFERENCES organizations(id),
    http_method VARCHAR(10) NOT NULL,
    request_path VARCHAR(500) NOT NULL,
    request_hash VARCHAR(64) NOT NULL, -- SHA-256 hex
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    -- processing | completed | conflict
    response_status_code INTEGER,
    response_body TEXT, -- full JSON response (≤4KB cached in Redis, full in DB)
    response_body_size INTEGER NOT NULL DEFAULT 0,
    response_headers JSONB, -- selected headers for replay
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Unique constraint: one active key per user+method+path combo
CREATE UNIQUE INDEX idx_idempotency_keys_active 
    ON idempotency_keys (idempotency_key, user_id, http_method, request_path)
    WHERE deleted_at IS NULL;

-- Index for reaper cleanup
CREATE INDEX idx_idempotency_keys_expires_at 
    ON idempotency_keys (expires_at) 
    WHERE deleted_at IS NULL;

-- Index for per-user lookups
CREATE INDEX idx_idempotency_keys_user_id 
    ON idempotency_keys (user_id) 
    WHERE deleted_at IS NULL;
```

### Redis Key Schema

| Key Pattern | TTL | Purpose |
|-------------|-----|---------|
| `idem:{orgID}:{userID}:{method}:{path}:{key}:lock` | 5 min | In-flight guard (processing state) |
| `idem:{orgID}:{userID}:{method}:{path}:{key}:record` | 24h (configurable) | Cached response (≤4KB) or `{status, refID}` for larger |

### Middleware Flow

```
Request arrives with Idempotency-Key header
    │
    ├── No header? → Pass through (skip middleware)
    ├── GET/DELETE? → Skip (HTTP-inherently-idempotent methods)
    ├── Invalid key format? → 400 Bad Request
    │
    ├── Has user_id from JWT?
    │   ├── Yes → Scope key: idem:{orgID}:{userID}:{method}:{path}:{key}
    │   └── No → Skip (unauthenticated endpoints not covered)
    │
    ├── Redis available?
    │   └── No → Log warning, pass through (fail-open)
    │
    ├── Redis GET lock key
    │   ├── Lock exists with DIFFERENT request hash → 409 Conflict (payload mismatch)
    │   ├── Lock exists with SAME hash, status=processing → 409 Conflict + Retry-After
    │   ├── Lock exists with SAME hash, status=completed → record key exists
    │   │   ├── Record body_size ≤ 4KB → replay from Redis directly
    │   │   └── Record body_size > 4KB → fetch from PostgreSQL, replay
    │   └── Lock does NOT exist → continue to creation
    │
    ├── Create lock in Redis (SET NX EX 300 with request hash)
    │   └── SET NX failed (race condition) → 409 Conflict
    │
    ├── Execute handler (capture response via responseWriter wrapper)
    │
    ├── Store response in Redis (record key)
    │   ├── body_size ≤ 4KB → store full JSON: {status, body, headers, hash, size}
    │   └── body_size > 4KB → store reference: {status, refID, hash, size}
    │       └── Also store full record in PostgreSQL
    │
    ├── Store record in PostgreSQL (best-effort, fire-and-forget goroutine)
    │   ├── Launch goroutine to call IdempotencyService.CreateRecord()
    │   ├── Log error if DB write fails (don't fail the response)
    │   └── No retry on DB write failure — Redis cache serves replays for TTL duration
    │
    └── Delete lock key → request complete
```

### Component Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       Echo Middleware Chain                    │
│                                                               │
│  Recover → RequestID → StructuredLogging → CORS → RateLimit  │
│                                                               │
│  ┌──── Per-Route-Group ──────────────────────────────┐       │
│  │  JWT → Idempotency → Permission → Audit → Handler  │       │
│  │       ▲                                             │       │
│  └───────┼─────────────────────────────────────────────┘       │
│          │                                                     │
│  ┌───────▼──────────────────────────────────────────┐         │
│  │            IdempotencyMiddleware                   │         │
│  │                                                    │         │
│  │  1. Validate key format                            │         │
│  │  2. Scope key per user+org+method+path             │         │
│  │  3. Check Redis (lock + record)                    │         │
│  │  4. SETNX lock or return 409                      │         │
│  │  5. Capture response (responseWriter)              │         │
│  │  6. Store in Redis + DB (async)                    │         │
│  │  7. Set X-Idempotency-Key + X-Idempotency-Replayed │         │
│  └────────────────────────────────────────────────────┘         │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│                  Background Components                        │
│                                                               │
│  IdempotencyReaper ─────► PostgreSQL                          │
│    (soft-deletes expired records via deleted_at)              │
│    (unlike ActivityReaper which uses archived_at —            │
│     idempotency records don't need archive vs delete dist.)   │
│                                                               │
│  IdempotencyService                                          │
│    - Create(record) → PostgreSQL                             │
│    - FindByKey(key, userID, method, path) → PostgreSQL       │
│    - ArchiveExpired(retentionDays) → PostgreSQL              │
│    - GetByID(id) → PostgreSQL                                │
│    - ListByUser(userID, page, pageSize) → PostgreSQL         │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### Interface Definitions

```go
// internal/repository/idempotency.go

type IdempotencyRepository interface {
    Create(ctx context.Context, record *domain.IdempotencyRecord) error
    FindByKey(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string, responseStatusCode int, responseBody string, responseHeaders map[string]string) error
    CleanupExpired(ctx context.Context, retentionDays int) (int64, error) // soft-deletes expired records via deleted_at
    ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error)
}
```

```go
// internal/service/idempotency.go

type IdempotencyService struct {
    repo   repository.IdempotencyRepository
    config config.IdempotencyConfig
    logger logger.Logger
    ctx    context.Context
    cancel context.CancelFunc
}

func NewIdempotencyService(
    repo repository.IdempotencyRepository,
    config config.IdempotencyConfig,
    logger logger.Logger,
) *IdempotencyService

func (s *IdempotencyService) CreateRecord(ctx context.Context, record *domain.IdempotencyRecord) error
func (s *IdempotencyService) FindRecord(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error)
func (s *IdempotencyService) FindRecordByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error)
func (s *IdempotencyService) UpdateRecord(ctx context.Context, id uuid.UUID, status string, statusCode int, body string, headers map[string]string) error
func (s *IdempotencyService) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error)
func (s *IdempotencyService) CleanupExpired(retentionDays int) (int64, error) // reaper calls this
func (s *IdempotencyService) StartReaper(ctx context.Context)
func (s *IdempotencyService) StopReaper()
```

```go
// internal/http/middleware/idempotency.go

type IdempotencyMiddleware struct {
    cache  cache.Driver
    service *service.IdempotencyService
    config config.IdempotencyConfig
    logger logger.Logger
}

func NewIdempotencyMiddleware(
    cache cache.Driver,
    service *service.IdempotencyService,
    config config.IdempotencyConfig,
    logger logger.Logger,
) *IdempotencyMiddleware

func (m *IdempotencyMiddleware) Middleware() echo.MiddlewareFunc
```

### Config Struct

```go
// internal/config/idempotency.go

type IdempotencyConfig struct {
    Enabled            bool          `mapstructure:"enabled"`
    MaxKeyLength       int           `mapstructure:"max_key_length"`
    KeyPattern        string        `mapstructure:"key_pattern"` // regex pattern
    DefaultTTL        time.Duration `mapstructure:"default_ttl"`
    MinTTL             time.Duration `mapstructure:"min_ttl"`
    MaxTTL             time.Duration `mapstructure:"max_ttl"`
    GuardTTL           time.Duration `mapstructure:"guard_ttl"` // in-flight lock TTL
    MaxCachedResponseSize int        `mapstructure:"max_cached_response_size"` // 4KB
    ReaperInterval     time.Duration `mapstructure:"reaper_interval"`
    RetentionDays      int           `mapstructure:"retention_days"`
}

func DefaultIdempotencyConfig() IdempotencyConfig {
    return IdempotencyConfig{
        Enabled:               true,
        MaxKeyLength:          128,
        KeyPattern:           `^[a-zA-Z0-9_-]+$`,
        DefaultTTL:            24 * time.Hour,
        MinTTL:                1 * time.Hour,
        MaxTTL:                72 * time.Hour,
        GuardTTL:              5 * time.Minute,
        MaxCachedResponseSize: 4096, // 4KB
        ReaperInterval:        1 * time.Hour,
        RetentionDays:        30,
    }
}
```

### Permissions

```go
// Added to main.go permission seed
{Name: "idempotency:view", Description: "View idempotency key records", Resource: "idempotency", Action: "view"},
{Name: "idempotency:manage", Description: "Manage idempotency configuration and trigger cleanup", Resource: "idempotency", Action: "manage"},
```

### Migration Design

**File**: `migrations/000027_create_idempotency_keys.up.sql`
- Creates `idempotency_keys` table with UUID PK, soft delete, proper indexes
- Partial unique index (WHERE deleted_at IS NULL) for active key uniqueness
- Index on `expires_at` for reaper cleanup
- Index on `user_id` for per-user lookups

**File**: `migrations/000027_create_idempotency_keys.down.sql`
- Drops indexes and table

## Implementation Phases

### Phase 1: Foundation (Domain, Config, Migration, Cache Extension)

**Files to create**:
- `internal/config/idempotency.go` — Config struct with defaults
- `internal/domain/idempotency.go` — Entity, DTOs, status constants
- `migrations/000027_create_idempotency_keys.up.sql` — Table creation
- `migrations/000027_create_idempotency_keys.down.sql` — Rollback

**Files to modify**:
- `internal/cache/cache.go` — Add `SetNX(ctx, key, value, ttlSeconds) (bool, error)` to `Driver` interface
- `internal/cache/redis.go` — Implement `SetNX` using `redis.Client.SetNX()`
- `internal/cache/memory.go` — Implement `SetNX` using mutex-protected map (check if key exists, if not set it)
- `internal/cache/noop.go` — Implement `SetNX` as no-op returning `false, nil`

**⚠️ Critical Dependency**: The `cache.Driver` interface currently lacks `SetNX` (Set if Not eXists), which the middleware requires for atomic in-flight guard locking. This must be added first. The `SetNX` method returns `(bool, error)` where `bool` indicates whether the key was set (true = acquired lock, false = key already exists). All three cache implementations (redis, memory, noop) must implement it.

**Dependencies**: None (foundation layer)

**Verification**: `go build` passes, all cache.Driver implementations satisfy the extended interface, migration up/down works

### Phase 2: Repository Layer

**Files to create**:
- `internal/repository/idempotency.go` — Interface + GORM implementation

**Dependencies**: Phase 1 (domain entity)

**Verification**: Unit tests for Create, FindByKey, UpdateStatus, ArchiveOlderThan, ListByUser

### Phase 3: Service Layer (Business Logic + Reaper)

**Files to create**:
- `internal/service/idempotency.go` — Service + embedded reaper goroutine

**Dependencies**: Phase 2 (repository), Phase 1 (config)

**Verification**: Unit tests for service methods, reaper Start/Stop lifecycle

### Phase 4: Middleware (Core Feature)

**Files to create**:
- `internal/http/middleware/idempotency.go` — Echo middleware

**Dependencies**: Phase 3 (service), cache.Driver interface

**Key Implementation Details**:
1. Skip GET, DELETE, OPTIONS, HEAD requests (spec FR-010)
2. Skip if no `Idempotency-Key` header (optional, not required)
3. Skip if no user ID in context (unauthenticated endpoints)
4. Validate key format with regex `^[a-zA-Z0-9_-]+$`, max 128 chars (FR-009)
5. Scope key per user+org: `idem:{orgID}:{userID}:{method}:{path}:{key}`
6. Check Redis for lock key → SETNX with guard TTL
7. If lock exists: check request hash → 409 on mismatch or in-flight
8. If record exists: replay cached response (Redis if ≤4KB, DB if larger)
9. Capture response via responseWriter wrapper (following audit.go pattern)
10. Store response in Redis (record key) + async DB write
11. Set `X-Idempotency-Key` and `X-Idempotency-Replayed` headers
12. Fail-open on Redis errors (FR-012)
13. Log audit event on create, replay, conflict (FR-014)

**Verification**: Unit tests for all middleware paths, integration tests with real Redis

### Phase 5: Wiring and Integration

**Files to modify**:
- `cmd/api/main.go` — Add config parsing, service creation, reaper start, permission seeds
- `internal/http/server.go` — Add `idempotencyService` field, setter, middleware wire-up in route groups
- `internal/config/config.go` — Add `Idempotency IdempotencyConfig` to Config struct

**Dependencies**: Phase 4 (middleware), Phase 3 (service)

**Wire-up order**:
```
Config → DB → Redis → Casbin → Cache → Invalidator →
Server → IdempotencyMiddleware (applied per-route-group after JWT) →
IdempotencyReaper.Start(ctx) →
Graceful shutdown: ... → idempotencyReaper.Stop() → ...
```

**Middleware position in route groups**:
```go
// After JWT, before Permission+Audit (need user_id, but before business logic)
protected.Use(middleware.JWT(...))
protected.Use(idempotencyMiddleware.Middleware()) // ← HERE
protected.Use(middleware.RequirePermission(...))
protected.Use(middleware.Audit(...))
```

**Verification**: Integration tests — full request lifecycle with idempotency

### Phase 6: Optional Admin Endpoints

**Files to create**:
- `internal/http/handler/idempotency.go` — Admin endpoints (P3 from spec)

**Endpoints**:
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/idempotency/keys` | idempotency:view | List idempotency records for current user |
| GET | `/api/v1/idempotency/keys/:id` | idempotency:view | Get specific record |
| POST | `/api/v1/idempotency/cleanup` | idempotency:manage | Trigger manual cleanup |

**Dependencies**: Phase 3 (service)

**Verification**: Handler tests

### Phase 7: Tests and Documentation

**Files to create/modify**:
- Unit tests for each layer (already in each phase)
- Integration tests in `tests/integration/`
- `specs/019-idempotency-keys/quickstart.md`
- `specs/019-idempotency-keys/contracts/idempotency.md`

**Test Coverage**:
- Middleware unit tests: key validation, Redis fail-open, cache hit, in-flight conflict, payload mismatch
- Service unit tests: Create, FindByKey, UpdateStatus, ArchiveExpired, reaper lifecycle
- Repository tests: CRUD, soft delete, partial unique index
- Integration tests: full request lifecycle with idempotency header
- Edge cases: expired TTL, Redis unavailable, 4KB threshold, concurrent requests, WebSocket skip

## Complexity Tracking

No constitution violations detected. The implementation follows all established patterns (middleware, reaper, repository, config, migration).

## Notes

### Key Design Choices

1. **Custom middleware vs. third-party**: The project constitution favors minimal dependencies. The idempotency requirements (4KB threshold, DB re-fetch, per-user scoping, fail-open) are specific enough that `bright-room/idem` or `velmie/idempo` would need significant wrapping anyway. Custom implementation gives full control and matches existing patterns.

2. **`cache.Driver` Interface Extension**: The existing `cache.Driver` interface lacks `SetNX(ctx, key, value, ttlSeconds) (bool, error)`, which is required for atomic in-flight guard locking. This method must be added to the interface and implemented in all three cache backends (redis → `redis.Client.SetNX()`, memory → mutex check-then-set, noop → returns `false, nil`). This is done in Phase 1 before any other implementation.

3. **Redis-first for in-flight guard**: Redis SETNX provides atomic distributed locking. The in-flight guard must be available before the handler executes, and Redis provides sub-millisecond latency for this. The DB write is best-effort (async) — durability, not correctness, depends on the DB record.

4. **409 Conflict instead of polling**: Returning 409 immediately with Retry-After is simpler than long-polling. The spec explicitly chose this (clarification session). The 5-minute guard TTL means a stuck request clears in at most 5 minutes.

5. **Async DB write via fire-and-forget goroutine**: The middleware writes to Redis synchronously (needed for correctness) but writes to PostgreSQL asynchronously in a goroutine (best-effort durability). If the DB write fails, the response is still served from Redis for the TTL duration. No retry on DB write failure — the reaper handles cleanup of records that do exist, and Redis serves the cached response regardless.

6. **Middleware position after JWT**: Idempotency middleware needs user_id from JWT claims for key scoping. It cannot be global (before JWT) because unauthenticated requests don't need idempotency protection.

7. **No EventBus integration**: Idempotency is a synchronous cross-cutting concern (middleware), not a domain event. Audit logging of idempotency events uses the existing AuditService directly, not EventBus.

8. **Reaper uses `deleted_at` (soft delete)**: Unlike `ActivityReaper` which distinguishes `archived_at` vs `deleted_at`, the idempotency reaper soft-deletes expired records via `deleted_at`. This matches the project constitution (soft deletes on all entities) and the partial unique index `WHERE deleted_at IS NULL` ensures expired keys can be reused after cleanup. The method is named `CleanupExpired` (not `ArchiveOlderThan`) to reflect this behavior.

### Potential Risks

| Risk | Mitigation |
|------|------------|
| Redis key explosion | TTL-based expiration + reaper cleanup |
| Network partition (Redis unavailable) | Fail-open: process without idempotency |
| Race condition on SETNX | Atomic Redis SETNX prevents duplicate claims |
| Large response replay | 4KB threshold in Redis, DB re-fetch for larger |
| Memory pressure (many concurrent requests) | 5-minute guard TTL limits lock duration |
| DB write failure | Best-effort async write; Redis still serves cached responses |
| `cache.Driver` interface change | Add `SetNX` method with noop/memory/redis implementations; backward compatible
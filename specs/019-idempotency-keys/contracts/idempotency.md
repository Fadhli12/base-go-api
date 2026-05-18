# Idempotency Key Service Contract

**Feature ID:** 019-idempotency-keys | **Date:** 2026-05-18

## Service Interface

```go
type IdempotencyService struct {
    repo   repository.IdempotencyRepository
    config config.IdempotencyConfig
    logger *slog.Logger
    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func NewIdempotencyService(
    repo repository.IdempotencyRepository,
    config config.IdempotencyConfig,
    logger *slog.Logger,
) *IdempotencyService
```

## Method Contracts

### CreateRecord(ctx, record *domain.IdempotencyRecord) error

**No RBAC** — called from middleware, not handler
**Persists** a new idempotency record to PostgreSQL
**Error cases**: wraps internal errors via `apperrors.WrapInternal(err)`

### FindRecord(ctx, key, userID, method, path) (*domain.IdempotencyRecord, error)

**No RBAC** — called from middleware and handler
**Locates** an active (non-deleted) record by composite key: `idempotency_key + user_id + http_method + request_path`
**Error cases**: `ErrNotFound` (via repository), wraps internal errors

### FindRecordByID(ctx, id) (*domain.IdempotencyRecord, error)

**No RBAC** — handler applies ownership check separately
**Retrieves** an idempotency record by primary key (UUID)
**Error cases**: `ErrNotFound` (via repository), wraps internal errors

### UpdateRecord(ctx, id, status, statusCode, body, headers) error

**No RBAC** — called from middleware async goroutine
**Updates** status and cached response fields on an existing record
**Error cases**: wraps internal errors via `apperrors.WrapInternal(err)`

### ListByUser(ctx, userID, page, pageSize) ([]*domain.IdempotencyRecord, int64, error)

**Handler RBAC**: `idempotency:view` route middleware
**Returns** paginated records for a specific user, ordered by `created_at DESC`
**Pagination defaults**: `DefaultPageSize` (20), `MaxPageSize` (100) from config
**Error cases**: wraps internal errors

### Delete(ctx, id) error

**Handler RBAC**: `idempotency:manage` route middleware + ownership check
**Soft-deletes** a record by setting `deleted_at`
**Error cases**: wraps internal errors, `ErrNotFound` if record not found or already deleted

### CleanupExpired(retentionDays int) (int64, error)

**No RBAC** — background reaper or handler trigger
**Soft-deletes** expired records where `expires_at < NOW() - retentionDays`
**Default**: uses `config.RetentionDays` if argument <= 0
**Returns**: count of records cleaned up
**Error cases**: wraps internal errors

### StartReaper(ctx) / StopReaper()

**Background goroutine lifecycle**
**Interval**: configurable via `config.ReaperInterval` (default 60s)
**StartReaper**: spawns goroutine with `context.WithCancel`, ticker-based cleanup
**StopReaper**: cancels context, calls `wg.Wait()` for graceful shutdown
**Callback**: calls `CleanupExpired(ctx, config.RetentionDays)` on each tick
**Panic safety**: reaper logs errors and continues; does not propagate panics

## Handler Contract

```go
type IdempotencyHandler struct {
    service      *service.IdempotencyService
    auditService *service.AuditService
    enforcer     *permission.Enforcer
}

func NewIdempotencyHandler(service *service.IdempotencyService, auditService *service.AuditService, enforcer *permission.Enforcer) *IdempotencyHandler

func (h *IdempotencyHandler) RegisterRoutes(api *echo.Group, jwtSecret string)
```

## Route Details

| Method | Path | Handler | Permission | Notes |
|--------|------|---------|-----------|-------|
| GET | `/api/v1/idempotency/keys` | List | `idempotency:view` | Paginated, scoped to current user |
| GET | `/api/v1/idempotency/keys/:id` | GetByID | `idempotency:view` | Ownership check: user_id or org_id match |
| DELETE | `/api/v1/idempotency/keys/:id` | Delete | `idempotency:manage` | Ownership check: user_id or org_id match |
| POST | `/api/v1/idempotency/cleanup` | TriggerCleanup | `idempotency:manage` | Returns 202 Accepted |

### Route Group Details

Three route groups with distinct permission levels:

1. **View group** (`/idempotency/keys`): JWT auth + `idempotency:view`
   - GET "" → List
   - GET "/:id" → GetByID

2. **Manage group** (`/idempotency/keys`): JWT auth + `idempotency:manage`
   - DELETE "/:id" → Delete

3. **Cleanup group** (`/idempotency/cleanup`): JWT auth + `idempotency:manage`
   - POST "" → TriggerCleanup

### Ownership Enforcement

GetByID and Delete enforce ownership at the handler level:

```go
// User can only access their own records, unless same organization
if record.UserID != userID {
    orgID, hasOrgID := middleware.GetOrganizationID(c)
    if !hasOrgID || orgID == uuid.Nil || record.OrganizationID == nil || *record.OrganizationID != orgID {
        return 403 Forbidden
    }
}
```

### Audit Logging

- **Delete**: logs `action=delete, resource=idempotency_key, id=UUID`
- **TriggerCleanup**: logs `action=cleanup, resource=idempotency_key`

## Middleware Contract

```go
type IdempotencyMiddleware struct {
    cache   cache.Driver
    idemSvc *service.IdempotencyService
    config  config.IdempotencyConfig
    logger  *slog.Logger
}

func NewIdempotencyMiddleware(cache cache.Driver, idemSvc *service.IdempotencyService, cfg config.IdempotencyConfig, logger *slog.Logger) *IdempotencyMiddleware

func (im *IdempotencyMiddleware) Middleware() echo.MiddlewareFunc
```

### Middleware Flow

1. **Skip conditions** (pass through to next handler):
   - No `Idempotency-Key` header present
   - HTTP method is GET, DELETE, OPTIONS, or HEAD
   - `config.Enabled == false`
   - `cache.Driver == nil` (logs warning)

2. **Key validation**:
   - Regex: `config.KeyPattern` (default `^[a-zA-Z0-9_-]+$`)
   - Max length: `config.MaxKeyLength` (default 128)
   - Returns 400 `INVALID_IDEMPOTENCY_KEY` on failure

3. **User scoping**:
   - Extract `userID` from JWT context via `GetUserID(c)`
   - Extract `orgID` from `GetOrganizationID(c)` (falls back to `"default"`)
   - Redis key format: `idem:{org}:{user}:{method}:{path}:{key}:lock` / `:record`

4. **Redis lock check** (`GET :lock`):
   - If `status == "processing"` → return 409 `IDEMPOTENCY_PROCESSING` + `Retry-After: 5`
   - If `status == "completed"` → check Redis record key for cached response

5. **Redis record check** (`GET :record`):
   - Cache hit → replay cached response: set `X-Idempotency-Replayed: true`
   - Cache miss → fall back to DB lookup via `FindRecord()`

6. **SETNX lock acquisition**:
   - Lock data: `{"status": "processing", "request_hash": "<sha256>"}`
   - TTL: `config.GuardTTL` (default 5 min)
   - Returns 409 `IDEMPOTENCY_CONFLICT` if lock not acquired + `Retry-After: 5`

7. **Request body hash** (SHA-256):
   - Computes `sha256(body)` for deduplication
   - Empty/no body: hashes nil → known SHA-256 digest of empty input
   - Body is re-read into buffer and restored via `io.NopCloser`

8. **Response capture** via `idempotencyResponseWriter`:
   - Wraps `http.ResponseWriter`
   - Captures: status code, response body (up to `MaxCachedResponseSize + 1KB`), selected headers
   - Skips capture for WebSocket upgrade requests
   - Replay header allowlist: `Content-Type`, `Content-Encoding`, `Content-Length`, `Location`, `X-Request-Id`, `Retry-After`

9. **Async storeResult** (fire-and-forget goroutine with panic recovery):
   - Updates lock key to `"completed"` status in Redis
   - Stores full response in Redis if body ≤ `MaxCachedResponseSize` (default 4KB)
   - Creates `IdempotencyRecord` in DB with status `"processing"` via `CreateRecord()`
   - Updates record to `status="completed"` via `UpdateRecord()`
   - Uses `context.Background()` (no deadline) for DB operations
   - Fail-open: logs warnings on Redis/DB errors, does not block the original response

### Response Headers Set by Middleware

| Header | Value | When |
|--------|-------|------|
| `X-Idempotency-Key` | Echo of submitted key | On all processed requests |
| `X-Idempotency-Replayed` | `"true"` or `"false"` | On all processed requests |
| `Retry-After` | `"5"` | On 409 Conflict responses |

### Cached Response Structure (Redis)

```json
{
  "status_code": 201,
  "body": "...",
  "headers": {
    "Content-Type": "application/json",
    "Location": "/api/v1/resources/123"
  }
}
```

## Repository Contract

```go
type IdempotencyRepository interface {
    Create(ctx context.Context, record *domain.IdempotencyRecord) error
    FindByKey(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status string, responseStatusCode int, responseBody string, responseHeaders map[string]string) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error)
    CleanupExpired(ctx context.Context, retentionDays int) (int64, error)
}
```

Error mapping: `gorm.ErrRecordNotFound` → `apperrors.ErrNotFound`

### FindByKey

- Composite WHERE: `idempotency_key = ? AND user_id = ? AND http_method = ? AND request_path = ?`
- GORM soft-delete scope (`deleted_at IS NULL`) automatically applied via `gorm.DeletedAt` field

### FindByID

- Primary key lookup with automatic soft-delete scope

### UpdateStatus

- Uses `map[string]interface{}` partial update: `status`, `response_status_code`, `response_body`, `response_body_size`, `response_headers`
- Returns `ErrNotFound` if `RowsAffected == 0`

### CleanupExpired

- Uses `Unscoped()` to include soft-deleted records in the WHERE clause
- Filters: `expires_at < cutoff AND deleted_at IS NULL`
- Sets `deleted_at = NOW()` on matching records
- Returns count of affected rows

### ListByUser

- WHERE `user_id = ?`, ordered by `created_at DESC`
- Paginated with offset/limit
- Returns total count for pagination metadata

### SoftDelete

- GORM standard: `db.Delete(&IdempotencyRecord{}, "id = ?")`
- Returns `ErrNotFound` if `RowsAffected == 0`

## Domain Contract

### Entity: IdempotencyRecord

```go
type IdempotencyRecord struct {
    ID                 uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    IdempotencyKey     string          `gorm:"type:varchar(128);not null"`
    UserID             uuid.UUID       `gorm:"type:uuid;not null;index"`
    OrganizationID     *uuid.UUID      `gorm:"type:uuid;index"`
    HTTPMethod         string          `gorm:"type:varchar(10);not null"`
    RequestPath        string          `gorm:"type:varchar(500);not null"`
    RequestHash         string          `gorm:"type:varchar(64);not null"`  // SHA-256 hex
    Status             string          `gorm:"type:varchar(20);not null;default:'processing'"`
    ResponseStatusCode int             `gorm:"integer"`
    ResponseBody       string          `gorm:"type:text"`
    ResponseBodySize    int             `gorm:"integer;not null;default:0"`
    ResponseHeaders     MapStringString `gorm:"type:jsonb"`
    ExpiresAt          time.Time       `gorm:"not null;index"`
    CreatedAt          time.Time
    UpdatedAt          time.Time
    DeletedAt          gorm.DeletedAt  `gorm:"index"`
}

func (IdempotencyRecord) TableName() string { return "idempotency_keys" }
```

### Helper Type: MapStringString

```go
type MapStringString map[string]string
```

JSONB-backed map for storing selected response headers.

### Status Constants

```go
const (
    IdempotencyStatusProcessing = "processing"
    IdempotencyStatusCompleted  = "completed"
    IdempotencyStatusConflict   = "conflict"
)
```

### Business Methods

```go
func (r *IdempotencyRecord) IsExpired() bool
// Returns true if time.Now().After(r.ExpiresAt)

func (r *IdempotencyRecord) IsProcessing() bool
// Returns true if r.Status == "processing"

func (r *IdempotencyRecord) IsCompleted() bool
// Returns true if r.Status == "completed"

func (r *IdempotencyRecord) ToResponse() *IdempotencyRecordResponse
// Strips ResponseBody and ResponseHeaders; includes ID, key, user_id, org_id,
// method, path, status, response_status_code, response_body_size, expires_at, created_at
```

### Response DTOs

```go
type IdempotencyRecordResponse struct {
    ID                 uuid.UUID  `json:"id"`
    IdempotencyKey     string     `json:"idempotency_key"`
    UserID             uuid.UUID  `json:"user_id"`
    OrganizationID      *string    `json:"organization_id,omitempty"`
    HTTPMethod         string     `json:"http_method"`
    RequestPath        string     `json:"request_path"`
    Status             string     `json:"status"`
    ResponseStatusCode int        `json:"response_status_code"`
    ResponseBodySize    int        `json:"response_body_size"`
    ExpiresAt          time.Time  `json:"expires_at"`
    CreatedAt          time.Time  `json:"created_at"`
}

type IdempotencyListResponse struct {
    Records []*IdempotencyRecordResponse `json:"data"`
    Meta    *ListMeta                    `json:"meta,omitempty"`
}

type ListMeta struct {
    Total   int64 `json:"total"`
    Page    int   `json:"page"`
    PerPage int   `json:"per_page"`
}
```

## Configuration

```go
type IdempotencyConfig struct {
    Enabled                bool          `mapstructure:"enabled"`
    MaxKeyLength           int           `mapstructure:"max_key_length"`
    KeyPattern             string        `mapstructure:"key_pattern"`
    DefaultTTL            time.Duration `mapstructure:"default_ttl"`
    MinTTL                time.Duration `mapstructure:"min_ttl"`
    MaxTTL                time.Duration `mapstructure:"max_ttl"`
    GuardTTL              time.Duration `mapstructure:"guard_ttl"`
    MaxCachedResponseSize int           `mapstructure:"max_cached_response_size"`
    ReaperInterval        time.Duration `mapstructure:"reaper_interval"`
    RetentionDays         int           `mapstructure:"retention_days"`
    DefaultPageSize      int           `mapstructure:"default_page_size"`
    MaxPageSize            int           `mapstructure:"max_page_size"`
}
```

| Variable | Default | Description |
|----------|---------|-------------|
| `IDEMPOTENCY_ENABLED` | `true` | Enable/disable idempotency middleware |
| `IDEMPOTENCY_MAX_KEY_LENGTH` | `128` | Maximum characters for idempotency key |
| `IDEMPOTENCY_KEY_PATTERN` | `^[a-zA-Z0-9_-]+$` | Regex pattern for key validation |
| `IDEMPOTENCY_DEFAULT_TTL` | `24h` | Default TTL for cached responses |
| `IDEMPOTENCY_GUARD_TTL` | `5m` | Lock TTL for in-flight requests |
| `IDEMPOTENCY_MAX_CACHED_RESPONSE_SIZE` | `4096` | Max response body size cached in Redis (bytes) |
| `IDEMPOTENCY_REAPER_INTERVAL` | `60s` | Cleanup interval for expired records |
| `IDEMPOTENCY_REAPER_RETENTION_DAYS` | `30` | Days before soft-deleting expired records |
| `IDEMPOTENCY_DEFAULT_PAGE_SIZE` | `20` | Default pagination page size |
| `IDEMPOTENCY_MAX_PAGE_SIZE` | `100` | Maximum pagination page size |

## Startup Wiring

```go
// cmd/api/main.go
idempotencyRepo := repository.NewIdempotencyKeyRepository(db)
idempotencyService := service.NewIdempotencyService(
    idempotencyRepo, cacheDriver, auditService, enforcer, slog.Default(), cfg.Idempotency,
)
idempotencyHandler := handler.NewIdempotencyHandler(idempotencyService, auditService, enforcer)
idempotencyHandler.RegisterRoutes(v1, jwtSecret)

// Middleware
idempotencyMW := middleware.NewIdempotencyMiddleware(cacheDriver, idempotencyService, cfg.Idempotency, slog.Default())
e.Use(idempotencyMW.Middleware())

// Background reaper
idempotencyService.StartReaper(ctx)

// Graceful shutdown
// Order: server → eventBus → activityReaper → analyticsReaper → idempotencyService → workers → enforcer → db → redis
idempotencyService.StopReaper()
```
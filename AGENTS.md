# Go API Base - Agent Instructions

**Generated:** 2026-05-03 | **Commit:** HEAD | **Branch:** 006-webhook-system

---

<!-- SPECKIT START -->
**Current Feature**: [specs/010-file-versioning/plan.md](specs/010-file-versioning/plan.md) - File Versioning
**Latest Feature**: [specs/009-background-job-queue/plan.md](specs/009-background-job-queue/plan.md) - Background Job Queue ✅
<!-- SPECKIT END -->

---

## OVERVIEW

Production-ready Go REST API with RBAC (Casbin), JWT, and permission management. Echo v4 + GORM. Uses SQL migrations (NOT AutoMigrate), soft deletes, and testcontainers for integration tests.

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Entry points | `cmd/api/main.go` | Cobra CLI: serve, migrate, seed, permission:sync |
| HTTP handlers | `internal/http/handler/` | Echo handlers, request validation |
| Business logic | `internal/service/` | Domain services, permission checks |
| Data layer | `internal/repository/` | GORM repositories, soft delete scopes |
| Permission system | `internal/permission/` | Casbin enforcer, Redis cache, pub/sub |
| Domain entities | `internal/domain/` | GORM models with UUID, soft delete |
| Example feature | `internal/module/invoice/` | Domain module pattern, scope checks |
| Migrations | `migrations/*.sql` | golang-migrate UP/DOWN files |
| Integration tests | `tests/integration/` | testcontainers-go suite |
| **Logging system** | **`internal/logger/`** | **Structured logging with slog, context propagation, multiple writers** |
| **Logging middleware** | **`internal/http/middleware/structured_logging.go`** | **Request/response logging, automatic context extraction** |
| **Webhook system** | **`internal/service/webhook*.go`** | **CRUD, worker, queue, rate limiter, signing, event dispatch** |
| **Webhook handler** | **`internal/http/handler/webhook.go`** | **8 endpoints: CRUD + delivery tracking + replay** |
| **Webhook domain** | **`internal/domain/webhook.go`, `webhook_events.go`** | **Entities, DTOs, EventBus with Go channels** |
| **Webhook migrations** | **`migrations/000010_webhooks*.sql`** | **Webhooks and webhook_deliveries tables** |
| **SendGrid provider** | **`internal/service/email_sendgrid_provider.go`** | **SendGrid email integration** |
| **SES provider** | **`internal/service/email_ses_provider.go`** | **AWS SES email integration** |
| **Activity feed system** | **`internal/service/activity*.go`** | **CRUD, read tracking, follow, reaper, event dispatch** |
| **Activity handler** | **`internal/http/handler/activity.go`** | **8 endpoints: list, count-unread, mark-all-read, mark-read, follow, unfollow, list-follows, delete** |
| **Activity domain** | **`internal/domain/activity.go`, `activity_events.go`** | **Entities, DTOs, ActionType mapping from EventBus events** |
| **Activity migrations** | **`migrations/000023_create_activities*.sql`** | **Activities, activity_reads, activity_follows tables** |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `runServer` | func | `cmd/api/main.go:116` | Server init, graceful shutdown |
| `runSeed` | func | `cmd/api/main.go:428` | Seed roles/permissions |
| `NewTestSuite` | func | `tests/integration/testsuite.go:43` | Ephemeral Postgres+Redis |
| `Enforcer` | struct | `internal/permission/enforcer.go` | Casbin RBAC |
| `Server` | struct | `internal/http/server.go:28` | Echo + middleware chain |
| `WebhookService` | struct | `internal/service/webhook.go` | Webhook CRUD + dispatch + delivery |
| `WebhookWorker` | struct | `internal/service/webhook_worker.go` | Background delivery processor |
| `EventBus` | struct | `internal/domain/webhook_events.go` | In-process event publisher/subscriber |
| `ActivityService` | struct | `internal/service/activity.go:19` | Activity feed CRUD + event dispatch + read tracking |
| `ActivityFollowService` | struct | `internal/service/activity_follow.go:16` | Follow/unfollow resources |
| `ActivityReaper` | struct | `internal/service/activity_reaper.go:14` | Background archival goroutine |
| `ActivityHandler` | struct | `internal/http/handler/activity.go:24` | 8 HTTP endpoints |

## COMMANDS

```bash
make serve             # Start API
make migrate           # Run migrations UP
make seed              # Seed roles/permissions
make test              # Unit tests
make test-integration  # Integration tests (Docker required)
make lint              # golangci-lint (5m, gosec)
make docker-run        # Start Postgres + Redis + API
make clean             # Stop containers, remove volumes
```

## CONVENTIONS

### Testing

```bash
# Unit (no Docker)
go test -v -race -coverprofile=coverage.txt ./tests/unit/...

# Integration (needs Docker)
go test -v -tags=integration ./tests/integration/... -timeout 5m
```

- **Unit tests**: No build tag, use `testify/mock`
- **Integration**: `//go:build integration`, use `testcontainers-go`
- **TestSuite**: `NewTestSuite(t)` → containers → `RunMigrations(t)` → `SetupTest(t)` per test

### Error Handling

```go
// pkg/errors/errors.go
var ErrNotFound = NewAppError("NOT_FOUND", "resource not found", 404)

// Handler
if errors.Is(err, errors.ErrNotFound) {
    return echo.NewHTTPError(http.StatusNotFound, "Not found")
}
```

### Response Envelope

```go
type Envelope struct {
    Data  interface{}  `json:"data,omitempty"`
    Error *ErrorDetail `json:"error,omitempty"`
    Meta  *Meta        `json:"meta,omitempty"` // request_id, timestamp
}
```

### Repository Pattern

```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
// All methods: context.Context first param
// Use r.db.WithContext(ctx) for all GORM operations
```

### Domain Entities

```go
type User struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    CreatedAt time.Time      
    UpdatedAt time.Time      
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // Soft delete
}
func (User) TableName() string { return "users" }
func (u *User) ToResponse() UserResponse { ... } // Strip PasswordHash
```

## ANTI-PATTERNS (THIS PROJECT)

### FORBIDDEN

| Pattern | Reason | Enforcement |
|---------|--------|-------------|
| GORM AutoMigrate | Silent drift | SQL migrations only |
| Hard deletes | Audit loss | `DeletedAt` on all |
| Permission enums | Deploy needed | DB + `permission:sync` |
| Long JWT | Security | Access: 15m, Refresh: 30d |
| DELETE system roles | Integrity | `is_system=true` protected |
| UPDATE/DELETE audit_logs | Compliance | DB trigger raises error |
| printf logging | Unstructured | `slog` key-value |
| Eventual perm consistency | Security | Sync invalidation |

### MANDATORY

- Soft deletes (`deleted_at`) on all entities
- Context propagation in all repo/service methods
- UUID primary keys
- Permission checks via `enforcer.Enforce(userID, domain, resource, action)`

## NOTES

### Gotchas

1. **Migrations NOT automatic** - Run `make migrate` after `docker-compose up`
2. **Seed ≠ Sync** - `make seed` creates roles; `permission:sync` reloads Casbin
3. **Integration tests need Docker** - testcontainers ephemeral containers
4. **Graceful shutdown: 30s** - Order: server → eventBus → workers → enforcer → db → redis

### Init Order

```
Config → DB → Redis → Casbin → Cache → Invalidator → 
Server → Health → Signal → Shutdown
```

### Config Priority

1. `DATABASE_URL` / `REDIS_URL` (production)
2. Individual env vars (development)
3. `.env` file (local)
4. Viper defaults

## CRITICAL CONSTRAINTS

From `.specify/memory/constitution.md`:

1. **RBAC runtime-configurable** - No enums. DB records + `permission:sync`
2. **Soft deletes** - All tables have `deleted_at`
3. **SQL migrations** - Never GORM AutoMigrate
4. **JWT fixed expiry** - Access: 15m, Refresh: 30d
5. **Permission cache** - Redis (5m TTL) + pub/sub invalidation
6. **Audit logging** - Before/after JSONB, immutable
7. **Test coverage** - 80% unit + integration

## TROUBLESHOOTING

| Issue | Solution |
|-------|----------|
| "rootless Docker not supported" | Start Docker Desktop |
| Permission checks failing | `go run ./cmd/api permission:sync` |
| Integration tests timeout | `-timeout 10m` |
| Missing JWT_SECRET | Set in `.env` (32+ chars) |

## ORGANIZATION FEATURE

### Multi-tenant RBAC with Organization Scoping

The organization feature adds team/organization support with Casbin domain-based permissions.

**Key Components:**
- **Domain Entities**: `internal/domain/organization.go`, `internal/domain/organization_member.go`
- **Repository**: `internal/repository/organization.go` - CRUD operations with soft delete
- **Service**: `internal/service/organization.go` - Business logic, permission checks, audit logging
- **Handler**: `internal/http/handler/organization.go` - HTTP endpoints with validation
- **Middleware**: `internal/http/middleware/organization.go` - X-Organization-ID context extraction
- **Migrations**: `migrations/000006_organizations.up.sql` - Organizations and members tables

**Casbin Domain Model:**
```go
// Organization ID as domain parameter
enforcer.Enforce(userID, orgID, "organization", "view")

// Role assignments by domain
g = _, _, _  // role with domain
r = sub, dom, obj, act  // request with domain
p = sub, dom, obj, act  // policy with domain
```

**Usage Pattern:**
```go
// Extract org context from header
orgID, ok := middleware.GetOrganizationID(c)
if !ok {
    // No org context - treat as global (backward compatible)
    orgID = uuid.Nil
}

// Permission check with domain
allowed, err := enforcer.Enforce(userID.String(), orgID.String(), "organization", "manage")

// Repository scoping (future)
query := r.db.WithContext(ctx)
if orgID != uuid.Nil {
    query = query.Where("organization_id = ?", orgID)
}
```

**Organization Roles:**
- `owner` - Full organization access (manage, invite, remove)
- `admin` - Management access (manage, invite)
- `member` - View access only

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/organizations` | Create organization |
| GET | `/api/v1/organizations` | List organizations |
| GET | `/api/v1/organizations/:id` | Get organization by ID |
| PUT | `/api/v1/organizations/:id` | Update organization |
| DELETE | `/api/v1/organizations/:id` | Delete organization |
| POST | `/api/v1/organizations/:id/members` | Add member |
| GET | `/api/v1/organizations/:id/members` | List members |
| DELETE | `/api/v1/organizations/:id/members/:user_id` | Remove member |

**Middleware Chain:**
1. JWT authentication (required for all org endpoints)
2. X-Organization-ID extraction (optional, for resource scoping)
3. Permission check (in service layer)

**Backward Compatibility:**
- Organization context is optional (NULL = global resources)
- X-Organization-ID header is not required for global context
- Existing resources remain accessible without organization scoping

---

## STRUCTURED LOGGING FEATURE

### 005-log-writer: Structured Logging System

The structured logging system provides a clean abstraction layer over Go's `log/slog` with context propagation, multiple output writers, and automatic field extraction.

**Key Components:**
- **Interface**: `internal/logger/logger.go` - Logger interface definition
- **Implementation**: `internal/logger/slog_logger.go` - slog-based implementation
- **Context**: `internal/logger/context.go` - Context keys and operations
- **Configuration**: `internal/logger/config.go` - Logger configuration
- **Writers**: `internal/logger/writer*.go` - Stdout, file, syslog writers
- **Middleware**: `internal/http/middleware/structured_logging.go` - Request/response logging

**Architecture:**
```go
// Logger interface (abstraction layer)
type Logger interface {
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    WithFields(fields ...Field) Logger
    WithError(err error) Logger
}

// Usage in handlers
func (h *Handler) GetUser(c echo.Context) error {
    log := middleware.GetLogger(c)
    ctx := c.Request().Context()
    
    log.Info(ctx, "fetching user",
        log.String("user_id", userID),
    )
    
    user, err := h.service.GetUser(ctx, userID)
    if err != nil {
        log.Error(ctx, "failed to fetch user",
            log.String("user_id", userID),
            log.Err(err),
        )
        return echo.NewHTTPError(http.StatusNotFound, "User not found")
    }
    
    return c.JSON(http.StatusOK, user)
}
```

**Automatic Context Fields:**
- `request_id` - From X-Request-ID header or generated UUID
- `user_id` - From JWT claims
- `org_id` - From X-Organization-ID header
- `trace_id` - From X-Trace-ID header or generated

**Configuration (Environment Variables):**
| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | info | debug, info, warn, error |
| `LOG_FORMAT` | json | json, text |
| `LOG_OUTPUTS` | stdout | Comma-separated: stdout, file, syslog |
| `LOG_FILE_PATH` | /var/log/api.log | Log file path |
| `LOG_FILE_MAX_SIZE` | 100 | MB before rotation |
| `LOG_FILE_MAX_BACKUPS` | 10 | Backup file count |
| `LOG_FILE_MAX_AGE` | 30 | Days to keep backups |
| `LOG_FILE_COMPRESS` | true | Compress rotated files |
| `LOG_SYSLOG_NETWORK` | "" | tcp, udp, or empty for local |
| `LOG_SYSLOG_ADDRESS` | "" | Syslog server address |
| `LOG_SYSLOG_TAG` | go-api | Syslog tag |
| `LOG_ADD_SOURCE` | false | Include file:line in logs |

**Middleware Chain Integration:**
```
Request → Recover → RequestID → CORS → RateLimit → JWT → StructuredLogging → Handler
                                                      │
                                                      ▼
                                            Extracts: request_id
                                            Extracts: user_id (from JWT)
                                            Extracts: org_id (from header)
                                            Logs: request start/end
                                            Logs: duration, status
```

**Field Types:**
```go
log.String("key", "value")
log.Int("count", 42)
log.Int64("timestamp", time.Now().Unix())
log.Float64("percentage", 99.9)
log.Bool("active", true)
log.Duration("elapsed", time.Second)
log.Time("created_at", time.Now())
log.Any("metadata", map[string]interface{}{...})
log.Err(err)
```

**Field Chaining:**
```go
// Create scoped logger with common fields
orderLog := log.WithFields(
    log.String("order_id", orderID),
    log.String("customer_id", customerID),
)

// All logs include order_id and customer_id
orderLog.Info(ctx, "validating order")
orderLog.Info(ctx, "processing payment")
orderLog.Error(ctx, "payment failed", log.Err(err))
```

**Context Propagation:**
```go
// In middleware - inject into context
c.Set("logger", logger)
ctx := logger.WithRequestID(c.Request().Context(), requestID)
ctx = logger.WithUserID(ctx, userID)
c.SetRequest(c.Request().WithContext(ctx))

// In handler - extract from context
log := middleware.GetLogger(c)
// OR
log := logger.FromContext(c.Request().Context())
```

**Writers:**
- `StdoutWriter` - Console output with JSON/text format
- `FileWriter` - File output with rotation via lumberjack
- `SyslogWriter` - Syslog output with severity mapping
- `MultiWriter` - Chains multiple writers

**Testing:**
```go
// Mock logger for unit tests
mockLog := logger.NewMockLogger()
handler := NewHandler(mockLog, ...)

// Assert logged messages
assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "user created"))
assert.True(t, mockLog.AssertField("user_id", userID))
```

**Performance:**
- Log call overhead: < 1ms
- Middleware overhead: < 2ms per request
- Zero-allocation for constant keys
- Goroutine-safe implementations

**Migration from slog:**
```go
// Before
slog.Info("user created", slog.String("id", userID))

// After (backward compatible - both work)
log := middleware.GetLogger(c)
log.Info(ctx, "user created", log.String("id", userID))
```

**Compliance:**
- ✅ Zero breaking changes - existing slog calls work
- ✅ Complements audit logging - doesn't replace it
- ✅ No PII by default - request/response body logging disabled
- ✅ Context propagation - enables request tracing
- ✅ 80%+ test coverage requirement

**Documentation:**
- [Plan](specs/005-log-writer/plan.md) - Implementation plan
- [Spec](specs/005-log-writer/spec.md) - Feature specification
- [Quickstart](specs/005-log-writer/quickstart.md) - Usage guide
- [Contract](specs/005-log-writer/contracts/logger.md) - Interface contracts

---

## WEBHOOK SYSTEM FEATURE

### 006-webhook-system: Webhook Delivery System

The webhook system enables users to register HTTP endpoints that receive event notifications when specific actions occur in the API.

**Architecture:**
```
Domain Service → EventBus.Publish(event) → WebhookService.SubscribeToEventBus()
                                                   → DispatchEvent()
                                                      → FindForEventDispatch(orgID) [global + org-scoped]
                                                      → RateLimiter.Allow()
                                                      → Queue.Enqueue(delivery)
                                                                              → Worker.Process()
                                                                                → HTTP POST (signed)
                                                                                → Repo.UpdateStatus()
```

**Key Components:**
- **Domain**: `internal/domain/webhook.go` - Webhook + WebhookDelivery entities, DTOs, business methods
- **EventBus**: `internal/domain/webhook_events.go` - In-process Go channel pub/sub (Subscribe, Publish, Start, Stop). Events carry optional `OrgID` for org-scoped dispatch.
- **Repository**: `internal/repository/webhook.go` - WebhookRepository + WebhookDeliveryRepository interfaces + GORM impl. `FindForEventDispatch(orgID)` returns global + org-scoped webhooks.
- **Service**: `internal/service/webhook.go` - WebhookService CRUD + DispatchEvent + SubscribeToEventBus + delivery methods. `DispatchEvent` uses `FindForEventDispatch` to match both global and org-scoped webhooks.
- **Worker**: `internal/service/webhook_worker.go` - Background delivery processor (goroutine pool, Start/Stop)
- **Queue**: `internal/service/webhook_queue.go` + `webhook_queue_redis.go` - WebhookQueue interface + Redis sorted set impl
- **Rate Limiter**: `internal/service/webhook_rate_limiter.go` + `webhook_rate_limiter_iface.go` - Sliding window, fails open
- **Signing**: `internal/service/webhook_sign.go` - HMAC-SHA256 signature generation (avoids circular import)
- **Handler**: `internal/http/handler/webhook.go` - 8 HTTP endpoints
- **Migrations**: `migrations/000010_webhooks.up.sql` + `000010_webhooks.down.sql`

**Event Emission (T028):**
Services emit events by calling `EventBus.Publish()` after successful operations:
- `UserService.Create()` → `user.created` (payload: `UserResponse`)
- `UserService.SoftDelete()` → `user.deleted` (payload: `{id}`)
- `invoice.Service.Create()` → `invoice.created` (payload: `InvoiceResponse`)
- `invoice.Service.UpdateStatus(paid)` → `invoice.paid` (payload: `InvoiceResponse`)
- `NewsService.UpdateStatus(published)` → `news.published` (payload: `NewsResponse`)
- `NewsService.Delete()` → `news.deleted` (payload: `{id}`)

EventBus is injected via `SetEventBus()` setter on each service (avoids constructor changes).

**Startup Wiring (cmd/api/main.go):**
```go
eventBus := domain.NewEventBus(256)
server.SetEventBus(eventBus)
webhookService.SubscribeToEventBus(eventBus)
userService.SetEventBus(eventBus)
invoiceService.SetEventBus(eventBus)
newsService.SetEventBus(eventBus)
// Start event bus in background
eventBus.Start(ctx)
// ... graceful shutdown: eventBus.Stop()
```

**Webhook Signature (HMAC-SHA256):**
```
X-Webhook-Signature: sha256=<hex-hmac>
X-Webhook-Timestamp: <unix-seconds>
X-Webhook-Event: <event-type>
X-Webhook-Delivery-ID: <uuid>
```
Secret prefix `whsec_` is stripped before HMAC computation.

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks` | Create webhook |
| GET | `/api/v1/webhooks` | List webhooks |
| GET | `/api/v1/webhooks/:id` | Get webhook |
| PUT | `/api/v1/webhooks/:id` | Update webhook |
| DELETE | `/api/v1/webhooks/:id` | Delete webhook |
| GET | `/api/v1/webhooks/:id/deliveries` | List deliveries |
| GET | `/api/v1/webhooks/deliveries/:id` | Get delivery |
| POST | `/api/v1/webhooks/deliveries/:id/replay` | Replay delivery |

**Configuration (Environment Variables):**
| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_WORKER_CONCURRENCY` | 5 | Number of concurrent delivery workers |
| `WEBHOOK_RETRY_MAX` | 3 | Maximum delivery retry attempts |
| `WEBHOOK_RATE_LIMIT` | 100 | Max deliveries per webhook per minute |
| `WEBHOOK_ALLOW_HTTP` | false | Allow HTTP URLs (default: HTTPS only) |
| `WEBHOOK_DELIVERY_TIMEOUT` | 10s | HTTP request timeout per delivery |
| `WEBHOOK_DELIVERY_RETENTION_DAYS` | 90 | Days to retain delivery records |
| `WEBHOOK_MAX_PAYLOAD_SIZE` | 1048576 | Maximum payload size in bytes (1MB) |

**Backoff Strategy:**
- Attempt 1 → +1min (next_retry_at)
- Attempt 2 → +5min
- Attempt 3+ → +30min

**Stuck Delivery Recovery:**
- Worker reaper runs every 60s
- Finds deliveries stuck in `processing` status for >5min
- Resets to `queued` with incremented attempt_number

**Design Decisions:**
- `signWebhookPayload` is duplicated in `service/webhook_sign.go` to avoid circular import (service → http/middleware → service)
- `WebhookQueue` and `WebhookRateLimiterInterface` interfaces enable testability without Redis
- `SetQueue()`/`SetRateLimiter()` setters on WebhookService for post-construction Redis wiring
- Handler-level ownership check (NOT service-level Enforce)
- `CanReplay()` = status NOT IN (queued, processing); distinct from `CanRetry()` (worker automatic retry)
- `processing_started_at` field for stuck-delivery recovery
- `WebhookEvent.OrgID` propagates org context through EventBus; `DispatchEvent` uses `FindForEventDispatch` to match global + org-scoped webhooks
- Global webhooks always receive all events (even org-scoped ones); org-scoped webhooks only receive events for their org
- Shutdown order: server → eventBus → workers → enforcer → db → redis

---

## FILE VERSIONING FEATURE

### 010-file-versioning: File Versioning System

The file versioning system enables versioned uploads of media files, allowing users to upload new versions, view version history, download specific versions, restore previous versions, and delete individual versions.

**Architecture:**
```
Upload → MediaVersion → VersionService → Storage Driver → Audit Log
                 │
                 ▼
    Retroactive v1 creation on first upload (if none exists)
    SHA-256 checksum deduplication (409 on identical content)
    Optimistic locking via current_version field
```

**Key Components:**
| Component | File | Purpose | Dependencies |
|-----------|------|---------|--------------|
| Domain | `internal/domain/media_version.go` | MediaVersion entity, response DTOs, business methods | Media, User |
| Domain (extended) | `internal/domain/media.go` | Extended Media with CurrentVersion, Versions relation | MediaVersion |
| Repository | `internal/repository/media_version.go` | 9-method repository (Create, FindByMediaID, FindByID, FindByMediaAndVersion, UpdateCurrentVersion, SoftDelete, FindLatestVersion, GetVersionCount, FindVersionByChecksum) | GORM, MediaVersion |
| Service | `internal/service/media_version.go` | VersionService: UploadVersion, ListVersions, GetVersion, DownloadVersion, GetVersionSignedURL, RestoreVersion, DeleteVersion | MediaVersionRepo, MediaRepo, StorageDriver, AuditService, Enforcer |
| Checksum utility | `internal/service/media_version.go:ComputeSHA256Checksum` | SHA-256 hash computation for deduplication | crypto/sha256 |
| Handler | `internal/http/handler/media_version.go` | 7 HTTP endpoints with handler-level permission checks | VersionService, MediaService, AuditService, Enforcer |
| Migration | `migrations/000019_create_media_versions.up.sql` | media_versions table + current_version column on media | PostgreSQL |

**Permissions (DB + permission:sync):**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `media_version:upload` | media_version | upload | Upload new versions |
| `media_version:view` | media_version | view | View version history and metadata |
| `media_version:download` | media_version | download | Download specific versions |
| `media_version:restore` | media_version | restore | Restore previous versions |
| `media_version:delete` | media_version | delete | Delete specific versions (admin only) |

**Startup Wiring (cmd/api/main.go):**
```go
// No EventBus integration — versioning is synchronous
mediaVersionRepo := repository.NewMediaVersionRepository(db)
versionService := service.NewVersionService(
    mediaRepo, mediaVersionRepo, storageDriver, signingKey, auditService, enforcer,
)
versionHandler := handler.NewMediaVersionHandler(versionService, mediaService, auditService, enforcer)
versionHandler.RegisterRoutes(v1, jwtSecret)
```

**Versioned Storage Path:**
```
{base_path}/{media_id}/v{version}/{uuid_filename}.{ext}
```
Files are stored in version-specific directories under the media root. Each version gets a UUID-based filename to avoid collisions. The original filename is preserved in the database record.

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/media/:media_id/versions` | Upload a new version (multipart form, field: "file") |
| GET | `/api/v1/media/:media_id/versions` | List all versions for a media file |
| GET | `/api/v1/media/:media_id/versions/:version` | Get version metadata by version number |
| GET | `/api/v1/media/:media_id/versions/:version/download` | Download a specific version's file |
| GET | `/api/v1/media/:media_id/versions/:version/url` | Get a signed URL for a specific version |
| POST | `/api/v1/media/:media_id/versions/:version/restore` | Restore a previous version as current |
| DELETE | `/api/v1/media/:media_id/versions/:version` | Soft-delete a specific version |

**Configuration:**
Versioning reuses the existing media storage configuration. No versioning-specific environment variables.
| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_DRIVER` | local | Storage backend: local, s3, or minio |
| `STORAGE_LOCAL_PATH` | ./storage/uploads | Local storage base path |
| `STORAGE_BASE_URL` | http://localhost:8080/storage | Public URL for storage |
| `STORAGE_S3_BUCKET` | "" | S3/MinIO bucket name |
| `STORAGE_SIGNING_KEY` | (JWT secret fallback) | Key for generating signed URLs |

**Optimistic Locking:**
```go
// VersionService checks media.CurrentVersion before writing
// If CurrentVersion changed between read and write, 409 Conflict
if media.CurrentVersion != expectedVersion {
    return nil, ErrOptimisticLockFailed
}
```

**Key Design Decisions:**
- Retroactive v1 creation: If a media file has no versions yet, the first upload creates v1 automatically (no separate "create" endpoint).
- SHA-256 checksum deduplication: Before writing a new version, the service checks if the uploaded file's checksum matches any existing version of the same media. If so, returns 409 Conflict (no duplicate content storage).
- Optimistic locking: The `current_version` field on media prevents race conditions between concurrent uploads and restores.
- Restore = pointer update: Restoring a version only updates `media.current_version` to the restored version number. No new version record or file is created.
- Delete = soft-delete + physical removal: Deleting a version soft-deletes the database record and removes the physical file from storage. Version numbers are never reused.
- Current version cannot be deleted: The version matching `media.current_version` is protected from deletion.
- Handler-level permission checks: Permissions are enforced via middleware groups in the handler (not service-level Enforce calls). Each endpoint group has its own permission requirement (view, upload, download, restore, delete).

**Checksum Deduplication Pattern:**
```go
// Service: Check before upload
existingVersion, _ := s.versionRepo.FindVersionByChecksum(ctx, mediaID, checksum)
if existingVersion != nil {
    return nil, errors.NewAppError("CONFLICT", "Version with identical content already exists", 409)
}
```

---

## ACTIVITY FEED FEATURE

### 023-activity-feed: Activity Feed System

The activity feed system tracks user actions across the platform, providing a chronological timeline of events with per-user read tracking, resource following, and automatic archival via a background reaper.

**Architecture:**
```
Domain Services → EventBus.Publish(event) → ActivityService.SubscribeToEventBus()
                                                  → handleEvent()
                                                      → Map event to ActionType
                                                      → Extract actorID, resourceID, orgID
                                                      → Build metadata
                                                      → activityRepo.Create()
ActivityReaper.Start() → ticker → repo.ArchiveOlderThan(retentionDays)
```

**Key Components:**
- **Domain**: `internal/domain/activity.go` - Activity, ActivityRead, ActivityFollow entities + DTOs
- **Event Mapping**: `internal/domain/activity_events.go` - ActivityMapping registry mapping EventBus types to ActionType/ResourceType
- **Repository**: `internal/repository/activity.go` - 3 interfaces (ActivityRepository, ActivityReadRepository, ActivityFollowRepository) + GORM implementations
- **Service**: `internal/service/activity.go` - ActivityService: ListByOrganization, FindByID, CountUnread, MarkAllRead, MarkAsRead, SoftDelete, SubscribeToEventBus
- **Follow Service**: `internal/service/activity_follow.go` - Follow, Unfollow, ListFollows, IsFollowing
- **Reaper**: `internal/service/activity_reaper.go` - Background goroutine with configurable interval (default 60s), archives activities older than RetentionDays
- **Handler**: `internal/http/handler/activity.go` - 8 HTTP endpoints
- **Migration**: `migrations/000023_create_activities.up.sql` + `.down.sql`

**Event Mapping (EventBus → Activity):**
- `user.created` → ActionType: `created`, ResourceType: `user`
- `user.deleted` → ActionType: `deleted`, ResourceType: `user`
- `invoice.created` → ActionType: `created`, ResourceType: `invoice`
- `invoice.paid` → ActionType: `paid`, ResourceType: `invoice`
- `news.published` → ActionType: `published`, ResourceType: `news`
- `news.deleted` → ActionType: `deleted`, ResourceType: `news`

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/activities` | List activities (paginated, filtered) |
| GET | `/api/v1/activities/count-unread` | Count unread activities |
| PUT | `/api/v1/activities/read-all` | Mark all activities as read |
| PUT | `/api/v1/activities/:id/read` | Mark single activity as read |
| POST | `/api/v1/activities/follow` | Follow a resource |
| DELETE | `/api/v1/activities/follow/:id` | Unfollow a resource |
| GET | `/api/v1/activities/follows` | List followed resources |
| DELETE | `/api/v1/activities/:id` | Soft-delete activity (admin) |

**Configuration (Environment Variables):**
| Variable | Default | Description |
|----------|---------|-------------|
| `ACTIVITY_RETENTION_DAYS` | 90 | Days to retain activities before archival |
| `ACTIVITY_DEFAULT_PAGE_SIZE` | 20 | Default pagination page size |
| `ACTIVITY_MAX_PAGE_SIZE` | 100 | Maximum pagination page size |
| `ACTIVITY_REAPER_INTERVAL` | 60s | Interval for archival reaper goroutine |

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `activity:view` | activity | view | View activity feed and history |
| `activity:manage` | activity | manage | Delete activities (admin) |

**Startup Wiring:**
```go
// EventBus subscription (cmd/api/main.go)
activityService.SubscribeToEventBus(eventBus)

// Activity reaper (cmd/api/main.go)
activityReaper := service.NewActivityReaper(activityRepo, cfg.Activity, slog.Default())
activityReaper.Start(ctx)

// Graceful shutdown order:
// server → eventBus → activityReaper → workers → enforcer → db → redis
```

**Design Decisions:**
- `archived_at` for 90-day reaper archival (NOT GORM soft-delete); `deleted_at` reserved for admin DELETE
- Per-user read tracking via `activity_reads` table (NOT `read_at` on Activity)
- Per-user resource following via `activity_follows` (composite unique on user_id + resource_type + resource_id)
- Batch `is_read`/`is_following` computation — separate queries, merged in Go, no LEFT JOIN on main query
- Buffered channel (256) for EventBus subscription, matching WebhookService pattern
- `SetEventBus()` setter for post-construction injection
- Handler-level permission checks (soft delete requires `activity:manage`, all others `activity:view`)
- `resolveActivityOrgDomain(hasOrgID, orgID)` returns orgID.String() or "default"
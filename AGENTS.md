# Go API Base - Agent Instructions

**Generated:** 2026-04-24 | **Commit:** HEAD | **Branch:** 004-email-service

---

<!-- SPECKIT START -->
**Current Feature**: [specs/004-email-service/plan.md](specs/004-email-service/plan.md) - Email Service  
**Latest Feature**: [specs/005-log-writer/spec.md](specs/005-log-writer/spec.md) - Structured Logging System (✅ COMPLETE)
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

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `runServer` | func | `cmd/api/main.go:116` | Server init, graceful shutdown |
| `runSeed` | func | `cmd/api/main.go:428` | Seed roles/permissions |
| `NewTestSuite` | func | `tests/integration/testsuite.go:43` | Ephemeral Postgres+Redis |
| `Enforcer` | struct | `internal/permission/enforcer.go` | Casbin RBAC |
| `Server` | struct | `internal/http/server.go:28` | Echo + middleware chain |

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
4. **Graceful shutdown: 30s** - Order: server → enforcer → db → redis

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
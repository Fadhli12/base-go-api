# Go API Base - Agent Instructions

**Generated:** 2026-04-23 | **Commit:** HEAD | **Branch:** 002-api-key-auth

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

## FILES

- **Migrations**: `migrations/000001_init.up.sql`, `migrations/000002_audit_logs.up.sql`
- **Config**: `.env.example` → copy to `.env`
- **Constitution**: `.specify/memory/constitution.md`
- **Implementation**: `IMPLEMENTATION_PLAN.md` (10 phases)
- **Specs**: `specs/go-api-base/`
- **Config Docs**: `docs/configuration.md`
- **Current Feature**: `specs/go-api-base/plan.md` - Dynamic Configuration (storage, image, cache, swagger)

## CURRENT FEATURE

### Branch: `001-dynamic-config`

**Dynamic Configuration System** - Runtime-configurable subsystems via environment variables.

**Modules:**
- **Storage**: `internal/storage/` - Local, S3, MinIO backends
- **Image Compression**: `internal/conversion/config.go` - Configurable quality/dimensions
- **Cache**: `internal/cache/` - Redis, memory, none drivers
- **Swagger**: `internal/http/server.go` - Toggle Swagger UI

**Files Changed:**
- `internal/config/*.go` - Config structs for all subsystems
- `internal/storage/*.go` - Storage factory pattern
- `internal/cache/*.go` - Cache driver interface
- `internal/conversion/config.go` - Image compression config
- `.env.example` - All new config variables
- `docs/configuration.md` - Detailed configuration guide

**Default Behavior:**
- All configs match existing hardcoded values (backward compatible)
- Existing `.env` files work without modification
- No mandatory variables for local storage (uses defaults)
# Go API Base Project — Requirements Document

Production-ready stack specification for an Echo + GORM base with a granular, customizable RBAC/permission system.

---

## 1. Core Stack

| Layer | Choice | Rationale |
|---|---|---|
| Language | Go 1.22+ | Generics, `slog`, routing improvements |
| HTTP Framework | `labstack/echo/v4` | Fast, middleware-friendly, clean API |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` | Eloquent-like, hooks, soft deletes |
| Database | PostgreSQL 15+ | JSONB for permission metadata, RLS optional |
| Cache / Permission Store | Redis 7+ | Permission cache, rate limiting, sessions |
| Config | `spf13/viper` + `.env` via `godotenv` | Multi-source config, hot reload optional |
| Validation | `go-playground/validator/v10` | Struct tag validation, integrates with Echo |
| Auth | `golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt` | Stateless JWT + refresh token rotation |
| Permission Engine | `casbin/casbin/v2` + `casbin/gorm-adapter/v3` | RBAC + ABAC hybrid, policy reloadable |
| Migrations | `golang-migrate/migrate` (SQL files) | Versioned, CI-friendly, separate from GORM AutoMigrate |
| Logging | `log/slog` (stdlib) + `lmittmann/tint` for dev | Structured, zero-dep core |
| UUID | `google/uuid` | Primary keys |
| CLI | `spf13/cobra` | `serve`, `migrate`, `seed`, `permission:sync` commands |
| Testing | `stretchr/testify` + `testcontainers-go` | Real Postgres in tests |
| API Docs | `swaggo/swag` + `swaggo/echo-swagger` | Generate OpenAPI from annotations |

---

## 2. Supporting Libraries

- `go-redis/redis/v9` — Redis client
- `robfig/cron/v3` — scheduled jobs (token cleanup, audit log rotation)
- `hibiken/asynq` — background jobs (email, webhooks) backed by Redis
- `resty/resty/v2` — outbound HTTP client
- `samber/lo` — generics utilities (optional)

---

## 3. Project Structure

```
.
├── cmd/
│   └── api/main.go              # entrypoint, cobra root
├── internal/
│   ├── config/                  # viper loader
│   ├── database/                # gorm connection, redis
│   ├── http/
│   │   ├── server.go            # echo instance, middleware chain
│   │   ├── middleware/          # auth, permission, request_id, recover
│   │   ├── handler/             # HTTP handlers (thin)
│   │   ├── request/             # DTOs + validation
│   │   └── response/            # standardized envelope
│   ├── domain/                  # entities + interfaces (no framework deps)
│   ├── repository/              # gorm implementations
│   ├── service/                 # business logic
│   ├── permission/              # casbin enforcer, sync, cache
│   ├── auth/                    # jwt, password hashing, token service
│   └── module/                  # feature modules (user, role, permission, audit)
├── migrations/                  # *.up.sql / *.down.sql
├── pkg/                         # reusable, exportable
├── config/                      # rbac_model.conf, policies.csv
└── docs/                        # swagger output
```

---

## 4. Granular Permission System — Design

### 4.1 Model: Hybrid RBAC with Resource-Action Permissions

Use **Casbin** with an RBAC-with-domains model, backed by GORM. Permissions are stored as records (customizable at runtime), not hardcoded.

**Entities:**

```
User ──< UserRole >── Role ──< RolePermission >── Permission
                       │
                       └── inherits ──> Role (hierarchical, optional)

Permission: { id, name, resource, action, scope, description }
  e.g. resource="invoice", action="update", scope="own|team|all"

User can also have direct Permission overrides (grant/deny)
  via UserPermission { user_id, permission_id, effect: allow|deny }
```

### 4.2 Casbin Model (`config/rbac_model.conf`)

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[role_definition]
g = _, _

[policy_effect]
e = !some(where (p.eft == deny)) && some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
```

Key points:
- `eft` (effect) supports explicit **deny** overriding **allow** — essential for user-level overrides.
- `keyMatch2` enables wildcards like `invoice/:id`.

### 4.3 Permission Naming Convention

Use `resource:action` (e.g., `invoice:create`, `user:read`, `user:*`). Store as rows so admins can create new permissions through API without deploys:

```sql
CREATE TABLE permissions (
  id UUID PRIMARY KEY,
  name VARCHAR(150) UNIQUE NOT NULL,   -- "invoice:update"
  resource VARCHAR(100) NOT NULL,
  action VARCHAR(50) NOT NULL,
  description TEXT,
  is_system BOOLEAN DEFAULT false,     -- prevent deletion of core perms
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
);
```

### 4.4 Middleware Pattern

```go
// usage in routes
g := e.Group("/api/v1/invoices", mw.JWT(), mw.Permission("invoice:read"))
g.POST("", h.Create, mw.Permission("invoice:create"))
g.PUT("/:id", h.Update, mw.Permission("invoice:update"))
```

The `Permission` middleware calls `enforcer.Enforce(userID, resource, action)`. Cache results in Redis with a short TTL (30–60s) keyed by `perm:{user_id}:{resource}:{action}`, invalidated on role/permission changes via pub/sub.

### 4.5 Scope / Ownership Layer

Casbin handles *"can user do X on resource type"*. For row-level (*"can user edit this invoice"*), add a second check in the service layer:

```go
if !scope.Check(user, invoice) {
    return ErrForbidden
}
```

Keep Casbin for coarse permissions, use policy objects for ownership/tenant scoping. Don't force row-level into Casbin — it gets messy fast.

---

## 5. Required API Endpoints (admin)

```
POST   /api/v1/permissions                      # create custom permission
GET    /api/v1/permissions
POST   /api/v1/roles
PUT    /api/v1/roles/:id
POST   /api/v1/roles/:id/permissions            # attach
DELETE /api/v1/roles/:id/permissions/:pid
POST   /api/v1/users/:id/roles
POST   /api/v1/users/:id/permissions            # direct grant/deny override
GET    /api/v1/users/:id/effective-permissions  # computed union
```

---

## 6. Cross-Cutting Concerns

- **Request ID / Correlation ID** — echo middleware, propagate to logs and downstream calls
- **Audit Log** — separate table logging `actor_id`, `action`, `resource`, `resource_id`, `before/after` JSONB, IP, UA
- **Rate limiting** — `echo/middleware.RateLimiter` with Redis store
- **CORS** — configurable origins per environment
- **Graceful shutdown** — `context.WithTimeout` on `e.Shutdown`
- **Health checks** — `/healthz` (liveness), `/readyz` (checks DB + Redis)
- **Error handling** — custom `echo.HTTPErrorHandler` returning standardized envelope `{error: {code, message, details}}`
- **Soft delete + timestamps** — GORM `gorm.Model` or custom base struct with UUID

---

## 7. Gotchas to Plan For

1. **Casbin policy reload** — when a permission is added/revoked, call `enforcer.LoadPolicy()` AND invalidate Redis cache. Use Redis pub/sub so multiple API instances stay in sync.
2. **GORM AutoMigrate vs migrations** — don't use AutoMigrate in production. Use `golang-migrate` with SQL files committed to the repo. AutoMigrate silently drifts.
3. **JWT revocation** — stateless JWTs can't be revoked. Either use short-lived access tokens (5–15 min) + refresh tokens in DB, or maintain a Redis blacklist.
4. **N+1 on permission checks** — preload user roles + permissions on login, stash in JWT claims OR fetch once per request and cache on `echo.Context`.
5. **System roles** — seed `super_admin`, `admin` as `is_system=true` and prevent deletion.
6. **Permission naming drift** — write a `permission:sync` cobra command that reads a manifest file and upserts known permissions on deploy.

---

## 8. Suggested Development Order

1. Config + DB + Redis wiring + graceful shutdown
2. Base migrations (users, roles, permissions, pivots, audit_logs)
3. Auth module (register, login, refresh, password hashing)
4. JWT middleware + request context (`ctx.Get("user")`)
5. Casbin integration + permission middleware
6. Role/Permission CRUD endpoints
7. Audit log middleware
8. Swagger docs + Postman collection
9. Example domain module (e.g., invoices) demonstrating the full pattern
10. Docker Compose (api + postgres + redis) + Makefile

---

## 9. Initial `go.mod` Dependencies

```go
require (
    github.com/labstack/echo/v4              v4.12.0
    gorm.io/gorm                             v1.25.10
    gorm.io/driver/postgres                  v1.5.9
    github.com/casbin/casbin/v2              v2.98.0
    github.com/casbin/gorm-adapter/v3        v3.25.0
    github.com/golang-jwt/jwt/v5             v5.2.1
    github.com/go-playground/validator/v10   v10.22.0
    github.com/spf13/viper                   v1.19.0
    github.com/spf13/cobra                   v1.8.1
    github.com/google/uuid                   v1.6.0
    github.com/redis/go-redis/v9             v9.6.1
    github.com/golang-migrate/migrate/v4     v4.17.1
    golang.org/x/crypto                      v0.26.0
    github.com/swaggo/echo-swagger           v1.4.1
    github.com/stretchr/testify              v1.9.0
)
```

# Implementation Plan: Go API Base

**Branch**: `feature/go-api-base` | **Date**: 2026-04-22 | **Spec**: `go-api-base-requirements.md`

**Input**: Requirements specification from `go-api-base-requirements.md`

---

## Summary

Build a production-ready Echo + GORM REST API base with:
- **Hybrid RBAC/permission system** using Casbin (RBAC-with-domains) backed by GORM
- **JWT authentication** with stateless tokens, refresh rotation, and revocation via Redis
- **Permission middleware** with Redis caching (30-60s TTL), pub/sub invalidation
- **Audit logging** with actor tracking, before/after JSONB, IP/UA capture
- **Cross-cutting concerns**: graceful shutdown, health checks, CORS, rate limiting, correlation IDs
- **Example domain module** (invoices) demonstrating full auth + permission + scope checking pattern

**Approach**: Follow 10-phase development order per requirements §8, starting with infra (config/DB/Redis) and building toward feature modules.

---

## Technical Context

| Item | Value |
|------|-------|
| **Language/Version** | Go 1.22+ (generics, slog, routing improvements) |
| **HTTP Framework** | `labstack/echo/v4` (fast, middleware-friendly) |
| **ORM** | `gorm.io/gorm` + `gorm.io/driver/postgres` (Eloquent-like, hooks, soft deletes) |
| **Database** | PostgreSQL 15+ (JSONB for permission metadata, RLS optional) |
| **Cache / Permission Store** | Redis 7+ (permission cache, rate limiting, sessions, pub/sub) |
| **Config** | `spf13/viper` + `.env` via `godotenv` (multi-source, hot reload) |
| **Validation** | `go-playground/validator/v10` (struct tag, Echo integration) |
| **Auth** | `golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt` (JWT + refresh rotation) |
| **Permission Engine** | `casbin/casbin/v2` + `casbin/gorm-adapter/v3` (RBAC + ABAC, reloadable) |
| **Migrations** | `golang-migrate/migrate` (SQL files, CI-friendly, separate from AutoMigrate) |
| **Logging** | `log/slog` (stdlib) + `lmittmann/tint` for dev (structured, zero-dep core) |
| **UUID** | `google/uuid` (primary keys) |
| **CLI** | `spf13/cobra` (serve, migrate, seed, permission:sync commands) |
| **Testing** | `stretchr/testify` + `testcontainers-go` (real Postgres in tests) |
| **API Docs** | `swaggo/swag` + `swaggo/echo-swagger` (OpenAPI annotations) |
| **Project Type** | REST API / Library base (reusable foundation) |
| **Target Platform** | Linux / Docker (containerized production deployment) |
| **Performance Goals** | Sub-100ms p99 on auth/permission checks; 1000+ req/s capacity |
| **Constraints** | JWT revocation < 15s via Redis; permission cache invalidation via pub/sub; soft deletes for audit trail |
| **Scale/Scope** | Foundation for multi-tenant or single-tenant APIs; 50+ tables with audit; 20+ core endpoints |

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**[PENDING: Constitution.md to be filled by project team]**

Anticipated constraints:
- ✅ **RBAC + Casbin mandatory** — permission system must support runtime customization (no hardcoded roles)
- ✅ **JWT + Redis revocation** — stateless tokens with short-lived access (5–15 min) + refresh rotation
- ✅ **Audit logging non-negotiable** — all user actions tracked with before/after snapshots
- ✅ **Soft deletes required** — preserve audit trail, no hard deletions of users/roles
- ✅ **PostgreSQL + migrations (not AutoMigrate)** — versioned SQL migrations checked into repo
- ✅ **Permission cache invalidation via pub/sub** — multi-instance API consistency

---

## Project Structure

### Documentation (this feature)

```text
.specify/
├── memory/
│   └── constitution.md           # Project principles (TBD)
└── ...existing files...

specs/go-api-base/                # Planning artifacts (if using Speckit specs)
├── plan.md                       # This file
├── research.md                   # Phase 0 output
├── data-model.md                 # Phase 1 output
├── contracts/                    # Phase 1 output (HTTP API contracts)
└── tasks.md                      # Phase 2 output
```

### Source Code (repository root)

```text
.
├── cmd/
│   └── api/
│       └── main.go               # entrypoint, cobra root
├── internal/
│   ├── config/                   # viper loader, env parsing
│   ├── database/
│   │   ├── postgres.go           # gorm connection
│   │   └── redis.go              # redis client, pub/sub
│   ├── http/
│   │   ├── server.go             # echo instance, middleware chain
│   │   ├── middleware/
│   │   │   ├── jwt.go            # JWT auth extraction
│   │   │   ├── permission.go      # Casbin enforce + Redis cache
│   │   │   ├── request_id.go      # correlation ID propagation
│   │   │   ├── audit.go           # audit log middleware
│   │   │   └── recover.go         # panic recovery
│   │   ├── handler/
│   │   │   ├── auth.go           # register, login, refresh, logout
│   │   │   ├── role.go           # role CRUD
│   │   │   ├── permission.go     # permission CRUD
│   │   │   ├── user.go           # user effective-permissions
│   │   │   └── health.go         # /healthz, /readyz
│   │   ├── request/
│   │   │   ├── auth.go           # LoginReq, RegisterReq, RefreshReq
│   │   │   ├── role.go           # CreateRoleReq, etc.
│   │   │   └── validation.go     # struct tag validation
│   │   └── response/
│   │       ├── envelope.go       # {error: {code, message, details}}
│   │       └── types.go          # DTO types
│   ├── domain/
│   │   ├── user.go               # User entity, interfaces
│   │   ├── role.go               # Role entity
│   │   ├── permission.go         # Permission entity
│   │   └── audit_log.go          # AuditLog entity
│   ├── repository/
│   │   ├── user.go               # GORM user queries
│   │   ├── role.go               # GORM role queries
│   │   ├── permission.go         # GORM permission queries
│   │   └── audit_log.go          # GORM audit log queries
│   ├── service/
│   │   ├── auth.go               # Login, register, token refresh
│   │   ├── role.go               # Role business logic
│   │   ├── permission.go         # Permission logic + sync
│   │   ├── user.go               # User permission computation
│   │   └── audit.go              # Audit log recording
│   ├── permission/
│   │   ├── enforcer.go           # Casbin enforcer setup + reload
│   │   ├── cache.go              # Redis permission cache layer
│   │   ├── invalidator.go        # Pub/sub invalidation handler
│   │   └── sync.go               # permission:sync CLI logic
│   ├── auth/
│   │   ├── jwt.go                # Token generation, claims
│   │   ├── password.go           # bcrypt hashing
│   │   └── token_service.go      # Refresh token DB management
│   └── module/
│       ├── invoice/              # Example domain module
│       │   ├── handler.go        # CRUD handlers
│       │   ├── service.go        # Business logic + scope check
│       │   ├── repository.go     # GORM queries
│       │   └── entity.go         # Invoice domain entity
│       └── ...other modules...
├── migrations/
│   ├── 000001_init.up.sql        # Base schema (users, roles, permissions, pivots)
│   ├── 000001_init.down.sql
│   ├── 000002_audit_logs.up.sql
│   ├── 000002_audit_logs.down.sql
│   └── ...numbered migrations...
├── config/
│   ├── rbac_model.conf           # Casbin RBAC model definition
│   ├── .env.example              # Sample environment variables
│   └── ...other config files...
├── pkg/
│   ├── utils/                    # Reusable, exportable utilities
│   ├── errors/                   # Custom error types
│   └── logger/                   # Logging helpers
├── tests/
│   ├── unit/                     # Unit tests (handlers, services)
│   ├── integration/              # Integration tests (DB + API)
│   └── contract/                 # Contract tests (HTTP API)
├── docs/
│   └── swagger/                  # Generated OpenAPI (swaggo output)
├── Dockerfile                    # Container image
├── docker-compose.yml            # API + Postgres + Redis local dev
├── Makefile                      # Build, migrate, test, serve
├── go.mod                        # Go modules
├── go.sum
└── README.md                     # Setup, running, API docs
```

**Structure Decision**: Single monolithic project (Go API base). All layers (HTTP, domain, repository, permission, auth) colocated in `internal/` with clean separation of concerns.

---

## Complexity Tracking

> **No violations anticipated at this stage.** Constitution to be ratified will enforce:
> - RBAC + Casbin mandatory
> - Soft deletes required
> - PostgreSQL migrations (not AutoMigrate)
> - Audit logging non-negotiable

---

## Phase 0: Research & Clarifications

**Goals**: Resolve NEEDS CLARIFICATION items, establish best practices for chosen tech stack.

### Research Tasks

1. **Casbin GORM adapter best practices**
   - Query: How to properly initialize Casbin with GORM adapter for PostgreSQL
   - Output: `research.md` with initialization pattern, policy reload mechanics, common pitfalls

2. **JWT refresh token rotation patterns in Go**
   - Query: Stateless JWT + refresh token DB storage, revocation strategies
   - Output: Token lifecycle diagram, refresh endpoint spec, blacklist vs. DB approach

3. **Redis pub/sub for permission invalidation across API instances**
   - Query: Multi-instance API consistency; permission cache sync via Redis pub/sub
   - Output: Pub/sub channel design, invalidation timing, message payload

4. **PostgreSQL soft deletes + audit trail strategy**
   - Query: GORM soft deletes (deleted_at), before/after JSONB logging
   - Output: Schema pattern, migration strategy, audit table design

5. **Testcontainers-go for PostgreSQL integration tests**
   - Query: Real Postgres in test containers; test setup/teardown
   - Output: Test harness pattern, migration strategy in tests

### Deliverable

**`specs/go-api-base/research.md`** with resolved clarifications:
- Decision: [what chosen]
- Rationale: [why]
- Alternatives: [what else evaluated]

---

## Phase 1: Design & Contracts

**Prerequisites**: `research.md` complete

### 1.1 Data Model (`data-model.md`)

Extract entities, fields, relationships, validation rules:

- **users** — id (UUID), email, password_hash, created_at, updated_at, deleted_at
- **roles** — id (UUID), name, is_system (bool), description, created_at, updated_at, deleted_at
- **permissions** — id (UUID), name, resource, action, scope, is_system, description, created_at, updated_at, deleted_at
- **user_roles** — user_id FK, role_id FK, created_at (pivot)
- **role_permissions** — role_id FK, permission_id FK, created_at (pivot)
- **user_permissions** — user_id FK, permission_id FK, effect (allow|deny), created_at (direct overrides)
- **audit_logs** — id (UUID), actor_id FK, action, resource, resource_id, before (JSONB), after (JSONB), ip_address, user_agent, created_at
- **refresh_tokens** — id (UUID), user_id FK, token_hash, expires_at, revoked_at, created_at
- **invoices** (example domain) — id (UUID), user_id FK, amount, status, created_at, updated_at, deleted_at

**Relationships**:
```
User --< UserRole >-- Role --< RolePermission >-- Permission
User --< UserPermission (direct override) >-- Permission
User --< RefreshToken >-- (1 revocation store)
User --< AuditLog (actor_id)
User --< Invoice (ownership for scope check)
```

### 1.2 HTTP Contracts (`contracts/`)

Define API endpoints, request/response envelopes, error codes:

**Contract file**: `contracts/api-v1.md`

#### Auth Endpoints
```
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
```

#### Permission/Role Admin Endpoints
```
POST   /api/v1/permissions
GET    /api/v1/permissions
POST   /api/v1/roles
PUT    /api/v1/roles/:id
POST   /api/v1/roles/:id/permissions
DELETE /api/v1/roles/:id/permissions/:pid
POST   /api/v1/users/:id/roles
POST   /api/v1/users/:id/permissions
GET    /api/v1/users/:id/effective-permissions
```

#### Health/Readiness
```
GET    /healthz              # liveness (always 200 if server running)
GET    /readyz               # readiness (checks DB + Redis)
```

#### Example Domain (Invoices)
```
GET    /api/v1/invoices
POST   /api/v1/invoices
GET    /api/v1/invoices/:id
PUT    /api/v1/invoices/:id
DELETE /api/v1/invoices/:id
```

**Response Envelope**:
```json
{
  "data": { /* payload */ },
  "error": null,
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Error Envelope**:
```json
{
  "data": null,
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "User does not have permission",
    "details": { /* optional context */ }
  }
}
```

### 1.3 Quickstart (`quickstart.md`)

Step-by-step setup for local development:
1. Clone repo
2. `cp config/.env.example config/.env`
3. `docker-compose up -d` (starts Postgres + Redis)
4. `make migrate` (runs migrations)
5. `make seed` (seeds system roles + permissions)
6. `make serve` (starts API on :8080)
7. Example: `curl -X POST http://localhost:8080/api/v1/auth/register ...`

### 1.4 Agent Context Update

Update `AGENTS.md` SPECKIT markers to point to the plan:

```markdown
<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
specs/go-api-base/plan.md
<!-- SPECKIT END -->
```

---

## Phase 2: Implementation Phases (10 phases per requirements §8)

*Note: Phase 2 is NOT executed by `/speckit.plan`. Use `/speckit.tasks` to generate tasks.md with detailed implementation steps.*

### Suggested Order (from requirements §8)

1. **Config + DB + Redis wiring + graceful shutdown**
   - Viper config loader
   - PostgreSQL connection + GORM setup
   - Redis client
   - Echo server with graceful shutdown on SIGTERM/SIGINT

2. **Base migrations** (users, roles, permissions, pivots, audit_logs)
   - SQL migration files
   - Verify schema with `golang-migrate`

3. **Auth module** (register, login, refresh, password hashing)
   - Register endpoint + validation
   - Login endpoint + password verification
   - Refresh token rotation + DB storage

4. **JWT middleware + request context**
   - JWT extraction from Authorization header
   - Token validation + claims parsing
   - Store user in `ctx.Get("user")`

5. **Casbin integration + permission middleware**
   - Casbin enforcer init with GORM adapter
   - RBAC model config
   - Permission middleware + Redis cache

6. **Role/Permission CRUD endpoints**
   - Create/read permission, role, user_roles, role_permissions

7. **Audit log middleware**
   - Capture action, resource, before/after, IP, UA
   - Store in audit_logs table

8. **Swagger docs + Postman collection**
   - Swaggo annotations on handlers
   - Generate OpenAPI
   - Export Postman collection

9. **Example domain module** (invoices)
   - Full CRUD with auth + permission + scope checks
   - Demonstrates ownership-based authorization

10. **Docker + Makefile**
    - Dockerfile for containerization
    - docker-compose.yml for local dev
    - Makefile for common commands

---

## Key Decisions & Rationale

| Decision | Why | Alternative Considered |
|----------|-----|------------------------|
| **Casbin + GORM adapter** | Runtime permission customization without deploys; standard RBAC-with-domains model | In-memory permission checks (no runtime updates) |
| **JWT + refresh tokens in DB** | Revocation possible; short-lived access tokens reduce exposure | Pure stateless JWT (cannot revoke) |
| **Redis pub/sub for cache invalidation** | Multi-instance API consistency; sub-second invalidation | Polling DB (30s+ lag); eventual consistency |
| **PostgreSQL soft deletes** | Audit trail preserved; compliance; undo capability | Hard deletes (audit loss) |
| **Migrations separate from GORM AutoMigrate** | Version control, reproducible CI/CD, no silent schema drift | AutoMigrate (unpredictable schema changes) |
| **Scope checks in service layer** | Casbin for coarse perms (can user do X); service for row-level (can user edit this row) | All checks in Casbin (complex, messy policies) |
| **Structured logging (slog)** | Zero-dependency, stdlib, structured fields | Printf-style logging (unstructured) |

---

## Gotchas & Mitigations

| Gotcha | Mitigation |
|--------|-----------|
| **Casbin policy reload lag** | Use Redis pub/sub to trigger `enforcer.LoadPolicy()` on all instances when permission added/revoked |
| **GORM AutoMigrate schema drift** | Only use `golang-migrate` with versioned SQL files; disable AutoMigrate in production |
| **JWT revocation impossible** | Short-lived access tokens (5–15 min); refresh tokens in DB with revocation flag; optional Redis blacklist |
| **N+1 permission checks** | Cache permission check result per request (echo.Context); preload user roles on login; Redis cache with TTL |
| **System role deletion** | Mark core roles `is_system=true`; prevent deletion in handler logic |
| **Permission naming drift** | Write `permission:sync` CLI command; reads manifest, upserts known permissions on deploy |

---

## Gates & Success Criteria

### Phase 0 Gate (before research)
- [ ] Constitution.md filled in (project team)
- [ ] NEEDS CLARIFICATION items identified

### Phase 0 Gate (after research)
- [ ] research.md completed with all clarifications resolved
- [ ] Casbin + JWT + Redis patterns documented

### Phase 1 Gate (before design)
- [ ] Research findings approved

### Phase 1 Gate (after design)
- [ ] data-model.md entity diagram finalized
- [ ] contracts/api-v1.md endpoint specs approved
- [ ] quickstart.md validated locally
- [ ] Constitution check re-run (no new violations)

### Phase 2 Gate (before implementation)
- [ ] Design documents approved
- [ ] tasks.md generated with detailed work items
- [ ] Team assigned

---

## Next Steps

1. **Fill Constitution.md** — project team defines principles, constraints, quality gates
2. **Run Phase 0 Research** — use `/speckit.plan` to dispatch research agents
3. **Generate tasks.md** — use `/speckit.tasks` to create detailed Phase 2 task breakdown
4. **Assign Phase 1 work** — design + contract review
5. **Begin Phase 2 implementation** — follow 10-phase order

---

**Version**: 1.0 | **Created**: 2026-04-22 | **Status**: Design Phase 0

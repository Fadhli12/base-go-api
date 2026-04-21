# Tasks: Go API Base

**Input**: Design documents from `specs/go-api-base/`
**Prerequisites**: `IMPLEMENTATION_PLAN.md`, `data-model.md`, `contracts/api-v1.md`, `research.md`, `quickstart.md`

**Tests**: Integration tests included per constitution (80% coverage required).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, etc.)
- Include exact file paths in descriptions

## Path Conventions

- Backend API located at repository root: `cmd/`, `internal/`, `migrations/`, `config/`
- Tests: `tests/unit/`, `tests/integration/`, `tests/contract/`
- Docker config at repository root

---

## Phase 1: Setup (Project Infrastructure)

**Purpose**: Initialize Go project, dependencies, and development tooling

- [ ] T001 Initialize Go module with go.mod at repository root
- [ ] T002 Create directory structure per IMPLEMENTATION_PLAN.md (cmd/, internal/, migrations/, config/, pkg/, tests/, docs/)
- [ ] T003 [P] Create Makefile with common commands (migrate, seed, serve, test, lint, docker-build)
- [ ] T004 [P] Create docker-compose.yml for local development (Postgres 15 + Redis 7 + API)
- [ ] T005 [P] Create .env.example in config/ with default environment variables
- [ ] T006 [P] Create .gitignore for Go project (excluding .env, bin/, vendor/)
- [ ] T007 Initialize golangci-lint configuration in .golangci.yml

**Checkpoint**: Project structure ready, dependencies installable, Docker services runnable

---

## Phase 2: Foundational (Core Infrastructure)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Configuration & Database

- [ ] T008 Implement Viper config loader in internal/config/config.go (reads .env + yaml)
- [ ] T009 [P] Implement PostgreSQL connection with GORM in internal/database/postgres.go
- [ ] T010 [P] Implement Redis connection in internal/database/redis.go
- [ ] T011 Implement graceful shutdown handler in cmd/api/main.go (context.WithTimeout on SIGTERM)
- [ ] T012 Create base migration 000001_init.up.sql in migrations/ (users, roles, permissions tables)
- [ ] T013 Create base migration 000001_init.down.sql in migrations/ (rollback schema)
- [ ] T014 Create migration 000002_audit_logs.up.sql in migrations/ (audit_logs table)
- [ ] T015 [P] Create migration 000002_audit_logs.down.sql in migrations/
- [ ] T016 Implement golang-migrate runner in internal/database/migrate.go (Up/Down/Version)

### Echo Server & Middleware Skeleton

- [ ] T017 Implement Echo server setup in internal/http/server.go (routes, middleware chain)
- [ ] T018 [P] Implement request ID middleware in internal/http/middleware/request_id.go (correlation ID)
- [ ] T019 [P] Implement panic recovery middleware in internal/http/middleware/recover.go
- [ ] T020 [P] Implement CORS middleware in internal/http/middleware/cors.go (configurable origins)
- [ ] T021 [P] Implement rate limiting middleware in internal/http/middleware/rate_limit.go (Redis-backed)
- [ ] T022 Create standardized response envelope in internal/http/response/envelope.go
- [ ] T023 Create custom error types in pkg/errors/errors.go (ErrNotFound, ErrUnauthorized, etc.)
- [ ] T024 Implement global error handler in internal/http/server.go (HTTPErrorHandler mapping to envelope)

### Health Checks

- [ ] T025 [P] Implement /healthz endpoint in internal/http/handler/health.go (liveness probe)
- [ ] T026 [P] Implement /readyz endpoint in internal/http/handler/health.go (checks DB + Redis)

### Logging

- [ ] T027 Implement structured logger with slog in pkg/logger/logger.go (tint for dev)
- [ ] T028 Add logging middleware in internal/http/middleware/logging.go (request/response logging)

**Checkpoint**: Foundation ready - API server starts, health checks pass, DB + Redis connected

---

## Phase 3: User Story 1 - User Registration & Login (Priority: P1) 🎯 MVP

**Goal**: Users can register accounts and login to obtain JWT tokens

**Independent Test**: Register new user via POST /auth/register, login via POST /auth/login, verify tokens returned

### Tests for User Story 1

- [ ] T029 [P] [US1] Integration test for user registration in tests/integration/auth_register_test.go
- [ ] T030 [P] [US1] Integration test for user login in tests/integration/auth_login_test.go

### Domain Models

- [ ] T031 [P] [US1] Create User entity in internal/domain/user.go (BaseModel, email, password_hash)
- [ ] T032 [P] [US1] Create RefreshToken entity in internal/domain/refresh_token.go (id, user_id, token_hash, expires_at, revoked_at)

### Repository Layer

- [ ] T033 [US1] Implement UserRepository in internal/repository/user.go (Create, FindByEmail, FindByID)
- [ ] T034 [US1] Implement RefreshTokenRepository in internal/repository/refresh_token.go (Create, FindByHash, MarkRevoked, RevokeAllByUser)

### Service Layer

- [ ] T035 [US1] Implement password hashing with bcrypt in internal/auth/password.go (Hash, Verify)
- [ ] T036 [US1] Implement JWT token generation in internal/auth/jwt.go (GenerateAccessToken, ParseToken, GetClaims)
- [ ] T037 [US1] Implement TokenService in internal/auth/token_service.go (GenerateRefreshToken, ValidateRefreshToken, RotateRefreshToken)
- [ ] T038 [US1] Implement AuthService in internal/service/auth.go (Register, Login, Logout)

### HTTP Layer

- [ ] T039 [US1] Create RegisterRequest DTO in internal/http/request/auth.go (validation: email format, password complexity)
- [ ] T040 [US1] Create LoginRequest DTO in internal/http/request/auth.go
- [ ] T041 [US1] Implement AuthHandler in internal/http/handler/auth.go (Register, Login endpoints)
- [ ] T042 [US1] Register auth routes in internal/http/server.go (POST /api/v1/auth/register, POST /api/v1/auth/login)

### Cobra CLI

- [ ] T043 [US1] Create serve command in cmd/api/main.go (starts Echo server)
- [ ] T044 [US1] Create migrate command in cmd/api/main.go (runs golang-migrate up/down)
- [ ] T045 [US1] Create seed command in cmd/api/main.go (seeds SuperAdmin, Admin, Viewer roles + permissions)

**Checkpoint**: Users can register and login, receiving JWT access tokens and refresh tokens stored in DB

---

## Phase 4: User Story 2 - Token Refresh & Logout (Priority: P1)

**Goal**: Users can refresh expired access tokens and logout (revoke refresh tokens)

**Independent Test**: After login, wait 15 min (or use short expiry), call POST /auth/refresh with refresh token, verify new tokens returned. Logout and verify refresh token revoked.

### Tests for User Story 2

- [ ] T046 [P] [US2] Integration test for token refresh in tests/integration/auth_refresh_test.go
- [ ] T047 [P] [US2] Integration test for logout in tests/integration/auth_logout_test.go

### Service Layer

- [ ] T048 [US2] Extend AuthService in internal/service/auth.go with Refresh method (validates refresh token, issues new access + refresh)
- [ ] T049 [US2] Extend AuthService in internal/service/auth.go with Logout method (revokes all user's refresh tokens)

### HTTP Layer

- [ ] T050 [US2] Create RefreshRequest DTO in internal/http/request/auth.go (refresh_token field)
- [ ] T051 [US2] Extend AuthHandler in internal/http/handler/auth.go (Refresh endpoint)
- [ ] T052 [US2] Extend AuthHandler in internal/http/handler/auth.go (Logout endpoint)
- [ ] T053 [US2] Register refresh and logout routes in internal/http/server.go (POST /api/v1/auth/refresh, POST /api/v1/auth/logout)

**Checkpoint**: Users can refresh tokens and logout (revoking refresh tokens in DB)

---

## Phase 5: User Story 3 - JWT Authentication Middleware (Priority: P1)

**Goal**: Protected endpoints require valid JWT token, user context propagated to handlers

**Independent Test**: Access protected endpoint without token → 401. With valid token → 200. With expired token → 401.

### Tests for User Story 3

- [ ] T054 [P] [US3] Integration test for JWT middleware in tests/integration/middleware_jwt_test.go

### Middleware

- [ ] T055 [US3] Implement JWT middleware in internal/http/middleware/jwt.go (extracts Bearer token, validates, sets ctx.Get("user"))
- [ ]  T056 [US3] Add user context helpers in internal/http/context.go (GetUserEmail, GetUserID from echo.Context)

### Service Layer

- [ ] T057 [US3] Implement UserService in internal/service/user.go (FindByID, GetEffectivePermissions)

### HTTP Layer

- [ ] T058 [US3] Implement me endpoint in internal/http/handler/user.go (GET /api/v1/me - returns current user info)
- [ ] T059 [US3] Register protected route group in internal/http/server.go (uses JWT middleware)

**Checkpoint**: Protected endpoints require JWT, user identity available in handlers

---

## Phase 6: User Story 4 - Role & Permission Management (Priority: P2)

**Goal**: Admins can create roles, permissions, and assign permissions to roles (runtime RBAC)

**Independent Test**: Login as SuperAdmin, create new permission via POST /permissions, create role via POST /roles, attach permission to role via POST /roles/:id/permissions

### Tests for User Story 4

- [ ] T060 [P] [US4] Integration test for permission CRUD in tests/integration/permission_test.go
- [ ] T061 [P] [US4] Integration test for role CRUD in tests/integration/role_test.go
- [ ] T062 [P] [US4] Integration test for role-permission assignment in tests/integration/role_permission_test.go

### Domain Models

- [ ] T063 [P] [US4] Create Permission entity in internal/domain/permission.go (id, name, resource, action, scope, is_system)
- [ ] T064 [P] [US4] Create Role entity in internal/domain/role.go (id, name, description, is_system)
- [ ] T065 [P] [US4] Create UserRole entity in internal/domain/user_role.go (user_id, role_id pivot)
- [ ] T066 [P] [US4] Create RolePermission entity in internal/domain/role_permission.go (role_id, permission_id pivot)

### Repository Layer

- [ ] T067 [US4] Implement PermissionRepository in internal/repository/permission.go (Create, FindAll, FindByID, FindByResource)
- [ ] T068 [US4] Implement RoleRepository in internal/repository/role.go (Create, FindByID, FindAll, Update, SoftDelete)
- [ ] T069 [US4] Implement RolePermissionRepository in internal/repository/role_permission.go (Attach, Detach, FindByRoleID)

### Service Layer

- [ ] T070 [US4] Implement PermissionService in internal/service/permission.go (Create, GetAll)
- [ ] T071 [US4] Implement RoleService in internal/service/role.go (Create, Update, AttachPermission, DetachPermission)

### HTTP Layer

- [ ] T072 [US4] Create CreatePermissionRequest DTO in internal/http/request/permission.go
- [ ] T073 [US4] Create CreateRoleRequest DTO in internal/http/request/role.go
- [ ] T075 [US4] Implement PermissionHandler in internal/http/handler/permission.go (Create, GetAll)
- [ ] T075a [US4] Extend PermissionHandler to support scope parameter in POST /permissions
- [ ] T076 [US4] Register permission routes in internal/http/server.go (POST/GET /api/v1/permissions)
- [ ] T077 [US4] Implement RoleHandler in internal/http/handler/role.go (Create, Update, AttachPermission, DetachPermission)
- [ ] T077a [US4] Implement RoleHandler.GetAll in internal/http/handler/role.go (GET /api/v1/roles with pagination)
- [ ] T077b [US4] Implement RoleHandler.SoftDelete in internal/http/handler/role.go (DELETE /api/v1/roles/:id, checks is_system)
- [ ] T078 [US4] Register role routes in internal/http/server.go (POST/GET/PUT/DELETE /api/v1/roles, POST/DELETE /api/v1/roles/:id/permissions/:pid)

**Checkpoint**: Admins can create permissions and roles dynamically via API, list roles, delete non-system roles

---

## Phase 7: User Story 5 - User Role Assignment & Override (Priority: P2)

**Goal**: Admins can assign roles to users, grant/deny permissions directly (overrides)

**Independent Test**: Assign role to user via POST /users/:id/roles, check effective permissions via GET /users/:id/effective-permissions

### Tests for User Story 5

- [ ] T078 [P] [US5] Integration test for user role assignment in tests/integration/user_role_test.go
- [ ] T079 [P] [US5] Integration test for user permission overrides in tests/integration/user_permission_test.go
- [ ] T080 [P] [US5] Integration test for effective permissions calculation in tests/integration/effective_permissions_test.go

### Domain Models

- [ ] T081 [US5] CreateUserPermission entity in internal/domain/user_permission.go (user_id, permission_id, effect allow|deny)

### Repository Layer

- [ ] T082 [US5] Implement UserRoleRepository in internal/repository/user_role.go (Assign, Remove, FindByUserID)
- [ ] T083 [US5] Implement UserPermissionRepository in internal/repository/user_permission.go (Grant, Deny, FindByUserID)

### Service Layer

- [ ] T084 [US5] Extend UserService in internal/service/user.go (AssignRole, GrantPermission, DenyPermission, GetEffectivePermissions)

### HTTP Layer

- [ ] T085 [US5] CreateUserRoleRequest DTO in internal/http/request/user.go (role_id field)
- [ ] T086 [US5] CreateUserPermissionRequest DTO in internal/http/request/user.go (permission_id, effect fields)
- [ ] T087 [US5] Extend UserHandler in internal/http/handler/user.go (AssignRole, RemoveRole, GrantPermission, RemovePermission, GetEffectivePermissions)
- [ ] T088 [US5] Register user admin routes in internal/http/server.go (POST/DELETE /api/v1/users/:id/roles, POST/DELETE /api/v1/users/:id/permissions, GET /api/v1/users/:id/effective-permissions)
- [ ] T089 [US5] Implement UserHandler.GetUserByID in internal/http/handler/user.go (GET /api/v1/users/:id)
- [ ] T090 [US5] Implement UserHandler.ListUsers in internal/http/handler/user.go (GET /api/v1/users with pagination)
- [ ] T091 [US5] Implement UserHandler.SoftDelete in internal/http/handler/user.go (DELETE /api/v1/users/:id)
- [ ] T092 [US5] Implement UserHandler.GetCurrentUser in internal/http/handler/user.go (GET /api/v1/me)
- [ ] T093 [US5] Register user CRUD routes in internal/http/server.go (GET /api/v1/users, GET /api/v1/users/:id, DELETE /api/v1/users/:id, GET /api/v1/me)

**Checkpoint**: Admins can assign roles and permissions to users, effective permissions computed correctly, full user CRUD available

---

## Phase 8: User Story 6 - Permission Enforcement (Priority: P2)

**Goal**: Endpoints require specific permissions, Casbin enforces RBAC + Redis cache for performance

**Independent Test**: User without invoice:create permission → POST /invoices returns 403. After granting permission → 201.

### Tests for User Story 6

- [ ] T089 [P] [US6] Integration test for permission enforcement in tests/integration/permission_enforcement_test.go

### Permission Engine

- [ ] T090 [US6] Initialize Casbin enforcer with GORM adapter in internal/permission/enforcer.go (NewEnforcer, LoadPolicy)
- [ ] T091 [US6] Implement permission cache layer in internal/permission/cache.go (Get, Set with TTL, InvalidateAll)
- [ ] T092 [US6] Implement EnforceWithCache in internal/permission/enforcer.go (checks Redis, falls back to Casbin)
- [ ] T093 [US6] Implement policy reload + cache invalidation in internal/permission/invalidator.go

### Middleware

- [ ] T094 [US6] Implement Permission middleware in internal/http/middleware/permission.go (calls EnforceWithCache)
- [ ] T095 [US6] Register permission middleware on protected routes in internal/http/server.go

### Redis Pub/Sub

- [ ] T096 [US6] Implement Redis pub/sub invalidation in internal/permission/invalidator.go (PublishInvalidate, SubscribeInvalidation)
- [ ] T097 [US6] Start invalidation listener on API startup in cmd/api/main.go (background goroutine)

### Cobra CLI

- [ ] T098 [US6] Implement permission:sync command in cmd/api/main.go (reads manifest, upserts known permissions)

**Checkpoint**: Permission enforcement working, cache invalidation via pub/sub operational

---

## Phase 9: User Story 7 - Audit Logging (Priority: P2)

**Goal**: All user actions logged with actor, action, resource, before/after snapshots, IP, User-Agent

**Independent Test**: Create invoice, check audit_logs table for row with action=create, resource=invoice, after=JSONB with invoice data

### Tests for User Story 7

- [ ] T099 [P] [US7] Integration test for audit logging in tests/integration/audit_log_test.go

### Domain Models

- [ ] T100 [US7] Create AuditLog entity in internal/domain/audit_log.go (id, actor_id, action, resource, resource_id, before/after JSONB, ip_address, user_agent, created_at)

### Repository Layer

- [ ] T101 [US7] Implement AuditLogRepository in internal/repository/audit_log.go (Create, FindByActorID, FindByResource)

### Service Layer

- [ ] T102 [US7] Implement AuditService in internal/service/audit.go (LogAction, async write)

### Middleware

- [ ] T103 [US7] Implement audit middleware in internal/http/middleware/audit.go (captures before/after, calls AuditService)
- [ ] T104 [US7] Register audit middleware in internal/http/server.go (on all mutating routes)

**Checkpoint**: Audit trail capturing all user actions with before/after snapshots

---

## Phase 10: User Story 8 - Example Domain: Invoices (Priority: P3)

**Goal**: Demonstrate full auth + permission + scope checking pattern on example entity

**Independent Test**: Create invoice, list invoices, update invoice, delete invoice with permission + ownership checks

### Tests for User Story 8

- [ ] T105 [P] [US8] Integration test for invoice CRUD in tests/integration/invoice_test.go
- [ ] T106 [P] [US8] Integration test for invoice scope checking (ownership) in tests/integration/invoice_scope_test.go

### Domain Models

- [ ] T107 [US8] Create Invoice entity in internal/module/invoice/entity.go (id, user_id, amount, status)

### Repository Layer

- [ ] T108 [US8] Implement InvoiceRepository in internal/module/invoice/repository.go (Create, FindByID, FindByUserID, Update, SoftDelete)

### Service Layer

- [ ] T109 [US8] Implement InvoiceService in internal/module/invoice/service.go (Create, GetByID with ownership check, ListByUser, Update, Delete)

### HTTP Layer

- [ ] T110 [US8] Create InvoiceRequest DTO in internal/http/request/invoice.go
- [ ] T111 [US8] Implement InvoiceHandler in internal/module/invoice/handler.go (Create, GetByID, List, Update, Delete)
- [ ] T112 [US8] Register invoice routes in internal/http/server.go (GET/POST /api/v1/invoices, GET/PUT/DELETE /api/v1/invoices/:id)

**Checkpoint**: Full CRUD pattern with permission + scope enforcement demonstrated on invoices

---

## Phase 11: User Story 9 - API Documentation (Priority: P3)

**Goal**: Swagger UI available for API exploration, Postman collection for testing

**Independent Test**: Access Swagger UI at /swagger/index.html, import Postman collection from docs/postman.json

### Tests for User Story 9

- [ ] T113 [P] [US9] Contract test: verify swagger.json matches API in tests/contract/swagger_test.go

### Swagger Integration

- [ ] T114 [US9] Add swaggo annotations to handlers in internal/http/handler/*.go (title, description, params, responses)
- [ ] T115 [US9] Install swaggo/echo-swagger and configure in internal/http/server.go
- [ ] T116 [US9] Generate swagger docs with swag init in Makefile (make swagger)
- [ ] T117 [US9] Expose /swagger/* endpoints in internal/http/server.go

### Postman Collection

- [ ] T118 [US9] Export Postman collection to docs/postman.json (includes all endpoints with examples)
- [ ] T119 [US9] Create API usage examples in docs/api-examples.md

**Checkpoint**: API documented via Swagger UI, Postman collection available for testing

---

## Phase 12: User Story 10 - Docker & Production Readiness (Priority: P3)

**Goal**: Containerized production deployment, health checks, Makefile automation

**Independent Test**: Docker build → docker run → curl /healthz returns 200

### Docker

- [ ] T120 [US10] Create Dockerfile at repository root (multi-stage build, Go 1.22-alpine)
- [ ] T121 [US10] Update docker-compose.yml with API service (depends_on Postgres + Redis)
- [ ] T122 [US10] Configure production environment in .env.example (DATABASE_URL, REDIS_URL, JWT_SECRET)

### Makefile

- [ ] T123 [US10] Add make docker-build target in Makefile (builds Docker image)
- [ ] T124 [US10] Add make docker-run target in Makefile (runs docker-compose up)
- [ ] T125 [US10] Add make clean target in Makefile (stops containers, removes volumes)

### Graceful Shutdown

- [ ] T126 [US10] Verify graceful shutdown in cmd/api/main.go (15s timeout, drain connections)

### Documentation

- [ ] T127 [US10] Create README.md at repository root (overview, setup steps per quickstart.md)
- [ ] T128 [US10] Create runbook in docs/runbook.md (deployment, monitoring, rollback procedures)

**Checkpoint**: API containerized, health checks pass, deployment-ready

---

## Phase 13: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

### Testing

- [ ] T129 [P] Add unit tests for all services in tests/unit/ (≥80% coverage target)
- [ ] T130 [P] Add integration tests for all repositories in tests/integration/repo_*_test.go
- [ ] T131 Verify testcontainers-go setup in tests/integration/testsuite.go (Postgres container per test suite)

### Performance

- [ ] T132 Profile permission check latency with Redis cache (target <10ms p99)
- [ ] T133 Optimize GORM queries (preload associations, avoid N+1)

### Security

- [ ] T134 Run security scan with gosec (no high/critical vulnerabilities)
- [ ] T135 Verify password hashing cost (bcrypt cost ≥12)
- [ ] T136 Verify JWT secret length (≥256 bits)

### Documentation

- [ ] T137 Update IMPLEMENTATION_PLAN.md with actual progress (mark phases complete)
- [ ] T138 Update quickstart.md with validated steps (run through full setup)

**Checkpoint**: Production-ready, documented, tested, secure

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-12)**: All depend on Foundational phase completion
  - US1 (Registration/Login) → Foundational complete
  - US2 (Refresh/Logout) → US1 complete
  - US3 (JWT Middleware) → US2 complete
  - US4 (Role/Permission CRUD) → US3 complete
  - US5 (User Role Assignment) → US4 complete
  - US6 (Permission Enforcement) → US5 complete
  - US7 (Audit Logging) → US3 complete (independent of US4-6)
  - US8 (Invoices) → US6 complete (needs permission enforcement)
  - US9 (Docs) → US8 complete (needs example domain)
  - US10 (Docker) → US1-9 complete (whole system)
- **Polish (Phase 13)**: Depends on all desired user stories being complete

### Within Each User Story

- Tests written and FAIL before implementation
- Domain models before repositories
- Repositories before services
- Services before handlers
- Handlers before route registration

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Domain models within a story marked [P] can run in parallel (different files)
- Tests for a user story marked [P] can run in parallel

---

## Parallel Example: User Story 4 (Role & Permission Management)

```bash
# Launch all domain models together:
Task: "Create Permission entity in internal/domain/permission.go"
Task: "Create Role entity in internal/domain/role.go"
Task: "Create UserRole entity in internal/domain/user_role.go"
Task: "Create RolePermission entity in internal/domain/role_permission.go"

# Launch all tests together:
Task: "Integration test for permission CRUD in tests/integration/permission_test.go"
Task: "Integration test for role CRUD in tests/integration/role_test.go"
Task: "Integration test for role-permission assignment in tests/integration/role_permission_test.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1-3 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (Registration & Login)
4. Complete Phase 4: User Story 2 (Refresh & Logout)
5. Complete Phase 5: User Story 3 (JWT Middleware)
6. **STOP and VALIDATE**: Test authentication flow end-to-end
7. Deploy MVP for early feedback

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1-3 → Core authentication working → Deploy MVP
3. Add US4-6 → RBAC system complete → Deploy increment
4. Add US7 → Audit trail → Deploy increment
5. Add US8-9 → Example domain + docs → Deploy increment
6. Add US10 + Polish → Production ready → Final release

### Estimated Timeline (Full-Time Team)

| Phase | Effort | Parallelizable |
|-------|--------|----------------|
| Phase 1 Setup | 1 day | High |
| Phase 2 Foundational | 3-4 days | Medium |
| US1-3 (Auth) | 4-5 days | Low (sequential) |
| US4-6 (RBAC) | 4-5 days | Medium |
| US7 (Audit) | 1-2 days | Low |
| US8 (Invoices) | 2-3 days | Low |
| US9 (Docs) | 1 day | High |
| US10 (Docker) | 1 day | Low |
| Polish | 2-3 days | Medium |
| **Total** | **19-23 days** | |

---

## Summary

| Metric | Value |
|--------|-------|
| **Total Tasks** | 138 |
| **User Stories** | 10 |
| **Tasks per Story** | US1: 16, US2: 8, US3: 6, US4: 18, US5: 11, US6: 9, US7: 6, US8: 8, US9: 7, US10: 9 |
| **Parallel Opportunities** | 47 tasks marked [P] |
| **MVP Scope** | US1-3 (Authentication core) |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Constitution requires 80% test coverage on business logic
- Permission cache invalidation CRITICAL for multi-instance consistency

---

**Version**: 1.0 | **Created**: 2026-04-22 | **Status**: Ready for Implementation
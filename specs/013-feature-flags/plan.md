# Implementation Plan: Feature Flags System

**Branch**: `013-feature-flags` | **Date**: 2026-05-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/013-feature-flags/spec.md`

## Summary

Runtime feature toggles for gradual rollout and A/B testing. CRUD API for managing flags, `IsEnabled()` evaluation with rollout percentage hashing and JSONB conditions, RBAC enforcement, audit logging, soft deletes. Follows existing codebase patterns (domain → repository → service → handler → server.go wiring).

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, Casbin, PostgreSQL (JSONB), Redis (caching)
**Storage**: PostgreSQL via golang-migrate SQL migrations
**Testing**: testify (unit), testcontainers-go (integration)
**Target Platform**: Linux server (REST API)
**Performance Goals**: <5ms evaluation latency, 1000 req/s
**Constraints**: Constitution Principles II–VII, no AutoMigrate, soft deletes, RBAC mandatory, audit logging
**Scale/Scope**: 10k flags, 100k evaluations/min

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ PASS | Repository pattern, structured logging, error envelopes, 80%+ test coverage |
| II. RBAC Mandatory, Not Hardcoded | ✅ PASS | `feature_flag:view` + `feature_flag:manage` via Casbin Enforce |
| III. Soft Deletes for Audit | ✅ PASS | `DeletedAt gorm.DeletedAt` on FeatureFlag entity |
| IV. Stateless JWT + Revocation | ✅ PASS | No auth changes needed |
| V. PostgreSQL + Versioned Migrations | ✅ PASS | `000021_create_feature_flags.up.sql` + `.down.sql` |
| VI. Multi-Instance Permission Consistency | ✅ PASS | Uses existing Casbin + Redis cache pattern |
| VII. Audit Logging Non-Negotiable | ✅ PASS | AuditService.LogAction on Create/Update/Delete |
| VIII–X. N/A | ✅ PASS | Not touched |

**No violations. No complexity tracking needed.**

## Project Structure

### Documentation (this feature)

```text
specs/013-feature-flags/
├── spec.md              # Feature specification (done)
├── plan.md              # This file
├── research.md           # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md         # Phase 1 output
└── contracts/            # Phase 1 output
    └── feature_flag.md   # Service/handler contracts
```

### Source Code (repository root)

```text
internal/
├── domain/
│   └── feature_flag.go            # FeatureFlag entity + DTOs + evaluation logic
├── repository/
│   └── feature_flag.go            # Interface + GORM implementation
├── service/
│   └── feature_flag.go            # FeatureFlagService (CRUD, evaluation, RBAC, audit)
├── http/
│   ├── handler/
│   │   └── feature_flag.go        # 7 HTTP endpoints
│   ├── request/
│   │   └── feature_flag.go        # Request DTOs + validation
│   └── response/
│       └── feature_flag.go        # Response envelope
├── config/
│   └── feature_flag.go           # Optional: cache TTL config (if Redis caching)
└── ...

migrations/
├── 000021_create_feature_flags.up.sql
└── 000021_create_feature_flags.down.sql

cmd/api/main.go                  # Permission seeding

tests/
├── unit/
│   └── feature_flag_service_test.go
└── integration/
    └── feature_flag_test.go
```

**Structure Decision**: Follows existing single-project Go layout matching webhook, settings, and data-portability features.

## Implementation Phases

### Phase 0: Foundation (Domain + Migration + Repository)

**Goal**: Create the database table, domain entity, and repository layer.

**Files**:
- `internal/domain/feature_flag.go` — Entity, DTOs, evaluation logic (`IsEnabled`, `EvaluateAll`)
- `internal/repository/feature_flag.go` — Interface + GORM implementation (6 methods)
- `migrations/000021_create_feature_flags.up.sql` — Table creation
- `migrations/000021_create_feature_flags.down.sql` — Drop table

**Entity Details**:
- `FeatureFlag` with UUID PK, `key` unique index, `DeletedAt` for soft delete
- `IsSystem bool` — system flags protected from deletion
- `Conditions datatypes.JSON` — JSONB for rollout conditions
- `ToResponse()` method following existing pattern

**Migration**:
```sql
CREATE TABLE feature_flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(100) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rollout INTEGER NOT NULL DEFAULT 100 CHECK (rollout >= 0 AND rollout <= 100),
    conditions JSONB,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);
CREATE INDEX idx_feature_flags_deleted_at ON feature_flags(deleted_at);
CREATE INDEX idx_feature_flags_key ON feature_flags(key) WHERE deleted_at IS NULL;
```

**Repository Interface** (6 methods):
- `Create`, `FindByID`, `FindByKey`, `FindAll` (paginated), `Update`, `SoftDelete`

**Validation**: Build passes, vet passes.

---

### Phase 1: Service Layer (CRUD + Evaluation + RBAC + Audit)

**Goal**: Business logic with permission enforcement, evaluation, and audit logging.

**Files**:
- `internal/service/feature_flag.go` — FeatureFlagService struct with:
  - `enforcer *permission.Enforcer` — RBAC
  - `audit *AuditService` — Audit logging
  - `cache` — Optional Redis cache for evaluations (TTL from config)

**Methods** (9):
1. `Create(ctx, flag) (*FeatureFlagResponse, error)` — RBAC: `feature_flag:manage`, audit: `create`
2. `GetByID(ctx, id) (*FeatureFlagResponse, error)` — RBAC: `feature_flag:view`
3. `GetByKey(ctx, key) (*FeatureFlagResponse, error)` — RBAC: `feature_flag:view`
4. `List(ctx, limit, offset) ([]*FeatureFlagResponse, int64, error)` — RBAC: `feature_flag:view`
5. `Update(ctx, id, updates) (*FeatureFlagResponse, error)` — RBAC: `feature_flag:manage`, audit: `update`
6. `Delete(ctx, id) error` — RBAC: `feature_flag:manage`, audit: `delete`, system flag protection
7. `IsEnabled(ctx, key, userID) bool` — No RBAC (authenticated-only), evaluation logic
8. `IsEnabledWithReason(ctx, key, userID) (*FeatureFlagEvaluation, error)` — No RBAC
9. `EvaluateAll(ctx, userID) (*BulkEvaluateResponse, error)` — No RBAC

**Evaluation Logic** (per spec Section 6):
1. Find flag by key → not found or disabled → false
2. Rollout 100% → true; Rollout 0% → false
3. Hash(userID + key) % 100 < rollout → true
4. Conditions check (user_ids, org_ids, envs)
5. Default → false

**RBAC Pattern** (matches settings/webhook):
```go
orgDomain := resolveOrgDomain(hasOrgID, orgID)
allowed, err := s.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
```

**Audit Pattern** (matches organization/settings):
```go
s.audit.LogAction(ctx, userID, domain.AuditActionCreate, "feature_flag", flag.ID.String(), nil, flag, ipAddress, userAgent)
```

**System Flag Protection**: Delete checks `IsSystem` flag; returns 403 if true.

**Soft Delete**: Uses `gorm.DeletedAt` scope, matching existing entity patterns.

**Validation**: Unit tests pass for evaluation logic (rollout hashing, conditions parsing).

---

### Phase 2: Handler + Request/Response + Validation

**Goal**: HTTP endpoints, request validation, response envelopes.

**Files**:
- `internal/http/handler/feature_flag.go` — 7 endpoints
- `internal/http/request/feature_flag.go` — Create/Update DTOs + validation
- `internal/http/response/feature_flag.go` — Response envelope types

**Request Validation** (`internal/http/request/feature_flag.go`):
- `CreateFeatureFlagRequest`: key required, min=1 max=100, name required, rollout 0-100, key format (lowercase + underscores)
- `UpdateFeatureFlagRequest`: all fields optional (pointer types), rollout 0-100 if provided
- Key format regex: `^[a-z][a-z0-9_]*$` (lowercase, starts with letter, underscores allowed)

**Handler Pattern** (matches settings):
```go
type FeatureFlagHandler struct {
    service *service.FeatureFlagService
    enforcer *permission.Enforcer
}
```

**Route Registration** (in `internal/http/server.go`):
```go
featureFlagHandler := handler.NewFeatureFlagHandler(featureFlagService, s.enforcer)
s.RegisterFeatureFlagRoutes(v1, featureFlagHandler)
```

**Evaluate Endpoints** (authenticated-only, no RBAC):
- `GET /api/v1/feature-flags/:key/evaluate` — Evaluate single flag
- `GET /api/v1/feature-flags/evaluate` — Evaluate all flags for user

**CRUD Endpoints** (RBAC enforced):
- `POST /api/v1/feature-flags` — `feature_flag:manage`
- `GET /api/v1/feature-flags` — `feature_flag:view`
- `GET /api/v1/feature-flags/:id` — `feature_flag:view`
- `PUT /api/v1/feature-flags/:id` — `feature_flag:manage`
- `DELETE /api/v1/feature-flags/:id` — `feature_flag:manage`

**Validation**: Integration tests pass for RBAC 403, auth 401, CRUD 200/201.

---

### Phase 3: Server Wiring + Permission Seeding

**Goal**: Wire everything together in `cmd/api/main.go` and seed permissions.

**Files**:
- `internal/http/server.go` — DI wiring + route registration
- `cmd/api/main.go` — Permission seeding (`feature_flag:view`, `feature_flag:manage`)

**Server Wiring** (following existing pattern):
```go
featureFlagRepo := repository.NewFeatureFlagRepository(db)
featureFlagService := service.NewFeatureFlagService(featureFlagRepo, s.enforcer, s.auditSvc, slog.Default())
featureFlagHandler := handler.NewFeatureFlagHandler(featureFlagService, s.enforcer)
s.RegisterFeatureFlagRoutes(v1, featureFlagHandler)
```

**Permission Seeding** (in `DefaultPermissions()`):
```go
{
    Name:     "feature_flag:view",
    Resource: "feature_flag",
    Action:   "view",
},
{
    Name:     "feature_flag:manage",
    Resource: "feature_flag",
    Action:   "manage",
},
```

**Validation**: `go build ./...` and `go vet ./...` pass. Server starts and routes are registered.

---

### Phase 4: Tests (Unit + Integration)

**Goal**: 80%+ coverage on business logic.

**Unit Tests** (`tests/unit/feature_flag_service_test.go`):
- `TestIsEnabled_GlobalEnabled` — flag enabled, rollout 100 → true
- `TestIsEnabled_Disabled` — flag exists but disabled → false
- `TestIsEnabled_NotFound` — flag doesn't exist → false
- `TestIsEnabled_Rollout100` — rollout 100 → true
- `TestIsEnabled_Rollout0` — rollout 0 → false
- `TestIsEnabled_RolloutPercentage` — deterministic hash check
- `TestIsEnabled_ConditionMatch_UserIDs` — user in conditions list → true
- `TestIsEnabled_ConditionNoMatch` — user not in conditions → false
- `TestCreate_AuditLogCalled` — audit logging
- `TestUpdate_AuditLogCalled` — audit logging
- `TestDelete_AuditLogCalled` — audit logging
- `TestDelete_SystemFlagProtected` — cannot delete is_system=true
- `TestCRUD_CreateAndGetByKey` — round-trip
- `TestCRUD_UpdatePartial` — update specific fields
- `TestCRUD_SoftDelete` — soft delete + not found after

**Integration Tests** (`tests/integration/feature_flag_test.go`):
- `TestFeatureFlag_CreateAndGet` — POST then GET
- `TestFeatureFlag_UpdateRollout` — PUT changes rollout
- `TestFeatureFlag_Delete` — DELETE soft-deletes
- `TestFeatureFlag_EvaluateEnabled` — evaluate returns true
- `TestFeatureFlag_EvaluateDisabled` — evaluate returns false
- `TestFeatureFlag_RBAC_CreateForbidden` — no manage permission → 403
- `TestFeatureFlag_RBAC_DeleteForbidden` — no manage permission → 403
- `TestFeatureFlag_UnauthCreateReturns401` — no token → 401

**Validation**: All tests pass, coverage >= 80%.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Evaluation performance under load | Low | Medium | Hash-based rollout is O(1); Redis cache TTL for hot keys |
| Race condition on flag update | Low | Low | GORM Upsert with row-level locking |
| Conditions JSONB schema drift | Low | Low | Validate on create/update against known keys |
| Missing migration number collision | Low | High | Number 000021 confirmed available (last is 000020) |

## Dependencies on Other Features

None. Feature flags are fully independent — no cross-feature dependencies.

## Ordering Guidance

Phases must be executed in order (0→1→2→3→4). Each phase ends with a build/vet gate.
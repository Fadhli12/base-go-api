# Research: Feature Flags System

**Feature ID:** 013-feature-flags | **Date:** 2026-05-11

## Existing Codebase Patterns

### Domain Entity Pattern
All domain entities follow the same structure:
- UUID primary key with `default:gen_random_uuid()`
- `CreatedAt time.Time`, `UpdatedAt time.Time`
- `DeletedAt gorm.DeletedAt` for soft deletes (Constitution III)
- `TableName()` method returning the SQL table name
- `ToResponse()` method converting entity to response DTO

### Repository Pattern
All repositories follow `internal/repository/X.go`:
- Interface definition with `context.Context` as first parameter
- GORM implementation struct with `db *gorm.DB`
- `NewXRepository(db *gorm.DB) XRepository` constructor
- Error wrapping with `pkg/errors`
- `FindBy*` methods for lookups, `SoftDelete` for removals

### Service Pattern
All services follow `internal/service/X.go`:
- Struct with repository + enforcer + audit + logger dependencies
- Constructor: `NewXService(repo, enforcer, audit, logger)`
- RBAC via `enforcer.Enforce(userID, orgDomain, resource, action)` before mutations
- Audit via `audit.LogAction(ctx, actorID, action, resource, resourceID, before, after, ip, userAgent)` after mutations
- Organization domain resolution via helper function
- AppError returns for business errors

### Handler Pattern
All handlers follow `internal/http/handler/X.go`:
- Struct with service + enforcer dependencies
- Constructor: `NewXHandler(service, enforcer)`
- JWT middleware on all routes via `RegisterXRoutes(api, handler)`
- User/org extraction from middleware context
- Error handling: check `apperrors.IsAppError` and return appropriate HTTP status

### Migration Pattern
All migrations are numbered sequentially: `000021_*.sql` is next after `000020_create_data_portability`.

### Permission Seeding
Permissions are seeded in `cmd/api/main.go` via `DefaultPermissions()`:
- Name format: `resource:action` (e.g., `feature_flag:view`)
- Stored in `permissions` table
- Synced to Casbin via `permission:sync` CLI command

## Research Items

### R1: Rollout Hashing Algorithm
**Decision**: Use FNV-1a hash combining `userID + key`, then modulo 100.
**Rationale**: Deterministic (same user always gets same result), no state needed, O(1).
**Implementation**:
```go
import "hash/fnv"

func rolloutHash(key string, userID uuid.UUID) int {
    h := fnv.New32a()
    h.Write([]byte(key))
    h.Write([]byte(userID.String()))
    return int(h.Sum32() % 100)
}
```

### R2: Conditions JSONB Schema
**Decision**: Validate against known keys: `user_ids` (array of UUID strings), `org_ids` (array of UUID strings), `envs` (array of strings).
**Rationale**: Simple, extensible, matches existing JSONB patterns (system_settings, webhook conditions).
**Implementation**: Type assertion on deserialized `map[string]interface{}`, validate each key's value type.

### R3: Evaluation Caching
**Decision**: No Redis caching in Phase 0. Direct DB lookup with optional cache added later.
**Rationale**: Simplicity first. DB lookup is <5ms. If performance becomes an issue, add `FeatureFlagCache` interface with Redis impl.

### R4: System Flag Protection
**Decision**: `IsSystem bool` field on FeatureFlag. Service `Delete()` method checks this and returns 403 if true.
**Rationale**: Matches `is_system` pattern used for roles in the existing codebase.

### R5: Evaluate Endpoint Authentication
**Decision**: Evaluate endpoints require JWT authentication only (no RBAC). Any authenticated user can check flags.
**Rationale**: Feature flags must be checkable by all logged-in users. Only CRUD operations need permission gates.

### R6: Key Format Validation
**Decision**: lowercase letters, digits, underscores. Must start with a letter. Max 100 chars. Regex: `^[a-z][a-z0-9_]*$`.
**Rationale**: Keys are used in code references and URLs. Simplicity and consistency.

### R7: Organization Scoping
**Decision**: Phase 0 does NOT include organization-scoped flag overrides. The entity has `organization_id` reserved but not used. Evaluate endpoints use the authenticated user's context only.
**Rationale**: Org-scoped flag overrides add complexity (flag hierarchy: global → org → user). Ship simple first, add org overrides in a future iteration.

### R8: Soft Delete Pattern
**Decision**: Use `DeletedAt gorm.DeletedAt` with `gorm.DeletedAt` index, matching all other entities.
**Rationale**: Constitution Principle III — soft deletes for audit & compliance.

All research items resolved. No NEEDS CLARIFICATION items remain.
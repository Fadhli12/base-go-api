# Implementation Plan: API Key Authentication

**Branch**: `002-api-key-auth` | **Date**: 2026-04-23 | **Spec**: `specs/api-key-auth/spec.md`

**Input**: Feature specification from `specs/api-key-auth/spec.md`

---

## Summary

Implement service-to-service authentication and API access via API keys with scoped permissions, rate limiting, expiration tracking, and comprehensive audit logging. API keys provide an alternative to JWT for machine-to-machine integrations.

**Technical Approach**:
- Entity: `api_keys` table with hashed key storage (bcrypt)
- Middleware: API key extraction and validation before permission checks
- Scopes: Simple `resource:action` model (subset of user permissions)
- Integration: Casbin for scope-to-permission validation

---

## Technical Context

| Item | Value |
|------|-------|
| **Language/Version** | Go 1.22+ |
| **Primary Dependencies** | Echo v4, GORM, Casbin, bcrypt, UUID |
| **Storage** | PostgreSQL (api_keys table) |
| **Testing** | testify + testcontainers (real DB) |
| **Target Platform** | Linux / Docker container |
| **Performance Goals** | >1000 API key validations/second, <10ms p99 key lookup |
| **Constraints** | bcrypt cost ≥12, 32-char keys, prefix for identification |
| **Scale/Scope** | Foundation for external integrations |

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Details |
|-----------|--------|---------|
| RBAC Mandatory | ✅ PASS | API keys use scope-based permissions, integrate with Casbin for validation |
| Soft Deletes Required | ✅ PASS | API keys use `deleted_at` and `revoked_at` (soft revocation) |
| JWT Alternative | ✅ PASS | API key is alternative auth method, JWT remains primary |
| Audit Logging Non-negotiable | ✅ PASS | All API key CRUD operations logged to audit_logs |
| Permission Cache | ✅ PASS | API key validation cached in Redis (shorter TTL than user permissions) |
| SQL Migrations | ✅ PASS | Migration file `000003_api_keys.up.sql` with explicit schema |
| Multi-instance Consistency | ✅ PASS | No additional pub/sub needed (keys don't change frequently) |

**Violations**: None

---

## Project Structure

### Documentation (this feature)

```text
specs/api-key-auth/
├── plan.md              # This file
├── research.md          # Research findings (complete)
├── spec.md              # Feature specification (complete)
├── data-model.md        # Database schema (complete)
├── quickstart.md        # 5-minute getting started (complete)
└── contracts/
    └── api-v1.md        # HTTP API contracts (complete)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   └── api_key.go                    # APIKey entity + ToResponse()
├── repository/
│   └── api_key.go                    # APIKeyRepository interface + implementation
├── service/
│   └── api_key.go                    # APIKeyService + scope validation
├── http/
│   ├── handler/
│   │   └── api_key.go                # HTTP handlers (create, list, revoke)
│   ├── middleware/
│   │   └── api_key.go                # API key authentication middleware
│   └── request/
│       └── api_key.go                # Request DTOs + validation
│
migrations/
├── 000003_api_keys.up.sql            # Create api_keys table
└── 000003_api_keys.down.sql          # Drop api_keys table

tests/
├── unit/
│   └── api_key_test.go               # Unit tests for service/repository
└── integration/
    └── api_key_test.go               # Integration tests with testcontainers
```

**Structure Decision**: Follows existing domain module pattern (see `internal/module/invoice/`). Entity in `internal/domain/`, service in `internal/service/`, repository in `internal/repository/`, handler in `internal/http/handler/`.

---

## Complexity Tracking

> **No violations detected.** Constitution check passed all gates.

---

## Phase 0: Research (COMPLETE)

**Status**: ✅ Complete

**Findings**:
- bcrypt hashing standard (cost ≥12)
- 32-character cryptographic keys with prefix
- Scope model: `resource:action` (simpler than full Casbin)
- Middleware order: Rate Limit → CORS → JWT Extract → API Key Extract → Permission Check → Handler
- Rate limiting: Separate limits for API keys vs user sessions
- Expiration: Optional ExpiresAt, checked on every auth
- Cleanup: Future consideration (expired key archival)

**Research Document**: `specs/api-key-auth/research.md`

---

## Phase 1: Design & Contracts (COMPLETE)

### Data Model

**Status**: ✅ Complete

**Document**: `specs/api-key-auth/data-model.md`

**Entity**: `api_keys`
- UUID primary key
- Foreign key to `users.id` (CASCADE on delete)
- bcrpyt key hash (unique)
- JSONB scopes array
- Timestamps: created, updated, expires, last_used, revoked, deleted

**Validation**:
- Name: required, max 255 chars, unique per user
- Key hash: bcrypt format, unique globally
- Scopes: valid format, subset of user permissions
- ExpiresAt: future timestamp if set

### API Contracts

**Status**: ✅ Complete

**Document**: `specs/api-key-auth/contracts/api-v1.md`

**Endpoints**:
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api-keys` | Create API key |
| GET | `/api-keys` | List API keys |
| GET | `/api-keys/:id` | Get API key details |
| DELETE | `/api-keys/:id` | Revoke API key |

**Authentication**:
- Management endpoints: JWT Bearer token
- Usage: X-API-Key header OR JWT Bearer token

**Scopes**:
- Read: `resource:read`
- Write: `resource:write` (create, update, delete)
- Manage: `resource:manage` (full CRUD)
- Wildcard: `*` (admin only)

### Quickstart

**Status**: ✅ Complete

**Document**: `specs/api-key-auth/quickstart.md`

**Contents**:
- Create API key
- Use API key for authentication
- Manage API keys
- Scope validation
- Rate limiting
- Troubleshooting

---

## Phase 2: Implementation

**Status**: 🔲 Pending (Use `/speckit.tasks` to generate task breakdown)

### Task Breakdown (High-Level)

#### 2.1 Database Migration

```
- [ ] Create migrations/000005_api_keys.up.sql
- [ ] Create migrations/000005_api_keys.down.sql
- [ ] Run migration: make migrate
- [ ] Verify table structure
```

#### 2.2 Domain Entity

```
- [ ] Create internal/domain/api_key.go
- [ ] Add APIKey struct with GORM tags
- [ ] Add APIKeyResponse struct
- [ ] Add ToResponse() method
- [ ] Add IsActive() method
- [ ] Add HasScope() method
- [ ] Add validation: TableName(), field constraints
```

#### 2.3 Repository Layer

```
- [ ] Create internal/repository/api_key.go
- [ ] Define APIKeyRepository interface
- [ ] Implement Create()
- [ ] Implement FindByID()
- [ ] Implement FindByKeyHash()
- [ ] Implement FindByUserID()
- [ ] Implement Update()
- [ ] Implement Revoke()
- [ ] Implement SoftDelete()
- [ ] Implement SoftDeleteByUserID()
- [ ] Implement UpdateLastUsedAt() - Use atomic UPDATE to avoid race conditions
- [ ] Implement CountActiveByUserID()
```

**Note on UpdateLastUsedAt**: Use atomic update to prevent race conditions during concurrent authentication:
```go
// Atomic update - no read-modify-write race
func (r *repository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID) error {
    return r.db.WithContext(ctx).
        Model(&APIKey{}).
        Where("id = ? AND revoked_at IS NULL AND deleted_at IS NULL", id).
        Update("last_used_at", time.Now()).
        Error
}
```

#### 2.4 Service Layer

```
- [ ] Create internal/service/api_key.go
- [ ] Implement APIKeyService struct
- [ ] Implement Create() with scope validation
- [ ] Implement List() with pagination
- [ ] Implement GetByID() with ownership check
- [ ] Implement Revoke() with audit logging
- [ ] Implement Validate() for authentication (skip last_used_at if updated within 1 min)
- [ ] Implement ValidateScopes() against user permissions
- [ ] Add key generation (crypto/rand, 32 chars) - Reference: internal/service/auth.go
- [ ] Add prefix extraction (first 12 chars)
- [ ] Add bcrypt hashing (cost 12)
```

**Note on Validate()**: Optimize last_used_at updates to reduce database load:
- Skip update if last_used_at was updated within the last 1 minute
- This prevents excessive writes during concurrent requests
- Use atomic UPDATE when updating

#### 2.5 Request DTOs

```
- [ ] Create internal/http/request/api_key.go
- [ ] Add CreateAPIKeyRequest struct
- [ ] Add UpdateAPIKeyRequest struct (if needed)
- [ ] Add APIKeyListQuery struct
- [ ] Add Validate() methods with go-playground/validator
- [ ] Add scope format validation helper
```

#### 2.6 HTTP Handler

```
- [ ] Create internal/http/handler/api_key.go
- [ ] Implement Create() handler
- [ ] Implement List() handler
- [ ] Implement GetByID() handler
- [ ] Implement Revoke() handler
- [ ] Add permission checks (api_key:create, api_key:read, etc.)
- [ ] Add audit logging for create/revoke
```

#### 2.7 Middleware Integration

```
- [ ] Create internal/http/middleware/api_key.go
- [ ] Implement APIKeyAuth() middleware
- [ ] Integrate with existing auth chain (after JWT, before permission)
- [ ] Add user context population from API key
- [ ] Add rate limiting by API key ID
```

#### 2.8 Permission Integration

```
- [ ] Add API key permissions to config/permissions.yaml
- [ ] Update permission:sync command to create new permissions
- [ ] Add scope-to-permission mapping logic
- [ ] Update permission middleware to check scopes
```

#### 2.9 Route Registration

```
- [ ] Update internal/http/server.go
- [ ] Register API key routes under /api/v1/api-keys
- [ ] Add route group with JWT middleware (management)
- [ ] Add X-API-Key header extraction to existing routes
```

#### 2.10 Testing

```
- [ ] Create tests/unit/api_key_test.go
- [ ] Test Create() with valid scopes
- [ ] Test Create() with invalid scopes
- [ ] Test Validate() with active key
- [ ] Test Validate() with revoked key
- [ ] Test Validate() with expired key
- [ ] Test Validate() with deleted key
- [ ] Test Scope validation
- [ ] Create tests/integration/api_key_test.go
- [ ] Test full auth flow with testcontainers
- [ ] Test rate limiting per API key
- [ ] Test audit logging
```

#### 2.11 Documentation

```
- [ ] Add OpenAPI annotations to handlers
- [ ] Update README.md with API key usage
- [ ] Add API key examples to quickstart
- [ ] Update AGENTS.md with new module info
```

---

## Testing Strategy

### Unit Tests

- **Domain**: Entity methods (IsActive, HasScope, ToResponse)
- **Repository**: GORM queries (mock DB or testcontainers)
- **Service**: Business logic (validation, key generation, revocation)
- **Handler**: HTTP response codes, error messages

### Integration Tests

- **Auth Flow**: Create key → authenticate → use key → revoke
- **Scope Validation**: Test scope-based permission checks
- **Expiration**: Test expired key returns 401
- **Revocation**: Test revoked key returns 401
- **Rate Limiting**: Test per-key rate limits

### Coverage Targets

- **Business Logic**: ≥80%
- **Handlers**: ≥80%
- **Repository**: ≥80% (with testcontainers)

### Unit Test Examples

```
### Domain Tests
- TestAPIKey_IsActive_WhenNotDeleted_ReturnsTrue
- TestAPIKey_IsActive_WhenRevoked_ReturnsFalse
- TestAPIKey_IsActive_WhenExpired_ReturnsFalse
- TestAPIKey_HasScope_WhenScopeMatches_ReturnsTrue
- TestAPIKey_HasScope_WhenWildcard_ReturnsTrue
- TestAPIKey_ToResponse_MasksSecretFields

### Service Tests
- TestCreateAPIKey_ValidScopes_ReturnsKeyWithSecret
- TestCreateAPIKey_UserLacksPermission_Returns403
- TestCreateAPIKey_DuplicateName_Returns409
- TestValidateAPIKey_ActiveKey_ReturnsUserAndScopes
- TestValidateAPIKey_RevokedKey_Returns401
- TestValidateAPIKey_ExpiredKey_Returns401
- TestValidateAPIKey_DeletedUser_Returns401
- TestValidateAPIKey_ConcurrentRequests_LastUsedAtConsistent
- TestRevokeAPIKey_ActiveKey_SetsRevokedAt
- TestRevokeAPIKey_AlreadyRevoked_Returns409
- TestValidateScopes_UserHasPermission_ReturnsNil
- TestValidateScopes_UserLacksPermission_ReturnsError

### Repository Tests
- TestCreate_DuplicateKeyHash_ReturnsError
- TestFindByKeyHash_NotFound_ReturnsError
- TestUpdateLastUsedAt_ConcurrentRequests_NoRace
- TestSoftDeleteByUserID_CascadesFromUser

### Middleware Tests
- TestAPIKeyAuth_ValidKey_SetsUserContext
- TestAPIKeyAuth_InvalidKey_Returns401
- TestAPIKeyAuth_MissingKey_PassesToJWT
- TestAPIKeyAuth_ExpiredKey_Returns401WithError

### Integration Tests
- TestAPIKey_FullAuthFlow_CreateUseRevoke
- TestAPIKey_RateLimiting_PerKeyLimits
- TestAPIKey_AuditLogging_CreateRevokeLogged
- TestAPIKey_ScopeValidation_CrossResourceDenied
```

---

## Migration Strategy

### Schema Changes

**File**: `migrations/000003_api_keys.up.sql`

```sql
-- Creates api_keys table with indexes
-- Adds foreign key to users.id (CASCADE DELETE)
-- Adds check constraints for key format
-- Adds triggers for updated_at
```

**Rollback**: `migrations/000003_api_keys.down.sql`

```sql
-- Drops all indexes
-- Drops triggers
-- Drops api_keys table
-- Note: CASCADE DELETE ensures cleanup
```

### Data Migration

**No data migration needed** - API keys are created by users after deployment.

### Permissions Migration

**Add to config/permissions.yaml**:
```yaml
- name: api_key:create
  resource: api_key
  action: create
  scope: all
  
- name: api_key:read
  resource: api_key
  action: read
  scope: own
  
- name: api_key:revoke
  resource: api_key
  action: revoke
  scope: own
  
- name: api_key:manage
  resource: api_key
  action: manage
  scope: all
```

**Run**: `make seed` or `go run cmd/api/main.go permission:sync`

---

## Deployment Checklist

### Pre-Deployment

- [ ] All tests passing (`make test`, `make test-integration`)
- [ ] Linting passing (`make lint`)
- [ ] Migration tested (up + down)
- [ ] Documentation updated
- [ ] Security review completed
- [ ] Constitution check passed

### Deployment

- [ ] Run migration: `make migrate`
- [ ] Run permission sync: `make seed`
- [ ] Deploy API binary
- [ ] Verify health check: `/healthz`
- [ ] Test API key creation: POST `/api-keys`
- [ ] Test API key authentication: GET `/invoices` with `X-API-Key`
- [ ] Monitor logs for errors

### Post-Deployment

- [ ] Monitor for 401 errors from API keys
- [ ] Check audit logs for `api_key:create` events
- [ ] Verify rate limiting working
- [ ] Check LastUsedAt updates

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Key leaked in logs | High | Never log full key, hash only, prefix in logs |
| bcrypt performance | Medium | Cost 12 is acceptable (<100ms on auth), cache valid keys |
| Scope escalation | High | Validate scopes ≤ user permissions on creation |
| Expired key usage | Low | Checked on every auth, error message clear |
| Rate limit bypass | Medium | Rate limit by key_id, separate from user limits |
| Migration failure | High | Test migration up/down, backup DB before deploy |

---

## Acceptance Criteria

- [ ] All functional requirements implemented (spec.md FR-01 to FR-10)
- [ ] All non-functional requirements met (NFR-01 to NFR-06)
- [ ] 80%+ test coverage
- [ ] Integration tests pass with testcontainers
- [ ] All API documented in OpenAPI
- [ ] README updated with API key usage
- [ ] Migration tested (up + down)
- [ ] Security review passed
- [ ] Constitution check passed

---

## Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| Domain entities | ✅ Ready | User, Role, Permission exist |
| Repository pattern | ✅ Ready | Follow existing pattern |
| Service layer | ✅ Ready | Follow existing pattern |
| HTTP handlers | ✅ Ready | Echo handlers pattern |
| Middleware chain | ✅ Ready | Integrate after JWT |
| Casbin integration | ✅ Ready | Permission checks |
| Audit logging | ✅ Ready | AuditService exists |
| Rate limiting | ✅ Ready | Redis-backed, add key-based limits |

---

## Timeline Estimate

| Phase | Effort | Days |
|-------|--------|------|
| 2.1 Migration | 0.5 | 0.5 |
| 2.2 Domain Entity | 0.5 | 0.5 |
| 2.3 Repository | 1 | 1 |
| 2.4 Service | 2 | 2 |
| 2.5 Request DTOs | 0.5 | 0.5 |
| 2.6 HTTP Handler | 1 | 1 |
| 2.7 Middleware | 1 | 1 |
| 2.8 Permission Integration | 0.5 | 0.5 |
| 2.9 Route Registration | 0.5 | 0.5 |
| 2.10 Testing | 2 | 2 |
| 2.11 Documentation | 0.5 | 0.5 |
| **Total** | **10 days** | **10 days** |

---

## References

- **Feature Spec**: `specs/api-key-auth/spec.md`
- **Data Model**: `specs/api-key-auth/data-model.md`
- **API Contracts**: `specs/api-key-auth/contracts/api-v1.md`
- **Quickstart**: `specs/api-key-auth/quickstart.md`
- **Research**: `specs/api-key-auth/research.md`
- **Existing Code**: `internal/module/invoice/` (domain module pattern)
- **Existing Code**: `internal/http/middleware/jwt.go` (auth middleware)
- **Existing Code**: `internal/http/middleware/permission.go` (permission checks)

---

**Version**: 1.0 | **Created**: 2026-04-23 | **Status**: Phase 1 Complete, Phase 2 Pending
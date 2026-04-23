# Feature Specification: API Key Authentication

**Version**: 1.0 | **Date**: 2026-04-23 | **Status**: Draft

---

## Feature Overview

### Summary

Implement service-to-service authentication and API access for external integrations via API keys. Keys have scoped permissions, rate limiting, expiration tracking, and comprehensive audit logging.

### Problem Statement

Currently, the Go API Base only supports JWT authentication for user sessions. External applications, services, and integrations cannot authenticate without user credentials. This creates friction for:

- Background jobs and scheduled tasks
- Third-party integrations (webhooks, data sync)
- Microservice-to-microservice communication
- Machine-to-machine (M2M) automation workflows

### Proposed Solution

Add API Key authentication as an alternative authentication method with:

1. **Scoped permissions** - API keys have limited permissions vs full user access
2. **Hashed storage** - Keys stored securely with bcrypt, not plaintext
3. **Expiration tracking** - Optional expiry with LastUsedAt for cleanup
4. **Revocation support** - Revoke without deletion for audit trail
5. **Prefix identification** - Visible prefix helps users identify keys in logs/UI
6. **Audit logging** - All API key operations logged for compliance

---

## Requirements

### Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| FR-01 | Create API keys with custom name and scopes | Must | User can name keys for identification |
| FR-02 | List all API keys for current user | Must | Show name, prefix, scopes, created date, expires, last used, revoked |
| FR-03 | Revoke API keys | Must | Soft revoke (RevokedAt), not hard delete |
| FR-04 | Authenticate requests via X-API-Key header | Must | Alternative to JWT, fallback if no JWT |
| FR-05 | Validate API key scopes on permission checks | Must | Keys have subset of user permissions |
| FR-06 | Track LastUsedAt on successful authentication | Must | Security monitoring |
| FR-07 | Support optional expiration (ExpiresAt) | Should | Key rotation pattern |
| FR-08 | Show key value only once on creation | Must | Security best practice |
| FR-09 | Log all API key usage in audit trail | Must | actor_id = user, resource = "api_key" |
| FR-10 | Rate limit API key requests separately | Should | Different rate limits for API keys |

### Non-Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| NFR-01 | Key hash must use bcrypt (cost ≥12) | Must | Same as password hashing |
| NFR-02 | Key lookup < 10ms p99 | Must | Index on key_hash |
| NFR-03 | 1000+ API key authentications per second | Should | Performance target |
| NFR-04 | Audit all API key CRUD operations | Must | Constitution requirement |
| NFR-05 | Soft delete for audit compliance | Must | Constitution requirement |
| NFR-06 | Scope validation before permission check | Must | Prevent privilege escalation |

---

## User Stories

### US-01: Developer Creates API Key for Integration

```
As a developer,
I want to create an API key with specific scopes,
So that my integration can access only the resources it needs.

Acceptance Criteria:
- POST /api-keys with {name, scopes[]} returns key (shown once)
- Key format: ak_live_xxxxxxxxxxxx (prefix + random value)
- Response includes key ID, prefix, scopes, created_at, expires_at (if set)
- Key is hashed and stored securely
- Audit log entry created for "api_key:create"
```

### US-02: Developer Lists Their API Keys

```
As a developer,
I want to list all my API keys,
So that I can manage them and see their status.

Acceptance Criteria:
- GET /api-keys returns list of keys for current user
- Shows: id, name, prefix, scopes, created_at, expires_at, last_used_at, is_revoked
- Does NOT show full key value (only prefix for identification)
- Pagination support (limit, offset)
```

### US-03: Developer Revokes API Key

```
As a developer,
I want to revoke an API key,
So that it can no longer be used for authentication.

Acceptance Criteria:
- DELETE /api-keys/:id sets revoked_at timestamp
- Key still exists for audit trail
- Subsequent authentication attempts return 401
- Audit log entry created for "api_key:revoke"
```

### US-04: External Service Authenticated with API Key

```
As an external service,
I want to authenticate using an API key,
So that I can access the API without user credentials.

Acceptance Criteria:
- X-API-Key header with key value authenticates request
- Middleware validates key, checks expiration, checks revocation
- User context populated from key's user_id
- Permission checks use key scopes (subset of user permissions)
- LastUsedAt updated on successful auth
- Rate limit applied per API key (not per user)
```

### US-05: Key Expiration Enforcement

```
As a security administrator,
I want API keys to expire automatically,
So that stale credentials don't remain active indefinitely.

Acceptance Criteria:
- Keys with ExpiresAt < NOW return 401 on authentication
- Error message: "API key expired"
- Scheduled job can optionally expire keys proactively
```

---

## Technical Design

### Entity Model

```go
type APIKey struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"` // Owner
    Name        string         `gorm:"size:255;not null"`         // Human-readable name
    Prefix      string         `gorm:"size:8;not null;uniqueIndex"` // Visible prefix (ak_live_xxxx)
    KeyHash     string         `gorm:"size:255;not null;uniqueIndex"` // bcrypt hash
    Scopes      datatypes.JSON `gorm:"type:jsonb"` // ["read:invoices", "write:news"]
    ExpiresAt   *time.Time     `gorm:"index"`                      // Optional expiration
    LastUsedAt  *time.Time     `gorm:"index"`                      // Last authentication
    RevokedAt   *time.Time     `gorm:"index"`                      // Soft revocation
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"` // Soft delete
}
```

### Key Format

```
ak_{env}_{random}

Where:
- ak = API key prefix
- env = "live" | "test" (environment indicator)
- random = 32 alphanumeric characters (cryptographically secure)

Examples:
- ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
- ak_test_z9y8x7w6v5u4t3s2r1q0p9o8n7m6l5k4

Stored in DB as:
- prefix: "ak_live_a1b2" (first 12 chars for identification)
- key_hash: bcrypt(key_value)
```

### Scopes Model

```go
// Scopes define limited permissions for API keys
// They are a subset of user permissions

type Scope string

const (
    // Read scopes
    ScopeUsersRead       Scope = "users:read"
    ScopeRolesRead       Scope = "roles:read"
    ScopePermissionsRead Scope = "permissions:read"
    ScopeInvoicesRead    Scope = "invoices:read"
    ScopeNewsRead        Scope = "news:read"
    ScopeAuditLogsRead   Scope = "audit_logs:read"
    
    // Write scopes
    ScopeInvoicesWrite   Scope = "invoices:write"
    ScopeNewsWrite       Scope = "news:write"
    
    // Administration scopes
    ScopeUsersManage     Scope = "users:manage"
    ScopeRolesManage     Scope = "roles:manage"
    
    // Wildcard
    ScopeAll             Scope = "*" // Only for super admin keys
)

// Validation: scopes must be subset of user's effective permissions
// User with "invoices:read" cannot create key with "invoices:write"
```

### Authentication Flow

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│   Client     │────▶│ API Gateway │────▶│ API Handler  │
└──────────────┘     └─────────────┘     └──────────────┘
       │                    │                     │
       │ X-API-Key: ak_... │                     │
       │ (or Bearer JWT)   │                     │
       ▼                    ▼                     ▼
       ┌─────────────────────────────────────────┐
       │         Auth Middleware Chain            │
       │  1. Extract auth (JWT or API Key)        │
       │  2. Validate API Key if present          │
       │     - Load by key_hash                   │
       │     - Check not revoked                  │
       │     - Check not expired                  │
       │     - Check user not soft-deleted        │
       │     - Update LastUsedAt                  │
       │  3. Validate JWT if present              │
       │     - Standard JWT flow                  │
       │  4. Set user context                     │
       │     - user_id from JWT or API Key        │
       │     - scopes from API Key (if auth)      │
       │     - scopes = all (if JWT)              │
       └─────────────────────────────────────────┘
       │                                         │
       ▼                                         ▼
       ┌─────────────────────────────────────────┐
       │      Permission Middleware               │
       │  1. Check user has permission            │
       │     - If JWT: check full permissions     │
       │     - If API Key: check scope allows     │
       │  2. Allow or deny access                  │
       └─────────────────────────────────────────┘
```

### API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/v1/api-keys` | Create API key | JWT |
| GET | `/api/v1/api-keys` | List API keys | JWT |
| GET | `/api/v1/api-keys/:id` | Get API key details | JWT |
| DELETE | `/api/v1/api-keys/:id` | Revoke API key | JWT |

### Permission Model

New permissions for API key management:

```
api_key:create   - Create new API key
api_key:read     - View own API keys
api_key:revoke   - Revoke own API keys
api_key:manage   - Manage all API keys (admin)
```

Scope translation to permission check:

```go
// When checking permission with API key:
// scope "invoices:read" maps to permission check "invoices:read"
// scope "invoices:write" maps to permission check "invoices:create" OR "invoices:update"

func (s *Service) CheckPermission(ctx context.Context, userID uuid.UUID, resource, action string, scopes []string) bool {
    // If JWT auth (no scopes), use full user permissions
    if len(scopes) == 0 {
        return s.enforcer.Enforce(userID, resource, action)
    }
    
    // If API key auth, check scope allows this action
    for _, scope := range scopes {
        if scope == "*" {
            return true // Wildcard scope
        }
        
        // Parse scope: resource:action
        parts := strings.Split(scope, ":")
        if len(parts) != 2 {
            continue
        }
        
        scopeResource := parts[0]
        scopeAction := parts[1]
        
        // Match resource
        if scopeResource != resource {
            continue
        }
        
        // Match action (write includes create, update, delete)
        if scopeAction == action || scopeAction == "write" || scopeAction == "*" {
            return true
        }
    }
    
    return false
}
```

---

## Constraints

### Security Constraints

1. **Key never shown twice** - Only returned during creation
2. **Bcrypt hashing** - Cost ≥ 12, same as passwords
3. **No super admin bypass** - API key scopes are always validated
4. **Rate limiting** - Separate rate limits for API keys
5. **Expiration enforcement** - Expired keys return 401
6. **Revocation enforcement** - Revoked keys return 401
7. **User deletion cascade** - API keys deleted when user soft-deleted

### Constitution Constraints

| Principle | Compliance |
|-----------|------------|
| RBAC Mandatory | ✅ API keys use scope-based permission checks, integrated with Casbin |
| Soft Deletes | ✅ API keys use DeletedAt, revoked keys use RevokedAt |
| JWT Alternative | ✅ API key is alternative auth method, not replacement |
| Audit Logging | ✅ All API key operations logged |
| Permission Cache | ✅ API key validation cached in Redis (shorter TTL) |

---

## Out of Scope

- API key rotation (manual create/revoke workflow)
- API key usage analytics dashboard
- Per-request rate limit configuration
- API key IP allowlisting
- API key webhook events
- Team/organization API keys (future: shared keys)

---

## Dependencies

### Internal Dependencies

- `internal/service/auth` - JWT middleware
- `internal/service/audit` - Audit logging
- `internal/permission` - Casbin enforcer
- `internal/repository` - GORM repositories
- `internal/domain/user` - User validation

### External Dependencies

- No new external dependencies (uses existing bcrypt, GORM, etc.)

---

## Success Criteria

1. **Functional**: All functional requirements pass
2. **Performance**: 1000+ API key auth per second (benchmarked)
3. **Security**: Key lookup < 10ms p99, bcrypt cost ≥ 12
4. **Compliance**: All operations audit logged
5. **Testing**: ≥80% coverage, integration tests with testcontainers

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Key leaked in logs | High | Key only shown once, hashed in DB, prefix-only in logs |
| Brute force key discovery | Medium | 32 char keys, bcrypt hash, rate limiting |
| Privilege escalation via scopes | High | Validate scopes ≤ user permissions on creation |
| Expired key usage | Medium | Check expiration on every auth |
| Revoked key usage | Low | Check revocation on every auth |

---

## Implementation Order

1. **Phase 1**: Entity + Migration + Repository
2. **Phase 2**: Service layer (create, validate, list, revoke)
3. **Phase 3**: Handler + API endpoints
4. **Phase 4**: Middleware integration (auth chain)
5. **Phase 5**: Scope validation
6. **Phase 6**: Configuration (rate limits, key format)
7. **Phase 7**: Testing (unit + integration)
8. **Phase 8**: Documentation (OpenAPI, README)

---

## Acceptance Criteria

- [ ] All functional requirements implemented
- [ ] All non-functional requirements met
- [ ] 80%+ test coverage
- [ ] Integration tests pass
- [ ] API documented in OpenAPI
- [ ] README updated with API key usage
- [ ] Migration tested (up + down)
- [ ] Security review passed

---

**Version History**:
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-04-23 | Sisyphus | Initial specification |
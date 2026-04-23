# Research: API Key Authentication

**Feature**: API Key Authentication  
**Date**: 2026-04-23  
**Status**: Complete

---

## Research Questions

### Q1: API Key Security Best Practices

**Decision**: Use bcrypt hashing with cost 12+ and 32-character random keys.

**Research**:
- Industry standard: Never store API keys in plaintext
- Hash with bcrypt (same approach as passwords)
- Key format: `{prefix}_{env}_{random}` for easy identification
- Prefix visible in DB for user identification without exposing full key

**Best Practices Applied**:
1. Show key value ONCE on creation (only opportunity to copy)
2. Store hash only (bcrypt with cost ≥12)
3. Include environment prefix (live vs test)
4. Rate limit authentication attempts
5. Log all key usage for audit

---

### Q2: Scope Model vs Full Permission Model

**Decision**: Use simpler scope model (resource:action) instead of full Casbin permissions.

**Rationale**:
- API keys should have LIMITED permissions
- Scopes are a subset of user's effective permissions
- Simpler mental model for developers
- Easier to validate and audit

**Implementation**:
```go
// Scope format: "resource:action" or "resource:write" (covers create/update/delete)
// Special scope: "*" for full access (admin keys only)

// Examples:
// "invoices:read" → Can read invoices
// "invoices:write" → Can create, update, delete invoices
// "news:read" → Can read news
// "*" → Full access (requires api_key:manage permission)
```

**Validation Rule**: User cannot create API key with scopes they don't have permission for.

### Scope Validation Flow

**At Key Creation:**

1. User requests key with scopes `["invoices:read", "invoices:write"]`
2. Service queries user's effective permissions from Casbin:
   ```go
   permissions := s.enforcer.GetPermissionsForUser(userID)
   ```
3. Service maps each scope to achievable permissions:
   - `invoices:read` → requires `invoices:read` permission
   - `invoices:write` → requires `invoices:create`, `invoices:update`, or `invoices:delete`
4. Service validates each scope:
   ```go
   func (s *Service) ValidateScopes(userID uuid.UUID, requestedScopes []string) error {
       userPerms := s.enforcer.GetPermissionsForUser(userID)
       for _, scope := range requestedScopes {
           if !s.scopeAchievable(scope, userPerms) {
               return ErrScopeNotAllowed
           }
       }
       return nil
   }
   ```
5. If validation passes, store scopes in `api_keys.scopes`
6. Return 403 if any scope exceeds user permissions

**At Request Time:**

1. Middleware extracts `X-API-Key` header
2. Service validates key (not revoked, not expired, user active)
3. Permission middleware checks: `scopeAllowsAction(scopes, resource, action)`
4. Even if user's permissions are later revoked, the key still works but permission check will deny access

**Key Insight**: Scopes are validated at creation time against user's permissions, but permission checks still happen at request time. This prevents privilege escalation even if user's permissions change after key creation.

---

### Q3: Key Format and Generation

**Decision**: Use `ak_{env}_{random32}` format with cryptographic random generation.

**Format**:
- `ak` = API key prefix (constant)
- `{env}` = "live" or "test" (environment indicator)
- `{random32}` = 32 alphanumeric characters (cryptographically secure)

**Examples**:
- Production: `ak_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`
- Test: `ak_test_z9y8x7w6v5u4t3s2r1q0p9o8n7m6l5k4`

**Storage**:
- `prefix` column: First 12 characters (e.g., "ak_live_a1b2")
- `key_hash` column: bcrypt hash of full key

**Rationale**:
- Prefix allows users to identify keys without exposing full value
- Environment separation prevents test keys in production
- 32 chars provides 62^32 ≈ 2^189 possible combinations
- bcrypt adds computational cost to brute force

---

### Q4: Middleware Integration Order

**Decision**: Add API key middleware after JWT extraction in auth chain.

**Order**:
```
Request → Rate Limit → CORS → JWT Extract → API Key Extract → 
          Permission Check → Audit → Handler
```

**Implementation**:
```go
// middleware/api_key.go
func APIKeyAuth(apiKeyService *service.APIKeyService) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // 1. Check if JWT already set user context
            if _, ok := c.Get("user_id").(uuid.UUID); ok {
                return next(c) // JWT auth, skip API key
            }
            
            // 2. Extract API key from header
            key := c.Request().Header.Get("X-API-Key")
            if key == "" {
                return next(c) // No API key, continue
            }
            
            // 3. Validate API key
            apiKey, user, err := apiKeyService.Validate(c.Request().Context(), key)
            if err != nil {
                return echo.NewHTTPError(401, "Invalid API key")
            }
            
            // 4. Set user context with scopes
            c.Set("user_id", user.ID)
            c.Set("user_email", user.Email)
            c.Set("api_key_id", apiKey.ID)
            c.Set("scopes", apiKey.Scopes)
            
            return next(c)
        }
    }
}
```

---

### Q5: Rate Limiting for API Keys

**Decision**: Apply rate limits per API key, not per user.

**Implementation**:
- Rate limit identifier: `api_key:{key_id}` or `user:{user_id}`
- Different limits: API keys have lower limits than user sessions
- Redis key: `ratelimit:api_key:{key_id}:{timestamp}`

**Configuration**:
```yaml
RATE_LIMIT_API_KEY_REQUESTS=60      # Requests per minute for API keys
RATE_LIMIT_API_KEY_WINDOW=60        # Window in seconds
RATE_LIMIT_USER_REQUESTS=100         # Requests per minute for users
```

---

### Q6: Expiration and Cleanup

**Decision**: Support optional expiration with scheduled cleanup.

**Approach**:
1. `ExpiresAt` column (nullable) - key expires at this timestamp
2. Check expiration on every authentication
3. Optional scheduled job to archive/revoke expired keys

**Cleanup Job** (future):
```go
// Daily cleanup job
func (s *Service) CleanupExpiredKeys(ctx context.Context) error {
    // Archive keys that expired > 30 days ago
    // Optionally send notification before expiration
}
```

---

### Q7: User Deletion Handling

**Decision**: Cascade delete API keys when user is soft-deleted.

**Implementation**:
- On user soft-delete, set `DeletedAt` on all associated API keys
- Authentication fails for keys with `DeletedAt != NULL`
- Foreign key constraint: `ON DELETE CASCADE` for referential integrity
- Query: Soft-delete keys when user is soft-deleted

---

### Q8: Audit Logging for API Keys

**Decision**: Log all API key CRUD operations and authenticate events.

**Events**:
| Action | Resource | Before | After |
|--------|----------|--------|-------|
| `api_key:create` | `api_key` | null | `{name, scopes, expires_at}` |
| `api_key:list` | `api_key` | null | null |
| `api_key:revoke` | `api_key` | `{name, scopes}` | `{name, scopes, revoked_at}` |
| `api_key:authenticate` | `api_key` | null | `{key_id, last_used_at}` |

**Note**: Key value (hash) is NEVER logged, only key ID and prefix.

---

## Dependencies Resolved

| Dependency | Resolution |
|------------|------------|
| Bcrypt | Already used for password hashing (`golang.org/x/crypto/bcrypt`) |
| UUID | Already used (`github.com/google/uuid`) |
| GORM | Already used (`gorm.io/gorm`) |
| JSONB | Already used (`gorm.io/datatypes`) |
| Casbin | Already integrated for permissions |

---

## Technical Decisions

1. **Key Hash Algorithm**: bcrypt with cost 12 (same as passwords)
2. **Key Generation**: crypto/rand with 32 alphanumeric characters
3. **Key Prefix**: 12 chars for identification without exposing full key
4. **Scope Format**: `resource:action` (simpler than Casbin model)
5. **Middleware Position**: After JWT extraction, before permission check
6. **Rate Limiting**: Per API key with lower limits than user sessions
7. **Expiration**: Optional ExpiresAt column, checked on every auth
8. **Soft Delete**: RevokedAt for revocation, DeletedAt cascades from user

---

## Alternatives Considered

### Alternative 1: Store Keys in Plaintext
**Rejected**: Security risk if DB compromised. bcrypt is standard practice.

### Alternative 2: Use JWT for Service Auth
**Rejected**: 
- JWTs have expiration built-in
- Keys are simpler for scripts/background jobs
- Keys can be revoked immediately (JWT needs blacklisting)
- Keys don't require clock synchronization

### Alternative 3: Full Casbin Permissions for API Keys
**Rejected**:
- Overkill for limited scopes
- Harder to explain to developers
- Scope model (resource:action) is simpler
- Still integrates with Casbin for validation

### Alternative 4: Single API Key Per User
**Rejected**:
- Developers need multiple keys for different integrations
- No way to rotate keys without breaking all integrations
- No way to name/describe key purpose

---

## Open Questions (Resolved)

| Question | Decision |
|-----------|----------|
| Can API keys create other API keys? | ❌ No, only JWT-authenticated users |
| Can API keys manage users/roles? | ❌ No, only JWT-authenticated admins |
| Should LastUsedAt be updated synchronously? | ✅ Yes, for security monitoring |
| Should we show key creation date? | ✅ Yes, for user awareness |
| Should we limit number of keys per user? | ⚠️ Future consideration |

---

## Implementation Notes

1. **Copy key value to clipboard on creation** - Frontend responsibility, not backend
2. **Prefix for identification** - Helps users identify which key was used
3. **Scopes as JSON array** - Flexible, can add metadata in future
4. **No hard delete** - Audit compliance requires soft delete
5. **Rate limit headers** - Expose X-RateLimit-* headers for API key users

---

**Research Complete**: All questions resolved, ready for data model design.
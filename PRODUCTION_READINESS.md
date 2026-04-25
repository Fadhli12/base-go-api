# Production Readiness Checklist - Go API Base

**Generated:** 2026-04-28
**Last Updated:** 2026-04-28

---

## Executive Summary

This document provides a comprehensive production-readiness assessment for the Go API Base codebase. Issues are organized by severity and module.

### Critical Issues (Must Fix Before Production)

| ID | Module | Issue | Status |
|----|--------|-------|--------|
| CRIT-001 | Auth | JWT Secret has insecure default value | ✅ Fixed (2026-04-28) |
| CRIT-002 | Auth | Password reset token never persisted | ✅ Fixed (2026-04-28) |

### High Priority Issues (Should Fix)

| ID | Module | Issue | Status |
|----|--------|-------|--------|
| HIGH-001 | Auth | Missing auth-specific rate limiting | ✅ Fixed (2026-04-28) |
| HIGH-002 | Auth | No auth operation audit logging | ✅ Fixed (2026-04-28) |
| HIGH-003 | Auth | Missing JWT claims (issuer, audience) | ✅ Fixed (2026-04-28) |
| HIGH-004 | Auth | Inconsistent refresh token hashing | ✅ Fixed (2026-04-28) |
| HIGH-005 | Auth | No password strength validation | ✅ Fixed (2026-04-28) |

### Medium Priority Issues (Recommended)

| ID | Module | Issue | Status |
|----|--------|-------|--------|
| MED-001 | Auth | Timing attack on user enumeration | ✅ Fixed (2026-04-28) |
| MED-002 | HTTP | CORS misconfiguration | ✅ Fixed (2026-04-28) |
| MED-003 | HTTP | Missing security headers | ✅ Fixed (2026-04-28) |
| MED-004 | Auth | No refresh token family tracking | ✅ Fixed (2026-04-28) |
| MED-005 | Auth | Unlimited active sessions | ✅ Fixed (2026-04-28) |
| MED-006 | Auth | No auth-specific observability | ✅ Fixed (2026-04-28) |

---

## 1. Authentication Module

### 1.1 CRITICAL Issues

#### CRIT-001: Insecure JWT Secret Default

**Location:** `internal/config/config.go:452`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Default JWT secret was a hardcoded string. If `JWT_SECRET` was not set, application started with this weak secret, allowing token forgery.

**Fix Applied:**
```go
// Before (INSECURE):
cfg.JWT.Secret = getEnvOrDefault("JWT_SECRET", "change-me-in-production")

// After (SECURE):
cfg.JWT.Secret = os.Getenv("JWT_SECRET")
// JWT_SECRET is required - application fails to start without it
```

Validation now returns a clear error:
```
missing required configuration: [JWT_SECRET (set a secure random string of at least 32 characters)]
```

**Impact:** Application now refuses to start without a properly configured JWT_SECRET, preventing production deployments with insecure defaults.

---

#### CRIT-002: Password Reset Token Never Persisted

**Location:** `internal/service/auth.go:276-318` (original)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Password reset tokens were generated but never stored. The function returned the raw token without any database record to validate against. Users could not actually reset passwords.

**Impact:** Password reset functionality was completely broken.

**Fix Applied:**
1. Created `internal/domain/password_reset_token.go` - Domain entity with `IsValid()`, `IsUsed()`, `IsExpired()` methods
2. Created `migrations/000009_password_reset_tokens.up.sql` - PostgreSQL table with proper indexes
3. Created `internal/repository/password_reset_token.go` - Repository interface and implementation
4. Updated `internal/service/auth.go`:
   - `RequestPasswordReset()` now persists token hash in database
   - `ResetPassword()` validates token, marks used, revokes all refresh tokens
   - Tokens are hashed with SHA256 (fast, appropriate for random tokens)
5. Added `PasswordResetConfig` to config for token expiry (default: 1 hour)
6. Added password reset endpoints in handler

**New Endpoints:**
- `POST /api/v1/auth/password-reset` - Request password reset email
- `POST /api/v1/auth/password-reset/confirm` - Confirm password reset with token

---

### 1.2 HIGH Issues

#### HIGH-001: Missing Auth-Specific Rate Limiting

**Location:** `internal/http/middleware/auth_rate_limit.go` (new file)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Global rate limiting (100 req/min per IP) applies to all endpoints uniformly. Auth endpoints need dedicated, stricter limits.

**Impact:** Brute force attacks, credential stuffing, account enumeration possible.

**Fix Applied:**
Created `AuthRateLimiter` with endpoint-specific limits:
- Login: 5 attempts per 15 min per IP+email
- Register: 5 per hour per IP
- Password reset: 3 per hour per email

**Usage:**
```go
authRateLimiter := middleware.NewAuthRateLimiter(cacheDriver, middleware.DefaultAuthRateLimiterConfig())
auth.POST("/login", authRateLimiter.LoginRateLimit()(authHandler.Login))
```

---

#### HIGH-002: No Auth Operation Audit Logging

**Location:** `internal/http/middleware/audit.go:35-36` (original skip)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Auth endpoints (`/auth/login`, `/auth/refresh`) explicitly excluded from audit logging. Security-sensitive operations invisible for forensic analysis.

**Impact:** No forensic evidence of brute force attacks, cannot track account compromise timeline, compliance violations.

**Fix Applied:**
1. Created `internal/service/auth_audit.go` with audit methods:
   - `AuditLoginSuccess()` - logs successful login
   - `AuditLoginFailure()` - logs failed login with reason
   - `AuditPasswordResetRequest()` - logs password reset request
   - `AuditPasswordResetComplete()` - logs successful password reset
2. Updated `AuthService` to include audit service dependency
3. Added audit constants: `AuditActionLoginFailed`, `AuditActionPasswordReset`, `AuditActionPasswordChange`
4. Wired audit service in server DI

**Implementation Note:** Audit logging happens asynchronously (goroutine) and does not block auth operations.

---

#### HIGH-003: Missing JWT Claims

**Location:** `internal/auth/jwt.go:18-38`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** JWTs lack `iss` (issuer) and `aud` (audience) claims.

**Impact:** Tokens can be reused across different services. Open to confused deputy attacks.

**Fix Applied:**
Added `issuer` and `audience` to JWT claims:
- New config options: `JWT_ISSUER`, `JWT_AUDIENCE` (defaults: "go-api-base", "api")
- New function: `GenerateAccessTokenWithClaims()` with iss/aud parameters
- Backward compatible: `GenerateAccessToken()` calls new function with empty strings
- TokenService updated to pass issuer/audience from config

---

#### HIGH-004: Inconsistent Refresh Token Hashing

**Location:** `internal/service/auth.go:49` (original), `internal/auth/token_hash.go` (new)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Two different hashing approaches:
- `auth.go` uses bcrypt
- `token_service.go` uses SHA256 (implicitly)

**Impact:** Token validation may fail depending on which path is used.

**Fix Applied:**
- Created `internal/auth/token_hash.go` with `HashToken()` and `VerifyToken()` functions using SHA256
- SHA256 is appropriate for randomly-generated tokens (unlike passwords which need bcrypt)
- All token hashing now uses `auth.HashToken()` consistently
- Refresh tokens and password reset tokens both use SHA256

---

#### HIGH-005: No Password Strength Validation

**Location:** `internal/http/request/auth.go:12` (original)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Password validation only checks length (8-72 chars). No complexity requirements.

**Impact:** Users can set weak passwords like "password", "12345678".

**Fix Applied:**
Added custom validator `password_strength`:
- Minimum 8 characters
- At least 1 uppercase letter
- At least 1 lowercase letter
- At least 1 number
- At least 1 special character
- Maximum 72 characters (bcrypt limit)

Applied to: `RegisterRequest.Password`, `PasswordResetConfirmRequest.Password`

---

### 1.3 MEDIUM Issues

#### MED-001: Timing Attack on User Enumeration

**Location:** `internal/service/auth.go:167-170` (original), `internal/service/auth.go:163-178` (fixed)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Error message is generic (good), but timing difference exists:
- Existing email + wrong password: bcrypt verify runs (~250ms)
- Non-existing email: early return (~1ms)

**Impact:** Attackers can enumerate valid email addresses by measuring response times.

**Fix Applied:**
Always run bcrypt verification for non-existent users using a dummy hash:
```go
if errors.Is(err, apperrors.ErrNotFound) {
    // Timing attack mitigation: always run bcrypt for non-existent users
    s.passwordHasher.Verify("$2a$12$dummyhash...", req.Password)
    return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
}
```

---

#### MED-002: CORS Misconfiguration

**Location:** `internal/http/middleware/cors.go:38, 50-52`

```go
if origin == "" {
    return "*"  // Allows any origin for requests without Origin header
}
```

**Issue:** Returns `*` wildcard for missing Origin header. Falls back to first allowed origin if not in list.

**Remediation:**
- Reject requests with no Origin in credential mode
- Return 403 for unauthorized origins
- Add `Vary: Origin` header

---

#### MED-003: Missing Security Headers

**Location:** `internal/http/middleware/security_headers.go` (new file)

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** No middleware adds security headers:
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security` (HSTS)
- `Content-Security-Policy`
- `Referrer-Policy`
- `Permissions-Policy`

**Fix Applied:**
Created `SecurityHeaders` middleware with:
- `DefaultSecurityHeadersConfig()` - sensible defaults
- `SecurityHeadersWithDefaults()` - development-friendly
- `ProductionSecurityHeaders(domain)` - production-ready with HSTS

**Usage:**
```go
e.Use(middleware.SecurityHeadersWithDefaults())
// Or for production:
e.Use(middleware.ProductionSecurityHeaders("example.com"))
```

---

#### MED-002: CORS Misconfiguration

**Location:** `internal/http/middleware/cors.go`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** CORS returned wildcard `*` when no Origin provided, leaked origin list to unauthorized requests.

**Fix Applied:**
1. **Origin rejection:** Returns 403 for unauthorized origins instead of silently allowing or leaking origin list
2. **No-Origin handling:** For browser requests without Origin (e.g., API tools), the request proceeds but without CORS headers
3. **Vary header:** Added `Vary: Origin` header for proper caching
4. **Subdomain support:** Added support for `*.example.com` patterns

**Usage:**
```go
// Configure allowed origins in config
cfg.CORS.AllowedOrigins = []string{
    "https://app.example.com",
    "https://admin.example.com",
    "*.example.com", // Subdomain wildcard
}

// Middleware rejects unauthorized origins with 403
e.Use(middleware.CORS(cfg))
```

---

#### MED-004: No Refresh Token Family Tracking

**Location:** `internal/domain/refresh_token.go`, `internal/service/auth.go`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** No token family tracking. Cannot detect token theft when attacker steals refresh token. When a token is used twice, there's no way to detect replay attacks.

**Fix Applied:**
1. **Token family tracking:** Added `FamilyID` field to `RefreshToken` model - all tokens in a rotation chain share the same family ID
2. **Migration:** Created `migrations/000010_refresh_token_family.up.sql` to add FamilyID column with index
3. **Rotation detection:** When a revoked token is used:
   - Revoke entire token family (all sessions in that chain)
   - Audit the potential attack with `AuditActionTokenReuse`
   - Return 401 error to prevent attacker access
4. **Session continuity:** New refresh tokens inherit the same `FamilyID` during rotation, maintaining the chain

**Security Impact:**
- Attacker who steals refresh token and uses it will trigger family revocation
- Legitimate user's next refresh attempt will fail (detected compromise)
- User can re-authenticate with credentials

---

#### MED-005: Unlimited Active Sessions

**Location:** `internal/domain/refresh_token.go`, `internal/repository/refresh_token.go`, `internal/service/auth.go`, `internal/http/handler/session.go`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** Each login creates new refresh token without limit. Users have unlimited concurrent sessions with no visibility or control.

**Fix Applied:**
1. **Session metadata:** Added to `RefreshToken`:
   - `UserAgent` - Browser/client identification
   - `IPAddress` - Login IP address
   - `DeviceName` - User-friendly device name

2. **Migration:** Created `migrations/000011_session_metadata.up.sql` to add metadata columns

3. **Session endpoints:** Created `internal/http/handler/session.go`:
   - `GET /api/v1/sessions` - List all active sessions
   - `DELETE /api/v1/sessions/:id` - Revoke specific session
   - `DELETE /api/v1/sessions/others` - Revoke all other sessions

4. **Service methods:** Added to `AuthService`:
   - `GetActiveSessions(ctx, userID)` - Filter valid sessions
   - `RevokeSession(ctx, userID, sessionID)` - Revoke by ID
   - `RevokeAllOtherSessions(ctx, userID, currentID)` - Revoke all except current

5. **Current session detection:** JWT middleware sets `tokenID` in context for current session identification

**Usage:**
```bash
# List active sessions
curl -H "Authorization: Bearer $ACCESS_TOKEN" /api/v1/sessions

# Response:
{
  "data": [
    {
      "id": "uuid-1",
      "device_name": "Chrome on Windows",
      "user_agent": "Mozilla/5.0...",
      "ip_address": "192.168.1.1",
      "created_at": "2026-04-28T10:00:00Z",
      "expires_at": "2026-05-28T10:00:00Z",
      "is_current": true
    }
  ]
}

# Revoke other session
curl -X DELETE -H "Authorization: Bearer $ACCESS_TOKEN" /api/v1/sessions/uuid-2
```

---

#### MED-006: No Auth-Specific Observability

**Location:** `internal/service/auth_metrics.go`, `internal/http/handler/metrics.go`

**Status:** ✅ Fixed (2026-04-28)

**Original Issue:** No metrics for failed login attempts, password resets, token refreshes. Cannot monitor auth-related security events in production.

**Fix Applied:**
1. **Auth metrics service:** Created `AuthMetrics` service with atomic counters:
   - `LoginSuccess` / `LoginFailed` / `LoginRateLimited`
   - `PasswordResetRequested` / `PasswordResetCompleted` / `PasswordResetFailed`
   - `TokenRefreshSuccess` / `TokenRefreshFailed` / `TokenReuseDetected`
   - `ActiveSessions` / `SessionRevoked`

2. **Integrated into auth flow:**
   - Login success/failure tracked
   - Token refresh success/failure tracked
   - Token reuse attacks logged
   - Password reset requests/completions tracked
   - Session revocations tracked

3. **Metrics endpoint:** Created `GET /api/v1/metrics/auth`:
   ```json
   {
     "data": {
       "login_success": 1234,
       "login_failed": 56,
       "login_rate_limited": 5,
       "password_reset_requested": 23,
       "password_reset_completed": 20,
       "password_reset_failed": 3,
       "token_refresh_success": 987,
       "token_refresh_failed": 12,
       "token_reuse_detected": 0,
       "active_sessions": 89,
       "session_revoked": 15
     }
   }
   ```

4. **Global metrics instance:** `GetAuthMetrics()` provides singleton access

**Monitoring Recommendations:**
- Alert on `token_reuse_detected > 0` (potential attack)
- Alert on `login_rate_limited` spikes (brute force attempt)
- Monitor `password_reset_failed` to detect token guessing attacks
- Track `login_failed` / `login_success` ratio for anomaly detection

---

### 1.4 LOW Issues

| ID | Issue | Impact |
|----|-------|--------|
| LOW-001 | JWT secret length warning only | App starts with weak secret |
| LOW-002 | Generic token ID generation | Predictable token IDs |
| LOW-003 | No token blacklisting after password change | Old tokens valid until expiry |
| LOW-004 | Email case sensitivity not normalized | Duplicate registrations possible |
| LOW-005 | Verbose errors in debug mode | May leak internal details |

---

## 2. RBAC/Permission Module

### 2.1 Configuration Issues

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| RBAC-001 | HIGH | Permission sync after role changes | ⚠️ Verify |
| RBAC-002 | MEDIUM | Cache invalidation on permission update | ⚠️ Verify |
| RBAC-003 | LOW | Soft delete filter in JOINs | ✅ Fixed |

### 2.2 Checklist

- [ ] Verify Casbin policy sync after role assignment
- [ ] Verify cache invalidation on permission changes
- [ ] Test concurrent permission checks for race conditions
- [ ] Verify permission hierarchy inheritance
- [ ] Test permission denial messages don't leak information

---

## 3. Organization Module

### 3.1 Multi-tenant Security

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| ORG-001 | HIGH | Organization context middleware | ⚠️ Verify |
| ORG-002 | MEDIUM | Invitation token security | ⚠️ Verify |
| ORG-003 | MEDIUM | Member role escalation prevention | ⚠️ Verify |

### 3.2 Checklist

- [ ] Verify X-Organization-ID header validation
- [ ] Test organization isolation (data leakage between orgs)
- [ ] Verify invitation token expiry
- [ ] Test member removal cascades
- [ ] Verify owner cannot remove themselves as last owner

---

## 4. Email Service Module

### 4.1 Security

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| EMAIL-001 | MEDIUM | Email template XSS prevention | ✅ Implemented |
| EMAIL-002 | MEDIUM | Rate limiting per recipient | ⚠️ Verify |
| EMAIL-003 | LOW | Bounce suppression list | ⚠️ Verify |

### 4.2 Checklist

- [ ] Template rendering escapes HTML properly
- [ ] Email queue rate limiting per recipient
- [ ] Bounce suppression prevents re-sending to bounced addresses
- [ ] Webhook signature verification for SendGrid/SES
- [ ] Email content sanitization

---

## 5. HTTP/Security Layer

### 5.1 Middleware Security

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| HTTP-001 | HIGH | No rate limiting per endpoint | 🔴 Not Fixed |
| HTTP-002 | HIGH | Audit middleware skips auth endpoints | 🔴 Not Fixed |
| HTTP-003 | MEDIUM | Missing security headers | 🔴 Not Fixed |
| HTTP-004 | MEDIUM | CORS configuration issues | 🔴 Not Fixed |

### 5.2 Security Headers

**Current State:** No security headers middleware.

**Required Headers:**
```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
```

---

## 6. Database/Migrations

### 6.1 Issues

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| DB-001 | MEDIUM | Missing foreign key constraints | ⚠️ Verify |
| DB-002 | LOW | Index optimization | ⚠️ Verify |

### 6.2 Checklist

- [ ] All foreign keys have proper constraints
- [ ] Indexes on frequently queried columns
- [ ] Soft delete filters consistent across queries
- [ ] Transaction boundaries properly defined
- [ ] Connection pool settings appropriate for production

---

## 7. Error Handling

### 7.1 Issues

| ID | Severity | Issue | Status |
|----|----------|-------|--------|
| ERR-001 | LOW | Error message consistency | ⚠️ Verify |

### 7.2 Checklist

- [ ] All errors use AppError wrapper
- [ ] Error codes consistent across modules
- [ ] No sensitive data in error messages
- [ ] Request ID included in all errors
- [ ] Panic recovery middleware in place ✅

---

## 8. Deployment Checklist

### 8.1 Environment Variables

**Required:**
- [ ] `DATABASE_URL` - PostgreSQL connection string
- [ ] `REDIS_URL` - Redis connection string  
- [ ] `JWT_SECRET` - At least 32 characters (**CRITICAL: no default!**)
- [ ] `STORAGE_SIGNING_KEY` - Media signing key

**New (for production):**
- [ ] `JWT_ISSUER` - Token issuer (default: "go-api-base")
- [ ] `JWT_AUDIENCE` - Token audience (default: "api")
- [ ] `PASSWORD_RESET_TOKEN_EXPIRY` - Password reset expiry (default: "1h")

**Optional with Defaults:**
- [ ] `EMAIL_PROVIDER` - smtp/sendgrid/ses
- [ ] `EMAIL_SMTP_HOST` - SMTP server
- [ ] `EMAIL_SMTP_PORT` - SMTP port
- [ ] `CACHE_DRIVER` - redis/memory

### 8.2 Database

- [ ] Run migrations: `make migrate`
- [ ] Seed roles/permissions: `make seed`
- [ ] Verify indexes exist
- [ ] Set up connection pool monitoring

### 8.3 Redis

- [ ] Configure max memory policy
- [ ] Set up persistence (AOF/RDB)
- [ ] Monitor memory usage

### 8.4 Monitoring

- [ ] Structured logging configured
- [ ] Request ID propagation ✅
- [ ] Health checks: `/healthz`, `/readyz` ✅
- [ ] Graceful shutdown (30s timeout) ✅

### 8.5 Security

- [ ] HTTPS enforced
- [ ] Rate limiting enabled
- [ ] CORS configured for production domains
- [ ] Security headers added
- [ ] JWT expiry times appropriate (15m access, 30d refresh)
- [ ] Secrets management (not in code)

---

## 9. Testing

### 9.1 Coverage

| Module | Unit Tests | Integration Tests |
|--------|-----------|-------------------|
| Auth | ✅ | ✅ |
| RBAC | ✅ | ✅ |
| Organization | ✅ | ✅ |
| Email | ✅ | ✅ |
| HTTP Layer | ✅ | ✅ |

### 9.2 Test Gaps

- [ ] Concurrency tests for race conditions
- [ ] Load tests for rate limiting
- [ ] Security penetration tests
- [ ] Timing attack tests

---

## 10. Action Items Summary

### Immediate (Block Production) - ✅ ALL FIXED

1. **CRIT-001:** ~~Remove default JWT_SECRET, require environment variable~~ ✅ **FIXED** (2026-04-28)
2. **CRIT-002:** ~~Implement password reset token storage~~ ✅ **FIXED** (2026-04-28)

### Short Term (1-2 Weeks) - ✅ ALL FIXED

1. **HIGH-001:** ~~Add auth-specific rate limiting~~ ✅ **FIXED** (2026-04-28)
2. **HIGH-002:** ~~Enable audit logging for auth operations~~ ✅ **FIXED** (2026-04-28)
   - Created `AuthAudit` service methods for login success/failure, password reset
   - Auth service now includes audit service dependency
3. **HIGH-003:** ~~Add JWT issuer/audience claims~~ ✅ **FIXED** (2026-04-28)
4. **HIGH-004:** ~~Standardize refresh token hashing~~ ✅ **FIXED** (2026-04-28)
5. **HIGH-005:** ~~Add password strength validation~~ ✅ **FIXED** (2026-04-28)

### Medium Term (3-4 Weeks) - Partial

1. **MED-001:** ~~Fix timing attack in user lookup~~ ✅ **FIXED** (2026-04-28)
2. **MED-002:** Fix CORS configuration 🔴 **TODO**
3. **MED-003:** ~~Add security headers middleware~~ ✅ **FIXED** (2026-04-28)
4. **MED-004:** Implement token family tracking 🔴 **TODO**
5. **MED-005:** Add session management 🔴 **TODO**
6. **MED-006:** Add auth metrics and monitoring 🔴 **TODO**

### Ongoing

1. Add automated security tests
2. Penetration testing
3. Monitor auth failure rates
4. Regular dependency updates
5. Security audit schedule

### Ongoing

1. Add automated security tests
2. Penetration testing
3. Monitor auth failure rates
4. Regular dependency updates
5. Security audit schedule

---

## Document History

| Date | Author | Changes |
|------|--------|---------|
| 2026-04-28 | Sisyphus | Initial production readiness assessment |
| 2026-04-28 | Sisyphus | Fixed CRIT-001: Removed insecure JWT_SECRET default, made it required |
| 2026-04-28 | Sisyphus | Fixed CRIT-002: Implemented password reset token storage with migration |
| 2026-04-28 | Sisyphus | Fixed HIGH-001: Added auth-specific rate limiting middleware |
| 2026-04-28 | Sisyphus | Fixed HIGH-002: Added explicit auth event audit logging |
| 2026-04-28 | Sisyphus | Fixed HIGH-003: Added JWT issuer/audience claims |
| 2026-04-28 | Sisyphus | Fixed HIGH-004: Standardized token hashing to SHA256 |
| 2026-04-28 | Sisyphus | Fixed HIGH-005: Added password strength validation |
| 2026-04-28 | Sisyphus | Fixed MED-001: Fixed timing attack in user enumeration |
| 2026-04-28 | Sisyphus | Fixed MED-003: Added security headers middleware |
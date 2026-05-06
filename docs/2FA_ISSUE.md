# Two-Factor Authentication (2FA) - Build Error

## Problem Summary

The `internal/service/two_factor.go` file has multiple compilation errors that block `go build ./internal/service/` and `go build ./cmd/api/`.

## Errors

| Error | Location | Cause |
|-------|----------|-------|
| `undefined: TokenService` | two_factor.go:23 | Type not defined in service package |
| `token.Is2FAPending` (undefined field) | two_factor.go:278, 376 | RefreshToken struct missing field |
| `s.auditSvc.LogAction` (wrong args) | two_factor.go:161, 255, 344, 449, 533 | Call passes 1 arg, signature requires 9 |

## Root Cause

The `two_factor.go` service was written expecting:

1. **User domain to have TwoFactor fields** (`TwoFactorEnabled`, `TwoFactorStatus`, `TwoFactorSecret`, `TwoFactorVerifiedAt`)
2. **TokenService type** defined in the service package (instead of imported from `auth` package)
3. **RefreshToken domain to have `Is2FAPending` bool field**
4. **AuditService.LogAction** to accept a `*domain.AuditLog` struct (instead of individual parameters)

## What Was Fixed (2026-05-07)

Added to `internal/domain/user.go`:

```go
TwoFactorEnabled   bool       `gorm:"default:false"`
TwoFactorStatus     string    `gorm:"size:20;default:'disabled'"`
TwoFactorSecret     string    `gorm:"size:255"`
TwoFactorVerifiedAt *time.Time
```

## What Remains

### 1. TokenService Type Missing

**Location:** `two_factor.go:23, 38`

The service struct references `*TokenService` which is not defined in the service package.

```go
tokenService *TokenService  // line 23
```

**Fix Required:** Either:
- Import `TokenService` from `internal/auth` package and use it as `*auth.TokenService`
- Or define `TokenService` interface in the service package

### 2. RefreshToken.Is2FAPending Missing

**Location:** `two_factor.go:278, 376, 337, 442`

The code references `token.Is2FAPending` but `domain.RefreshToken` does not have this field.

Current `RefreshToken` struct fields:
- `ID`, `UserID`, `TokenHash`, `FamilyID`, `ExpiresAt`, `RevokedAt`, `CreatedAt`, `UserAgent`, `IPAddress`, `DeviceName`

**Fix Required:** Add `Is2FAPending bool` field to `internal/domain/refresh_token.go`

### 3. AuditService.LogAction Signature Mismatch

**Location:** Multiple places in two_factor.go

The current calls in `two_factor.go` pass a single `*domain.AuditLog` struct:

```go
s.auditSvc.LogAction(ctx, &domain.AuditLog{
    ActorID:    userID,
    Action:     "disable_2fa",
    // ...
})
```

But the actual `AuditService.LogAction` signature (from `internal/service/audit.go:58-68`) expects 9 separate arguments:

```go
func (s *AuditService) LogAction(
    ctx context.Context,
    actorID uuid.UUID,
    action string,
    resource string,
    resourceID string,
    before interface{},
    after interface{},
    ipAddress string,
    userAgent string,
) error
```

**Fix Required:** Change all `LogAction` calls in `two_factor.go` to use individual arguments, e.g.:

```go
s.auditSvc.LogAction(ctx, userID, "disable_2fa", "user", userID.String(), nil, nil, "", "")
```

Or consider adding an overloaded method/variant that accepts `*domain.AuditLog`.

## Impact

- `go build ./internal/service/` fails
- `go build ./cmd/api/` fails
- No 2FA functionality available until resolved

## Fix Required

1. Add `Is2FAPending bool` to `internal/domain/refresh_token.go`
2. Import `TokenService` from `internal/auth` or create appropriate type
3. Fix all `LogAction` calls in `two_factor.go` to pass 9 arguments
4. Run `go build ./internal/service/` to verify all errors resolved
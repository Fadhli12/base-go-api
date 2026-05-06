# Two-Factor Service Task Learnings

## Task: Fix Verify2FALogin and UseRecoveryCode Method Signatures

### Changes Made

1. **Added `tokenService` dependency** to `TwoFactorService` struct
2. **Changed method signatures** to accept `(ctx, pendingToken, totpCode/code)` instead of `(ctx, userID, pendingToken, totpCode/code)`
3. **Updated implementations** to:
   - Hash the pending token and look it up via `refreshTokenRepo.FindByHash(ctx, tokenHash)`
   - Validate `token.Is2FAPending == true` and token not expired
   - Extract `userID` from `token.UserID`
   - Proceed with existing logic using real `TokenService` for token generation
   - Revoke the pending token via `refreshTokenRepo.MarkRevoked(ctx, tokenHash)`
   - Issue new refresh token (not 2FA pending) via `refreshTokenRepo.Create`

### Implementation Notes

- The pending token IS the refresh token with `is_2fa_pending=true`
- Handler doesn't pass userID - service extracts it from the token
- After successful 2FA, pending token is revoked and new real tokens are issued
- Used `auth.HashToken(pendingToken)` to hash token before lookup (same pattern as auth.go)

### Files Modified

- `internal/service/two_factor.go`

### Tests Need Update

- `tests/unit/two_factor_service_test.go` - `NewTwoFactorService` call needs `tokenService` parameter
- `tests/integration/two_factor_test.go` - same issue

### Related Files

- `internal/auth/token_hash.go` - contains `HashToken` function
- `internal/repository/refresh_token.go` - `FindByHash`, `MarkRevoked`, `Create` methods
- `internal/service/auth.go` - `TokenService` type and usage pattern

---

## Session 2026-05-05: two_factor.go API Fixes

### Issues Fixed:

1. **totp.GenerateOptions → totp.GenerateOpts** (lines 71, 186)
2. **key.RawSecret() → key.Secret()** (line 82) - returns string, not []byte
3. **totp.Validate → totp.ValidateCustom** with proper error handling (lines 133, 246, 438) - ValidateCustom returns (bool, error)
4. **AuditLog field names fixed**: ActorID not UserID, IPAddress not SourceIP, removed Status/MetadataJSON
5. **LogAction signature** changed from 9 params to (*domain.AuditLog) - updated all callers

### AuditLog Domain Change:

- Added BeforeJSON and AfterJSON (datatypes.JSON) fields for GORM persistence
- Changed Before/After from datatypes.JSON to interface{} with gorm:"-" for caller convenience
- LogAction marshals interface{} to JSON bytes and stores in *JSON fields

### Pre-existing Bugs (NOT fixed per task constraints):

- VerifyAndEnable (lines 137-165): Logic bug - disables 2FA instead of enabling
- token.IsExpired() doesn't exist (lines 221, 319): Should be token.IsValid()
- int vs int64 in LoginResult.ExpiresIn (lines 299, 406)

### Files Modified:

- internal/domain/audit_log.go
- internal/service/audit.go
- internal/service/auth_audit.go
- internal/service/api_key.go
- internal/service/organization.go
- internal/service/two_factor.go
# Tasks: Two-Factor Authentication (2FA)

**Input**: Design documents from `/specs/007-two-factor-auth/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)
**Branch**: `007-two-factor-auth`

**Tests**: Unit tests with testify/mock for service layer. Integration tests using testcontainers (ephemeral Postgres+Redis) per existing project pattern.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Domain entities: `internal/domain/`
- Repositories: `internal/repository/`
- Services: `internal/service/`
- HTTP handlers: `internal/http/handler/`
- HTTP request DTOs: `internal/http/request/`
- HTTP response DTOs: `internal/http/response/`
- Config: `internal/config/`
- Migrations: `migrations/`
- Error codes: `pkg/errors/`
- Unit tests: `tests/unit/`
- Integration tests: `tests/integration/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Dependency, config, and error code scaffolding.

- [ ] T001 Add `github.com/pquerna/otp` v1.4.0 dependency in `go.mod` — `go get github.com/pquerna/otp@v1.4.0 && go mod tidy`
- [ ] T002 [P] Add `TWO_FACTOR_ENCRYPTION_KEY` env var to `internal/config/config.go` — 32-byte base64-encoded AES-GCM key with Viper binding and validation (must not be empty in production)
- [ ] T003 [P] Create 9 2FA error sentinel variables in `pkg/errors/errors.go` per plan.md §2FA Error Codes: `ErrTwoFactorRequired` (403), `ErrInvalidTOTPCode` (401), `ErrTOTPSessionExpired` (401), `Err2FAAlreadyEnabled` (409), `Err2FANotEnabled` (400), `ErrRecoveryCodeInvalid` (401), `ErrRecoveryCodesExhausted` (400), `ErrRecoveryCodeRegenLimit` (429), `ErrTOTPSecretDecrypt` (500). All use `NewAppError()` pattern matching existing sentinel conventions
- [ ] T004 [P] Add `2fa:enable`, `2fa:disable`, `2fa:verify` permissions to seed data and `permission:sync` manifest in `cmd/api/main.go` — follow existing `permission:sync` pattern (manifest is a Go map, sync uses Casbin adapter upsert)

**Checkpoint**: `go get` succeeds, config loads, error codes compile, `go run ./cmd/api permission:sync` adds 2FA permissions

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Database schema, domain entities, repositories, and core service logic that ALL user stories depend on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Create migration `migrations/000015_two_factor_auth.up.sql` — (1) ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN DEFAULT false, ADD COLUMN two_factor_secret TEXT, ADD COLUMN two_factor_status VARCHAR(20) DEFAULT 'disabled', ADD COLUMN two_factor_verified_at TIMESTAMPTZ; (2) ALTER TABLE refresh_tokens ADD COLUMN is_2fa_pending BOOLEAN DEFAULT false; (3) CREATE TABLE two_factor_recovery_codes (id UUID PK DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, code_hash TEXT NOT NULL UNIQUE, used_at TIMESTAMPTZ, created_at TIMESTAMPTZ DEFAULT NOW(), deleted_at TIMESTAMPTZ)
- [ ] T006 [P] Create migration `migrations/000015_two_factor_auth.down.sql` — (1) DROP TABLE two_factor_recovery_codes; (2) ALTER TABLE refresh_tokens DROP COLUMN is_2fa_pending; (3) ALTER TABLE users DROP COLUMN two_factor_verified_at, DROP COLUMN two_factor_status, DROP COLUMN two_factor_secret, DROP COLUMN two_factor_enabled
- [ ] T007 [P] Create `internal/domain/two_factor.go` — Types: `TwoFactorStatus` constants (`disabled`, `pending`, `enabled`), `TwoFactorSetupResponse` (Secret, QRCodeURL, RecoveryCodes), `TwoFactorVerifyRequest` (UserID, TOTPCode, Secret), `TwoFactorRecoveryRequest` (UserID, Code). Also include `LoginResult` struct from plan.md line 165-172 (User, AccessToken, RefreshToken, Requires2FA, PendingToken, ExpiresIn)
- [ ] T008 [P] Create `internal/domain/two_factor_recovery_code.go` — `TwoFactorRecoveryCode` entity: ID (uuid.UUID), UserID (uuid.UUID), CodeHash (string), UsedAt (*time.Time), CreatedAt (time.Time), DeletedAt (gorm.DeletedAt). Implement `TableName()` returning `"two_factor_recovery_codes"` and `ToResponse()` method. Follow existing entity conventions (UUID PK, soft delete, JSON tags)
- [ ] T009 [P] Extend `internal/domain/user.go` — Add 4 fields to `User` struct: `TwoFactorEnabled bool`, `TwoFactorSecret string` (json:"-"), `TwoFactorStatus string`, `TwoFactorVerifiedAt *time.Time`. Add to `ToResponse()` method (exclude `TwoFactorSecret`, include others)
- [ ] T010 Create `internal/repository/two_factor_recovery_code.go` — `TwoFactorRecoveryCodeRepository` interface + GORM implementation: `Create(ctx, *TwoFactorRecoveryCode) error`, `FindByUserAndHash(ctx, userID uuid.UUID, codeHash string) (*TwoFactorRecoveryCode, error)` (returns unused codes only), `MarkUsed(ctx, id uuid.UUID) error` (SET used_at=NOW()), `DeleteAllByUser(ctx, userID uuid.UUID) error` (soft delete), `CountUnusedByUser(ctx, userID uuid.UUID) (int64, error)`. All methods use `WithContext(ctx)` and scope to non-deleted
- [ ] T011 [P] Extend `internal/repository/refresh_token.go` — Add `Is2FAPending bool` field to `RefreshToken` struct/model. Add `FindPending2FAToken(ctx, userID uuid.UUID) (bool, error)` method to check if any pending 2FA token exists for user. Add `InvalidatePendingTokens(ctx, userID uuid.UUID) error` to revoke all pending 2FA tokens for a user (needed for new login invalidation per edge cases)
- [ ] T012 [P] Extend `internal/repository/user.go` — Add `UpdateTwoFactorFields(ctx, userID uuid.UUID, fields map[string]interface{}) error` method for atomic update of 2FA columns. Add `FindWithTwoFactorStatus(ctx, userID uuid.UUID) (*domain.User, error)` — returns user with `TwoFactorEnabled`, `TwoFactorStatus`, `TwoFactorSecret` (decrypted by caller)
- [ ] T013 Create `internal/service/two_factor_crypto.go` — AES-GCM encrypt/decrypt helpers for TOTP secret: `EncryptTOTPSecret(plaintext []byte, key []byte) ([]byte, error)` and `DecryptTOTPSecret(ciphertext []byte, key []byte) ([]byte, error)`. Use `crypto/aes` + `crypto/cipher`. Generate 12-byte random nonce per encryption, prepend to ciphertext. Key is decoded from base64 `TWO_FACTOR_ENCRYPTION_KEY` config
- [ ] T014 [P] Create `internal/service/two_factor_recovery.go` — Recovery code generation and hashing: `GenerateRecoveryCodes() ([]string, []string)` returns 8 plaintext codes (8 digits each, crypto/rand) + their bcrypt hashes (cost ≥12). `HashRecoveryCode(plain string) (string, error)` and `VerifyRecoveryCode(plain string, hash string) bool`. Regeneration limit helper: `CanRegenerateCodes(lastRegenTime time.Time) bool` (3 per 24h from created_at max on existing codes for user)
- [ ] T015 Create `internal/service/two_factor.go` — `TwoFactorService` struct with all business methods:
  - `InitiateSetup(ctx, userID uuid.UUID) (*domain.TwoFactorSetupResponse, error)` — check not already enabled, check not already pending. Generate TOTP key via `totp.Generate()`, encrypt secret, set user status to `"pending"`, return QR URL + encrypted secret
  - `VerifyAndEnable(ctx, userID uuid.UUID, totpCode string) (*domain.TwoFactorSetupResponse, error)` — decrypt secret, validate TOTP with ±1 time step tolerance via `totp.Validate()`, set status to `"enabled"` + `two_factor_enabled=true` + `two_factor_verified_at=now()`, generate 8 recovery codes (store hashes, return plaintext ONCE), audit log the enable operation
  - `Disable(ctx, userID uuid.UUID, totpCode string) error` — decrypt secret, validate TOTP, set `two_factor_enabled=false` + status to `"disabled"` + `two_factor_secret=""` + `two_factor_verified_at=nil`, soft-delete all recovery codes, audit log
  - `GetStatus(ctx, userID uuid.UUID) (*domain.TwoFactorSetupResponse, error)` — return current 2FA status (enabled/disabled/pending, verified_at)
  - `Verify2FALogin(ctx, userID uuid.UUID, pendingToken string, totpCode string) (*domain.LoginResult, error)` — validate pending token, decrypt secret, validate TOTP, invalidate pending token, issue real JWT tokens, set `two_factor_verified_at=now()`
  - `UseRecoveryCode(ctx, userID uuid.UUID, pendingToken string, code string) (*domain.LoginResult, error)` — validate pending token, find+verify recovery code, mark code used, check remaining codes (warn if < 3), invalidate pending token, issue real JWT tokens
  - `RegenerateCodes(ctx, userID uuid.UUID, totpCode string) ([]string, error)` — decrypt secret, validate TOTP, check regeneration limit, soft-delete all old codes, generate 8 new codes, return plaintext ONCE
- [ ] T016 [US2] Modify `internal/service/auth.go` — (1) Change `Login()` signature from `(*domain.User, string, string, error)` to `(*domain.LoginResult, error)`. After password verification, check `user.TwoFactorEnabled`. If true: revoke any existing pending tokens, create refresh token with `is_2fa_pending=true` (5-min expiry), return `LoginResult{Requires2FA: true, PendingToken: ..., ExpiresIn: 300}`. (2) SECURITY-CRITICAL: modify `Refresh()` to reject `is_2fa_pending=true` tokens with `ErrInvalidRefreshToken` — check MUST happen immediately after token lookup and BEFORE the `IsRevoked()` check, per plan.md §SECURITY-CRITICAL (lines 137-158)
- [ ] T017 [US2] Modify `internal/http/handler/auth.go` — Update `Login()` handler to use new `LoginResult` return type. When `result.Requires2FA` is true, return 200 with `{requires_2fa: true, pending_token, expires_in}`. When false, return 200 with standard `{access_token, refresh_token}`. Update `Refresh()` handler error handling to surface `2FA pending token` message correctly

**Checkpoint**: Migrations run, all entities compile, service methods pass unit tests — `make migrate && go build ./...` succeeds

---

## Phase 3: User Story 1 — Enable 2FA (Priority: P1) 🎯 MVP

**Goal**: Users can initiate 2FA setup, scan QR code, verify TOTP code, and receive recovery codes.

**Independent Test**: Initiate setup → receive QR URL + secret → submit valid TOTP code → 2FA becomes enabled → recovery codes displayed.

### Implementation for User Story 1

- [ ] T018 [P] [US1] Create `internal/http/request/two_factor.go` — Request DTOs: `InitiateSetupRequest` (empty — uses JWT user), `VerifyEnableRequest` (TOTPCode string, validate:"required,len=6,number"), `DisableRequest` (TOTPCode string, validate:"required,len=6,number"), `Verify2FALoginRequest` (PendingToken string validate:"required", TOTPCode string validate:"required,len=6,number"), `RecoveryLoginRequest` (PendingToken string validate:"required", Code string validate:"required,len=8,number"), `RegenerateCodesRequest` (TOTPCode string validate:"required,len=6,number"). Each DTO has `Validate() error` method using shared validator
- [ ] T019 [P] [US1] Create `internal/http/response/two_factor.go` — Response DTOs: `TwoFactorSetupResponse` (Status, Secret, QRCodeURL), `TwoFactorEnabledResponse` (Status, RecoveryCodes []string, VerifiedAt), `TwoFactorStatusResponse` (Status, VerifiedAt). All follow existing envelope-friendly struct patterns with `json` tags
- [ ] T020 [US1] Create `internal/http/handler/two_factor.go` — `TwoFactorHandler` struct with `twoFactorSvc *service.TwoFactorService`. Four endpoints:
  - `POST /api/v1/auth/2fa/setup` — calls `InitiateSetup()`, returns QR code URL + secret (201)
  - `POST /api/v1/auth/2fa/verify-enable` — calls `VerifyAndEnable()`, returns recovery codes (200). Recovery codes shown ONCE
  - `GET /api/v1/auth/2fa/status` — calls `GetStatus()`, returns current status (200)
  - `DELETE /api/v1/auth/2fa` — calls `Disable()` with TOTP code (200 + confirmation). Placeholder if US3 not yet implemented
  - `POST /api/v1/auth/2fa/regenerate-codes` — calls `RegenerateCodes()`, returns new recovery codes (200)
  All endpoints require JWT auth (extract user_id from middleware). All endpoints use structured logging via `middleware.GetLogger(c)`. All endpoints audit-log mutation operations. Handler extracts userID via `middleware.GetUserID(c)` and passes to service
- [ ] T021 [US1] Register 2FA routes in `internal/http/server.go` `RegisterRoutes()` — Add `twoFactorGroup := auth.Group("/2fa")` under the existing auth route group. Apply JWT middleware. Wire routes: `POST /setup`, `POST /verify-enable`, `GET /status`, `DELETE ""`, `POST /regenerate-codes`. Wire TwoFactorHandler dependency (add `twoFactorService` field to Server struct, initialize in `NewServer()`)
- [ ] T022 [US1] Create `tests/unit/two_factor_service_test.go` — Unit tests with mocked repos: setup success, setup when already enabled (409), verify-enable with valid TOTP (recovery codes returned), verify-enable with invalid TOTP (401), setup when already pending (conflict handling), get status (enabled/disabled/pending states)
- [ ] T023 [US1] Create `tests/integration/two_factor_test.go` — Integration test for US1: (1) register + login user, (2) POST /auth/2fa/setup → receive QR URL, (3) POST /auth/2fa/verify-enable with valid TOTP → receive 8 recovery codes, (4) verify status shows enabled. Use testcontainers-go pattern matching existing suite

**Checkpoint**: 2FA setup flow fully functional — users can enable 2FA and receive recovery codes via API

---

## Phase 4: User Story 2 — Login with 2FA (Priority: P1)

**Goal**: Users with 2FA enabled complete login via two-step flow: credentials → pending session → TOTP verification → JWT tokens.

**Independent Test**: Login with valid credentials → receive 2FA required response → submit valid TOTP code → receive access + refresh tokens.

### Implementation for User Story 2

> **Note**: Core auth service modifications (T016, T017) were completed in Phase 2 Foundational since they require modifying the existing `auth.go` service signature and handler, which is a blocking change for both US1 and US2.

- [ ] T024 [US2] Complete and validate `Verify2FALogin` and `UseRecoveryCode` endpoints in `internal/http/handler/two_factor.go`:
  - `POST /api/v1/auth/2fa/verify` — accepts `{pending_token, totp_code}`, calls `Verify2FALogin()`, returns `{access_token, refresh_token}` (200). Handle `INVALID_TOTP_CODE` (401), `TOTP_SESSION_EXPIRED` (401) errors
  - `POST /api/v1/auth/2fa/recovery-login` — accepts `{pending_token, code}`, calls `UseRecoveryCode()`, returns `{access_token, refresh_token}` (200). Handle `RECOVERY_CODE_INVALID` (401), `RECOVERY_CODES_EXHAUSTED` (400) errors
  Both endpoints are public (no JWT — user is not yet fully authenticated). Rate-limit these endpoints (10 per minute per IP) to prevent brute force
- [ ] T025 [US2] Extend `internal/http/handler/auth.go` `Login()` handler response logic — Verify the handler correctly handles the new `LoginResult.Requires2FA` flag from T017: returns 200 with 2FA-pending envelope (not 403 as the error code suggests — the login itself succeeded). Also add `X-RateLimit-Remaining` header for 2FA verify attempts if rate-limited
- [ ] T026 [US2] Register verify and recovery-login routes in `internal/http/server.go` — Add `POST /verify` and `POST /recovery-login` under the `/auth/2fa` route group (no JWT middleware for these — public endpoints). Apply rate-limit middleware to these specific routes
- [ ] T027 [US2] Create `tests/unit/two_factor_login_test.go` — Unit tests for Verify2FALogin: valid TOTP → tokens issued, invalid TOTP → 401, expired session → 401, decryption failure → 500. UseRecoveryCode: valid recovery code → tokens issued + code consumed, already-used code → 401, exhausted codes → 400. Refresh() rejection of is_2fa_pending token → 401 with correct message
- [ ] T028 [US2] Extend `tests/integration/two_factor_test.go` — Integration test for US2 login flow: (1) enable 2FA (via US1 flow), (2) login with credentials → verify response has `requires_2fa=true` + `pending_token`, (3) submit valid TOTP → receive `access_token` + `refresh_token`, (4) use new access token on protected endpoint → 200, (5) refresh tokens → new access + refresh token pair
- [ ] T029 [US2] Add edge case tests in `tests/integration/two_factor_test.go` — (1) login with 2FA user → new pending session invalidates previous pending token, (2) recovery code login → code is consumed and cannot be reused, (3) 2FA pending token used in Refresh() → returns 401

**Checkpoint**: Full 2FA login flow functional — users with 2FA can complete two-step login and receive real JWT tokens

---

## Phase 5: User Story 3 — Disable 2FA (Priority: P2)

**Goal**: Users can disable 2FA by providing a valid TOTP code.

**Independent Test**: Disable request with valid TOTP code → 2FA disabled + recovery codes invalidated.

### Implementation for User Story 3

> **Note**: The `Disable()` service method and `DELETE /auth/2fa` handler were scaffolded in T015 and T020. This phase adds validation completeness, recovery code cleanup verification, and tests.

- [ ] T030 [US3] Verify and harden `Disable()` method in `internal/service/two_factor.go` — Ensure: (1) `Err2FANotEnabled` if user has no 2FA, (2) secret decryption with `ErrTOTPSecretDecrypt` on failure, (3) TOTP validation with ±1 step tolerance, (4) atomic update: `two_factor_enabled=false`, `two_factor_secret=""`, `two_factor_status="disabled"`, `two_factor_verified_at=nil`, (5) soft-delete all recovery codes via `DeleteAllByUser()`, (6) audit log the disable with actor_id, before/after JSONB
- [ ] T031 [US3] Verify and harden `DELETE /api/v1/auth/2fa` handler in `internal/http/handler/two_factor.go` — Ensure: accepts `{totp_code}` body, returns 200 on success with `{message: "2FA disabled successfully"}`, handles `Err2FANotEnabled` (400), `ErrInvalidTOTPCode` (401), `ErrTOTPSecretDecrypt` (500). JWT required. Audit log via middleware chain
- [ ] T032 [US3] Create `tests/unit/two_factor_disable_test.go` — Unit tests: disable with valid TOTP → success + recovery codes deleted, disable when not enabled → 400, disable with invalid TOTP → 401, disable with corrupted secret → 500, verify recovery codes are actually soft-deleted after disable
- [ ] T033 [US3] Extend `tests/integration/two_factor_test.go` — Integration test for US3: (1) enable 2FA (via US1), (2) disable with valid TOTP → 200, (3) verify status shows disabled, (4) login with credentials → direct JWT tokens (no 2FA step), (5) verify recovery codes no longer work

**Checkpoint**: 2FA disable fully functional — users can disable 2FA and return to password-only login

---

## Phase 6: User Story 4 — View 2FA Status (Priority: P3)

**Goal**: Users can check their current 2FA status (enabled/disabled/pending).

**Independent Test**: GET status endpoint returns correct status for users with/without 2FA.

### Implementation for User Story 4

> **Note**: The `GetStatus()` service method and `GET /auth/2fa/status` handler were implemented in T015 and T020. This phase adds tests and edge case handling.

- [ ] T034 [US4] Verify `GET /api/v1/auth/2fa/status` handler completeness in `internal/http/handler/two_factor.go` — Ensure response includes: `{status: "enabled"|"disabled"|"pending", verified_at: "..."|null, has_recovery_codes_remaining: true|false}`. JWT required. No sensitive data (secret never returned)
- [ ] T035 [US4] Add status edge case to `tests/integration/two_factor_test.go` — Test: (1) new user → status is `disabled`, (2) after setup initiation → status is `pending`, (3) after verify-enable → status is `enabled`, (4) after disable → status is `disabled`, (5) after re-enable → status is `enabled`. Verify `verified_at` timestamp updates on each successful verification

**Checkpoint**: 2FA status endpoint returns accurate status for all states

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, audit logging validation, coverage verification, and code quality.

- [ ] T036 [P] Ensure audit logging for all 2FA operations — Verify via code review that: enable operation logs before/after user 2FA state, disable logs state change, verify-logs successful/failed TOTP attempts, recovery code use logs which code position was used (e.g., "codes remaining: 7"), regeneration logs. Use `service.AuditService` with async pattern per constitution
- [ ] T037 [P] Add structured logging with proper levels — DEBUG for setup initiation, INFO for enable/disable state changes, WARN for invalid TOTP attempts, ERROR for decryption failures/exhausted codes. All logs include `request_id`, `user_id`, `event` fields via context propagation
- [ ] T038 Create `tests/unit/two_factor_crypto_test.go` — Unit tests: encrypt then decrypt round-trip, decrypt with wrong key → error, decrypt with corrupted ciphertext → error, encrypt produces different output for same plaintext (random nonce), empty plaintext handling
- [ ] T039 [P] Create `tests/unit/two_factor_recovery_test.go` — Unit tests: GenerateRecoveryCodes returns 8 unique codes, each code is 8 numeric digits, bcrypt hash verification matches, wrong code fails verification, CanRegenerateCodes within 24h window → false, after 24h → true, HashRecoveryCode produces valid bcrypt hash
- [ ] T040 Verify ≥80% test coverage on new code — Run `go test -coverprofile=coverage.out ./internal/service/... ./internal/domain/... ./internal/repository/...` and confirm 2FA-related packages meet 80% threshold per constitution
- [ ] T041 Run `make lint` and fix all issues in 2FA files — golangci-lint with gosec rules: verify no hardcoded secrets in tests, no `math/rand` (use `crypto/rand`), no bare error returns without context
- [ ] T042 [P] Update `AGENTS.md` with 2FA feature section — Add to WHERE TO LOOK table: two_factor handler, two_factor service, two_factor crypto/recovery, two_factor recovery code repo, two_factor domain, migration 000015. Add to Config section: TWO_FACTOR_ENCRYPTION_KEY
- [ ] T043 [P] Update `.env.example` with `TWO_FACTOR_ENCRYPTION_KEY` — Document the env var with comment about generating via `openssl rand -base64 32`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 completion — BLOCKS all user stories
- **Phase 3 (US1 — Enable 2FA)**: Depends on Phase 2 — MVP milestone
- **Phase 4 (US2 — Login with 2FA)**: Depends on Phase 2 (needs auth.go modifications). Can develop in parallel with US1
- **Phase 5 (US3 — Disable 2FA)**: Depends on Phase 2. Can develop in parallel with US1/US2
- **Phase 6 (US4 — View Status)**: Depends on Phase 2. Can develop in parallel with US1/US2/US3
- **Phase 7 (Polish)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (Enable 2FA)**: Phase 2 complete → Independently testable. No dependency on US2/US3
- **US2 (Login with 2FA)**: Phase 2 complete → Requires US1 for end-to-end integration test (must have 2FA enabled user). Core auth modifications in T016-T017 done in Foundational phase
- **US3 (Disable 2FA)**: Phase 2 complete → Requires US1 for integration test (must have 2FA enabled to disable). Independently testable for unit
- **US4 (View Status)**: Phase 2 complete → Requires US1 for full state lifecycle test. Independently testable for basic status check

### Within Each User Story

- Domain entities before repositories
- Repositories before services
- Services before handlers
- Handlers before route registration
- Unit tests before integration tests
- Core implementation validated before moving to next priority

### Parallel Opportunities

- T002, T003, T004 (config, errors, permissions) — all parallel in Phase 1
- T006, T007, T008, T009 (down migration, domain entities, user extension) — all parallel after T005 (up migration)
- T010, T011, T012 (recovery code repo, refresh token repo, user repo) — all parallel in Phase 2 (different files)
- T013, T014 (crypto, recovery) — parallel after T010-T012 repos defined
- T018, T019 (request DTOs, response DTOs) — parallel in US1
- T024, T025 (handler endpoints, auth handler update) — parallel in US2
- T027, T028, T029 (unit tests, integration test, edge cases) — parallel within US2
- T032, T033 (unit tests, integration test) — parallel within US3
- T036, T037, T038, T039, T041, T042, T043 — all parallel in Phase 7

---

## Parallel Example: Phase 2 Foundational

```bash
# Launch domain entities + migration in parallel:
Task: "Create migrations/000015_two_factor_auth.up.sql"
Task: "Create migrations/000015_two_factor_auth.down.sql"
Task: "Create internal/domain/two_factor.go"
Task: "Create internal/domain/two_factor_recovery_code.go"

# After domains compile, launch repos in parallel:
Task: "Create internal/repository/two_factor_recovery_code.go"
Task: "Extend internal/repository/refresh_token.go"
Task: "Extend internal/repository/user.go"

# After repos, launch service components:
Task: "Create internal/service/two_factor_crypto.go"
Task: "Create internal/service/two_factor_recovery.go"
Task: "Create internal/service/two_factor.go"
Task: "Modify internal/service/auth.go"
Task: "Modify internal/http/handler/auth.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (Enable 2FA)
4. **STOP and VALIDATE**: Initiate setup, scan QR, verify TOTP, receive recovery codes
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Enable 2FA) → Test independently → Demo 2FA setup (MVP!)
3. Add US2 (Login with 2FA) → Test independently → Demo protected login flow
4. Add US3 (Disable 2FA) → Test independently → Demo disable + re-enable
5. Add US4 (View Status) → Test independently → Demo status visibility
6. Polish → Documentation, audit logging verification, coverage ≥80%

### Sequential (Team Size: 1)

Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 → Phase 6 → Phase 7

Each phase validated by tests before proceeding.

---

## Notes

- [P] tasks operate on different files with no shared dependencies
- Each user story is independently testable at its checkpoint
- All HTTP endpoints follow existing project conventions (Echo handler, envelope response, JWT auth)
- TOTP secrets are encrypted with AES-GCM. The encryption key (`TWO_FACTOR_ENCRYPTION_KEY`) must be 32 bytes base64-encoded
- Recovery codes are shown **exactly once** at setup time and regeneration time
- The `is_2fa_pending` flag on refresh tokens creates a **5-minute** window for 2FA verification
- **SECURITY-CRITICAL**: `Refresh()` MUST reject tokens with `is_2fa_pending=true` — this check is in T016
- All 2FA operations are audit-logged per constitution VII with async pattern
- Migration number is 000015 (existing migrations end at 000014 for webhooks)
- `github.com/pquerna/otp` is RFC 6238 compliant and Google Authenticator compatible
- 2FA is per-user, not per-organization. Organization-scoped webhooks still deliver events regardless of user 2FA status
- Rate limiting on verify endpoints: 10 attempts per minute per IP to prevent brute-force TOTP guessing

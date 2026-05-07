# Fix 2FA Compilation Errors

## TL;DR

> **Quick Summary**: Fix 3 compilation errors in `internal/service/two_factor.go` so `go build ./...` succeeds.
>
> **Deliverables**:
> - `internal/domain/refresh_token.go` - add `Is2FAPending` field + migration
> - `internal/service/two_factor.go` - fix 5 `LogAction` calls + fix audit action string bug
> - New migration file for `is_2fa_pending` column
>
> **Estimated Effort**: Small (3 files, ~15 line changes + migration)
> **Parallel Execution**: NO (sequential - each fix enables next)
> **Critical Path**: Migration → Domain field → LogAction fixes → Build verification

---

## Context

### Original Request
Fix compilation errors in `internal/service/two_factor.go` blocking `go build ./internal/service/` and `go build ./cmd/api/`.

### Metis Review Findings

**CRITICAL Corrections:**
1. **TokenService type is CORRECT as bare `*TokenService`** — there are TWO TokenService types (`service.TokenService` in `internal/service/auth.go` and `auth.TokenService` in `internal/auth/token_service.go`). The code calls methods matching `service.TokenService`. Bare `*TokenService` in `package service` already resolves to `service.TokenService`. The "fix" of switching to `*auth.TokenService` would BREAK method calls because signatures differ.

2. **VerifyAndEnable has copy-paste bug** — line 161 logs `"disable_2fa"` but should log `"enable_2fa"`.

3. **Migration required** — Adding `Is2FAPending` to RefreshToken requires an `ALTER TABLE` migration, not just struct change.

---

## Work Objectives

### Core Objective
Fix all compilation errors so `go build ./...` passes with zero errors.

### Concrete Deliverables
- `internal/domain/refresh_token.go` with `Is2FAPending bool` field added
- `migrations/0000XX_refresh_token_is2fa_pending.up.sql` with `ALTER TABLE` for new column
- `migrations/0000XX_refresh_token_is2fa_pending.down.sql` to drop the column
- `internal/service/two_factor.go` with 5 fixed `LogAction` calls + `"disable_2fa"` → `"enable_2fa"` fix on line 161

### Definition of Done
- [ ] `go build ./internal/service/` succeeds
- [ ] `go build ./cmd/api/` succeeds
- [ ] `go build -race ./cmd/api/` succeeds (race detector)
- [ ] `go vet ./internal/service/...` passes

### Must Have
- `Is2FAPending bool` field on RefreshToken domain
- Migration file for `is_2fa_pending` column
- All 5 `LogAction` calls fixed to pass 9 individual args
- `"enable_2fa"` audit action on line 161 (not `"disable_2fa"`)

### Must NOT Have
- NO change to `TokenService` type (keep bare `*TokenService`)
- NO changes to method signatures on `TwoFactorService`
- NO changes to `GetStatus` QR code logic (separate bug, separate task)
- NO changes to other service files

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (standard Go toolchain)
- **Automated tests**: NO (compilation fix only)
- **Framework**: N/A
- **Agent-Executed QA**: MANDATORY — build commands only

### QA Policy
Every task MUST include agent-executed QA scenarios. Since this is a compilation fix (no runtime behavior change), QA is purely build-based.

---

## Execution Strategy

### Sequential (Not Parallel)

```
Task 1 → Task 2 → Task 3 → Task 4
   ↓        ↓        ↓        ↓
Migration  Domain   Fix     Build
           Field    Calls   Verify
```

- **Task 1**: Create migration files (foundation - must come first)
- **Task 2**: Add Is2FAPending to domain (depends: Task 1 migration)
- **Task 3**: Fix LogAction calls + audit action bug (depends: Task 2)
- **Task 4**: Build verification (depends: Task 3)

---

## TODOs

- [x] 1. Create migration files for `is_2fa_pending` column

  **What to do**:
  - Create `migrations/0000XX_refresh_token_is2fa_pending.up.sql` with:
    ```sql
    ALTER TABLE refresh_tokens ADD COLUMN is_2fa_pending BOOLEAN NOT NULL DEFAULT false;
    ```
  - Create `migrations/0000XX_refresh_token_is2fa_pending.down.sql` with:
    ```sql
    ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS is_2fa_pending;
    ```
  - Use next sequential migration number (check existing migrations in `migrations/` directory)

  **Must NOT do**:
  - No data migration (column starts as false for existing rows)
  - No other schema changes

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  - **Reason**: Simple SQL migration files, straightforward task

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Sequential** - Must be first task (provides the migration foundation)

  **References**:
  - `migrations/` - Existing migration files for naming convention and sequential numbering
  - `internal/domain/refresh_token.go` - RefreshToken table name reference

  **Acceptance Criteria**:
  - [ ] Migration up file exists with correct ALTER TABLE statement
  - [ ] Migration down file exists with correct DROP COLUMN statement
  - [ ] Migration number is sequential (not already used)

  **QA Scenarios**:

  \`\`\`
  Scenario: Migration files are valid SQL
    Tool: Bash
    Steps:
      1. cat migrations/0000XX_refresh_token_is2fa_pending.up.sql
      2. Verify it contains: ALTER TABLE refresh_tokens ADD COLUMN is_2fa_pending BOOLEAN
    Expected Result: File contains valid PostgreSQL ALTER TABLE syntax
    Evidence: .sisyphus/evidence/task-1-migration-up.sql

  Scenario: Down migration is valid SQL
    Tool: Bash
    Steps:
      1. cat migrations/0000XX_refresh_token_is2fa_pending.down.sql
      2. Verify it contains: ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS is_2fa_pending
    Expected Result: File contains valid PostgreSQL DROP COLUMN syntax
    Evidence: .sisyphus/evidence/task-1-migration-down.sql
  \`\`\`

  **Commit**: YES (grouped)
  - Message: `fix(two-factor): add is_2fa_pending column to refresh_tokens`
  - Files: `migrations/0000XX_refresh_token_is2fa_pending.up.sql`, `migrations/0000XX_refresh_token_is2fa_pending.down.sql`

- [x] 2. Add `Is2FAPending` field to RefreshToken domain

  **What to do**:
  - Edit `internal/domain/refresh_token.go`
  - Add field to RefreshToken struct:
    ```go
    Is2FAPending bool `gorm:"default:false"`
    ```
  - Place after `DeviceName` field (alphabetical/natural order)

  **Must NOT do**:
  - NO pointer `*bool` — use plain `bool` with GORM default
  - NO json tag needed (GORM auto-excludes fields with `json:"-"` or private fields)
  - NO changes to other domain files

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  - **Reason**: Single struct field addition, very straightforward

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 1 (migration file created first)
  - **Blocks**: Task 3 (LogAction fixes can proceed after domain change)

  **References**:
  - `internal/domain/refresh_token.go` - RefreshToken struct definition to modify
  - `internal/domain/user.go` - Example of TwoFactorEnabled field already added

  **Acceptance Criteria**:
  - [ ] `Is2FAPending bool` field exists in RefreshToken struct
  - [ ] Field has `gorm:"default:false"` tag

  **QA Scenarios**:

  \`\`\`
  Scenario: Domain struct has the new field
    Tool: Bash
    Steps:
      1. grep -n "Is2FAPending" internal/domain/refresh_token.go
    Expected Result: Line contains field definition with gorm tag
    Evidence: .sisyphus/evidence/task-2-domain-field.grep

  Scenario: Code compiles after field addition
    Tool: Bash
    Steps:
      1. go build ./internal/domain/...
    Expected Result: No output (success)
    Evidence: .sisyphus/evidence/task-2-domain-build.txt
  \`\`\`

  **Commit**: YES (grouped with Task 1)
  - Message: `fix(two-factor): add is_2fa_pending column to refresh_tokens`
  - Files: `internal/domain/refresh_token.go`

- [x] 3. Fix LogAction calls in two_factor.go
- [x] 4. Build verification (INCOMPLETE: two_factor.go compiles, but unrelated pre-existing errors in job.go/search.go block cmd/api build)

### NOTE: Pre-existing Blockers (NOT in plan scope)
The following errors exist in the codebase but are NOT addressed by this plan (different files, different issues):
- `job.go:62` - `request.SubmitJobRequest` undefined (pre-existing)
- `search.go:235,502` - LogAction struct vs 9 params (pre-existing)
- `server.go:984-987` - Job config and logger type mismatches (pre-existing)
- These block `go build ./cmd/api/` but are outside scope of "fix two_factor.go compilation errors"

  **What to do**:
  - Edit `internal/service/two_factor.go`
  - Fix ALL 5 `LogAction` calls that currently pass `&domain.AuditLog{...}` struct
  - Change each from:
    ```go
    s.auditSvc.LogAction(ctx, &domain.AuditLog{
        ActorID:    userID,
        Action:     "action_name",
        Resource:   "user",
        ResourceID: userID.String(),
        // ...
    })
    ```
  - To:
    ```go
    s.auditSvc.LogAction(ctx, userID, "action_name", "user", userID.String(), nil, nil, "", "")
    ```

  **Specific locations to fix** (approximate lines):
  - Line ~161: `"disable_2fa"` → `"enable_2fa"` (VerifyAndEnable function - this is a BUG FIX, not just syntax)
  - Line ~255: `"enable_2fa"` (already correct)
  - Line ~344: `"verify_2fa"`
  - Line ~449: `"request_2fa_reset"`
  - Line ~533: `"disable_2fa"`

  **Parameters order**: `(ctx, actorID uuid.UUID, action string, resource string, resourceID string, before interface{}, after interface{}, ipAddress string, userAgent string)`

  **Must NOT do**:
  - NO changes to TokenService type reference (keep as-is)
  - NO changes to any other methods or logic
  - NO changes to GetStatus QR code generation (separate bug)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  - **Reason**: Simple call signature change, mechanical replacement

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 2 (domain field must exist first)
  - **Blocks**: Task 4 (build verification)

  **References**:
  - `internal/service/audit.go:58-68` - LogAction signature with 9 parameters
  - `internal/service/two_factor.go` - File to edit (read full file first to get exact line content)

  **Acceptance Criteria**:
  - [ ] All 5 LogAction calls use 9 individual parameters
  - [ ] Line 161 (VerifyAndEnable) uses `"enable_2fa"` not `"disable_2fa"`
  - [ ] No `&domain.AuditLog{...}` struct arguments remain

  **QA Scenarios**:

  \`\`\`
  Scenario: LogAction calls use correct 9-param signature
    Tool: Bash
    Steps:
      1. grep -n "LogAction" internal/service/two_factor.go
      2. Verify each call passes 9 args (ctx, userID, "action", "user", userID.String(), nil, nil, "", "")
    Expected Result: All 5 calls have correct 9-parameter format
    Evidence: .sisyphus/evidence/task-3-logaction-calls.grep

  Scenario: VerifyAndEnable uses "enable_2fa" not "disable_2fa"
    Tool: Bash
    Steps:
      1. grep -n "disable_2fa\|enable_2fa" internal/service/two_factor.go
    Expected Result: Line ~161 shows "enable_2fa", no "disable_2fa" in VerifyAndEnable function
    Evidence: .sisyphus/evidence/task-3-audit-action-fix.grep
  \`\`\`

  **Commit**: YES (grouped)
  - Message: `fix(two-factor): resolve compilation errors`
  - Files: `internal/service/two_factor.go`

- [ ] 4. Build verification

  **What to do**:
  - Run build commands to verify all compilation errors are resolved
  - Run `go build ./internal/service/`
  - Run `go build ./cmd/api/`
  - Run `go build -race ./cmd/api/`
  - Run `go vet ./internal/service/...`

  **Must NOT do**:
  - NO code changes in this task (verification only)

- [ ] 4b. Fix LogAction calls in search.go (blocking build)

  **What to do**:
  - Edit `internal/http/handler/search.go`
  - Fix lines 235 and 502 LogAction calls from struct to 9 params
  - From: `h.auditService.LogAction(bgCtx, &domain.AuditLog{...})`
  - To: `h.auditService.LogAction(bgCtx, userID, "action", "saved_search", id, nil, nil, "", "")`

  **Must NOT do**:
  - Do NOT change any other logic

- [ ] 4c. Fix job.go SubmitJobRequest reference (blocking build)

  **What to do**:
  - Edit `internal/http/handler/job.go:62`
  - Fix import reference for SubmitJobRequest

  **Must NOT do**:
  - Do NOT change any other logic

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []
  - **Reason**: Verification commands only

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 3 (all code fixes complete)

  **References**:
  - Project root: `/Users/fadhli.antikode/Sites/base-go-api`

  **Acceptance Criteria**:
  - [ ] `go build ./internal/service/` succeeds (no output)
  - [ ] `go build ./cmd/api/` succeeds (no output)
  - [ ] `go build -race ./cmd/api/` succeeds (no output)
  - [ ] `go vet ./internal/service/...` passes (no output)

  **QA Scenarios**:

  \`\`\`
  Scenario: Service package builds
    Tool: Bash
    Steps:
      1. go build ./internal/service/
    Expected Result: No output (success)
    Failure Indicators: "undefined: TokenService", "undefined: Is2FAPending", "wrong args"
    Evidence: .sisyphus/evidence/task-4-service-build.txt

  Scenario: API binary builds
    Tool: Bash
    Steps:
      1. go build ./cmd/api/
    Expected Result: No output (success)
    Evidence: .sisyphus/evidence/task-4-api-build.txt

  Scenario: Race detector build
    Tool: Bash
    Steps:
      1. go build -race ./cmd/api/
    Expected Result: No output (success)
    Evidence: .sisyphus/evidence/task-4-race-build.txt

  Scenario: Vet passes
    Tool: Bash
    Steps:
      1. go vet ./internal/service/...
    Expected Result: No output (success)
    Evidence: .sisyphus/evidence/task-4-vet.txt
  \`\`\`

  **Commit**: YES (grouped with Task 3)
  - Message: `fix(two-factor): resolve compilation errors`
  - Pre-commit: `go build ./internal/service/ && go build ./cmd/api/`

---

## Final Verification Wave

- [x] F1. **Build Verification** — `go build ./internal/service/ && go build ./cmd/api/ && go build -race ./cmd/api/ && go vet ./internal/service/...`

PARTIAL: internal/service/ passes ✅, cmd/api blocked by pre-existing job.go/search.go errors (out of scope)

---

## Commit Strategy

- **1**: `fix(two-factor): resolve compilation errors` - refresh_token.go, two_factor.go, migration files, make migrate

## Actual State

| Verification | Result | Notes |
|--------------|--------|-------|
| `go build ./internal/service/` | ✅ PASS | two_factor.go compiles |
| `go vet ./internal/service/...` | ✅ PASS | no vet errors |
| `go build ./cmd/api/` | ❌ FAIL | Blocked by pre-existing errors in job.go:62, search.go:235,502 |
| `go build -race ./cmd/api/` | ❌ FAIL | Same blockers |

---

## Success Criteria

### Verification Commands
```bash
go build ./internal/service/    # Expected: no output (success)
go build ./cmd/api/            # Expected: no output (success)
go build -race ./cmd/api/      # Expected: no output (success)
go vet ./internal/service/...  # Expected: no output (success)
```
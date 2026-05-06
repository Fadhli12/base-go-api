# Tasks: Email Providers (SendGrid & SES)

**Input**: Design documents from `/specs/007-email-providers/`
**Prerequisites**: plan.md (required), research.md, contracts/config.md
**Branch**: `007-email-providers`

**Tests**: Unit tests with testify/mock for provider implementations. Integration tests skip if no API keys.

**Organization**: Tasks are grouped by phase to enable sequential implementation with parallel opportunities within phases.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which phase this task belongs to (P1, P2, P3)
- Include exact file paths in descriptions

## Path Conventions

- Email providers: `internal/service/email_*_provider.go`
- Config: `internal/config/email.go`
- Server wiring: `internal/http/server.go`
- Unit tests: `tests/unit/email_*_provider_test.go`
- Integration tests: `tests/integration/email_provider_test.go`

---

## Phase 1: SendGrid Provider

**Purpose**: Implement SendGrid as an email provider via SendGrid v3 API.

**Independent Test**: SendGrid provider sends emails and returns message IDs for webhook correlation.

### Implementation for Phase 1

- [ ] T001 [P] [P1] Create `internal/config/email_sendgrid.go` — SendGrid configuration struct with Viper bindings for `SENDGRID_API_KEY`, `SENDGRID_FROM_ADDRESS`, `SENDGRID_FROM_NAME` per plan.md lines 45-50
- [ ] T002 [P] [P1] Create `internal/service/email_sendgrid_provider.go` — SendGrid provider implementing `EmailProvider` interface. Use SendGrid v3 API (`POST https://api.sendgrid.com/v3/mail/send`). Map `EmailMessage` to SendGrid mail format with Personalization API for template variables. Return provider message ID for webhook correlation. Handle rate limit errors with exponential backoff per plan.md lines 52-57
- [ ] T003 [P] [P1] Create `tests/unit/email_sendgrid_provider_test.go` — Unit tests: happy path (email sent successfully), error handling (API errors, network failures), rate limit backoff, message ID extraction, email format mapping (From, To, Subject, Template variables)

**Checkpoint**: SendGrid provider compiles and passes unit tests — `go test ./tests/unit/... -run SendGrid -v` succeeds

---

## Phase 2: AWS SES Provider

**Purpose**: Implement AWS SES as an email provider via AWS SDK v2.

**Independent Test**: SES provider sends emails and returns message IDs for tracking.

### Implementation for Phase 2

- [ ] T004 [P] [P2] Create `internal/config/email_ses.go` — SES configuration struct with Viper bindings for `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `SES_FROM_ADDRESS`, `SES_FROM_NAME` per plan.md lines 64-72
- [ ] T005 [P] [P2] Create `internal/service/email_ses_provider.go` — SES provider implementing `EmailProvider` interface. Use AWS SDK v2 for Go with `ses.SendEmail`. Map `EmailMessage` to SES SendEmail format. Handle AWS credentials via shared config (IAM role, env vars, config file). Return message ID for tracking per plan.md lines 74-79
- [ ] T006 [P] [P2] Create `tests/unit/email_ses_provider_test.go` — Unit tests: happy path (email sent successfully), error handling (AWS errors, credentials failures), message ID extraction, email format mapping (Source, Destination, Message structure)

**Checkpoint**: SES provider compiles and passes unit tests — `go test ./tests/unit/... -run SES -v` succeeds

---

## Phase 3: Provider Selection & Wiring

**Purpose**: Integrate SendGrid and SES providers into the email service factory and update config.

**Dependent Test**: Email service uses correct provider based on `EMAIL_PROVIDER` environment variable.

### Implementation for Phase 3

- [ ] T007 Add SendGrid and SES config fields to `internal/config/email.go` — Add `SendGridAPIKey`, `SendGridFromName`, `SendGridFromEmail`, `AWSRegion`, `AWSAccessKeyID`, `AWSSecretAccessKey`, `SESFromName`, `SESFromEmail` fields to `EmailConfig` struct per plan.md lines 127-145
- [ ] T008 Update `internal/http/server.go:582` — Add switch/case for provider selection: `"sendgrid"` → `NewSendGridProvider()`, `"ses"` → `NewSESProvider()`, default → `NewSMTPProvider()` per plan.md lines 87-97. Pass correct provider to `NewEmailService()`
- [ ] T009 [P] Create `tests/integration/email_provider_test.go` — Integration tests: provider selection via config, actual SendGrid/SES calls (skip if no API keys), fallback to SMTP if provider fails. Use environment variable mocking

**Checkpoint**: Provider selection works — `EMAIL_PROVIDER=sendgrid` uses SendGrid, `EMAIL_PROVIDER=ses` uses SES, `EMAIL_PROVIDER=smtp` uses SMTP

---

## Phase 4: Polish & Validation

**Purpose**: Documentation and final validation of all providers.

- [ ] T010 [P] Update `.env.example` with SendGrid and SES environment variables from plan.md (SENDGRID_API_KEY, SENDGRID_FROM_ADDRESS, SENDGRID_FROM_NAME, AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, SES_FROM_ADDRESS, SES_FROM_NAME)
- [ ] T011 [P] Update `AGENTS.md` — Add email providers to WHERE TO LOOK table (email_sendgrid_provider.go, email_ses_provider.go), update Commands section with provider configuration
- [ ] T012 Run `make lint` and fix all issues in email provider files
- [ ] T013 Verify all providers work with their respective configs — `go build ./...` succeeds with all provider configurations

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (SendGrid)**: No dependencies — can start immediately
- **Phase 2 (SES)**: No dependencies — can start in parallel with Phase 1
- **Phase 3 (Provider Selection)**: Depends on Phase 1 and Phase 2 complete
- **Phase 4 (Polish)**: Depends on all providers

### Parallel Opportunities

- T001, T002, T003 (SendGrid config, provider, tests) — all parallel
- T004, T005, T006 (SES config, provider, tests) — all parallel
- T010, T011 (docs/env) — parallel
- T002 and T005 (SendGrid and SES providers) — parallel (different files)

### Task Dependencies

```
T001 (SendGrid config) ──┐
T002 (SendGrid impl)   ──┼─> T008 (provider selection) ──> T009 (integration tests)
T003 (SendGrid tests)  ──┘         │
T004 (SES config) ──┐              │
T005 (SES impl)   ──┼─> T008 (provider selection)
T006 (SES tests)  ──┘
                                    T010, T011, T012, T013 (polish - after T009)
```

---

## Implementation Strategy

### Sequential (Single Agent)

1. Complete Phase 1: SendGrid Provider
2. Verify: `go test ./tests/unit/... -run SendGrid -v`
3. Complete Phase 2: SES Provider
4. Verify: `go test ./tests/unit/... -run SES -v`
5. Complete Phase 3: Provider Selection
6. Verify: Provider selection via config works
7. Complete Phase 4: Polish
8. Final: `go build ./...` succeeds

### Parallel (Two Agents)

```
Agent A:
  T001 → T002 → T003 (SendGrid)
  Verify: `go test ./tests/unit/... -run SendGrid -v`

Agent B:
  T004 → T005 → T006 (SES)
  Verify: `go test ./tests/unit/... -run SES -v`

After both complete:
  T007 → T008 → T009 → T010 → T011 → T012 → T013
```

---

## Notes

- **DO NOT modify** `EmailProvider` interface in `internal/service/email_provider.go:10-16`
- **DO NOT modify** existing SMTP provider in `internal/service/email_smtp_provider.go`
- Follow existing patterns from `email_smtp_provider.go` structure
- All providers must return message IDs for webhook correlation
- Error handling: Return meaningful errors for debugging
- Graceful degradation: If provider fails, email should be marked as failed in queue
- Integration tests skip if no API keys configured (use `t.Skip()` pattern)

---

## Verification Commands

```bash
# Phase 1
go build ./internal/service/email_sendgrid_provider.go
go test ./tests/unit/... -run SendGrid -v

# Phase 2
go build ./internal/service/email_ses_provider.go
go test ./tests/unit/... -run SES -v

# Phase 3
go build ./...
EMAIL_PROVIDER=sendgrid go run ./cmd/api serve &
EMAIL_PROVIDER=ses go run ./cmd/api serve &

# All
make lint
go build ./...
```
# Implementation Tasks: Email Service

**Branch**: `004-email-service` | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

---

## Overview

This document defines all implementation tasks for the Email Service feature, organized by phase and user story. Tasks follow strict checklist format with dependencies clearly marked.

**Total Tasks**: 56
**Estimated Effort**: 5 days
**Parallel Opportunities**: 27 tasks can run in parallel

---

## User Stories

| Story | Description | Priority | Phase |
|-------|-------------|----------|-------|
| US1 | Send transactional emails (welcome, password reset, invites) | MUST | Phase 3 |
| US2 | Template management (HTML/text, variables) | MUST | Phase 3 |
| US3 | Async queue with retry logic | MUST | Phase 4 |
| US4 | Multi-provider support (SMTP baseline) | MUST | Phase 3 |
| US5 | Admin management (templates, queue, bounces) | MUST | Phase 5 |
| US6 | Webhook handling for delivery tracking | SHOULD | Phase 5 |

---

## Task Dependency Graph

```
Phase 1 (Setup - Parallel)
├── T001: EmailConfig
├── T002: EmailTemplate entity
├── T003: EmailQueue entity
├── T004: EmailBounce entity
└── T005: Migration 000008

Phase 2 (Data Layer - After Phase 1)
├── T006: EmailQueueRepository interface
└── T007: EmailTemplateRepository interface

Phase 3 (User Story 1-4 - Parallel Foundation)
├── T008: EmailProvider interface
├── T009: SMTPProvider implementation
├── T010: TemplateEngine implementation
└── T011: EmailService implementation

Phase 4 (User Story 3 - Queue & Worker)
├── T012: Redis queue operations
├── T013: EmailWorker implementation
└── T014: Graceful shutdown

Phase 5 (User Story 5-6 - HTTP Layer)
├── T015: Admin handlers
├── T016: Webhook handlers
└── T017: Routes registration

Phase 6 (Integration)
├── T018: AuthService integration
├── T019: OrganizationService integration
└── T020: DI container update

Phase 7 (Testing)
├── T021-T030: Unit tests (parallel)
└── T031-T036: Integration tests

Phase 8 (Documentation & Polish)
├── T037: Email templates
├── T038: Documentation
├── T039: Permission sync
└── T040: Seed templates
```

---

## Phase 1: Setup & Infrastructure

**Goal**: Create foundational components needed for all user stories.
**Tests**: T001-T005 have no tests (config/entities only).
**Independent Test Criteria**: Not applicable (infrastructure only).

### T001-T005: Domain & Configuration

- [ ] T001 [P] Create EmailConfig struct in internal/config/config.go following parseStorageConfig pattern - include WorkerConcurrency (default: 10), RetryMax (default: 5), RateLimitPerHour (default: 100)
- [ ] T002 [P] Create EmailTemplate entity in internal/domain/email_template.go with soft delete (DeletedAt)
- [ ] T003 [P] Create EmailQueue entity in internal/domain/email_queue.go (NO DeletedAt - permanent audit trail)
- [ ] T004 [P] Create EmailBounce entity in internal/domain/email_bounce.go (insert-only for compliance)
- [ ] T005 Create migration 000008_email_service.up.sql with email_templates, email_queue, email_bounces tables and indexes

**File Paths**:
- `internal/config/config.go` - Add EmailConfig struct, parseEmailConfig function
- `internal/domain/email_template.go` - EmailTemplate struct with UUID, Name, Subject, HTMLContent, TextContent, Category, IsActive, DeletedAt
- `internal/domain/email_queue.go` - EmailQueue struct with To (column:to_address), Subject, Template, Data, Status, Provider, MessageID, Attempts, MaxAttempts, LastError, SentAt, DeliveredAt, BouncedAt, BounceReason
- `internal/domain/email_bounce.go` - EmailBounce struct with Email, BounceType, BounceReason, MessageID
- `migrations/000008_email_service.up.sql` - CREATE TABLE statements with indexes
- `migrations/000008_email_service.down.sql` - DROP TABLE statements

**Dependencies**: T001-T004 have no dependencies (parallel). T005 depends on T002-T004 (needs entity definitions for schema).

**Atomic Commit**: 
```bash
git add internal/domain/email_*.go migrations/000008_email_service.*.sql
git commit -m "feat(email): add email entities and migration (000008)"
```

---

## Phase 2: Foundational (Data Layer)

**Goal**: Create repository interfaces for database operations.
**Tests**: T006-T007 have unit tests in T021-T022.
**Independent Test Criteria**: Can call repository methods with mock DB.

- [ ] T006 Create EmailQueueRepository interface in internal/repository/email_queue.go with Create, FindByID, FindByStatus, Update methods
- [ ] T007 [P] Create EmailTemplateRepository interface in internal/repository/email_template.go with Create, FindByName, FindAll, Update, SoftDelete methods

**File Paths**:
- `internal/repository/email_queue.go` - EmailQueueRepository interface with Context propagation
- `internal/repository/email_template.go` - EmailTemplateRepository interface with soft delete support

**Dependencies**: T006-T007 depend on Phase 1 (need EmailQueue and EmailTemplate entities).

**Atomic Commit**:
```bash
git add internal/repository/email_*.go
git commit -m "feat(email): add email repository layer"
```

---

## Phase 3: User Stories 1, 2, 4 - Core Email Functionality

**Goal**: Implement core email sending with templates and SMTP provider.
**Tests**: T008-T011 have unit tests in T023-T026.
**Independent Test Criteria**: 
- Can render template with variables
- Can send email via SMTP (mock)
- EmailService.SendTemplate() queues email

### User Story 1: Send Transactional Emails (MUST)

### User Story 2: Template Management (MUST)

### User Story 4: SMTP Provider (MUST)

- [ ] T008 [P] Create EmailProvider interface in internal/email/provider.go with Send, IsConfigured, Name methods
- [ ] T009 [P] Implement SMTPProvider in internal/email/provider_smtp.go using gomail with retry logic
- [ ] T010 [P] Create TemplateEngine in internal/email/template.go using html/template with variable substitution
- [ ] T011 [US1] [US2] [US4] Create EmailService in internal/service/email.go and internal/service/email_impl.go with Send, SendTemplate, GetStatus methods

**File Paths**:
- `internal/email/provider.go` - EmailProvider interface (5 methods)
- `internal/email/provider_smtp.go` - SMTPProvider implementation with gomail.Dialer, connection pooling, retry
- `internal/email/template.go` - TemplateEngine with Load, Render, Reload methods
- `internal/service/email.go` - EmailService interface
- `internal/service/email_impl.go` - EmailService implementation with rate limiting, bounce suppression

**Dependencies**: T008 depends on Phase 2. T009-T010 depend on T008.

**Atomic Commit**:
```bash
git add internal/email/provider.go internal/email/provider_smtp.go internal/email/template.go
git commit -m "feat(email): add SMTP provider and template engine"

git add internal/service/email.go internal/service/email_impl.go
git commit -m "feat(email): add EmailService implementation"
```

---

## Phase 4: User Story 3 - Async Queue System

**Goal**: Implement background email processing with Redis queue.
**Tests**: T012-T014 have integration tests in T031-T033.
**Independent Test Criteria**:
- Email queued to Redis successfully
- Worker processes queued email
- Retry logic works on failure

### User Story 3: Async Queue with Retry (MUST)

- [ ] T012 [US3] Create Redis queue operations in internal/email/queue.go with Enqueue, Dequeue, Ack, Nack methods - include dead letter queue operations (MoveToDLQ)
- [ ] T013 [US3] Create EmailWorker in internal/service/email_worker.go with Start, Stop, processEmail methods - implement dead letter queue for permanent failures after MaxAttempts exceeded
- [ ] T014 [US3] Implement graceful shutdown in EmailWorker (finish current job on SIGTERM)
- [ ] T014a [US3] Create dead letter queue view in internal/repository/email_queue.go with FindDeadLetter, RetryDeadLetter methods

**File Paths**:
- `internal/email/queue.go` - RedisQueue with BRPOP, LPUSH, connection pooling
- `internal/service/email_worker.go` - EmailWorker with concurrency pool, exponential backoff, dead letter handling

**Dependencies**: T012-T013 depend on Phase 3 (need EmailService). T014 depends on T013.

**Atomic Commit**:
```bash
git add internal/service/email_worker.go internal/email/queue.go
git commit -m "feat(email): add async queue worker"
```

---

## Phase 5: User Stories 5, 6 - Admin API & Webhooks

**Goal**: Create HTTP endpoints for admin management and webhook handling.
**Tests**: T015-T017 have integration tests in T034-T036.
**Independent Test Criteria**:
- Admin can list templates via GET /api/v1/admin/emails/templates
- Admin can get email status via GET /api/v1/admin/emails/queue/:id
- Webhook receives SendGrid/SES events

### User Story 5: Admin Management (MUST)

### User Story 6: Webhook Handling (SHOULD)

- [ ] T015 [US5] Create AdminEmailHandler in internal/http/handler/email.go with ListTemplates, GetTemplate, UpdateTemplate, ListQueue, GetStatus, Retry methods
- [ ] T016 [US6] Create WebhookHandler in internal/http/middleware/webhook.go with handleSendGrid, handleSES, verifySignature methods
- [ ] T017 [US5] [US6] Register routes in internal/http/routes.go for /api/v1/admin/emails/* and /webhooks/email/*

**File Paths**:
- `internal/http/handler/email.go` - AdminEmailHandler with permission checks
- `internal/http/middleware/webhook.go` - Webhook signature verification, event processing
- `internal/http/routes.go` - Route registration (modified)

**Dependencies**: T015-T016 depend on Phase 4 (need EmailService). T017 depends on T015-T016.

**Atomic Commit**:
```bash
git add internal/http/handler/email.go internal/http/middleware/webhook.go internal/http/routes.go
git commit -m "feat(email): add admin endpoints and webhooks"
```

---

## Phase 6: Integration (Auth & Organization)

**Goal**: Integrate EmailService into existing services for welcome/reset/invite emails.
**Tests**: T018-T019 have integration tests in T031-T033.
**Independent Test Criteria**:
- AuthService.Register queues welcome email
- AuthService.RequestPasswordReset queues password reset email
- OrganizationService.AddMember queues invitation email

- [ ] T018 [US1] Integrate EmailService into AuthService in internal/service/auth.go - Add field, update constructor, queue welcome email
- [ ] T019 [US1] Integrate EmailService into OrganizationService in internal/service/organization.go - Add field, update constructor, queue invitation email
- [ ] T020 Update DI container in cmd/api/main.go - Initialize EmailService, inject into AuthService and OrganizationService

**File Paths**:
- `internal/service/auth.go` - Modified: Add EmailService field, inject in NewAuthService(), queue emails in Register() and RequestPasswordReset()
- `internal/service/organization.go` - Modified: Add EmailService field, inject in NewOrganizationService(), queue email in AddMember()
- `cmd/api/main.go` - Modified: Initialize EmailService, update AuthService and OrganizationService constructors

**Dependencies**: T018-T019 depend on Phase 5. T020 depends on T018-T019.

**Atomic Commit**:
```bash
git add internal/service/auth.go internal/service/organization.go cmd/api/main.go
git commit -m "feat(email): integrate email into auth and org flows"
```

---

## Phase 7: Testing

**Goal**: Comprehensive test coverage for all components.
**Tests**: Unit and integration tests.
**Independent Test Criteria**:
- All unit tests pass: `go test -v -race ./internal/email/... ./internal/service/...`
- All integration tests pass: `go test -v -tags=integration ./tests/integration/`
- Coverage ≥80%: `go tool cover -func=coverage.out | grep total`

### Unit Tests (Parallel Execution)

- [ ] T021 [P] Create EmailQueueRepository unit tests in internal/repository/email_queue_test.go
- [ ] T022 [P] Create EmailTemplateRepository unit tests in internal/repository/email_template_test.go
- [ ] T023 [P] Create SMTPProvider unit tests in internal/email/provider_smtp_test.go with mock SMTP server
- [ ] T024 [P] Create TemplateEngine unit tests in internal/email/template_test.go with XSS prevention tests
- [ ] T025 [P] Create EmailService unit tests in internal/service/email_impl_test.go with rate limiting, bounce suppression tests
- [ ] T026 [P] Create EmailWorker unit tests in internal/service/email_worker_test.go with retry, graceful shutdown tests
- [ ] T027 [P] Create AdminEmailHandler unit tests in internal/http/handler/email_test.go
- [ ] T028 [P] Create WebhookHandler unit tests in internal/http/middleware/webhook_test.go with signature verification tests
- [ ] T029 [P] Create RedisQueue unit tests in internal/email/queue_test.go
- [ ] T030 [P] Create auth integration tests in tests/unit/auth_email_test.go

**File Paths**:
- `internal/repository/*_test.go` - Repository unit tests
- `internal/email/*_test.go` - Email provider and template unit tests
- `internal/service/*_test.go` - Service layer unit tests
- `internal/http/**/*_test.go` - Handler unit tests

**Dependencies**: T021-T030 depend on their respective implementation phases.

**Atomic Commit**:
```bash
git add internal/repository/*_test.go internal/email/*_test.go internal/service/*_test.go internal/http/**/*_test.go
git commit -m "test(email): add unit tests for email components"
```

### Integration Tests (Sequential After Unit Tests)

- [ ] T031 Create EmailService integration test in tests/integration/email_test.go with testcontainers
- [ ] T032 Create EmailWorker integration test in tests/integration/email_worker_test.go with Redis queue
- [ ] T033 Create Auth integration test in tests/integration/auth_email_test.go with welcome email verification
- [ ] T034 [P] Create Organization integration test in tests/integration/org_email_test.go with invitation email verification
- [ ] T035 [P] Create webhook integration test in tests/integration/webhook_test.go with mock SendGrid/SES payloads
- [ ] T036 [P] Run all integration tests: `make test-integration`

**File Paths**:
- `tests/integration/email_test.go` - EmailService integration tests
- `tests/integration/email_worker_test.go` - Worker integration tests
- `tests/integration/auth_email_test.go` - Auth email integration tests
- `tests/integration/org_email_test.go` - Organization email integration tests
- `tests/integration/webhook_test.go` - Webhook integration tests

**Dependencies**: T031-T035 depend on T021-T030. T036 depends on T031-T035.

**Atomic Commit**:
```bash
git add tests/integration/email*.go tests/integration/*_email_test.go tests/integration/webhook_test.go
git commit -m "test(email): add integration tests for email workflow"
```

---

## Phase 8: Documentation & Deployment

**Goal**: Templates, documentation, permissions, and seeding.
**Tests**: Manual verification of template rendering, permission sync, and documentation completeness.
**Independent Test Criteria**:
- Templates render correctly: `go test ./internal/email -run TestTemplateRendering`
- Permissions sync: `go run ./cmd/api permission:sync` succeeds
- All templates seeded: `psql -d go_api_base -c "SELECT name FROM email_templates"`

- [ ] T037 [P] Create email templates in templates/emails/ - welcome.html, password-reset.html, invitation.html, base layout
- [ ] T037a [P] [FR-2.4] Add i18n template support in internal/email/template_i18n.go with language detection and template fallback
- [ ] T038 [P] Create documentation in docs/email-templates.md with template variables, examples, i18n notes
- [ ] T039 Add email permissions to config/permissions.yaml (template:read/write, queue:read/manage, bounce:read) and run `go run ./cmd/api permission:sync`
- [ ] T040 Create seed command in cmd/api/main.go for default templates - welcome, password-reset, invitation
- [ ] T041c [P] [NFR-3.2] Encrypt provider API keys at rest in internal/email/api_key_encryption.go using AES-256-GCM

**File Paths**:
- `templates/emails/welcome.html` - Welcome email template with UserName, AppName, LoginURL variables
- `templates/emails/password-reset.html` - Password reset template with UserName, ResetURL, ExpiresIn variables
- `templates/emails/invitation.html` - Organization invitation template with OrgName, InviterName, Role, AcceptURL variables
- `templates/emails/layouts/base.html` - Base layout with header, footer, responsive styles
- `docs/email-templates.md` - Template documentation
- `config/permissions.yaml` - Modified: Add email permissions
- `cmd/api/main.go` - Modified: Add template seeding

**Dependencies**: T037-T038 depend on Phase 7. T039-T040 depend on T037.

**Atomic Commit**:
```bash
git add templates/emails/ docs/email-templates.md config/permissions.yaml
git commit -m "docs(email): add email templates and documentation"

git add cmd/api/main.go
git commit -m "feat(email): add template seeding on startup"
```

---

## Optional: Phase 9 - Additional Providers (SHOULD/COULD)

**Goal**: Implement additional email providers (SendGrid, SES).
**Tests**: Unit tests for each provider.
**Independent Test Criteria**:
- SendGridProvider passes all tests
- SESProvider passes all tests
- Provider switching works via EMAIL_DRIVER config

- [ ] T041 [P] [US4] Implement SendGridProvider in internal/email/provider_sendgrid.go using sendgrid-go (SHOULD)
- [ ] T042 [P] [US4] Implement SESProvider in internal/email/provider_ses.go using aws-sdk-go-v2/service/ses (SHOULD)
- [ ] T043 [P] [US4] Create provider router in internal/email/provider_router.go to select provider by config
- [ ] T044 [P] [US4] Create SendGridProvider unit tests in internal/email/provider_sendgrid_test.go
- [ ] T045 [P] [US4] Create SESProvider unit tests in internal/email/provider_ses_test.go

**File Paths**:
- `internal/email/provider_sendgrid.go` - SendGridProvider implementation
- `internal/email/provider_ses.go` - SESProvider implementation
- `internal/email/provider_router.go` - Provider selection logic
- `internal/email/provider_*_test.go` - Provider unit tests

**Dependencies**: T041-T045 depend on T008 (EmailProvider interface).

**Note**: These are optional tasks. Only implement if SendGrid or SES is required.

---

## Implementation Strategy

### MVP Scope (Minimum Viable Product)

**Phase 1-6 only** (T001-T020):
- Setup & Infrastructure
- Foundational (Repositories)
- Core Email (Template, SMTP, Service)
- Async Queue
- Admin API & Webhooks
- Integration

**Estimated MVP Time**: 3 days

**MVP Acceptance Criteria**:
- [x] User receives welcome email after registration
- [x] User receives password reset email within 30 seconds
- [x] Organization member receives invitation email instantly
- [x] Admin can view email queue status
- [x] Email queue persists across restarts
- [x] Failed emails retry with exponential backoff

### Incremental Delivery

1. **Day 1**: T001-T011 (Infrastructure + Core Service)
2. **Day 2**: T012-T020 (Queue + Worker + Integration)
3. **Day 3**: T021-T036 (Testing + CI/CD green)
4. **Day 4**: T037-T040 (Documentation + Deployment)
5. **Day 5**: T041-T045 (Optional providers)

### Parallel Execution Opportunities

**Maximum Parallelization**:
- T001-T004 (4 parallel tasks)
- T008-T010 (3 parallel tasks)
- T021-T030 (10 parallel tasks)
- T034-T036 (3 parallel tasks)
- T037-T038 (2 parallel tasks)
- T041-T045 (5 parallel tasks)

**Total Parallel Tasks**: 27 of 52 (52%)

---

## QA Checklist

### Pre-Merge Checks

- [ ] All unit tests pass: `go test -v -race ./...`
- [ ] All integration tests pass: `make test-integration`
- [ ] Linting passes: `make lint`
- [ ] Coverage ≥80%: `go tool cover -func=coverage.out | grep total`
- [ ] Migrations tested: `make migrate && make migrate-down && make migrate`
- [ ] Templates render: `go test ./internal/email -run TestTemplateRendering`
- [ ] Permissions synced: `go run ./cmd/api permission:sync`
- [ ] No vulnerable dependencies: `make security-check`
- [ ] Performance benchmarks pass: `go test -bench=. ./tests/benchmark/`
  - Queue latency < 100ms (NFR-1.1)
  - Throughput ≥ 100 emails/second (NFR-1.2)
  - Template rendering < 10ms (NFR-1.3)

### Manual Testing

1. **Registration Flow**:
   - Register new user
   - Check email_queue table for 'welcome' template
   - Verify email received

2. **Password Reset Flow**:
   - Request password reset
   - Check email_queue table for 'password-reset' template
   - Verify reset link works

3. **Organization Invitation**:
   - Add member to organization
   - Check email_queue table for 'invitation' template
   - Verify invitation received

4. **Admin Dashboard**:
   - GET /api/v1/admin/emails/templates
   - GET /api/v1/admin/emails/queue/:id
   - POST /api/v1/admin/emails/queue/:id/retry

5. **Webhooks**:
   - POST /webhooks/email/sendgrid (mock delivery event)
   - Check email_queue.status updated to 'delivered'

---

## Success Metrics

| Metric | Target | Verification |
|--------|--------|-------------|
| Email queue latency | < 100ms | Integration test timing |
| Email send throughput | 100+ emails/sec | Benchmark test |
| Template rendering | < 10ms | Unit test timing |
| Test coverage | ≥ 80% | `go tool cover` |
| All unit tests pass | 100% | `go test ./...` |
| All integration tests pass | 100% | `make test-integration` |
| Linting passes | 0 issues | `make lint` |
| Race detector clean | 0 races | `go test -race` |

---

## Notes

### TDD Workflow Reminder

For each implementation task:
1. **RED**: Write failing test first
2. **GREEN**: Implement minimum code to pass
3. **REFACTOR**: Clean up, optimize
4. **VERIFY**: Ensure tests still pass, coverage ≥80%

### Constitution Compliance

- ✅ EmailTemplate uses soft delete (DeletedAt)
- ✅ EmailQueue has NO soft delete (permanent audit trail)
- ✅ EmailBounce is insert-only (compliance)
- ✅ All operations use context propagation
- ✅ Permission checks via Casbin enforcer
- ✅ SQL migration (NOT AutoMigrate)
- ✅ Audit logging via EmailQueue table

---

**Generated**: 2026-04-24 | **Total Tasks**: 52 | **Parallel Opportunities**: 27
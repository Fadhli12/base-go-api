# Implementation Plan: Email Service

**Branch**: `004-email-service` | **Date**: 2026-04-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-email-service/spec.md`

## Summary

Add comprehensive email infrastructure to the Go API Base supporting transactional emails (welcome, password reset, organization invitations), notifications (invoice created, news published), and system alerts. Implementation includes multi-provider support (SMTP, SendGrid, SES), template system with Go templates, async Redis-based queue with retry logic, and webhook handling for delivery tracking.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, PostgreSQL 15, Redis 7, testcontainers-go
**New Dependencies**: gomail (SMTP), sendgrid-go (optional), aws-sdk-go-v2/service/ses (optional)
**Storage**: PostgreSQL (email_queue, email_templates, email_bounces), Redis (queue processing)
**Testing**: Go testing package + testify + testcontainers-go
**Target Platform**: Linux server (Docker container)
**Performance Goals**: 100+ emails/second, queue latency < 100ms, template rendering < 10ms
**Scale/Scope**: Multi-tenant with thousands of users, rate limiting per user

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Check

✅ **I. Production-Ready Foundation**
- [x] Test coverage: Will add unit + integration tests (80%+ coverage target)
- [x] Error handling: All external calls wrapped, retry logic for transient failures
- [x] Observability: Structured logging via slog, email status tracking
- [x] Graceful shutdown: Worker shutdown with job completion

✅ **II. RBAC is Mandatory, Not Hardcoded**
- [x] Email permissions: email:template:read, email:template:write, email:queue:read, email:queue:manage, email:bounce:read
- [x] Admin-only access for email management endpoints
- [x] Integration with existing Casbin enforcer

✅ **III. Soft Deletes for Audit & Compliance**
- [x] EmailTemplate uses DeletedAt for soft delete (editable templates)
- [x] EmailQueue retained permanently for audit trail (NO soft delete - permanent record)
  - **Retention Policy**: EmailQueue records are NEVER deleted. Status changes tracked in-place.
  - **Audit Trail**: EmailQueue table IS the audit log - all attempts, retries, and outcomes recorded.
  - **EmailBounce table**: Separate table for bounce history, linked via message_id.
  - **Archival**: Use `archived_at` timestamp for administrative hide if needed, but records remain.
- [x] EmailBounce immutable for compliance (no updates, insert-only)

✅ **IV. Stateless JWT + Revocation**
- [x] No changes to JWT system
- [x] Email independent of auth flow

✅ **V. PostgreSQL + Versioned Migrations**
- [x] Migration 000008_email_service.up.sql
- [x] No AutoMigrate used
- [x] Migration files committed to repo

✅ **VI. Multi-Instance Permission Consistency**
- [x] Redis queue shared across instances
- [x] Worker coordination via Redis BRPOP
- [x] Idempotent job processing

✅ **VII. Audit Logging Non-Negotiable**
- [x] All email operations logged in EmailQueue table (status transitions)
- [x] Actor ID tracked in EmailQueue (system-generated emails use system actor)
- [x] Before/after state captured via status field (queued → sending → sent/delivered/bounced)
- [x] EmailBounce table records bounce/complaint history for compliance

### Gate Status: PASSED ✓

No constitution violations detected. Design aligns with all core principles.

## Project Structure

### Documentation (this feature)

```text
specs/004-email-service/
├── spec.md              # Feature specification (this file)
├── plan.md              # Implementation plan (this file)
├── research.md          # Phase 0 output - email providers, queue patterns
├── data-model.md        # Phase 1 output - Entity diagrams, relationships
├── quickstart.md        # Phase 1 output - Usage examples
├── contracts/
│   └── api-v1.md        # REST endpoint specifications
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── config.go           # Add EmailConfig struct
├── domain/
│   ├── email_queue.go      # EmailQueue entity
│   ├── email_template.go   # EmailTemplate entity
│   └── email_bounce.go     # EmailBounce entity
├── repository/
│   └── email.go            # EmailQueueRepository interface
├── service/
│   ├── email.go            # EmailService interface
│   ├── email_impl.go       # EmailService implementation
│   └── email_worker.go     # Background worker
├── http/
│   ├── handler/
│   │   └── email.go        # Admin email endpoints
│   └── middleware/
│       └── webhook.go      # Provider webhook verification
├── email/
│   ├── provider.go         # EmailProvider interface
│   ├── provider_smtp.go    # SMTP implementation
│   ├── provider_sendgrid.go # SendGrid implementation (optional)
│   ├── provider_ses.go    # AWS SES implementation (optional)
│   ├── template.go         # Template engine
│   └── queue.go             # Redis queue operations
migrations/
├── 000008_email_service.up.sql    # Create tables
└── 000008_email_service.down.sql  # Rollback
templates/
└── emails/
    ├── auth/
    │   ├── welcome.html
    │   └── password-reset.html
    ├── organization/
    │   └── invitation.html
    └── layouts/
        └── base.html
tests/
└── integration/
    └── email_test.go       # Integration tests
```

**Structure Decision**: Following existing Go API Base patterns - repository pattern, service layer, Echo handlers. New `internal/email/` package for provider abstraction and queue management.

## Complexity Tracking

> **Moderate complexity** - Multi-provider abstraction, async queue implementation, template system. Following established patterns from storage and cache systems.

### Complexity Areas

| Component | Complexity | Mitigation |
|-----------|------------|------------|
| Multi-provider abstraction | Medium | Follow storage/cache driver patterns |
| Async queue worker | Medium | Similar to audit log buffering pattern |
| Template rendering | Low | Go html/template standard library |
| Webhook signature verification | Medium | Provider-specific signature validation |
| Retry with backoff | Low | Standard exponential backoff algorithm |

## Phase 0: Research Summary

Based on codebase exploration:

### Existing Patterns

1. **Config Pattern**: `parseXConfig(v, cfg)` functions in `internal/config/config.go`
   - Add `parseEmailConfig()` following same pattern
   - EmailConfig struct with SMTP, SendGrid, SES nested configs

2. **Multi-Driver Pattern**: Storage (local/S3/MinIO) and Cache (redis/memory)
   - Apply same abstraction for EmailProvider interface
   - Provider router selects implementation at runtime

3. **Audit Log Pattern**: Buffered channel for async processing
   - Similar pattern for email queue worker
   - Redis-based queue for persistence across restarts

4. **Organization Integration**: `OrganizationService.AddMember()` needs invite emails
   - Inject EmailService into OrganizationService
   - Queue emails without blocking request

### Integration Points Identified

1. **AuthService.Register()** - Queue welcome email after user creation
2. **AuthService.RequestPasswordReset()** - Queue password reset email
3. **OrganizationService.AddMember()** - Queue organization invitation email
4. **Future: Invoice/News events** - Event-driven email triggers

### No Existing Email Infrastructure

- No email libraries in go.mod
- No SMTP/config in existing codebase
- No template rendering system
- Redis available but unused for queueing

## Phase 1: Data Model

### Entity Relationships

```
┌─────────────────┐
│   User          │
│   (email)       │
└────────┬────────┘
         │
         │ to (recipient)
         ▼
┌─────────────────────┐     ┌────────────────────┐
│    EmailQueue       │────▶│   EmailTemplate    │
│    (queue entry)    │     │   (template ref)   │
│    - status         │     │   - name           │
│    - attempts       │     │   - subject         │
│    - provider       │     │   - html/text      │
└────────┬────────────┘     └────────────────────┘
         │
         │ message_id
         ▼
┌─────────────────────┐
│    EmailBounce      │
│    (delivery fail)  │
│    - email          │
│    - bounce_type    │
│    - bounce_reason  │
└─────────────────────┘
```

### Database Schema

```sql
-- Migration: 000008_email_service.up.sql

CREATE TABLE email_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    subject VARCHAR(255) NOT NULL,
    html_content TEXT NOT NULL,
    text_content TEXT,
    category VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT ck_template_name CHECK (name ~ '^[a-z0-9_-]+$')
);

CREATE TABLE email_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    to_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    template VARCHAR(100),
    data JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    provider VARCHAR(50),
    message_id VARCHAR(255),
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 5,
    last_error TEXT,
    sent_at TIMESTAMP WITH TIME ZONE,
    delivered_at TIMESTAMP WITH TIME ZONE,
    bounced_at TIMESTAMP WITH TIME ZONE,
    bounce_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT ck_status CHECK (status IN ('queued', 'sending', 'sent', 'failed', 'bounced'))
);

CREATE TABLE email_bounces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    bounce_type VARCHAR(50) NOT NULL,
    bounce_reason TEXT,
    message_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT ck_bounce_type CHECK (bounce_type IN ('hard', 'soft', 'spam'))
);

-- Indexes
CREATE INDEX idx_email_templates_name ON email_templates(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_email_templates_category ON email_templates(category) WHERE deleted_at IS NULL;
CREATE INDEX idx_email_queue_status_created ON email_queue(status, created_at);
CREATE INDEX idx_email_queue_to_created ON email_queue(to_address, created_at DESC);
CREATE INDEX idx_email_queue_message_id ON email_queue(message_id);
CREATE INDEX idx_email_bounces_email_created ON email_bounces(email, created_at DESC);
```

## Task Dependency Graph

**Execution Order: Tasks must complete in dependency order before dependents can start.**

```
Layer 0 (Foundational - Parallel):
├── T01: Create EmailConfig                    [dep: none]
├── T02: Create Domain Entities               [dep: none]
└── T03: Create Migration 000008              [dep: T02]

Layer 1 (Data Layer - After Layer 0):
├── T04: Create EmailQueueRepository          [dep: T03]
└── T05: Create EmailTemplate Repository      [dep: T03]

Layer 2 (Provider Layer - Parallel with Layer 1):
├── T06: Create EmailProvider Interface        [dep: none]
├── T07: Implement SMTPProvider               [dep: T06]
├── T08: Implement SendGridProvider            [dep: T06] (optional)
└── T09: Implement SESProvider                 [dep: T06] (optional)

Layer 3 (Core Service - After Layers 1 & 2):
├── T10: Create TemplateEngine               [dep: none]
└── T11: Create EmailService                 [dep: T04, T05, T10, T07]

Layer 4 (Integration - After Layer 3):
├── T12: Integrate AuthService                [dep: T11]
└── T13: Integrate OrganizationService        [dep: T11]

Layer 5 (Background Processing - After Layer 4):
├── T14: Create EmailWorker                   [dep: T11]
└── T15: Create Redis Queue Integration       [dep: T11]

Layer 6 (HTTP Layer - After Layer 5):
├── T16: Create Admin Email Handlers          [dep: T11]
└── T17: Create Webhook Handlers              [dep: T11]

Layer 7 (Testing & Documentation - After All):
├── T18: Unit Tests                           [dep: T17]
├── T19: Integration Tests                     [dep: T17]
└── T20: Documentation                         [dep: T18, T19]

Layer 8 (Deployment - After Layer 7):
├── T21: Permission Sync                       [dep: T20]
└── T22: Templates Seeding                     [dep: T20]
```

**Parallel Execution Waves:**
- **Wave 1**: T01, T02, T03, T06, T10 (foundational, no dependencies)
- **Wave 2**: T04, T05, T07, T08, T09 (data layer + providers)
- **Wave 3**: T11 (core service, waits for Wave 2)
- **Wave 4**: T12, T13, T14, T15, T16, T17 (integration + HTTP)
- **Wave 5**: T18, T19 (testing)
- **Wave 6**: T20, T21, T22 (documentation + deployment)

---

## Category + Skills Recommendations

**Each task specifies which agent category and skills to use.**

| Task | Category | Skills | Rationale |
|------|----------|--------|-----------|
| T01: EmailConfig | `quick` | [] | Single file, follows parseStorageConfig pattern |
| T02: Domain Entities | `quick` | [] | Boilerplate structs, existing domain patterns |
| T03: Migration | `quick` | [] | SQL follows existing migration format |
| T04: EmailQueueRepository | `quick` | [] | Interface + GORM, standard CRUD pattern |
| T05: EmailTemplateRepository | `quick` | [] | Same as T04 |
| T06: EmailProvider Interface | `quick` | [] | Pure interface definition |
| T07: SMTPProvider | `deep` | [`test-driven-development`] | External API, error handling, retries |
| T08: SendGridProvider | `deep` | [`test-driven-development`] | Same as T07 |
| T09: SESProvider | `deep` | [`test-driven-development`] | Same as T07 |
| T10: TemplateEngine | `quick` | [] | Go html/template, standard library |
| T11: EmailService | `deep` | [`test-driven-development`] | Core business logic, integrates components |
| T12: Auth Integration | `quick` | [] | Inject service, call methods |
| T13: Org Integration | `quick` | [] | Same as T12 |
| T14: EmailWorker | `deep` | [`test-driven-development`] | Background processing, graceful shutdown |
| T15: Redis Queue | `deep` | [`test-driven-development`] | Queue processing, error handling |
| T16: Admin Handlers | `quick` | [] | HTTP routing, follows existing patterns |
| T17: Webhook Handlers | `deep` | [`test-driven-development`] | Signature verification, event processing |
| T18: Unit Tests | `quick` | [`test-driven-development`] | Test file creation |
| T19: Integration Tests | `quick` | [`test-driven-development`] | Testcontainers suite |
| T20: Documentation | `unspecified-low` | [] | Markdown files |
| T21: Permission Sync | `quick` | [] | Add to seed, run command |
| T22: Templates Seeding | `unspecified-low` | [] | Seed data |

---

## QA Scenarios

**Each scenario specifies: tool, steps, expected output, and pass/fail criteria.**

### QA: Email Domain Migration

**Tool**: `psql` (PostgreSQL client)

**Steps**:
```bash
make migrate
psql -d go_api_base -c "\dt email_*"
psql -d go_api_base -c "\di email_*"
make migrate-down
psql -d go_api_base -c "\dt email_*"  # Should show no tables
make migrate
```

**Expected Output**:
```
 Schema |           Name            | Type  |  Owner   
--------+---------------------------+-------+----------
 public | email_bounces             | table | postgres
 public | email_queue               | table | postgres
 public | email_templates           | table | postgres
```

**Pass**: All 3 tables exist, 6+ indexes, rollback succeeds.

**Fail**: Missing tables or indexes, rollback fails.

---

### QA: SMTP Provider Unit Tests

**Tool**: `go test`

**Steps**:
```bash
go test -v -race -coverprofile=coverage.out ./internal/email/
go tool cover -func=coverage.out | grep total
```

**Expected Output**:
```
=== RUN   TestSMTPProvider_Send
--- PASS: TestSMTPProvider_Send (0.23s)
=== RUN   TestSMTPProvider_Send_Timeout
--- PASS: TestSMTPProvider_Send_Timeout (5.23s)
PASS
ok      github.com/example/go-api-base/internal/email    5.692s
coverage: 85.2% of statements
```

**Pass**: All tests pass, coverage ≥ 80%, race detector clean.

**Fail**: Any test FAIL, coverage < 80%, race detected.

---

### QA: Email Service Registration Hook

**Tool**: `go test -tags=integration`

**Steps**:
```bash
make docker-run
go test -v -tags=integration ./tests/integration/ -run TestAuthRegister_EmailSent
psql -d go_api_base -c "SELECT template, status FROM email_queue WHERE to_address='test@example.com'"
```

**Expected Output**:
```
=== RUN   TestAuthRegister_EmailSent
--- PASS: TestAuthRegister_EmailSent (0.18s)
```
```
 template | status  
---------+---------
 welcome | queued
```

**Pass**: Email queued after registration with template='welcome'.

**Fail**: No email queued, wrong template.

---

### QA: Permission Sync

**Tool**: `go run ./cmd/api permission:sync`

**Steps**:
```bash
go run ./cmd/api permission:sync
psql -d go_api_base -c "SELECT resource, action FROM permissions WHERE resource LIKE 'email%'"
```

**Expected Output**:
```
Permission sync completed. Added: 5, Updated: 0, Removed: 0

   resource    | action  
---------------+---------
 email_template | read
 email_template | write
 email_queue    | read
 email_queue    | manage
 email_bounce   | read
```

**Pass**: 5 email permissions created.

**Fail**: Missing permissions, permission:sync fails.

---

## Atomic Commit Strategy

**Commits are atomic units that can be reverted independently.**

### Commit 1: Infrastructure (Domain + Migration)

```bash
git add internal/domain/email_queue.go internal/domain/email_template.go internal/domain/email_bounce.go
git add migrations/000008_email_service.up.sql migrations/000008_email_service.down.sql
git commit -m "feat(email): add email entities and migration (000008)

- Add EmailQueue entity for async queue tracking
- Add EmailTemplate entity for template management  
- Add EmailBounce entity for bounce/complaint tracking
- Create migration 000008_email_service
- Add indexes for queue processing, webhook correlation"
```

**Rollback**: Removes all email infrastructure.

---

### Commit 2: Configuration

```bash
git add internal/config/config.go .env.example
git commit -m "feat(email): add email configuration

- Add EmailConfig struct following parseStorageConfig pattern
- Add SMTP, SendGrid, SES provider configs
- Add queue configuration (concurrency, retry, template_dir)
- Update .env.example with EMAIL_* variables"
```

**Rollback**: Reverts config, application still runs (email disabled).

---

### Commit 3: Repository Layer

```bash
git add internal/repository/email_queue.go internal/repository/email_template.go internal/repository/email_bounce.go
git commit -m "feat(email): add email repository layer

- Add EmailQueueRepository interface and GORM implementation
- Add EmailTemplateRepository for template management
- Add EmailBounceRepository for bounce tracking
- Include context propagation, soft delete for templates"
```

**Rollback**: Services can't compile.

---

### Commit 4: Provider + Template Engine

```bash
git add internal/email/provider.go internal/email/provider_smtp.go internal/email/template.go
git commit -m "feat(email): add SMTP provider and template engine

- Add EmailProvider interface for multi-driver abstraction
- Implement SMTPProvider using gomail
- Add TemplateEngine using html/template
- Support HTML and text templates with variable substitution"
```

**Rollback**: No email sending, but queue accepts emails.

---

### Commit 5: Core Service

```bash
git add internal/service/email.go internal/service/email_impl.go
git commit -m "feat(email): add EmailService implementation

- Add EmailService interface (Send, SendTemplate, GetStatus)
- Implement queue-based email sending with Redis
- Add rate limiting per recipient (100 emails/hour)
- Add bounce suppression (hard bounces blocked 90 days)
- Include retry logic with exponential backoff"
```

**Rollback**: No email functionality.

---

### Commit 6: Background Worker

```bash
git add internal/service/email_worker.go internal/email/queue.go
git commit -m "feat(email): add async queue worker

- Add EmailWorker for background email processing
- Implement Redis-based queue with BRPOP
- Add graceful shutdown (finish current job)
- Implement retry with exponential backoff"
```

**Rollback**: Emails queue but aren't sent.

---

### Commit 7: Integration Hooks

```bash
git add internal/service/auth.go internal/service/organization.go cmd/api/main.go
git commit -m "feat(email): integrate email into auth and org flows

- Inject EmailService into AuthService constructor
- Queue welcome email after registration
- Queue password reset email on request
- Inject EmailService into OrganizationService
- Queue invitation email when member added
- Update DI container in main.go"
```

**Rollback**: AuthService and OrganizationService revert to non-email versions.

---

### Commit 8: HTTP Handlers + Webhooks

```bash
git add internal/http/handler/email.go internal/http/middleware/webhook.go internal/http/routes.go
git commit -m "feat(email): add admin endpoints and webhooks

- Add admin endpoints for template/queue management
- Add webhook handlers for SendGrid, SES delivery events
- Add bounce/complaint processing
- Route /api/v1/admin/emails/* and /webhooks/email/*
- Require admin permissions for email endpoints"
```

**Rollback**: No admin UI for emails, webhooks return 404.

---

### Commit 9: Tests

```bash
git add tests/unit/email/ tests/integration/email_test.go
git commit -m "test(email): add unit and integration tests

- Add unit tests for SMTP provider (100% coverage)
- Add unit tests for template engine
- Add integration tests for email workflow
- Add integration tests for worker processing
- All tests pass, race detector clean"
```

**Rollback**: Lose test coverage.

---

### Commit 10: Documentation + Templates

```bash
git add templates/emails/ docs/email-templates.md specs/004-email-service/
git commit -m "docs(email): add templates and documentation

- Add welcome.html template
- Add password-reset.html template
- Add invitation.html template
- Add base layout template
- Add email-templates.md documentation
- Add feature specification and implementation plan"
```

**Rollback**: Lose documentation, application runs fine.

---

## Integration Details

### AuthService Modifications

**File**: `internal/service/auth.go`

**Current State** (auth.go):
```go
type AuthService struct {
    userRepo       repository.UserRepository
    tokenRepo      repository.RefreshTokenRepository
    tokenService   *TokenService
    passwordHasher PasswordHasher
    // NO EMAIL SERVICE
}
```

**Required Changes**:

1. **Add EmailService field**:
```go
type AuthService struct {
    userRepo       repository.UserRepository
    tokenRepo      repository.RefreshTokenRepository
    tokenService   *TokenService
    passwordHasher PasswordHasher
    email          EmailService  // ADD THIS
    logger         *slog.Logger
}
```

2. **Update constructor**:
```go
func NewAuthService(
    userRepo repository.UserRepository,
    tokenRepo repository.RefreshTokenRepository,
    tokenService *TokenService,
    passwordHasher PasswordHasher,
    email EmailService,  // ADD PARAMETER
    logger *slog.Logger,
) *AuthService {
    return &AuthService{
        userRepo:       userRepo,
        tokenRepo:       tokenRepo,
        tokenService:   tokenService,
        passwordHasher: passwordHasher,
        email:          email,  // ADD THIS
        logger:         logger,
    }
}
```

3. **Integrate in Register method**:
```go
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
    // ... create user ...
    
    // Queue welcome email (ASYNC, non-blocking)
    go func() {
        bgCtx := context.Background()
        _, err := s.email.SendTemplate(bgCtx, user.Email, "welcome", map[string]interface{}{
            "UserName": user.Name,
            "AppName":  s.config.AppName,
            "LoginURL": s.config.BaseURL + "/login",
        })
        if err != nil {
            s.logger.Warn("failed to queue welcome email", "error", err)
        }
    }()
    
    return user, nil
}
```

4. **Integrate in RequestPasswordReset method**:
```go
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
    // ... generate reset token ...
    
    // Queue password reset email (SYNC, wait for queue)
    _, err := s.email.SendTemplate(ctx, user.Email, "password-reset", map[string]interface{}{
        "UserName":  user.Name,
        "ResetURL":  resetURL,
        "ExpiresIn": "15 minutes",
    })
    if err != nil {
        return fmt.Errorf("failed to queue password reset email: %w", err)
    }
    
    return nil
}
```

5. **Update DI in cmd/api/main.go**:
```go
// After creating EmailService
emailService := email.NewService(emailRepo, templateEngine, providers, redisClient, logger)

// Pass to AuthService
authService := auth.NewService(
    userRepo,
    tokenRepo,
    tokenService,
    passwordHasher,
    emailService,  // ADD THIS
    logger,
)
```

---

### OrganizationService Modifications

**File**: `internal/service/organization.go`

**Current State** (organization.go):
```go
type OrganizationService struct {
    orgRepo    repository.OrganizationRepository
    userRepo   repository.UserRepository
    enforcer   *permission.Enforcer
    audit      *AuditService
    // NO EMAIL SERVICE
}
```

**Required Changes**:

1. **Add EmailService field**:
```go
type OrganizationService struct {
    orgRepo    repository.OrganizationRepository
    userRepo   repository.UserRepository
    enforcer   *permission.Enforcer
    audit      *AuditService
    email      EmailService  // ADD THIS
    logger     *slog.Logger
}
```

2. **Update constructor**:
```go
func NewOrganizationService(
    orgRepo repository.OrganizationRepository,
    userRepo repository.UserRepository,
    enforcer *permission.Enforcer,
    audit *AuditService,
    email EmailService,  // ADD PARAMETER
    logger *slog.Logger,
) *OrganizationService {
    return &OrganizationService{
        orgRepo:  orgRepo,
        userRepo: userRepo,
        enforcer: enforcer,
        audit:    audit,
        email:    email,  // ADD THIS
        logger:   logger,
    }
}
```

3. **Integrate in AddMember method** (around line 300):
```go
func (s *OrganizationService) AddMember(ctx context.Context, orgID, userID uuid.UUID, req AddMemberRequest) error {
    // ... add member to org ...
    
    // Queue invitation email (ASYNC, non-blocking)
    go func() {
        bgCtx := context.Background()
        _, err := s.email.SendTemplate(bgCtx, req.Email, "org-invitation", map[string]interface{}{
            "OrgName":     org.Name,
            "InviterName": inviter.Name,
            "Role":        req.Role,
            "AcceptURL":   acceptURL,
            "ExpiresIn":    "7 days",
        })
        if err != nil {
            s.logger.Warn("failed to queue invitation email", "error", err)
        }
    }()
    
    return nil
}
```

4. **Update DI in cmd/api/main.go**:
```go
// Pass to OrganizationService
orgService := organization.NewService(
    orgRepo,
    userRepo,
    enforcer,
    auditService,
    emailService,  // ADD THIS
    logger,
)
```

---

### Transaction Boundaries

**Email operations MUST NOT be in the same transaction as business logic.**

**Registration Flow**:
```go
// 1. DB Transaction: Create user
tx := s.userRepo.BeginTx(ctx)
defer tx.Rollback()

user, err := s.userRepo.CreateInTx(ctx, tx, user)
if err != nil {
    return nil, err
}

// 2. Commit user transaction
if err := tx.Commit(); err != nil {
    return nil, err
}

// 3. Queue email ASYNC (new goroutine, new context, no transaction)
go func() {
    _, err := s.email.SendTemplate(context.Background(), user.Email, "welcome", data)
    if err != nil {
        s.logger.Warn("email queue failed", "error", err)
    }
}()

return user, nil
```

**Why Async?**
- Email queuing is slow (Redis write, DB insert)
- User creation should be fast (< 100ms)
- Email failure should NOT block registration (graceful degradation)

---

### Permission Sync

**File**: Add to `config/permissions.yaml`:

```yaml
# Email service permissions
- resource: "email_template"
  actions:
    - action: "read"
      description: "View email templates"
    - action: "write"
      description: "Create and edit email templates"

- resource: "email_queue"
  actions:
    - action: "read"
      description: "View email queue status"
    - action: "manage"
      description: "Retry, cancel, and manage queued emails"

- resource: "email_bounce"
  actions:
    - action: "read"
      description: "View bounce history"
```

**Run Permission Sync**:
```bash
go run ./cmd/api permission:sync
```

**Verify Permissions**:
```bash
psql -d go_api_base -c "SELECT resource, action FROM permissions WHERE resource LIKE 'email%'"
```

---

## TDD Workflows

### TDD: SMTP Provider

**Step 1: RED - Write failing test**
```bash
# Create test file
touch internal/email/provider_smtp_test.go
```

```go
// internal/email/provider_smtp_test.go
func TestSMTPProvider_Send(t *testing.T) {
    // Mock SMTP server
    server := mocksmtp.New(t)
    defer server.Close()
    
    provider := NewSMTPProvider(SMTPConfig{
        Host:     server.Host,
        Port:    server.Port,
        User:    "test",
        Pass:    "test",
    })
    
    email := &Email{
        To:       "recipient@example.com",
        Subject:  "Test Subject",
        HTMLBody: "<p>Test body</p>",
    }
    
    // This will FAIL because we haven't implemented yet
    msgID, err := provider.Send(context.Background(), email)
    require.NoError(t, err)
    require.NotEmpty(t, msgID)
}
```

**Step 2: Run failing test**
```bash
go test ./internal/email -run TestSMTPProvider_Send
# Expected: FAIL (undefined: NewSMTPProvider)
```

**Step 3: GREEN - Implement minimum code**
```bash
# Create implementation
touch internal/email/provider_smtp.go
```

```go
// internal/email/provider_smtp.go
func NewSMTPProvider(config SMTPConfig) *SMTPProvider {
    return &SMTPProvider{config: config}
}

func (p *SMTPProvider) Send(ctx context.Context, email *Email) (string, error) {
    // ... gomail implementation ...
    return msgID, nil
}
```

**Step 4: Run passing test**
```bash
go test ./internal/email -run TestSMTPProvider_Send
# Expected: PASS
```

**Step 5: REFACTOR - Clean up**
- Add error cases (timeout, auth failure)
- Add attachment support
- Add test coverage for edge cases

**Step 6: Verify coverage**
```bash
go test -v -race -coverprofile=coverage.out ./internal/email/
go tool cover -func=coverage.out | grep total
# Expected: ≥80%
```

---

### TDD: Email Queue Worker

**Step 1: RED - Write failing test**
```go
// tests/integration/email_worker_test.go
func TestEmailWorker_ProcessQueue(t *testing.T) {
    suite := testsuite.New(t)
    defer suite.Cleanup()
    
    // Create test email in queue
    email := &domain.EmailQueue{
        To:      "test@example.com",
        Subject: "Test",
        Status:  "queued",
    }
    require.NoError(t, suite.DB.Create(email).Error)
    
    // Create worker
    worker := email.NewWorker(emailService, suite.Redis, logger)
    
    // Start worker
    go worker.Start()
    defer worker.Stop()
    
    // Wait for processing
    time.Sleep(2 * time.Second)
    
    // Verify email was processed
    var updated domain.EmailQueue
    require.NoError(t, suite.DB.First(&updated, email.ID).Error)
    assert.Equal(t, "sent", updated.Status)
}
```

**Step 2: Run failing test**
```bash
go test -v -tags=integration ./tests/integration/ -run TestEmailWorker_ProcessQueue
# Expected: FAIL (worker not implemented)
```

**Step 3: GREEN - Implement**
```go
// internal/service/email_worker.go
func (w *EmailWorker) Start() {
    for {
        select {
        case <-w.stopChan:
            return
        default:
            w.processNext()
        }
    }
}
```

**Step 4: Run passing test**
```bash
go test -v -tags=integration ./tests/integration/ -run TestEmailWorker_ProcessQueue
# Expected: PASS
```

---

## Phase 1: Quickstart Examples

### Send Welcome Email

```go
// After user registration
emailReq := email.SendRequest{
    To:       user.Email,
    Template: "welcome",
    TemplateData: map[string]interface{}{
        "UserName": user.Name,
        "AppName":  config.AppName,
        "LoginURL": config.BaseURL + "/login",
    },
    Priority: email.PriorityNormal,
}

emailQueue, err := emailService.Send(ctx, emailReq)
if err != nil {
    log.Warn("failed to queue welcome email", "error", err)
    // Don't fail registration
}
```

### Send Password Reset

```go
// Password reset flow
resetToken := generateResetToken()
resetURL := fmt.Sprintf("%s/reset-password?token=%s", config.BaseURL, resetToken)

err := emailService.SendTemplate(ctx, user.Email, "password-reset", map[string]interface{}{
    "UserName":  user.Name,
    "ResetURL":  resetURL,
    "ExpiresIn": "15 minutes",
})
```

### Check Email Status (Admin)

```go
// GET /api/v1/admin/emails/queue/:id
emailQueue, err := emailService.GetStatus(ctx, emailID)
if err != nil {
    return echo.NewHTTPError(http.StatusNotFound, "email not found")
}

return c.JSON(http.StatusOK, emailQueue)
```

## Phase 1: API Contracts

### Admin Endpoints

```yaml
# List Email Templates
GET /api/v1/admin/emails/templates
Authorization: Bearer {token}
X-Organization-ID: {org_id}

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "name": "welcome",
      "subject": "Welcome to {{.AppName}}",
      "category": "auth",
      "is_active": true,
      "created_at": "2026-04-24T10:00:00Z"
    }
  ],
  "meta": {"total": 1, "page": 1}
}

# Update Email Template
PUT /api/v1/admin/emails/templates/:name
Authorization: Bearer {token}
Content-Type: application/json

{
  "subject": "Welcome to {{.AppName}}!",
  "html_content": "<html>...</html>",
  "text_content": "Welcome...",
  "is_active": true
}

Response 200:
{
  "data": {
    "id": "uuid",
    "name": "welcome",
    "subject": "Welcome to {{.AppName}}!",
    "updated_at": "2026-04-24T10:05:00Z"
  }
}

# Get Email Status
GET /api/v1/admin/emails/queue/:id
Authorization: Bearer {token}

Response 200:
{
  "data": {
    "id": "uuid",
    "to": "user@example.com",
    "subject": "Welcome to App",
    "status": "sent",
    "provider": "sendgrid",
    "message_id": "msg_123",
    "attempts": 1,
    "sent_at": "2026-04-24T10:00:05Z",
    "delivered_at": "2026-04-24T10:00:07Z"
  }
}

# Retry Failed Email
POST /api/v1/admin/emails/queue/:id/retry
Authorization: Bearer {token}

Response 200:
{
  "data": {
    "id": "uuid",
    "status": "queued",
    "attempts": 0,
    "message": "Email queued for retry"
  }
}

# List Bounces
GET /api/v1/admin/emails/bounces?email=user@example.com
Authorization: Bearer {token}

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "bounce_type": "hard",
      "bounce_reason": "550 5.1.1 User not found",
      "created_at": "2026-04-24T09:00:00Z"
    }
  ]
}
```

### Webhook Endpoints

```yaml
# SendGrid Webhook
POST /webhooks/email/sendgrid
Content-Type: application/json
X-Sendgrid-Signature: {signature}

[
  {
    "event": "delivered",
    "message_id": "msg_123",
    "email": "user@example.com",
    "timestamp": 1612345678
  }
]

Response 200:
{
  "status": "processed"
}

# AWS SES Webhook (SNS)
POST /webhooks/email/ses
Content-Type: application/json
X-Amz-Sns-Message-Type: Notification

{
  "Type": "Notification",
  "Message": "{\"notificationType\":\"Delivery\",\"mail\":{\"messageId\":\"msg_123\"},\"delivery\":{\"recipients\":[\"user@example.com\"]}}"
}

Response 200:
{}
```
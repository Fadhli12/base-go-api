# Feature Specification: Email Service

**Version**: 1.0 | **Date**: 2026-04-24 | **Status**: Draft

---

## Overview

### Problem Statement

The Go API Base currently has no email infrastructure. Users cannot receive:
- Welcome emails after registration
- Password reset links
- Organization invitation notifications
- Transactional alerts (invoice created, news published)
- System notifications

This creates friction for:
- User onboarding (no welcome flow)
- Security workflows (password reset requires manual intervention)
- Collaboration (organization members don't know they've been invited)
- Engagement (no notification of important events)

### Proposed Solution

Add a comprehensive email service with:
- **Multi-provider support**: SMTP, SendGrid, AWS SES, Mailgun
- **Template system**: HTML/text templates with variable substitution
- **Async queue**: Background sending via Redis queue
- **Transaction tracking**: Email status and delivery confirmation
- **Integration points**: Auth, Organization, and event hooks

### Success Criteria

- ✅ Users receive welcome email on registration
- ✅ Password reset emails sent within 30 seconds
- ✅ Organization invite emails delivered instantly
- ✅ Configurable email providers without code changes
- ✅ Template-based emails with i18n support
- ✅ Email queue with retry logic (exponential backoff)
- ✅ Bounce and complaint tracking (webhooks)
- ✅ Full audit trail for all email operations

---

## Requirements

### Functional Requirements

#### FR-1: Email Sending

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | Send transactional emails (welcome, password reset, invites) | MUST |
| FR-1.2 | Send notification emails (alerts, system notifications) | MUST |
| FR-1.3 | Support multiple email providers (SMTP, SendGrid, SES, Mailgun) | MUST |
| FR-1.4 | Queue emails asynchronously for background delivery | MUST |
| FR-1.5 | Retry failed emails with exponential backoff | MUST |
| FR-1.6 | Track email delivery status (queued, sent, delivered, failed, bounced) | SHOULD |

#### FR-2: Template System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | HTML/text templates stored in filesystem | MUST |
| FR-2.2 | Variable substitution with Go templates | MUST |
| FR-2.3 | Template preview endpoint for admins | SHOULD |
| FR-2.4 | i18n support for multiple languages | SHOULD |

#### FR-3: Queue System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | Redis-based email queue | MUST |
| FR-3.2 | Worker process for email sending | MUST |
| FR-3.3 | Configurable concurrency (default: 10 workers) | SHOULD |
| FR-3.4 | Dead letter queue for permanent failures | MUST |

#### FR-4: Provider Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | SMTP provider (baseline) | MUST |
| FR-4.2 | SendGrid API integration | SHOULD |
| FR-4.3 | AWS SES integration | SHOULD |
| FR-4.4 | Mailgun integration | COULD |
| FR-4.5 | Provider abstraction interface | MUST |

#### FR-5: Webhooks & Events

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-5.1 | Webhook endpoints for delivery status (SendGrid, SES) | SHOULD |
| FR-5.2 | Bounce handling and complaint tracking | SHOULD |
| FR-5.3 | Event publishing (email.sent, email.bounced, email.complaint) | COULD |

### Non-Functional Requirements

#### NFR-1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | Email queue latency | < 100ms |
| NFR-1.2 | Email send throughput | 100+ emails/second |
| NFR-1.3 | Template rendering | < 10ms |

#### NFR-2: Reliability

| ID | Requirement |
|----|-------------|
| NFR-2.1 | Email queue persists across restarts (Redis RDB) |
| NFR-2.2 | Retry on transient failures (up to 5 attempts) |
| NFR-2.3 | Dead letter queue for permanent failures |
| NFR-2.4 | Graceful degradation (log and continue if queue unavailable) |

#### NFR-3: Security

| ID | Requirement |
|----|-------------|
| NFR-3.1 | Email content sanitized (XSS prevention) |
| NFR-3.2 | API keys stored encrypted at rest |
| NFR-3.3 | SMTP credentials in environment variables (never logged) |
| NFR-3.4 | Rate limiting per user (prevent abuse) |
| NFR-3.5 | Unsubscribe links in marketing emails (CAN-SPAM compliance) |

---

## Data Model

### Entities

#### EmailTemplate

```go
type EmailTemplate struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name        string         `gorm:"size:100;not null;uniqueIndex"` // "welcome", "password-reset"
    Subject     string         `gorm:"size:255;not null"`             // "Welcome to {{.AppName}}"
    HTMLContent string         `gorm:"type:text;not null"`            // HTML template
    TextContent string         `gorm:"type:text"`                     // Plain text version
    Category    string         `gorm:"size:50;not null;index"`        // "auth", "notification", "marketing"
    IsActive    bool           `gorm:"default:true;index"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}
```

#### EmailQueue

```go
type EmailQueue struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    To          string         `gorm:"column:to_address;size:255;not null;index"` // Recipient email (column: to_address)
    Subject     string         `gorm:"size:500;not null"`
    Template    string         `gorm:"size:100"`                     // Template name (optional)
    Data        datatypes.JSON `gorm:"type:jsonb"`                   // Template variables
    Status      string         `gorm:"size:20;not null;index"`       // "queued", "sending", "sent", "failed", "bounced"
    Provider     string         `gorm:"size:50"`                      // "smtp", "sendgrid", "ses"
    MessageID   string         `gorm:"size:255;index"`               // Provider message ID
    Attempts     int            `gorm:"default:0"`                    // Retry count
    MaxAttempts int            `gorm:"default:5"`                    // Max retries
    LastError   string         `gorm:"type:text"`                    // Last error message
    SentAt      *time.Time     `gorm:"index"`                        // When sent
    DeliveredAt *time.Time     `gorm:"index"`                        // When delivered (webhook)
    BounceAt    *time.Time                                        // When bounced
    BounceReason string         `gorm:"type:text"`                    // Bounce details
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Note**: EmailQueue does NOT use soft delete (no DeletedAt field). Records are retained permanently for audit trail and compliance.

#### EmailBounce

```go
type EmailBounce struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Email        string         `gorm:"size:255;not null;index"`     // Bounced email
    BounceType   string         `gorm:"size:50;not null"`             // "hard", "soft", "spam"
    BounceReason string         `gorm:"type:text"`                    // Provider bounce message
    MessageID    string         `gorm:"size:255;index"`               // Original message ID
    CreatedAt    time.Time
}

// Composite index for bounce lookups
// CREATE INDEX idx_email_bounces_email_created ON email_bounces(email, created_at DESC);
```

### Relationships

```
User (1) ────────< (N) EmailQueue (recipient)
                        │
                     Status: queued, sending, sent, failed, bounced

EmailTemplate ────────> EmailQueue (optional reference)
```

### Indexes

- `email_templates(name)` - UNIQUE for template lookup
- `email_queue(status, created_at)` - Queue processing
- `email_queue(to, created_at)` - User email history
- `email_queue(message_id)` - Webhook correlation
- `email_bounces(email, created_at)` - Bounce history

---

## Architecture

### Email Flow

```
                                    ┌─────────────────┐
                                    │  Email Worker   │
                                    │  (background)   │
                                    └────────┬────────┘
                                             │
┌──────────┐     ┌──────────┐     ┌─────────▼─────────┐     ┌──────────────┐
│  Service │────▶│  Queue   │────▶│  Provider Router  │────▶│ SMTP/SendGrid│
│  Layer   │     │  (Redis) │     │                   │     │    /SES      │
└──────────┘     └──────────┘     └───────────────────┘     └──────────────┘
     │                  │                    │                      │
     │                  │                    │                      │
     ▼                  ▼                    ▼                      ▼
  Auth/Org          Persist              Retry               External API
  Invoice            Email                on                      │
  News             to DB               Failure                    │
                                                  ┌──────────────▼──────────────┐
                                                  │   Webhook Handler           │
                                                  │   /webhooks/email/:provider │
                                                  └──────────────┬──────────────┘
                                                                 │
                                                                 ▼
                                                        Update EmailQueue
                                                        (delivered/bounced)
```

### Components

#### 1. EmailService Interface

```go
type EmailService interface {
    // Send queues an email for async delivery
    Send(ctx context.Context, req SendRequest) (*EmailQueue, error)
    
    // SendSync sends email immediately (blocks until sent)
    SendSync(ctx context.Context, req SendRequest) error
    
    // SendTemplate renders and queues template-based email
    SendTemplate(ctx context.Context, to, template string, data map[string]interface{}) (*EmailQueue, error)
    
    // GetStatus retrieves email delivery status
    GetStatus(ctx context.Context, id uuid.UUID) (*EmailQueue, error)
    
    // GetBounceHistory retrieves bounce history for an email
    GetBounceHistory(ctx context.Context, email string, limit int) ([]EmailBounce, error)
}

type SendRequest struct {
    To          string                 `json:"to"`
    Subject     string                 `json:"subject"`
    HTMLBody    string                 `json:"html_body,omitempty"`
    TextBody    string                 `json:"text_body,omitempty"`
    Template    string                 `json:"template,omitempty"`
    TemplateData map[string]interface{} `json:"template_data,omitempty"`
    Priority    Priority               `json:"priority"` // high, normal, low
}
```

#### 2. Provider Interface

```go
type EmailProvider interface {
    // Name returns provider identifier
    Name() string
    
    // Send delivers email and returns message ID
    Send(ctx context.Context, email *Email) (messageID string, err error)
    
    // ProcessWebhook handles provider webhook events
    ProcessWebhook(ctx context.Context, payload []byte) ([]WebhookEvent, error)
    
    // IsConfigured returns true if provider is properly configured
    IsConfigured() bool
}

type Email struct {
    To          string
    Subject     string
    HTMLBody    string
    TextBody    string
    From        string
    FromName    string
    ReplyTo     string
    Headers     map[string]string
    Attachments []Attachment
}
```

#### 3. Queue Worker

```go
type EmailWorker struct {
    queue       EmailQueueRepository
    providers   map[string]EmailProvider
    redis       *redis.Client
    concurrency int
    stopChan    chan struct{}
}

func (w *EmailWorker) Start()
func (w *EmailWorker) Stop()
func (w *EmailWorker) processEmail(ctx context.Context, job *EmailQueue) error
func (w *EmailWorker) retryWithBackoff(job *EmailQueue) time.Duration
```

---

## Integration Points

### 1. AuthService Integration

```go
// After registration
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*User, error) {
    // ... create user ...
    
    // Queue welcome email
    _, err = s.email.SendTemplate(ctx, user.Email, "welcome", map[string]interface{}{
        "UserName": user.Name,
        "AppName": s.config.AppName,
        "LoginURL": s.config.BaseURL + "/login",
    })
    if err != nil {
        s.logger.Warn("failed to queue welcome email", "error", err)
        // Don't fail registration if email fails
    }
    
    return user, nil
}

// Password reset
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
    // ... generate reset token ...
    
    return s.email.SendTemplate(ctx, email, "password-reset", map[string]interface{}{
        "UserName":    user.Name,
        "ResetURL":    resetURL,
        "ExpiresIn":   "15 minutes",
    })
}
```

### 2. OrganizationService Integration

```go
// Member invitation
func (s *OrganizationService) AddMember(ctx context.Context, req AddMemberRequest) error {
    // ... add member to org ...
    
    return s.email.SendTemplate(ctx, req.Email, "org-invitation", map[string]interface{}{
        "OrgName":      org.Name,
        "InviterName":  inviter.Name,
        "Role":         req.Role,
        "AcceptURL":    acceptURL,
        "ExpiresIn":    "7 days",
    })
}
```

### 3. Event Hooks (Future)

```go
// Event-driven email triggers
type EmailEventHandler struct {
    email   EmailService
    bus     EventBus
}

func (h *EmailEventHandler) OnInvoiceCreated(event InvoiceCreatedEvent) {
    h.email.SendTemplate(context.Background(), event.CustomerEmail, "invoice-created", map[string]interface{}{
        "InvoiceNumber": event.InvoiceNumber,
        "Amount":        event.Amount,
        "DueDate":       event.DueDate,
        "ViewURL":       event.ViewURL,
    })
}
```

---

## Configuration

### Environment Variables

```bash
# Email Provider Selection
EMAIL_DRIVER=smtp              # smtp, sendgrid, ses, mailgun

# SMTP Configuration (when EMAIL_DRIVER=smtp)
EMAIL_SMTP_HOST=smtp.example.com
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USER=your-smtp-user
EMAIL_SMTP_PASSWORD=your-smtp-password
EMAIL_SMTP_ENCRYPTION=starttls  # none, starttls, tls

# SendGrid Configuration (when EMAIL_DRIVER=sendgrid)
EMAIL_SENDGRID_API_KEY=your-sendgrid-api-key

# AWS SES Configuration (when EMAIL_DRIVER=ses)
EMAIL_SES_REGION=us-east-1
EMAIL_SES_ACCESS_KEY=your-aws-access-key
EMAIL_SES_SECRET_KEY=your-aws-secret-key

# Sending Configuration
EMAIL_FROM_ADDRESS=noreply@example.com
EMAIL_FROM_NAME="App Name"
EMAIL_REPLY_TO=support@example.com

# Queue Configuration
EMAIL_QUEUE_ENABLED=true
EMAIL_QUEUE_CONCURRENCY=10
EMAIL_RETRY_MAX=5
EMAIL_RETRY_BACKOFF=exponential  # exponential, linear

# Template Configuration
EMAIL_TEMPLATE_DIR=./templates/emails
```

### Config Struct

```go
type EmailConfig struct {
    Driver        string `mapstructure:"driver"`
    FromAddress   string `mapstructure:"from_address"`
    FromName      string `mapstructure:"from_name"`
    ReplyTo       string `mapstructure:"reply_to"`
    QueueEnabled  bool   `mapstructure:"queue_enabled"`
    QueueConcurrency int  `mapstructure:"queue_concurrency"`
    RetryMax     int    `mapstructure:"retry_max"`
    TemplateDir   string `mapstructure:"template_dir"`
    
    // Provider-specific
    SMTP  SMTPConfig  `mapstructure:"smtp"`
    SendGrid SendGridConfig `mapstructure:"sendgrid"`
    SES   SESConfig   `mapstructure:"ses"`
}
```

---

## Email Templates

### Directory Structure

```
templates/emails/
├── auth/
│   ├── welcome.html
│   ├── welcome.txt
│   ├── password-reset.html
│   ├── password-reset.txt
│   ├── email-verification.html
│   └── email-verification.txt
├── organization/
│   ├── invitation.html
│   ├── invitation.txt
│   ├── member-added.html
│   └── member-removed.html
├── notification/
│   ├── invoice-created.html
│   ├── news-published.html
│   ├── system-alert.html
│   └── system-maintenance.html
└── layouts/
    ├── base.html
    └── base.txt
```

### Template Example (Welcome)

```html
<!-- templates/emails/auth/welcome.html -->
{{ template "layouts/base" . }}

{{ define "content" }}
<h1>Welcome to {{ .AppName }}!</h1>

<p>Hi {{ .UserName }},</p>

<p>Thank you for joining {{ .AppName }}. Your account is ready!</p>

<p>Here's what you can do next:</p>
<ul>
    <li>Complete your profile</li>
    <li>Explore the dashboard</li>
    <li>Join or create an organization</li>
</ul>

<a href="{{ .LoginURL }}" class="button">Get Started</a>

<p>If you didn't create this account, you can safely ignore this email.</p>

<p>Best,<br>The {{ .AppName }} Team</p>
{{ end }}
```

### Template Rendering

```go
type TemplateEngine struct {
    templates map[string]*template.Template
    mu        sync.RWMutex
}

func (e *TemplateEngine) Render(name string, data map[string]interface{}) (html, text string, err error)
func (e *TemplateEngine) Reload() error // Hot reload templates
```

---

## API Endpoints

### Email Management (Admin Only)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/admin/emails/templates` | List email templates | admin |
| GET | `/api/v1/admin/emails/templates/:name` | Get template | admin |
| PUT | `/api/v1/admin/emails/templates/:name` | Update template | admin |
| GET | `/api/v1/admin/emails/queue` | List queued emails | admin |
| GET | `/api/v1/admin/emails/queue/:id` | Get email status | admin |
| POST | `/api/v1/admin/emails/queue/:id/retry` | Retry failed email | admin |
| GET | `/api/v1/admin/emails/bounces` | List bounces | admin |

### Webhook Endpoints (Provider-specific)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/webhooks/email/sendgrid` | SendGrid delivery events |
| POST | `/webhooks/email/ses` | AWS SES notifications |
| POST | `/webhooks/email/mailgun` | Mailgun webhook events |

---

## Permission Model

New permissions for email management:

```
email:template:read     - View email templates
email:template:write    - Create/edit email templates
email:queue:read        - View email queue
email:queue:manage      - Retry/cancel queued emails
email:bounce:read       - View bounce history
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Day 1)

- [ ] Create EmailConfig in config.go
- [ ] Create EmailQueue and EmailTemplate entities
- [ ] Create EmailQueueRepository interface
- [ ] Create database migration (000008_email_service.up.sql)
- [ ] Create EmailService interface

### Phase 2: SMTP Provider (Day 2 AM)

- [ ] Implement SMTPProvider
- [ ] Create EmailTemplateEngine
- [ ] Create base templates (welcome, password-reset, invitation)
- [ ] Test with real SMTP server

### Phase 3: Queue System (Day 2 PM)

- [ ] Implement RedisEmailQueue
- [ ] Create EmailWorker with concurrency
- [ ] Implement retry logic with exponential backoff
- [ ] Create dead letter queue handling

### Phase 4: Service Integration (Day 3 AM)

- [ ] Integrate with AuthService (register, password reset)
- [ ] Integrate with OrganizationService (invitations)
- [ ] Add EmailService dependency injection
- [ ] Write integration tests

### Phase 5: Additional Providers (Day 3 PM)

- [ ] Implement SendGridProvider
- [ ] Implement SESProvider
- [ ] Create provider router
- [ ] Add provider configuration validation

### Phase 6: Webhooks & Events (Day 4 AM)

- [ ] Create webhook handlers for providers
- [ ] Implement bounce/complaint tracking
- [ ] Create EmailBounce entity and repository
- [ ] Update email status on delivery

### Phase 7: Admin API (Day 4 PM)

- [ ] Create admin endpoints for template management
- [ ] Create admin endpoints for queue management
- [ ] Add bounce viewing endpoint
- [ ] Add Swagger documentation

### Phase 8: Testing & Documentation (Day 5)

- [ ] Unit tests for all providers
- [ ] Integration tests with testcontainers
- [ ] E2E tests for email flows
- [ ] Update AGENTS.md with email patterns
- [ ] Create email template documentation

---

## Constitution Check

| Principle | Compliance |
|-----------|------------|
| Production-Ready Foundation | ✅ Async queue, retry logic, graceful degradation |
| RBAC Mandatory | ✅ Email permissions in Casbin, scoped to admin |
| Soft Deletes | ✅ EmailTemplate uses DeletedAt, EmailQueue retained for audit |
| Stateless JWT | ✅ No changes to JWT system |
| PostgreSQL + Versioned Migrations | ✅ Migration 000008_email_service.up.sql |
| Multi-Instance Consistency | ✅ Redis queue shared across instances |
| Audit Logging | ✅ All email operations logged |

---

## Risks and Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Email delivery delays | High | Medium | Async queue with retry, fallback provider |
| SMTP credentials leaked | High | Low | Environment variables, never logged |
| Template injection | Medium | Low | Go template auto-escaping, sanitize inputs |
| Bounce rate too high | High | Medium | Bounce tracking, hard bounce suppression |
| Queue overflow | Medium | Low | Queue size limits, backpressure |
| Provider API changes | Medium | Low | Provider abstraction interface |
| Rate limited by provider | High | High | Queue with throttling, configurable concurrency |

---

## Dependencies

### Internal Dependencies

- `internal/config` - Email configuration loading
- `internal/service/auth` - Registration hooks
- `internal/service/organization` - Invitation hooks
- `internal/repository` - Data access patterns
- `internal/permission` - Admin permission checks
- `internal/audit` - Audit logging

### External Dependencies

- `github.com/go-gomail/gomail` - SMTP client (baseline)
- `github.com/sendgrid/sendgrid-go` - SendGrid API (optional)
- `github.com/aws/aws-sdk-go-v2/service/ses` - AWS SES (optional)
- `github.com/mailgun/mailgun-go` - Mailgun API (optional)
- `html/template` - Go standard library for templates

---

## Success Metrics

- [ ] All functional requirements implemented
- [ ] All integration tests pass
- [ ] Email delivery time < 30 seconds (p95)
- [ ] Email queue latency < 100ms
- [ ] Template rendering < 10ms
- [ ] 80%+ test coverage
- [ ] Retry logic handles transient failures
- [ ] Bounce tracking operational
- [ ] All providers validated (SMTP baseline + 1 additional)

---

## Out of Scope

- Email campaign management
- A/B testing for emails
- Marketing automation
- Email analytics dashboard
- User preference center
- Email throttling per user
- Scheduled email campaigns
- Attachment support (Phase 2)

---

**Version History**:
| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-04-24 | Sisyphus | Initial specification |
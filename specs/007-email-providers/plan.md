# Implementation Plan: Additional Email Providers (SendGrid & SES)

**Branch**: `007-email-providers` | **Date**: 2026-05-06 | **Spec**: Existing `004-email-service`
**Input**: FEATURE_STATUS.md indicates only SMTP provider implemented

## Summary

Implement SendGrid and AWS SES email providers to complement the existing SMTP provider. Both providers are configured via environment variables and implement the `EmailProvider` interface defined in `internal/service/email_provider.go`.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Existing `EmailProvider` interface, Echo v4, GORM, Redis
**Storage**: PostgreSQL (existing email_queue table), Redis (existing email queue)
**Testing**: testify (require/assert), testcontainers-go for integration
**Target Platform**: Linux server (Docker)

## What Is Already Implemented

### ✅ Email Service Architecture

| Component | Location | Notes |
|-----------|----------|-------|
| `EmailProvider` interface | `internal/service/email_provider.go:10-16` | Defines `Send()` and `Name()` |
| `EmailMessage` struct | `internal/service/email_provider.go:18-26` | Shared message format |
| `EmailBounceInfo` struct | `internal/service/email_provider.go:28-35` | Bounce webhook data |
| SMTP Provider | `internal/service/email_smtp_provider.go` | Working implementation |
| `EmailConfig` | `internal/config/email.go` | Supports `provider` field |

### ✅ Provider Selection Pattern

The email service supports multiple providers via the `EmailProvider` interface. The provider is selected based on `config.EmailConfig.Provider`:
- `"smtp"` → `NewSMTPProvider()`
- `"sendgrid"` → `NewSendGridProvider()` (to implement)
- `"ses"` → `NewSESProvider()` (to implement)

## What Needs To Be Built

### 1. SendGrid Provider

**Files to create:**
- `internal/service/email_sendgrid_provider.go`

**Configuration env vars:**
```
EMAIL_PROVIDER=sendgrid
SENDGRID_API_KEY=your-api-key
SENDGRID_FROM_ADDRESS=noreply@example.com
SENDGRID_FROM_NAME=Base Go API
```

**Implementation requirements:**
- Implement `EmailProvider` interface
- Use SendGrid v3 API: `POST https://api.sendgrid.com/v3/mail/send`
- Personalization API for template variables
- Return provider message ID for webhook correlation
- Handle rate limit errors with exponential backoff

### 2. AWS SES Provider

**Files to create:**
- `internal/service/email_ses_provider.go`

**Configuration env vars:**
```
EMAIL_PROVIDER=ses
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
SES_FROM_ADDRESS=noreply@example.com
SES_FROM_NAME=Base Go API
```

**Implementation requirements:**
- Implement `EmailProvider` interface
- Use AWS SDK v2 for Go
- Amazon SES API via `ses.SendEmail`
- Handle AWS credentials via shared config (IAM role, env vars, config file)
- Return message ID for tracking

### 3. Provider Factory / Selection Logic

**Files to modify:**
- `internal/http/server.go:582` - Add provider selection based on config

**Pattern:**
```go
var provider service.EmailProvider
switch s.config.Email.Provider {
case "sendgrid":
    provider = service.NewSendGridProvider(&s.config.Email)
case "ses":
    provider = service.NewSESProvider(&s.config.Email)
default:
    provider = service.NewSMTPProvider(&s.config.Email)
}
```

## Project Structure

```
specs/007-email-providers/
├── plan.md              # This file
├── research.md          # Research on SendGrid/SES APIs
├── contracts/           # API contracts
│   └── config.md        # Environment variable contracts
└── tasks.md             # Implementation tasks (from /speckit.tasks)
```

## Source Code Structure

```text
internal/service/
├── email_provider.go          # Existing interface (DO NOT MODIFY)
├── email_smtp_provider.go     # Existing (DO NOT MODIFY)
├── email_sendgrid_provider.go # NEW - SendGrid implementation
└── email_ses_provider.go      # NEW - SES implementation

internal/config/
└── email.go                   # ADD: SendGrid and SES config fields

internal/http/server.go        # MODIFY: Provider factory logic
```

## Config Changes

**Add to `internal/config/email.go`:**

```go
type EmailConfig struct {
    // ... existing fields ...

    // SendGrid
    SendGridAPIKey     string `mapstructure:"sendgrid_api_key"`
    SendGridFromName   string `mapstructure:"sendgrid_from_name"`
    SendGridFromEmail  string `mapstructure:"sendgrid_from_email"`

    // AWS SES
    AWSRegion         string `mapstructure:"aws_region"`
    AWSAccessKeyID     string `mapstructure:"aws_access_key_id"`
    AWSSecretAccessKey string `mapstructure:"aws_secret_access_key"`
    SESFromName        string `mapstructure:"ses_from_name"`
    SESFromEmail       string `mapstructure:"ses_from_email"`
}
```

## Implementation Phases

### Phase 1: SendGrid Provider

1. Create `email_sendgrid_provider.go`
2. Implement `EmailProvider` interface
3. Add HTTP client with SendGrid API integration
4. Map `EmailMessage` to SendGrid v3 mail format
5. Handle API errors and return message ID
6. Add unit tests

### Phase 2: SES Provider

1. Create `email_ses_provider.go`
2. Implement `EmailProvider` interface
3. Configure AWS SDK client
4. Map `EmailMessage` to SES SendEmail format
5. Handle AWS errors and return message ID
6. Add unit tests

### Phase 3: Provider Selection

1. Update `internal/http/server.go:582`
2. Add switch/case for provider selection
3. Pass correct provider to `NewEmailService()`
4. Add integration tests (if Docker available)

## Testing Strategy

### Unit Tests

- `tests/unit/email_sendgrid_provider_test.go`
- `tests/unit/email_ses_provider_test.go`

**Mock pattern:**
```go
// Mock HTTP client for SendGrid
type MockSendGridClient struct {
    SendFunc func(*SendGridRequest) (*SendGridResponse, error)
}

// Mock AWS SDK for SES
type MockSESClient struct {
    SendEmailFunc func(*ses.SendEmailInput) (*ses.SendEmailOutput, error)
}
```

### Integration Tests

- `tests/integration/email_provider_test.go` - Test actual provider calls (skip if no API keys)

## Constraints & Guidelines

1. **DO NOT modify** `EmailProvider` interface - maintain compatibility
2. **DO NOT modify** existing SMTP provider - must keep working
3. **Follow existing patterns** - match `email_smtp_provider.go` structure
4. **Error handling** - Return meaningful errors for debugging
5. **Message ID** - Must return provider's message ID for webhook correlation
6. **Graceful degradation** - If provider fails, email should be marked as failed in queue

## Acceptance Criteria

- [ ] SendGrid provider implements `EmailProvider` interface
- [ ] SES provider implements `EmailProvider` interface
- [ ] Provider selection works via `config.EmailConfig.Provider`
- [ ] Unit tests cover happy path and error cases
- [ ] Existing SMTP provider still works
- [ ] All providers return message IDs for tracking
- [ ] Code follows existing project patterns

## Files Reference

| Action | File | Notes |
|--------|------|-------|
| CREATE | `internal/service/email_sendgrid_provider.go` | SendGrid implementation |
| CREATE | `internal/service/email_ses_provider.go` | SES implementation |
| MODIFY | `internal/config/email.go` | Add SendGrid/SES config fields |
| MODIFY | `internal/http/server.go:582` | Provider factory logic |
| CREATE | `tests/unit/email_sendgrid_provider_test.go` | Unit tests |
| CREATE | `tests/unit/email_ses_provider_test.go` | Unit tests |
| CREATE | `specs/007-email-providers/tasks.md` | Task breakdown |
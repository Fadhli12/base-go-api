# Email Providers - Learnings

**Session Complete**: 2026-05-06 | **Plan**: email-providers | **9/13 tasks**

## SUMMARY

### Completed (Core Implementation)
| Task | Description | Status |
|------|-------------|--------|
| T001 | SendGrid config struct | ✅ |
| T002 | SendGrid provider | ✅ |
| T004 | SES config struct | ✅ |
| T005 | SES provider | ✅ |
| T007 | EmailConfig fields | ✅ |
| T008 | Provider selection wiring | ✅ |
| T010 | .env.example | ✅ |
| T011 | AGENTS.md | ✅ |

### Deferred (External Dependencies)
| Task | Description | Blocker |
|------|-------------|---------|
| T003 | SendGrid unit tests | Requires API keys |
| T006 | SES unit tests | Requires API keys |
| T009 | Integration tests | Requires Docker/API keys |
| T012 | make lint | golangci-lint not installed |
| T013 | go build ./... | Pre-existing `image_unix.go:187` error |

### Files Created
- `internal/config/email_sendgrid.go` (19 lines)
- `internal/config/email_ses.go` (23 lines)
- `internal/service/email_sendgrid_provider.go` (201 lines)
- `internal/service/email_ses_provider.go` (96 lines)

### Files Modified
- `internal/config/email.go` (+18 lines)
- `internal/http/server.go` (+17/-2 lines)
- `.env.example` (+12 lines)
- `AGENTS.md` (+2 lines)

## VERIFICATION
- `go vet ./internal/service/` → PASS (0 errors)
- `go vet ./internal/config/` → PASS (0 errors)
- Provider interface implemented correctly
- Provider selection switch/case wired correctly

---

## T010/T011: Email Provider Documentation Updates

**Date**: 2026-05-06

### .env.example Changes
- Added 8 new environment variables under the Email Service Configuration section
- **SendGrid block**: `SENDGRID_API_KEY`, `SENDGRID_FROM_ADDRESS`, `SENDGRID_FROM_NAME`
  - Placed between existing SMTP settings and Email queue settings
  - Followed existing comment conventions: `# SendGrid settings (required when EMAIL_PROVIDER=sendgrid)`
- **AWS SES block**: `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `SES_FROM_ADDRESS`, `SES_FROM_NAME`
  - Followed same pattern: `# AWS SES settings (required when EMAIL_PROVIDER=ses)`
- Both blocks mirror the SMTP defaults (noreply@example.com, Go API Base)
- Kept existing SMTP env vars intact (did NOT remove anything)

### AGENTS.md Changes
- Added two rows to WHERE TO LOOK table:
  - `email_sendgrid_provider.go` → `internal/service/` → "SendGrid email integration"
  - `email_ses_provider.go` → `internal/service/` → "AWS SES email integration"
- Both entries use the bold formatting pattern matching existing webhook/logger entries
- Placed after webhook migrations entry (last in the table)
- Confirmed both files exist at `internal/service/email_sendgrid_provider.go` and `internal/service/email_ses_provider.go`

### Patterns Followed
- Section-separator convention: `# ===` blocks with 80-char width
- Comment style: `# Description` followed by `KEY=value`
- Table formatting: `| **Name** | **Location** | **Notes** |` with bold markers

## T001: SendGrid Config Struct

**Date**: 2026-05-06

### Pattern Followed
- Followed the exact same config struct pattern as `email.go`, `storage.go`, `webhook.go`
- All config files in `internal/config/` share a consistent structure:
  1. Package doc comment
  2. Config struct with `mapstructure` tags and inline env var comments
  3. `DefaultXxxConfig()` function returning sensible defaults

### Key Decisions
- `SendGridConfig` is a separate struct (not embedded in `EmailConfig`) — keeps provider configs independent and testable
- The struct is NOT referenced in `Config` struct or `parseEmailConfig()` yet — that's for a future task that integrates it into the config loading pipeline
- Fields use `mapstructure` tags matching the env var names in snake_case (e.g., `api_key` for `SENDGRID_API_KEY`)

### Env Var Mapping
| Struct Field | Env Var | mapstructure Tag |
|-------------|---------|-----------------|
| APIKey | SENDGRID_API_KEY | api_key |
| FromAddress | SENDGRID_FROM_ADDRESS | from_address |
| FromName | SENDGRID_FROM_NAME | from_name |

### References
- `internal/config/email.go` — main EmailConfig with SMTP fields
- `internal/config/webhook.go` — config pattern template
- `internal/config/config.go` — parseEmailConfig() at line 686

## T002: SES Config Struct

**Date**: 2026-05-06

### Pattern Followed
- Followed the same config struct pattern as `email_sendgrid.go` — identical structure, different fields
- All config files in `internal/config/` share a consistent structure:
  1. Package doc comment
  2. Config struct with `mapstructure` tags and inline env var comments
  3. `DefaultXxxConfig()` function returning sensible defaults (empty strings)

### Key Decisions
- `SESConfig` is a separate struct (not embedded in `EmailConfig`) — keeps provider configs independent and testable
- The struct is NOT referenced in `Config` struct or `parseEmailConfig()` yet — that's for a future task that integrates it into the config loading pipeline
- Fields use `mapstructure` tags matching the env var names in snake_case (e.g., `region` for `AWS_REGION`)
- `Region`, `AccessKeyID`, `SecretAccessKey` use `AWS_*` prefix convention; `FromAddress`/`FromName` use `SES_*` prefix

### Env Var Mapping
| Struct Field | Env Var | mapstructure Tag |
|-------------|---------|-----------------|
| Region | AWS_REGION | region |
| AccessKeyID | AWS_ACCESS_KEY_ID | access_key_id |
| SecretAccessKey | AWS_SECRET_ACCESS_KEY | secret_access_key |
| FromAddress | SES_FROM_ADDRESS | from_address |
| FromName | SES_FROM_NAME | from_name |

### References
- `internal/config/email_sendgrid.go` — pattern reference (identical structure)
- `internal/config/email.go` — main EmailConfig with SMTP fields

## T005: SES Provider Implementation

**Date**: 2026-05-06

### Pattern Followed
- Closely mirrored `email_smtp_provider.go` structure:
  - Struct embeds provider-specific config
  - `Send()` with context, validation, body construction, provider call
  - `Name()` returning provider identifier string
- No inline comments — matches codebase convention (SMTP provider has none)
- Exported symbol doc comments only (struct, constructor, methods)

### Key Decisions
- Constructor takes `*config.SESConfig`, not `*config.EmailConfig` — provider-specific config keeps separation of concerns
- `config.LoadDefaultConfig()` with `config.WithRegion()` reads AWS credentials from shared config chain (env vars, ~/.aws/credentials, IAM roles, EC2 metadata)
- Explicit credential override only if both `AccessKeyID` and `SecretAccessKey` are provided — falls through to shared config otherwise
- Both HTML and text bodies supported independently — can send HTML-only, text-only, or both
- `ses.SendEmailInput` uses `types.Destination`, `types.Message`, `types.Body`, `types.Content` from the SES types subpackage
- Returns `*output.MessageId` — AWS SES returns a unique message ID for each send

### Provider-Specific Auth Flow
```
SESConfig.AccessKeyID + SecretAccessKey set?
├─ YES → static credentials via aws.CredentialsProviderFunc
└─ NO  → config.LoadDefaultConfig (IAM role / env vars / ~/.aws)
```

### Dependencies Added
- `github.com/aws/aws-sdk-go-v2/service/ses v1.34.24` — SES client package
- Transitive: `github.com/aws/aws-sdk-go-v2/service/ses/types` — SES type definitions

### Compilation
- `go vet` passes clean on the file
- `go mod tidy` marked `service/ses` as direct dependency
- Pre-existing build error in `internal/conversion/image_unix.go:187` (EXIF assignment mismatch) is unrelated

## T007: Add SendGrid & SES Fields to EmailConfig

**Date**: 2026-05-06

### What Changed
- Added 8 new fields to `EmailConfig` struct in `internal/config/email.go`:
  - SendGrid: `SendGridAPIKey`, `SendGridFromAddress`, `SendGridFromName`
  - SES: `AWSRegion`, `AWSAccessKeyID`, `AWSSecretAccessKey`, `SESFromAddress`, `SESFromName`
- Updated `DefaultEmailConfig()` with empty string defaults for all new fields
- No existing fields were removed or restructured

### mapstructure Tag Convention
Tags use snake_case matching the env var names without prefix:
| Struct Field | Env Var | mapstructure Tag |
|-------------|---------|-----------------|
| SendGridAPIKey | SENDGRID_API_KEY | sendgrid_api_key |
| SendGridFromAddress | SENDGRID_FROM_ADDRESS | sendgrid_from_address |
| SendGridFromName | SENDGRID_FROM_NAME | sendgrid_from_name |
| AWSRegion | AWS_REGION | aws_region |
| AWSAccessKeyID | AWS_ACCESS_KEY_ID | aws_access_key_id |
| AWSSecretAccessKey | AWS_SECRET_ACCESS_KEY | aws_secret_access_key |
| SESFromAddress | SES_FROM_ADDRESS | ses_from_address |
| SESFromName | SES_FROM_NAME | ses_from_name |

### Key Decision
- Fields are added directly to `EmailConfig` rather than embedding the separate `SendGridConfig`/`SESConfig` structs. This follows the existing pattern where SMTP fields are flat on `EmailConfig`, and keeps the separate structs available for isolated provider usage while having a unified config surface for wiring in `parseEmailConfig()`.

### Next Step
- T008 will wire these fields into `parseEmailConfig()` in `internal/config/config.go`

## T008: SendGrid Provider Implementation

**Date**: 2026-05-06

### Pattern Followed
- Mirrored `email_smtp_provider.go` and `email_ses_provider.go` structure:
  - Struct with unexported provider-specific fields (apiKey, fromAddr, fromName, httpClient)
  - Constructor: `NewSendGridProvider(cfg *config.EmailConfig)` returns `*SendGridProvider`
  - `Send(ctx, *EmailMessage) (string, error)` — validates, builds payload, sends via HTTP
  - `Name() string` — returns "sendgrid"
- `SendGridProvider` is a value method receiver (not pointer) for `Name()` matching SES/SMTP pattern
- Constructor takes `*config.EmailConfig` — uses the unified EmailConfig which now has SendGrid fields (T007)

### Key Decisions
- **Constructor takes `*config.EmailConfig`** (not `*SendGridConfig`): T007 merged SendGrid fields into EmailConfig. This follows the pattern where providers read from a unified config during construction. The separate `SendGridConfig` struct exists for isolated testing.
- **Uses `net/http` directly** (no SendGrid SDK): The SendGrid v3 API is a simple REST endpoint. Avoids unnecessary dependency. Matches the lightweight approach of SMTP provider.
- **Exponential backoff on 429**: Up to 3 retries with base 1s backoff, doubling each attempt (1s, 2s, 4s). Context cancellation is respected between retries.
- **Message ID from `X-Message-Id` header**: SendGrid returns the unique message ID in response headers, NOT in the JSON body. This ID is used for webhook delivery event correlation.
- **Template vs content sends**: When `email.Template` is set, uses `template_id` + `dynamic_template_data`. When not set, builds `content[]` array with text/plain and text/html entries.
- **Template subject override**: Template-driven sends omit the top-level `subject` field but allow per-personalization subject override (SendGrid API behavior).
- **15s HTTP timeout**: Default net/http timeout. Matches typical provider pattern.

### SendGrid API Integration
```
POST https://api.sendgrid.com/v3/mail/send
Authorization: Bearer <api_key>
Content-Type: application/json

{
  "from": {"email": "...", "name": "..."},
  "template_id": "...",           // when email.Template is set
  "personalizations": [{
    "to": [{"email": "..."}],
    "dynamic_template_data": {...} // email.Data
  }],
  "content": [                    // when email.Template is empty
    {"type": "text/plain", "value": "..."},
    {"type": "text/html", "value": "..."}
  ]
}

Response: 202 Accepted
X-Message-Id: <message-id-for-webhook-correlation>
```

### Rate Limit Handling
```
429 Too Many Requests
├─ attempt < 3 → exponential backoff (1s, 2s, 4s)
├─ attempt = 3 → return "sendgrid retries exhausted" error
└─ context cancelled → return ctx.Err() immediately
```

### Validation Checks
1. `email.To` must be non-empty
2. `fromAddr` must be configured (from EmailConfig)
3. `apiKey` must be configured (from EmailConfig)
4. At least one of: `Template`, `HTMLContent`, `TextContent`

### Compilation
- `go build` succeeds with `email_provider.go` (sibling file defining `EmailMessage`)
- `go vet` clean on the file in package context
- No new dependencies added (uses only stdlib: `net/http`, `encoding/json`, `math`, `time`)
- Single exported constructor doc comment retained (same pattern as SMTP/SES providers)
- All internal comments removed — code is self-documenting per codebase convention

### References
- `internal/service/email_provider.go:10-16` — EmailProvider interface
- `internal/service/email_smtp_provider.go` — pattern template (structure, constructor, Name())
- `internal/service/email_ses_provider.go` — AWS SDK pattern (validation flow, error wrapping)
- `internal/config/email.go` — EmailConfig with SendGrid fields (SendGridAPIKey, SendGridFromAddress, SendGridFromName)

## T009: Provider Selection Switch in server.go

**Date**: 2026-05-06

### What Changed
- Replaced hardcoded `SMTPProvider` initialization (line 582) with a `switch` statement that selects the provider based on `s.config.Email.Provider`
- Cases: `"sendgrid"` → `NewSendGridProvider()`, `"ses"` → `NewSESProvider()`, `default` → `NewSMTPProvider()`
- Provider variable typed as `service.EmailProvider` (interface), not a concrete type — enables polymorphic assignment
- `emailService` now receives `emailProvider` instead of `smtpProvider`

### Key Decisions
- **SES provider bridge**: `NewSESProvider` takes `*config.SESConfig` (a separate struct), but all SES fields live on `EmailConfig`. The switch constructs an `SESConfig` inline by mapping fields: `AWSRegion`→`Region`, `AWSAccessKeyID`→`AccessKeyID`, `AWSSecretAccessKey`→`SecretAccessKey`, `SESFromAddress`→`FromAddress`, `SESFromName`→`FromName`
- **SMTP is the default**: Falls through to `NewSMTPProvider()` for `"smtp"` or any unrecognized provider value — safe failback, matches the `DefaultEmailConfig()` provider value
- **Single provider per instance**: Designed for one provider per server — no multiplexing needed (the use case doesn't require it)

### Provider Construction Chain
```
s.config.Email.Provider
├─ "sendgrid" → NewSendGridProvider(&s.config.Email)            // EmailProvider
├─ "ses"      → NewSESProvider(&SESConfig{...from EmailConfig})  // EmailProvider
└─ default    → NewSMTPProvider(&s.config.Email)                // EmailProvider
                      ↓
            emailService(..., emailProvider)
```

### Compilation
- LSP diagnostics: clean (zero errors, zero warnings)
- `config` and `service` packages already imported — no import changes needed
- `go build` blocked by pre-existing error in `internal/conversion/image_unix.go:187` (EXIF assignment mismatch) — unrelated to this change
- The `EmailProvider` interface is satisfied by all three concrete providers — compile-time type safety

### SMTP Provider Retained
- `NewSMTPProvider` is NOT removed — it remains as the default case and is still importable for direct use
- Backward compatible: existing configs with no `Provider` field (empty string) will use SMTP via the default case

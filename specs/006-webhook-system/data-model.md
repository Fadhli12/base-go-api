# Data Model: Webhook System

**Feature**: 006-webhook-system | **Date**: 2026-05-02 | **Status**: Phase 1 Design

---

## Entity Relationship Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│ Webhook System                                                    │
├──────────────────────────────────────────────────────────────────┤
│                                                                    │
│  organizations ──1:N──► webhooks ──1:N──► webhook_deliveries     │
│       │                    │                          │            │
│       │                    │                          │            │
│       │               (soft delete)              (immutable)       │
│       │                    │                          │            │
│       │               events: JSONB              status: enum     │
│       │               secret: VARCHAR           payload: JSONB    │
│       │               url: VARCHAR            response_code: INT  │
│       │               active: BOOL           next_retry_at: TS    │
│       │                                       duration_ms: INT     │
│                                                                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## Entities

### 1. webhooks

**Purpose**: Outbound webhook subscription configuration. Users create webhooks to receive HTTP callbacks when specified events occur.

```sql
CREATE TABLE webhooks (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(255) NOT NULL,
  url             VARCHAR(500) NOT NULL,
  secret          VARCHAR(255) NOT NULL,
  events          JSONB NOT NULL DEFAULT '[]',
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  rate_limit      INTEGER NOT NULL DEFAULT 100,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at      TIMESTAMPTZ DEFAULT NULL,

  CONSTRAINT uq_webhooks_url_org UNIQUE (url, organization_id) WHERE deleted_at IS NULL,
  CONSTRAINT chk_webhooks_events_not_empty CHECK (jsonb_array_length(events) > 0)
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK, `gen_random_uuid()` | Unique webhook identifier |
| `organization_id` | UUID | FK organizations, nullable | NULL = global webhook; org-scoped otherwise |
| `name` | VARCHAR(255) | NOT NULL | Human-readable display name |
| `url` | VARCHAR(500) | NOT NULL | Target endpoint URL (must be HTTPS in production) |
| `secret` | VARCHAR(255) | NOT NULL | HMAC-SHA256 signing key (32 bytes hex-encoded, stored with `whsec_` prefix — strip prefix before HMAC computation) |
| `events` | JSONB | NOT NULL, `[]` | Array of subscribed event types: `["user.created","invoice.paid"]` |
| `active` | BOOLEAN | NOT NULL, `TRUE` | Toggle without deletion |
| `rate_limit` | INTEGER | NOT NULL, `100` | Max deliveries per minute |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last modification |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete (constitution principle III) |

**Indexes**:
- `UNIQUE INDEX uq_webhooks_url_org ON webhooks(url, organization_id) WHERE deleted_at IS NULL` — unique URL per org
- `INDEX idx_webhooks_organization_id ON webhooks(organization_id) WHERE deleted_at IS NULL` — list org webhooks
- `INDEX idx_webhooks_active ON webhooks(active) WHERE deleted_at IS NULL` — find active webhooks
- `GIN INDEX idx_webhooks_events ON webhooks USING gin(events)` — event type filtering

**Go Domain Entity**:
```go
type Webhook struct {
    ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    OrganizationID *uuid.UUID     `gorm:"type:uuid;index" json:"organization_id,omitempty"`
    Organization   *Organization  `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
    Name           string         `gorm:"size:255;not null" json:"name"`
    URL            string         `gorm:"size:500;not null" json:"url"`
    Secret         string         `gorm:"size:255;not null" json:"-"` // Hidden from API
    Events         datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"events"` // Requires gorm.io/datatypes
    Active         bool           `gorm:"default:true" json:"active"`
    RateLimit      int            `gorm:"default:100" json:"rate_limit"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
    DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Webhook) TableName() string { return "webhooks" }
```

**Response DTO** (secret excluded):
```go
type WebhookResponse struct {
    ID             string   `json:"id"`
    OrganizationID *string  `json:"organization_id,omitempty"`
    Name           string   `json:"name"`
    URL            string   `json:"url"`
    Events         []string `json:"events"`
    Active         bool     `json:"active"`
    RateLimit      int      `json:"rate_limit"`
    CreatedAt      string   `json:"created_at"`
    UpdatedAt      string   `json:"updated_at"`
}

func (w *Webhook) ToResponse() WebhookResponse { /* maps fields, formats timestamps, parses events JSON */ }
```

**Create Response DTO** (includes secret, returned ONLY on POST /webhooks):
```go
type WebhookCreateResponse struct {
    ID             string   `json:"id"`
    OrganizationID *string  `json:"organization_id,omitempty"`
    Name           string   `json:"name"`
    URL            string   `json:"url"`
    Secret         string   `json:"secret"`
    Events         []string `json:"events"`
    Active         bool     `json:"active"`
    RateLimit      int      `json:"rate_limit"`
    CreatedAt      string   `json:"created_at"`
    UpdatedAt      string   `json:"updated_at"`
}

func (w *Webhook) ToCreateResponse() WebhookCreateResponse {
    resp := WebhookCreateResponse{
        ID:             w.ID.String(),
        Name:           w.Name,
        URL:            w.URL,
        Secret:         w.Secret, // Included only on creation
        Active:         w.Active,
        RateLimit:      w.RateLimit,
        CreatedAt:      w.CreatedAt.Format(time.RFC3339),
        UpdatedAt:      w.UpdatedAt.Format(time.RFC3339),
    }
    // ... map OrganizationID and Events
    return resp
}
```

**Business Methods**:
```go
func (w *Webhook) IsSubscribedTo(event string) bool  // Check if event in Events JSONB
func (w *Webhook) IsActive() bool                    // active AND not soft-deleted
func (w *Webhook) IsGlobal() bool                    // OrganizationID == nil
```

**Dispatch Flow**:
When the EventBus receives an event, the webhook service dispatches deliveries as follows:
1. Query all webhooks where `IsActive() == true` AND `IsSubscribedTo(event) == true` AND matching the organization scope (or global).
2. For each matching webhook, check per-webhook rate limit (Redis sliding window). If exceeded, create a `rate_limited` delivery instead.
3. Create a `WebhookDelivery` record with `status=queued` and push to the Redis delivery queue.
4. Worker picks up deliveries and performs HTTP delivery.

**Key rule**: Inactive webhooks (`active=false`) and soft-deleted webhooks do NOT receive deliveries. The `active` toggle exists specifically to pause deliveries without deleting the configuration.

---

### 2. webhook_deliveries

**Purpose**: Immutable audit trail of all webhook delivery attempts. No soft delete — records are retained for compliance and debugging.

```sql
CREATE TABLE webhook_deliveries (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  webhook_id            UUID NOT NULL REFERENCES webhooks(id) ON DELETE RESTRICT,
  event                 VARCHAR(100) NOT NULL,
  payload               JSONB,
  status                VARCHAR(20) NOT NULL DEFAULT 'queued'
                          CHECK (status IN ('queued','processing','delivered','failed','rate_limited')),
  response_code         INTEGER,
  response_body         TEXT,
  duration_ms           INTEGER,
  attempt_number        INTEGER NOT NULL DEFAULT 1,
  max_attempts          INTEGER NOT NULL DEFAULT 3,
  last_error            TEXT,
  next_retry_at         TIMESTAMPTZ,
  processing_started_at  TIMESTAMPTZ,
  delivered_at          TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT chk_webhook_deliveries_attempt CHECK (attempt_number >= 1 AND attempt_number <= max_attempts)
);
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK, `gen_random_uuid()` | Unique delivery identifier |
| `webhook_id` | UUID | FK webhooks, NOT NULL, RESTRICT | Reference to webhook config |
| `event` | VARCHAR(100) | NOT NULL | Event type that triggered delivery (e.g., `user.created`) |
| `payload` | JSONB | NULL | Full request body sent to endpoint |
| `status` | VARCHAR(20) | NOT NULL, `queued` | State: queued → processing → delivered/failed/rate_limited |
| `response_code` | INTEGER | NULL | HTTP status code received from endpoint |
| `response_body` | TEXT | NULL | Response body (truncated to 1KB in code before storage) |
| `duration_ms` | INTEGER | NULL | Delivery duration in milliseconds |
| `attempt_number` | INTEGER | NOT NULL, `1` | Current attempt (1-3) |
| `max_attempts` | INTEGER | NOT NULL, `3` | Maximum retry attempts |
| `last_error` | TEXT | NULL | Last error message |
| `next_retry_at` | TIMESTAMPTZ | NULL | When to retry (exponential backoff) |
| `processing_started_at` | TIMESTAMPTZ | NULL | When the worker started processing (stuck-recovery marker) |
| `delivered_at` | TIMESTAMPTZ | NULL | When successfully delivered |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation |

**Indexes**:
- `INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id)` — list deliveries for webhook
- `INDEX idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status IN ('queued','rate_limited')` — worker dequeue
- `INDEX idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status IN ('queued','rate_limited') AND next_retry_at IS NOT NULL` — backoff queue
- `INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(created_at)` — retention cleanup
- `INDEX idx_webhook_deliveries_stuck ON webhook_deliveries(processing_started_at) WHERE status = 'processing' AND processing_started_at IS NOT NULL` — stuck-delivery recovery

**Go Domain Entity**:
```go
type WebhookDeliveryStatus string

const (
    WebhookDeliveryStatusQueued     WebhookDeliveryStatus = "queued"
    WebhookDeliveryStatusProcessing WebhookDeliveryStatus = "processing"
    WebhookDeliveryStatusDelivered   WebhookDeliveryStatus = "delivered"
    WebhookDeliveryStatusFailed     WebhookDeliveryStatus = "failed"
    WebhookDeliveryStatusRateLimited WebhookDeliveryStatus = "rate_limited"
)

type WebhookDelivery struct {
    ID            uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    WebhookID     uuid.UUID             `gorm:"type:uuid;not null;index" json:"webhook_id"`
    Event         string                `gorm:"size:100;not null" json:"event"`
    Payload       datatypes.JSON `gorm:"type:jsonb" json:"payload,omitempty"` // Requires gorm.io/datatypes
    Status        WebhookDeliveryStatus  `gorm:"size:20;not null;default:'queued';index" json:"status"`
    ResponseCode  *int                   `json:"response_code,omitempty"`
    ResponseBody  string                `gorm:"type:text" json:"response_body,omitempty"`
    DurationMs    *int64                `json:"duration_ms,omitempty"`
    AttemptNumber int                   `gorm:"default:1" json:"attempt_number"`
    MaxAttempts   int                   `gorm:"default:3" json:"max_attempts"`
    LastError     string                `gorm:"type:text" json:"last_error,omitempty"`
    NextRetryAt        *time.Time            `gorm:"index" json:"next_retry_at,omitempty"`
    ProcessingStartedAt *time.Time           `json:"processing_started_at,omitempty"`
    DeliveredAt        *time.Time            `json:"delivered_at,omitempty"`
    CreatedAt     time.Time             `json:"created_at"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

// CanRetry returns true if the worker can automatically retry this delivery.
// Automatic retry respects attempt_number < max_attempts.
func (d *WebhookDelivery) CanRetry() bool {
    return d.AttemptNumber < d.MaxAttempts && d.Status != WebhookDeliveryStatusDelivered
}

// CanReplay returns true if a user can manually replay this delivery.
// Replay is allowed for any delivery NOT currently being processed.
// This includes delivered, failed, and rate_limited statuses.
func (d *WebhookDelivery) CanReplay() bool {
    return d.Status != WebhookDeliveryStatusQueued && d.Status != WebhookDeliveryStatusProcessing
}

func (d *WebhookDelivery) IsTerminal() bool {
    return d.Status == WebhookDeliveryStatusDelivered ||
        (d.Status == WebhookDeliveryStatusFailed && d.AttemptNumber >= d.MaxAttempts)
}

// Replay resets a delivery for manual retry. Sets attempt_number=1,
// clears response fields, and marks as queued for immediate pickup.
func (d *WebhookDelivery) Replay() {
    d.Status = WebhookDeliveryStatusQueued
    d.AttemptNumber = 1
    d.ResponseCode = nil
    d.ResponseBody = ""
    d.DurationMs = nil
    d.LastError = ""
    d.NextRetryAt = nil
    d.ProcessingStartedAt = nil
    d.DeliveredAt = nil
}
```

---

## Status State Machine

```
queued ──► processing ──► delivered     (success)
   │            │
   │            └──► failed            (retry → back to queued with next_retry_at)
   │                    │
   └──► rate_limited    └──► failed    (max retries exceeded, permanent)
          │
          └──► queued   (retry after rate limit window)
```

**Transitions**:
| From | To | Condition |
|------|----|-----------|
| queued | processing | Worker picks up |
| processing | delivered | 2xx response |
| processing | failed | Non-2xx or timeout; attempt < max → queued (with next_retry_at) |
| processing | failed | attempt >= max → permanent fail |
| queued | rate_limited | Rate limit exceeded |
| rate_limited | queued | After rate window expires |

---

## Event Constants

```go
const (
    WebhookEventUserCreated    = "user.created"
    WebhookEventUserDeleted    = "user.deleted"
    WebhookEventInvoiceCreated = "invoice.created"
    WebhookEventInvoicePaid    = "invoice.paid"
    WebhookEventNewsPublished  = "news.published"
)

// All valid events for validation
var ValidWebhookEvents = map[string]bool{
    WebhookEventUserCreated:    true,
    WebhookEventUserDeleted:    true,
    WebhookEventInvoiceCreated: true,
    WebhookEventInvoicePaid:    true,
    WebhookEventNewsPublished:  true,
}
```

---

## Relationships Summary

```
organizations (1) ──< (M) webhooks (1) ──< (M) webhook_deliveries
users (1) ──< (M) webhooks  (via organization membership)
```

---

## Retention Policy

- `webhook_deliveries`: 90-day automated cleanup (configurable via `WEBHOOK_DELIVERY_RETENTION_DAYS`)
- `webhooks`: Retained indefinitely (soft delete only)
- Cleanup runs as periodic task in webhook worker

## Stuck-Delivery Recovery

Deliveries left in `processing` state after a crash or unclean shutdown are recovered by the worker:

- **Detection**: `processing_started_at` is set when the worker picks up a delivery. A periodic reaper goroutine (every 60s) queries for deliveries where `status = 'processing' AND processing_started_at < now() - WEBHOOK_DELIVERY_TIMEOUT * 2`.
- **Recovery**: Mark as `failed` with `last_error = 'processing timeout: worker recovery'` and schedule retry if `CanRetry() == true`.
- **Rationale**: Mirrors the email worker pattern but adds an explicit timestamp to avoid indefinite stuck states.

---

**Version**: 1.0 | **Created**: 2026-05-02 | **Status**: Phase 1 Design
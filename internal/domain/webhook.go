package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WebhookDeliveryStatus represents the current state of a webhook delivery.
// WebhookDelivery records are NEVER deleted - they serve as the audit trail.
type WebhookDeliveryStatus string

const (
	WebhookDeliveryStatusQueued      WebhookDeliveryStatus = "queued"
	WebhookDeliveryStatusProcessing  WebhookDeliveryStatus = "processing"
	WebhookDeliveryStatusDelivered   WebhookDeliveryStatus = "delivered"
	WebhookDeliveryStatusFailed      WebhookDeliveryStatus = "failed"
	WebhookDeliveryStatusRateLimited WebhookDeliveryStatus = "rate_limited"
)

// WebhookEvent constants for supported webhook events.
const (
	WebhookEventUserCreated    = "user.created"
	WebhookEventUserDeleted    = "user.deleted"
	WebhookEventInvoiceCreated = "invoice.created"
	WebhookEventInvoicePaid    = "invoice.paid"
	WebhookEventNewsPublished  = "news.published"
)

// ValidWebhookEvents is a lookup map for valid webhook event types.
var ValidWebhookEvents = map[string]bool{
	WebhookEventUserCreated:    true,
	WebhookEventUserDeleted:    true,
	WebhookEventInvoiceCreated: true,
	WebhookEventInvoicePaid:    true,
	WebhookEventNewsPublished:  true,
}

// Webhook represents an outgoing webhook endpoint.
type Webhook struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID *uuid.UUID     `gorm:"type:uuid;index" json:"organization_id,omitempty"`
	Organization   *Organization  `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Name           string         `gorm:"size:255;not null" json:"name"`
	URL            string         `gorm:"size:500;not null" json:"url"`
	Secret         string         `gorm:"size:255;not null" json:"-"`
	Events         datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"events"`
	Active         bool           `gorm:"default:true" json:"active"`
	RateLimit      int            `gorm:"default:100" json:"rate_limit"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for Webhook.
func (Webhook) TableName() string {
	return "webhooks"
}

// WebhookResponse represents a webhook response without sensitive fields.
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

// WebhookCreateResponse represents a webhook response on creation, including the secret.
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

// ToResponse converts Webhook to WebhookResponse.
func (w *Webhook) ToResponse() WebhookResponse {
	var events []string
	if len(w.Events) > 0 {
		_ = json.Unmarshal(w.Events, &events)
	}

	var orgID *string
	if w.OrganizationID != nil {
		s := w.OrganizationID.String()
		orgID = &s
	}

	return WebhookResponse{
		ID:             w.ID.String(),
		OrganizationID: orgID,
		Name:           w.Name,
		URL:            w.URL,
		Events:         events,
		Active:         w.Active,
		RateLimit:      w.RateLimit,
		CreatedAt:      w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      w.UpdatedAt.Format(time.RFC3339),
	}
}

// ToCreateResponse converts Webhook to WebhookCreateResponse.
func (w *Webhook) ToCreateResponse() WebhookCreateResponse {
	var events []string
	if len(w.Events) > 0 {
		_ = json.Unmarshal(w.Events, &events)
	}

	var orgID *string
	if w.OrganizationID != nil {
		s := w.OrganizationID.String()
		orgID = &s
	}

	return WebhookCreateResponse{
		ID:             w.ID.String(),
		OrganizationID: orgID,
		Name:           w.Name,
		URL:            w.URL,
		Secret:         w.Secret,
		Events:         events,
		Active:         w.Active,
		RateLimit:      w.RateLimit,
		CreatedAt:      w.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      w.UpdatedAt.Format(time.RFC3339),
	}
}

// IsSubscribedTo returns true if the webhook is subscribed to the given event.
func (w *Webhook) IsSubscribedTo(event string) bool {
	var events []string
	if len(w.Events) > 0 {
		if err := json.Unmarshal(w.Events, &events); err != nil {
			return false
		}
	}
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

// IsActive returns true if the webhook is active and not soft-deleted.
func (w *Webhook) IsActive() bool {
	return w.Active && !w.DeletedAt.Valid
}

// IsGlobal returns true if the webhook is not scoped to an organization.
func (w *Webhook) IsGlobal() bool {
	return w.OrganizationID == nil
}

// WebhookDelivery represents a single webhook delivery attempt.
// IMPORTANT: This table has NO soft delete - records are retained permanently
// for audit trail and compliance.
type WebhookDelivery struct {
	ID                    uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WebhookID             uuid.UUID             `gorm:"type:uuid;not null;index" json:"webhook_id"`
	Webhook               *Webhook              `gorm:"foreignKey:WebhookID" json:"-"`
	Event                 string                `gorm:"size:100;not null" json:"event"`
	Payload               datatypes.JSON        `gorm:"type:jsonb" json:"payload,omitempty"`
	Status                WebhookDeliveryStatus `gorm:"size:20;not null;default:'queued';index" json:"status"`
	ResponseCode          *int                  `json:"response_code,omitempty"`
	ResponseBody          string                `gorm:"type:text" json:"response_body,omitempty"`
	DurationMs            *int64                `json:"duration_ms,omitempty"`
	AttemptNumber         int                   `gorm:"default:1" json:"attempt_number"`
	MaxAttempts           int                   `gorm:"default:3" json:"max_attempts"`
	LastError             string                `gorm:"type:text" json:"last_error,omitempty"`
	NextRetryAt           *time.Time            `gorm:"index" json:"next_retry_at,omitempty"`
	ProcessingStartedAt   *time.Time            `json:"processing_started_at,omitempty"`
	DeliveredAt           *time.Time            `json:"delivered_at,omitempty"`
	CreatedAt             time.Time             `json:"created_at"`
}

// TableName returns the table name for WebhookDelivery.
func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}

// WebhookDeliveryResponse represents a webhook delivery record response.
type WebhookDeliveryResponse struct {
	ID                  string                `json:"id"`
	WebhookID           string                `json:"webhook_id"`
	Event               string                `json:"event"`
	Payload             json.RawMessage      `json:"payload,omitempty"`
	Status              WebhookDeliveryStatus `json:"status"`
	ResponseCode        *int                  `json:"response_code,omitempty"`
	ResponseBody        string                `json:"response_body,omitempty"`
	DurationMs          *int64                `json:"duration_ms,omitempty"`
	AttemptNumber       int                   `json:"attempt_number"`
	MaxAttempts         int                   `json:"max_attempts"`
	LastError           string                `json:"last_error,omitempty"`
	NextRetryAt         *string               `json:"next_retry_at,omitempty"`
	ProcessingStartedAt *string               `json:"processing_started_at,omitempty"`
	DeliveredAt         *string               `json:"delivered_at,omitempty"`
	CreatedAt           string                `json:"created_at"`
}

// ToResponse converts WebhookDelivery to WebhookDeliveryResponse.
func (d *WebhookDelivery) ToResponse() WebhookDeliveryResponse {
	var payload json.RawMessage
	if len(d.Payload) > 0 {
		payload = json.RawMessage(d.Payload)
	}

	var nextRetryAt, processingStartedAt, deliveredAt *string
	if d.NextRetryAt != nil {
		s := d.NextRetryAt.Format(time.RFC3339)
		nextRetryAt = &s
	}
	if d.ProcessingStartedAt != nil {
		s := d.ProcessingStartedAt.Format(time.RFC3339)
		processingStartedAt = &s
	}
	if d.DeliveredAt != nil {
		s := d.DeliveredAt.Format(time.RFC3339)
		deliveredAt = &s
	}

	return WebhookDeliveryResponse{
		ID:                  d.ID.String(),
		WebhookID:           d.WebhookID.String(),
		Event:               d.Event,
		Payload:             payload,
		Status:              d.Status,
		ResponseCode:        d.ResponseCode,
		ResponseBody:        d.ResponseBody,
		DurationMs:          d.DurationMs,
		AttemptNumber:       d.AttemptNumber,
		MaxAttempts:         d.MaxAttempts,
		LastError:           d.LastError,
		NextRetryAt:         nextRetryAt,
		ProcessingStartedAt: processingStartedAt,
		DeliveredAt:         deliveredAt,
		CreatedAt:           d.CreatedAt.Format(time.RFC3339),
	}
}

// CanRetry returns true if the delivery can be retried.
func (d *WebhookDelivery) CanRetry() bool {
	return d.AttemptNumber < d.MaxAttempts && d.Status != WebhookDeliveryStatusDelivered
}

// CanReplay returns true if the delivery can be replayed.
// Deliveries in queued or processing status cannot be replayed.
func (d *WebhookDelivery) CanReplay() bool {
	return d.Status != WebhookDeliveryStatusQueued && d.Status != WebhookDeliveryStatusProcessing
}

// IsTerminal returns true if the delivery is in a terminal state (no more processing).
func (d *WebhookDelivery) IsTerminal() bool {
	return d.Status == WebhookDeliveryStatusDelivered ||
		(d.Status == WebhookDeliveryStatusFailed && d.AttemptNumber >= d.MaxAttempts)
}

// Replay resets the delivery to its initial queued state for replay.
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

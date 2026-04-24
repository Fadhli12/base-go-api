package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// EmailStatus represents the current state of an email in the queue.
// EmailQueue records are NEVER deleted - they serve as the audit trail.
type EmailStatus string

const (
	EmailStatusQueued     EmailStatus = "queued"     // Email queued for delivery
	EmailStatusProcessing EmailStatus = "processing" // Email currently being processed/sent
	EmailStatusSent       EmailStatus = "sent"       // Email sent successfully (awaiting delivery confirmation)
	EmailStatusFailed     EmailStatus = "failed"     // Email failed after max retries
	EmailStatusDelivered  EmailStatus = "delivered"  // Email delivered (webhook confirmation)
	EmailStatusBounced    EmailStatus = "bounced"    // Email bounced (hard or soft)
)

// EmailQueue represents an email queued for delivery.
// IMPORTANT: This table has NO soft delete - records are retained permanently
// for audit trail and compliance. Status changes are tracked in-place.
// Use `archived_at` timestamp for administrative hide if needed.
type EmailQueue struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ToAddress    string         `gorm:"column:to_address;size:255;not null;index" json:"to_address"` // Recipient email (column: to_address for reserved word)
	Subject      string         `gorm:"size:500;not null" json:"subject"`
	Template     string         `gorm:"size:100" json:"template,omitempty"`                      // Template name (optional)
	Data         datatypes.JSON `gorm:"type:jsonb" json:"data,omitempty"`                         // Template variables
	Status       EmailStatus   `gorm:"size:20;not null;default:'queued';index" json:"status"` // Current status
	Provider     string         `gorm:"size:50" json:"provider,omitempty"`                        // Email provider (smtp, sendgrid, ses)
	MessageID    string         `gorm:"size:255;index" json:"message_id,omitempty"`              // Provider message ID for webhook correlation
	Attempts     int           `gorm:"default:0" json:"attempts"`                               // Number of send attempts
	MaxAttempts  int           `gorm:"default:5" json:"max_attempts"`                           // Maximum retry attempts
	LastError    string        `gorm:"type:text" json:"last_error,omitempty"`                    // Last error message
	SentAt       *time.Time    `gorm:"index" json:"sent_at,omitempty"`                           // When email was sent
	DeliveredAt  *time.Time    `gorm:"index" json:"delivered_at,omitempty"`                      // When delivery confirmed (webhook)
	BouncedAt    *time.Time    `json:"bounced_at,omitempty"`                                     // When bounce detected
	BounceReason string        `gorm:"type:text" json:"bounce_reason,omitempty"`                // Bounce details
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// TableName returns the table name for EmailQueue
func (EmailQueue) TableName() string {
	return "email_queue"
}

// EmailQueueResponse represents an email queue record response
type EmailQueueResponse struct {
	ID           uuid.UUID     `json:"id"`
	ToAddress    string        `json:"to_address"`
	Subject      string        `json:"subject"`
	Template     string        `json:"template,omitempty"`
	Data         interface{}  `json:"data,omitempty"`
	Status       EmailStatus  `json:"status"`
	Provider     string        `json:"provider,omitempty"`
	MessageID     string       `json:"message_id,omitempty"`
	Attempts     int          `json:"attempts"`
	LastError    string        `json:"last_error,omitempty"`
	SentAt       *time.Time    `json:"sent_at,omitempty"`
	DeliveredAt  *time.Time    `json:"delivered_at,omitempty"`
	BouncedAt    *time.Time    `json:"bounced_at,omitempty"`
	BounceReason string        `json:"bounce_reason,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ToResponse converts EmailQueue to EmailQueueResponse
func (q *EmailQueue) ToResponse() EmailQueueResponse {
	return EmailQueueResponse{
		ID:           q.ID,
		ToAddress:    q.ToAddress,
		Subject:      q.Subject,
		Template:     q.Template,
		Data:         q.Data,
		Status:       q.Status,
		Provider:     q.Provider,
		MessageID:    q.MessageID,
		Attempts:     q.Attempts,
		LastError:    q.LastError,
		SentAt:       q.SentAt,
		DeliveredAt:  q.DeliveredAt,
		BouncedAt:    q.BouncedAt,
		BounceReason: q.BounceReason,
		CreatedAt:    q.CreatedAt,
		UpdatedAt:    q.UpdatedAt,
	}
}

// CanRetry returns true if the email can be retried
func (q *EmailQueue) CanRetry() bool {
	return q.Attempts < q.MaxAttempts && q.Status != EmailStatusDelivered
}

// IsTerminal returns true if the email is in a terminal state (no more processing)
func (q *EmailQueue) IsTerminal() bool {
	return q.Status == EmailStatusDelivered || q.Status == EmailStatusBounced || 
		(q.Status == EmailStatusFailed && q.Attempts >= q.MaxAttempts)
}
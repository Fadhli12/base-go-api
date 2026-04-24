package domain

import (
	"time"

	"github.com/google/uuid"
)

// BounceType represents the type of email bounce
type BounceType string

const (
	BounceTypeHard      BounceType = "hard"      // Permanent bounce (invalid email, domain not found)
	BounceTypeSoft      BounceType = "soft"      // Temporary bounce (mailbox full, server timeout)
	BounceTypeSpam      BounceType = "spam"      // Recipient marked as spam (complaint)
	BounceTypeTechnical BounceType = "technical" // Technical failure (infrastructure issue)
)

// EmailBounce represents a record of an email bounce or complaint.
// This table is INSERT-ONLY for compliance - no updates or deletes.
// Used to suppress future emails to bounced addresses.
type EmailBounce struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string     `gorm:"size:255;not null;index" json:"email"`              // Bounced email address
	BounceType   BounceType `gorm:"size:50;not null" json:"bounce_type"`              // Type of bounce
	BounceReason string     `gorm:"type:text" json:"bounce_reason,omitempty"`           // Provider bounce message
	MessageID    string     `gorm:"size:255;index" json:"message_id,omitempty"`       // Original message ID
	CreatedAt    time.Time  `json:"created_at"`                                         // When bounce occurred
}

// TableName returns the table name for EmailBounce
func (EmailBounce) TableName() string {
	return "email_bounces"
}

// EmailBounceResponse represents a bounce record response
type EmailBounceResponse struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	BounceType   BounceType `json:"bounce_type"`
	BounceReason string     `json:"bounce_reason,omitempty"`
	MessageID    string     `json:"message_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ToResponse converts EmailBounce to EmailBounceResponse
func (b *EmailBounce) ToResponse() EmailBounceResponse {
	return EmailBounceResponse{
		ID:           b.ID,
		Email:        b.Email,
		BounceType:   b.BounceType,
		BounceReason: b.BounceReason,
		MessageID:    b.MessageID,
		CreatedAt:    b.CreatedAt,
	}
}

// IsHardBounce returns true if this is a permanent bounce that should suppress future emails
func (b *EmailBounce) IsHardBounce() bool {
	return b.BounceType == BounceTypeHard || b.BounceType == BounceTypeSpam
}

// ShouldSuppress returns true if emails to this address should be suppressed
// based on bounce type and recency (hard bounces: always, soft bounces: recent only)
func (b *EmailBounce) ShouldSuppress() bool {
	return b.IsHardBounce()
}
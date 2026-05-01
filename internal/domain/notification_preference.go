package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationPreference represents a user's delivery preferences for a notification type.
// Supports soft delete so preference records can be logically removed without data loss.
type NotificationPreference struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	NotificationType NotificationType `gorm:"size:30;not null" json:"notification_type"`
	EmailEnabled     bool           `gorm:"default:true;not null" json:"email_enabled"`
	PushEnabled      bool           `gorm:"default:true;not null" json:"push_enabled"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the NotificationPreference model
func (NotificationPreference) TableName() string {
	return "notification_preferences"
}

// NotificationPreferenceResponse represents a notification preference in API responses
type NotificationPreferenceResponse struct {
	ID               string `json:"id"`
	UserID           string `json:"user_id"`
	NotificationType string `json:"notification_type"`
	EmailEnabled     bool   `json:"email_enabled"`
	PushEnabled      bool   `json:"push_enabled"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// ToResponse converts a NotificationPreference to NotificationPreferenceResponse
func (p *NotificationPreference) ToResponse() NotificationPreferenceResponse {
	return NotificationPreferenceResponse{
		ID:               p.ID.String(),
		UserID:           p.UserID.String(),
		NotificationType: string(p.NotificationType),
		EmailEnabled:     p.EmailEnabled,
		PushEnabled:      p.PushEnabled,
		CreatedAt:        p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
	}
}

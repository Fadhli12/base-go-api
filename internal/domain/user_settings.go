package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UserSettings represents user-level preferences per organization
type UserSettings struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID          `gorm:"type:uuid;not null;index" json:"organization_id"`
	UserID         uuid.UUID          `gorm:"type:uuid;not null;index" json:"user_id"`
	Settings       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	DeletedAt      gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for the UserSettings model
func (UserSettings) TableName() string {
	return "user_settings"
}

// UserSettingsResponse represents user settings in API responses
type UserSettingsResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	UserID         uuid.UUID       `json:"user_id"`
	Settings       json.RawMessage `json:"settings" swaggertype:"object"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ToResponse converts UserSettings to UserSettingsResponse
func (u *UserSettings) ToResponse() UserSettingsResponse {
	var settings json.RawMessage
	if len(u.Settings) > 0 {
		settings = json.RawMessage(u.Settings)
	}

	return UserSettingsResponse{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		UserID:         u.UserID,
		Settings:       settings,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}

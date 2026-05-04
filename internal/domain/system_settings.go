package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SystemSettings represents organization-wide system configuration
type SystemSettings struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex" json:"organization_id"`
	Settings       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	DeletedAt      gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for the SystemSettings model
func (SystemSettings) TableName() string {
	return "system_settings"
}

// SystemSettingsResponse represents system settings in API responses
type SystemSettingsResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Settings       json.RawMessage `json:"settings" swaggertype:"object"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ToResponse converts SystemSettings to SystemSettingsResponse
func (s *SystemSettings) ToResponse() SystemSettingsResponse {
	var settings json.RawMessage
	if len(s.Settings) > 0 {
		settings = json.RawMessage(s.Settings)
	}

	return SystemSettingsResponse{
		ID:             s.ID,
		OrganizationID: s.OrganizationID,
		Settings:       settings,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// DefaultSystemSettings returns the default system settings for a new organization
func DefaultSystemSettings(orgID uuid.UUID) *SystemSettings {
	defaults := datatypes.JSON(`{
		"app_name": "API Base",
		"maintenance_mode": false,
		"rate_limits": {
			"requests_per_minute": 60,
			"notifications_per_hour": 100
		},
		"email_config": {
			"from_address": "noreply@example.com",
			"from_name": "API Base"
		}
	}`)

	return &SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       defaults,
	}
}

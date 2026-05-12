package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DashboardPreference represents per-organization dashboard metric visibility preferences.
type DashboardPreference struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrganizationID   uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex"`
	MetricCategories datatypes.JSON `gorm:"type:jsonb"`
	UpdatedByUserID  uuid.UUID      `gorm:"type:uuid;not null"`
	CreatedAt        time.Time      `gorm:"autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime"`
}

// TableName returns the table name for the DashboardPreference model.
func (DashboardPreference) TableName() string {
	return "dashboard_preferences"
}

// ToResponse converts DashboardPreference to DashboardPreferenceResponse.
func (d *DashboardPreference) ToResponse() DashboardPreferenceResponse {
	var categories map[string]bool
	if len(d.MetricCategories) > 0 {
		_ = json.Unmarshal(d.MetricCategories, &categories)
	}

	return DashboardPreferenceResponse{
		OrganizationID:   d.OrganizationID.String(),
		MetricCategories: categories,
		UpdatedByUserID: d.UpdatedByUserID.String(),
		CreatedAt:       d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       d.UpdatedAt.Format(time.RFC3339),
	}
}

// DashboardPreferenceResponse represents a dashboard preference in API responses.
type DashboardPreferenceResponse struct {
	OrganizationID   string         `json:"organization_id"`
	MetricCategories map[string]bool `json:"metric_categories"`
	UpdatedByUserID string         `json:"updated_by_user_id"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

// DefaultMetricCategories returns the default category visibility (all visible).
func DefaultMetricCategories() map[string]bool {
	return map[string]bool{
		"user_activity":       true,
		"content_metrics":    true,
		"engagement_metrics": true,
		"system_metrics":     true,
	}
}
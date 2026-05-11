package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FeatureFlag represents a runtime feature toggle for gradual rollout and A/B testing.
type FeatureFlag struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Key         string         `gorm:"size:100;uniqueIndex:idx_feature_flags_key,not null" json:"key"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Enabled     bool           `gorm:"default:false;not null" json:"enabled"`
	Rollout     int            `gorm:"default:100;not null" json:"rollout"`
	Conditions  datatypes.JSON `gorm:"type:jsonb" json:"conditions"`
	IsSystem    bool           `gorm:"default:false;not null" json:"is_system"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for the FeatureFlag model
func (FeatureFlag) TableName() string {
	return "feature_flags"
}

// ToResponse converts FeatureFlag to FeatureFlagResponse
func (f *FeatureFlag) ToResponse() FeatureFlagResponse {
	var conditions json.RawMessage
	if len(f.Conditions) > 0 {
		conditions = json.RawMessage(f.Conditions)
	}

	return FeatureFlagResponse{
		ID:          f.ID,
		Key:         f.Key,
		Name:        f.Name,
		Description: f.Description,
		Enabled:     f.Enabled,
		Rollout:     f.Rollout,
		Conditions:  conditions,
		IsSystem:    f.IsSystem,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

// FeatureFlagResponse represents a feature flag in API responses
type FeatureFlagResponse struct {
	ID          uuid.UUID       `json:"id"`
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Rollout     int             `json:"rollout"`
	Conditions  json.RawMessage `json:"conditions"`
	IsSystem    bool            `json:"is_system"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// FeatureFlagEvaluation represents the result of evaluating a single feature flag
type FeatureFlagEvaluation struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
	Rollout int    `json:"rollout"`
}

// BulkEvaluateResponse represents the result of evaluating all feature flags for a user
type BulkEvaluateResponse struct {
	Flags []FeatureFlagEvaluation `json:"flags"`
}
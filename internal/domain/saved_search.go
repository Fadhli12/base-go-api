package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SavedSearch represents a user's saved search with filters
type SavedSearch struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	QueryText string         `gorm:"type:text" json:"query_text"`
	Filters   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"filters"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the SavedSearch model
func (SavedSearch) TableName() string {
	return "saved_searches"
}

// SavedSearchResponse represents a saved search in API responses
type SavedSearchResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	QueryText string `json:"query_text,omitempty"`
	Filters   any    `json:"filters,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ToResponse converts a SavedSearch to SavedSearchResponse
func (s *SavedSearch) ToResponse() SavedSearchResponse {
	return SavedSearchResponse{
		ID:        s.ID.String(),
		UserID:    s.UserID.String(),
		Name:      s.Name,
		QueryText: s.QueryText,
		Filters:   s.Filters,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

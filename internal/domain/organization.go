package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Organization represents a multi-tenant organization
type Organization struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Slug      string         `gorm:"uniqueIndex;size:100;not null" json:"slug"`
	OwnerID   uuid.UUID      `gorm:"type:uuid;not null" json:"owner_id"`
	Settings  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"settings,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Owner   *User                 `gorm:"foreignKey:OwnerID" json:"-"`
	Members []*OrganizationMember `gorm:"foreignKey:OrganizationID" json:"-"`
}

// TableName returns the table name for the Organization model
func (Organization) TableName() string {
	return "organizations"
}

// OrganizationResponse represents an organization response without sensitive fields
type OrganizationResponse struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Slug        string             `json:"slug"`
	OwnerID     uuid.UUID          `json:"owner_id"`
	Settings    json.RawMessage    `json:"settings,omitempty" swaggertype:"object"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	MemberCount int                `json:"member_count,omitempty"`
}

// ToResponse converts an Organization to OrganizationResponse
func (o *Organization) ToResponse() *OrganizationResponse {
	var settings json.RawMessage
	if len(o.Settings) > 0 {
		settings = json.RawMessage(o.Settings)
	}

	resp := &OrganizationResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		OwnerID:   o.OwnerID,
		Settings:  settings,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}

	if o.Members != nil {
		resp.MemberCount = len(o.Members)
	}

	return resp
}

// NewJSONB converts a map to datatypes.JSON
func NewJSONB(data map[string]interface{}) (datatypes.JSON, error) {
	b, err := json.Marshal(data)
	return datatypes.JSON(b), err
}
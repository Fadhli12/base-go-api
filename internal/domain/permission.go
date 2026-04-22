package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Permission represents a granular access control permission in the system.
// Permissions follow the pattern: resource:action with optional scope limitations.
type Permission struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"uniqueIndex;size:100;not null" json:"name"`     // e.g., "invoice:create"
	Resource  string         `gorm:"size:50;not null" json:"resource"`               // e.g., "invoice"
	Action    string         `gorm:"size:20;not null" json:"action"`                 // e.g., "create"
	Scope     string         `gorm:"size:20;not null;default:'all'" json:"scope"`    // e.g., "all", "own"
	IsSystem  bool           `gorm:"default:false" json:"is_system"`                 // Cannot be deleted
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Permission entity.
func (Permission) TableName() string {
	return "permissions"
}
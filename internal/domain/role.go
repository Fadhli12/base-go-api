package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role represents a collection of permissions that can be assigned to users.
// Roles provide a way to group permissions and simplify access control management.
type Role struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"uniqueIndex;size:50;not null" json:"name"`         // e.g., "admin"
	Description string         `gorm:"size:255" json:"description"`
	IsSystem    bool           `gorm:"default:false" json:"is_system"`                    // Cannot be deleted
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Permissions []Permission   `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// TableName specifies the table name for Role entity.
func (Role) TableName() string {
	return "roles"
}
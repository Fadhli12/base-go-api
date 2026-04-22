package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserRole represents a many-to-many relationship between users and roles.
type UserRole struct {
	UserID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	RoleID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"role_id"`
	AssignedAt time.Time `gorm:"autoCreateTime" json:"assigned_at"`
	AssignedBy uuid.UUID `gorm:"type:uuid" json:"assigned_by"` // Who assigned this role
}

// TableName returns the table name for UserRole.
func (UserRole) TableName() string {
	return "user_roles"
}
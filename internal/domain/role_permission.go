package domain

import (
	"time"

	"github.com/google/uuid"
)

// RolePermission represents a many-to-many relationship between roles and permissions.
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"role_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"permission_id"`
	AssignedAt   time.Time `gorm:"autoCreateTime" json:"assigned_at"`
}

// TableName returns the table name for RolePermission.
func (RolePermission) TableName() string {
	return "role_permissions"
}
package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserPermission represents a direct permission assignment to a user.
// This allows granting or denying specific permissions outside of role assignments.
// The Effect field determines whether the permission is allowed or denied.
type UserPermission struct {
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"permission_id"`
	Effect       string    `gorm:"size:10;not null;default:'allow'" json:"effect"` // "allow" or "deny"
	AssignedAt   time.Time `gorm:"autoCreateTime" json:"assigned_at"`
	AssignedBy   uuid.UUID `gorm:"type:uuid" json:"assigned_by"`
}

// TableName specifies the table name for UserPermission entity.
func (UserPermission) TableName() string {
	return "user_permissions"
}
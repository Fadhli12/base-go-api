package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Effect constants for UserPermission
const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// ErrInvalidEffect is returned when Effect is not "allow" or "deny"
var ErrInvalidEffect = errors.New("effect must be 'allow' or 'deny'")

// UserPermission represents a direct permission assignment to a user.
// This allows granting or denying specific permissions outside of role assignments.
// The Effect field determines whether the permission is allowed or denied.
type UserPermission struct {
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"permission_id"`
	Effect       string    `gorm:"size:10;not null;default:'allow';check:effect IN ('allow', 'deny')" json:"effect"` // "allow" or "deny"
	AssignedAt   time.Time `gorm:"autoCreateTime" json:"assigned_at"`
	AssignedBy   uuid.UUID `gorm:"type:uuid" json:"assigned_by"`
}

// TableName specifies the table name for UserPermission entity.
func (UserPermission) TableName() string {
	return "user_permissions"
}

// IsValidEffect returns true if the effect is valid
func IsValidEffect(effect string) bool {
	return effect == EffectAllow || effect == EffectDeny
}

// Validate checks if the UserPermission has valid values
func (up *UserPermission) Validate() error {
	if !IsValidEffect(up.Effect) {
		return ErrInvalidEffect
	}
	return nil
}
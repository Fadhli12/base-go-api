package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TwoFactorRecoveryCode represents a recovery code for two-factor authentication
type TwoFactorRecoveryCode struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	CodeHash  string         `gorm:"size:255;not null" json:"-"` // Hidden from JSON
	UsedAt    *time.Time    `gorm:"index" json:"used_at,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for TwoFactorRecoveryCode
func (TwoFactorRecoveryCode) TableName() string {
	return "two_factor_recovery_codes"
}

// TwoFactorRecoveryCodeResponse represents a safe response for recovery codes
type TwoFactorRecoveryCodeResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// ToResponse converts a TwoFactorRecoveryCode to TwoFactorRecoveryCodeResponse
func (t *TwoFactorRecoveryCode) ToResponse() TwoFactorRecoveryCodeResponse {
	return TwoFactorRecoveryCodeResponse{
		ID:        t.ID,
		UserID:    t.UserID,
		Used:      t.UsedAt != nil,
		CreatedAt: t.CreatedAt,
	}
}

// IsUsed returns true if the recovery code has been used
func (t *TwoFactorRecoveryCode) IsUsed() bool {
	return t.UsedAt != nil
}
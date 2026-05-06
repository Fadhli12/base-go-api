package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user account in the system
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Two-Factor Authentication fields
	TwoFactorEnabled   bool       `gorm:"default:false" json:"two_factor_enabled,omitempty"`
	TwoFactorStatus     string    `gorm:"size:20;default:'disabled'" json:"two_factor_status,omitempty"`
	TwoFactorSecret     string    `gorm:"size:255" json:"-"`
	TwoFactorVerifiedAt *time.Time `json:"two_factor_verified_at,omitempty"`
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// UserResponse represents a user response without sensitive fields
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse converts a User to UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

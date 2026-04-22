package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a refresh token for authentication
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:255;uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`

	// BelongsTo relationship
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

// TableName returns the table name for the RefreshToken model
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// IsRevoked checks if the token has been revoked
func (rt *RefreshToken) IsRevoked() bool {
	return rt.RevokedAt != nil
}

// IsValid checks if the token is valid (not expired and not revoked)
func (rt *RefreshToken) IsValid() bool {
	if rt.IsRevoked() {
		return false
	}
	return time.Now().Before(rt.ExpiresAt)
}

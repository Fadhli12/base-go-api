package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a refresh token for authentication
// MED-004: Added FamilyID for token family tracking (refresh token rotation detection)
// MED-005: Added session metadata for session management
type RefreshToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash  string     `gorm:"size:255;uniqueIndex;not null" json:"-"`
	FamilyID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"family_id"` // MED-004: Token family for rotation detection
	ExpiresAt  time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	// MED-005: Session management metadata
	UserAgent  string `gorm:"size:500" json:"user_agent,omitempty"`  // Browser/client info
	IPAddress  string `gorm:"size:45" json:"ip_address,omitempty"`   // Login IP address
	DeviceName string `gorm:"size:255" json:"device_name,omitempty"` // User-friendly device name

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

// IsInFamily checks if a token belongs to a specific family
func (rt *RefreshToken) IsInFamily(familyID uuid.UUID) bool {
	return rt.FamilyID == familyID
}

package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// APIKey represents an API key for service-to-service authentication
type APIKey struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	User       User           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Name       string         `gorm:"size:255;not null" json:"name"`
	Prefix     string         `gorm:"size:12;not null" json:"prefix"`
	KeyHash    string         `gorm:"size:255;not null;uniqueIndex" json:"-"`
	Scopes     datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"scopes"`
	ExpiresAt  *time.Time     `gorm:"index" json:"expires_at,omitempty"`
	LastUsedAt *time.Time     `gorm:"index" json:"last_used_at,omitempty"`
	RevokedAt  *time.Time     `gorm:"index" json:"revoked_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for the APIKey model
func (APIKey) TableName() string {
	return "api_keys"
}

// APIKeyResponse represents an API key in API responses (excludes sensitive fields)
type APIKeyResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Prefix     string    `json:"prefix"`
	Scopes     []string  `json:"scopes"`
	ExpiresAt  *string   `json:"expires_at,omitempty"`
	LastUsedAt *string   `json:"last_used_at,omitempty"`
	IsRevoked  bool      `json:"is_revoked"`
	IsExpired  bool      `json:"is_expired"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  string    `json:"created_at"`
	UpdatedAt  string    `json:"updated_at"`
}

// ToResponse converts an APIKey to APIKeyResponse
func (k *APIKey) ToResponse() APIKeyResponse {
	var scopes []string
	if k.Scopes != nil {
		_ = json.Unmarshal(k.Scopes, &scopes)
	}

	var expiresAtStr, lastUsedAtStr *string
	if k.ExpiresAt != nil {
		formatted := k.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &formatted
	}
	if k.LastUsedAt != nil {
		formatted := k.LastUsedAt.Format(time.RFC3339)
		lastUsedAtStr = &formatted
	}

	return APIKeyResponse{
		ID:         k.ID.String(),
		UserID:     k.UserID.String(),
		Name:       k.Name,
		Prefix:     k.Prefix,
		Scopes:     scopes,
		ExpiresAt: expiresAtStr,
		LastUsedAt: lastUsedAtStr,
		IsRevoked:  k.IsRevoked(),
		IsExpired:  k.IsExpired(),
		IsActive:   k.IsActive(),
		CreatedAt:  k.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  k.UpdatedAt.Format(time.RFC3339),
	}
}

// IsRevoked checks if the API key has been revoked
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return k.ExpiresAt.Before(time.Now())
}

// IsActive checks if the API key is active (not deleted, not revoked, not expired)
func (k *APIKey) IsActive() bool {
	// Check soft delete
	if k.DeletedAt.Valid {
		return false
	}
	// Check revocation
	if k.IsRevoked() {
		return false
	}
	// Check expiration
	if k.IsExpired() {
		return false
	}
	return true
}

// HasScope checks if the API key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	var scopes []string
	if k.Scopes == nil {
		return false
	}
	if err := json.Unmarshal(k.Scopes, &scopes); err != nil {
		return false
	}

	for _, s := range scopes {
		// Wildcard scope matches everything
		if s == "*" {
			return true
		}
		// Exact match
		if s == scope {
			return true
		}
		// resource:* matches all actions on resource
		resource, _, hasAction := parseScopeParts(scope)
		if hasAction {
			for _, s2 := range scopes {
				s2Resource, s2Action, s2HasAction := parseScopeParts(s2)
				if s2HasAction && s2Resource == resource && s2Action == "*" {
					return true
				}
			}
		}
	}
	return false
}

// HasAnyScope checks if the API key has any of the specified scopes
func (k *APIKey) HasAnyScope(scopes []string) bool {
	for _, scope := range scopes {
		if k.HasScope(scope) {
			return true
		}
	}
	return false
}

// GetScopes returns the list of scopes as a string slice
func (k *APIKey) GetScopes() []string {
	var scopes []string
	if k.Scopes == nil {
		return scopes
	}
	_ = json.Unmarshal(k.Scopes, &scopes)
	return scopes
}

// parseScopeParts parses a scope into resource and action parts
// Returns (resource, action, hasAction) where hasAction is true if the scope has an action part
func parseScopeParts(scope string) (resource string, action string, hasAction bool) {
	for i := 0; i < len(scope); i++ {
		if scope[i] == ':' {
			return scope[:i], scope[i+1:], true
		}
	}
	return scope, "", false
}

// IsOwnedBy checks if the API key is owned by the given user
func (k *APIKey) IsOwnedBy(userID uuid.UUID) bool {
	return k.UserID == userID
}
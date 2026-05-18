package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthAccount represents a link between a user and their OAuth social identity.
type OAuthAccount struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	User           *User          `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	ProviderID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"provider_id"`
	Provider       *OAuthProvider `gorm:"foreignKey:ProviderID;references:ID" json:"provider,omitempty"`
	ProviderUserID string         `gorm:"size:255;not null" json:"provider_user_id"`
	Email          string         `gorm:"size:255" json:"email,omitempty"`
	EmailVerified  bool           `gorm:"default:false" json:"email_verified"`
	DisplayName    string         `gorm:"size:255" json:"display_name,omitempty"`
	AvatarURL      string         `gorm:"size:500" json:"avatar_url,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for OAuthAccount.
func (OAuthAccount) TableName() string { return "oauth_accounts" }

// OAuthAccountResponse — DTO for API responses.
type OAuthAccountResponse struct {
	ID          uuid.UUID `json:"id"`
	Provider    string    `json:"provider"`
	Email       string    `json:"email,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	LinkedAt    time.Time `json:"linked_at"`
}

// ToResponse converts OAuthAccount to API response.
func (a *OAuthAccount) ToResponse() OAuthAccountResponse {
	providerName := ""
	if a.Provider != nil {
		providerName = a.Provider.Name
	}
	return OAuthAccountResponse{
		ID:          a.ID,
		Provider:    providerName,
		Email:       a.Email,
		DisplayName: a.DisplayName,
		AvatarURL:   a.AvatarURL,
		LinkedAt:    a.CreatedAt,
	}
}

// ProviderProfile — normalized user profile from all OAuth providers (internal, not persisted).
type ProviderProfile struct {
	ProviderID    string `json:"provider_id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
}

// OAuthState — stored in Redis for CSRF protection (internal, not persisted).
type OAuthState struct {
	CallbackURL  string    `json:"callback_url"`
	Provider     string    `json:"provider"`
	Intent       string    `json:"intent"`        // "login" or "link"
	UserID       uuid.UUID `json:"user_id"`       // uuid.Nil for login, actual user UUID for link
	CodeVerifier string    `json:"code_verifier"` // PKCE S256
	OrgID        uuid.UUID `json:"org_id"`
	CreatedAt    time.Time `json:"created_at"`
}
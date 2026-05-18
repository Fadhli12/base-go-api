package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// OAuth provider name constants — strings, NOT Go enum/iota
const (
	OAuthProviderGoogle    = "google"
	OAuthProviderGitHub    = "github"
	OAuthProviderMicrosoft = "microsoft"
)

// ValidOAuthProviders is a lookup map for valid provider names.
var ValidOAuthProviders = map[string]bool{
	OAuthProviderGoogle:    true,
	OAuthProviderGitHub:    true,
	OAuthProviderMicrosoft: true,
}

// DefaultOAuthScopes maps provider names to their default OAuth scopes.
var DefaultOAuthScopes = map[string][]string{
	OAuthProviderGoogle:    {"openid", "email", "profile"},
	OAuthProviderGitHub:    {"user:email", "read:user"},
	OAuthProviderMicrosoft: {"openid", "email", "profile"},
}

// OAuthProvider represents an OAuth 2.0 identity provider configuration.
type OAuthProvider struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name                  string         `gorm:"size:50;not null" json:"name"`
	DisplayName           string         `gorm:"size:100;not null" json:"display_name"`
	ClientID             string         `gorm:"size:500;not null" json:"client_id"`
	ClientSecretEncrypted string         `gorm:"type:text;not null" json:"-"` // NEVER expose in JSON
	RedirectURL          string         `gorm:"size:500;not null" json:"redirect_url"`
	AdditionalScopes     datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"additional_scopes"`
	Config               datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"config,omitempty"`
	IsEnabled            bool           `gorm:"default:true;not null" json:"is_enabled"`
	IsSystem             bool           `gorm:"default:false;not null" json:"is_system"`
	OrganizationID       *uuid.UUID     `gorm:"type:uuid;index" json:"organization_id,omitempty"`
	Organization         *Organization  `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for OAuthProvider.
func (OAuthProvider) TableName() string { return "oauth_providers" }

// OAuthProviderResponse — DTO returned in API responses, strips ClientSecretEncrypted.
type OAuthProviderResponse struct {
	ID               uuid.UUID      `json:"id"`
	Name             string         `json:"name"`
	DisplayName      string         `json:"display_name"`
	ClientID         string         `json:"client_id"`
	RedirectURL      string         `json:"redirect_url"`
	Scopes           []string       `json:"scopes"`
	AdditionalScopes []string       `json:"additional_scopes"`
	Config           datatypes.JSON `json:"config,omitempty"`
	IsEnabled        bool           `json:"is_enabled"`
	IsSystem         bool           `json:"is_system"`
	OrganizationID   *uuid.UUID     `json:"organization_id,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// ToResponse converts OAuthProvider to API response, stripping the encrypted secret.
func (p *OAuthProvider) ToResponse() OAuthProviderResponse {
	var additionalScopes []string
	if p.AdditionalScopes != nil {
		_ = json.Unmarshal(p.AdditionalScopes, &additionalScopes)
	}
	return OAuthProviderResponse{
		ID:               p.ID,
		Name:             p.Name,
		DisplayName:      p.DisplayName,
		ClientID:         p.ClientID,
		RedirectURL:      p.RedirectURL,
		Scopes:           p.GetEffectiveScopes(),
		AdditionalScopes: additionalScopes,
		Config:           p.Config,
		IsEnabled:        p.IsEnabled,
		IsSystem:         p.IsSystem,
		OrganizationID:   p.OrganizationID,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

// GetEffectiveScopes returns the merged default + additional scopes for this provider.
func (p *OAuthProvider) GetEffectiveScopes() []string {
	var additional []string
	if p.AdditionalScopes != nil {
		_ = json.Unmarshal(p.AdditionalScopes, &additional)
	}
	defaults := DefaultOAuthScopes[p.Name]
	merged := make([]string, len(defaults))
	copy(merged, defaults)
	for _, s := range additional {
		found := false
		for _, d := range merged {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, s)
		}
	}
	return merged
}

// PublicOAuthProviderResponse — minimal info for unauthenticated listing.
type PublicOAuthProviderResponse struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Scopes      []string `json:"scopes"`
}

// ToPublicResponse converts OAuthProvider to public (no secrets, no config) response.
func (p *OAuthProvider) ToPublicResponse() PublicOAuthProviderResponse {
	return PublicOAuthProviderResponse{
		Name:        p.Name,
		DisplayName: p.DisplayName,
		Scopes:      p.GetEffectiveScopes(),
	}
}
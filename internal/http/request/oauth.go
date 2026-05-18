package request

import (
	"encoding/json"

	"github.com/example/go-api-base/internal/domain"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// CreateOAuthProviderRequest represents the request body for creating an OAuth provider.
type CreateOAuthProviderRequest struct {
	Name              string          `json:"name" validate:"required,oneof=google github microsoft"`
	DisplayName       string          `json:"display_name" validate:"required,max=100"`
	ClientID          string          `json:"client_id" validate:"required,max=500"`
	ClientSecret      string          `json:"client_secret" validate:"required,min=16,max=500"`
	RedirectURL       string          `json:"redirect_url" validate:"required,url,max=500"`
	AdditionalScopes  []string        `json:"additional_scopes" validate:"max=10,dive,max=50"`
	Config            json.RawMessage `json:"config,omitempty" validate:"omitempty"`
	OrganizationID    *uuid.UUID     `json:"organization_id,omitempty"`
}

// UpdateOAuthProviderRequest represents the request body for updating an OAuth provider.
// All fields are optional — only non-nil fields are applied.
type UpdateOAuthProviderRequest struct {
	DisplayName      *string         `json:"display_name,omitempty" validate:"omitempty,max=100"`
	ClientID         *string         `json:"client_id,omitempty" validate:"omitempty,max=500"`
	ClientSecret     *string         `json:"client_secret,omitempty" validate:"omitempty,min=16,max=500"`
	RedirectURL      *string         `json:"redirect_url,omitempty" validate:"omitempty,url,max=500"`
	AdditionalScopes *[]string       `json:"additional_scopes,omitempty" validate:"omitempty,max=10,dive,max=50"`
	Config           json.RawMessage `json:"config,omitempty" validate:"omitempty"`
	IsEnabled        *bool           `json:"is_enabled,omitempty"`
}

// Validate validates the CreateOAuthProviderRequest.
func (r *CreateOAuthProviderRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the UpdateOAuthProviderRequest.
func (r *UpdateOAuthProviderRequest) Validate() error {
	return validate.Struct(r)
}

// ListOAuthProvidersQuery represents query parameters for listing OAuth providers.
type ListOAuthProvidersQuery struct {
	Limit  int  `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int  `query:"offset" validate:"omitempty,min=0"`
}

// Validate validates the ListOAuthProvidersQuery and applies defaults.
func (q *ListOAuthProvidersQuery) Validate() error {
	if err := validate.Struct(q); err != nil {
		return err
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	return nil
}

// GetLimit returns the limit with default fallback.
func (q *ListOAuthProvidersQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 20
	}
	if q.Limit > 100 {
		return 100
	}
	return q.Limit
}

// GetOffset returns the offset with default fallback.
func (q *ListOAuthProvidersQuery) GetOffset() int {
	if q.Offset < 0 {
		return 0
	}
	return q.Offset
}

func init() {
	// Register OAuth provider name validator
	validate.RegisterValidation("oauth_provider_name", func(fl validator.FieldLevel) bool {
		return domain.ValidOAuthProviders[fl.Field().String()]
	})
}
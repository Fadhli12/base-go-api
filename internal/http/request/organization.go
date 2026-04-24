package request

import (
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Custom regex for alphanumeric + dash/underscore validation
var alphanumdashRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// CreateOrganizationRequest represents a request to create an organization
type CreateOrganizationRequest struct {
	Name     string                 `json:"name" validate:"required,min=1,max=255"`
	Slug     string                 `json:"slug" validate:"required,min=1,max=100,alphanumdash"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// UpdateOrganizationRequest represents a request to update an organization
type UpdateOrganizationRequest struct {
	Name     string                 `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Slug     string                 `json:"slug,omitempty" validate:"omitempty,min=1,max=100,alphanumdash"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// AddMemberRequest represents a request to add a member to an organization
type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Role   string    `json:"role" validate:"required,oneof=owner admin member"`
}

// Validate validates the CreateOrganizationRequest
func (r *CreateOrganizationRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the UpdateOrganizationRequest
func (r *UpdateOrganizationRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the AddMemberRequest
func (r *AddMemberRequest) Validate() error {
	return validate.Struct(r)
}

// init registers custom validators
func init() {
	validate.RegisterValidation("alphanumdash", func(fl validator.FieldLevel) bool {
		return alphanumdashRegex.MatchString(fl.Field().String())
	})
}
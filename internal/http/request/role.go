package request

// CreateRoleRequest represents a role creation request
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required,max=50"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// UpdateRoleRequest represents a role update request
type UpdateRoleRequest struct {
	Name        string `json:"name" validate:"omitempty,max=50"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// AttachPermissionRequest represents a request to attach a permission to a role
type AttachPermissionRequest struct {
	PermissionID string `json:"permission_id" validate:"required,uuid"`
}

// Validate validates the request struct using go-playground/validator
func (r *CreateRoleRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *UpdateRoleRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *AttachPermissionRequest) Validate() error {
	return validate.Struct(r)
}
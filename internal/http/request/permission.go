package request

// CreatePermissionRequest represents a permission creation request
type CreatePermissionRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Resource string `json:"resource" validate:"required,max=50"`
	Action   string `json:"action" validate:"required,max=20"`
	Scope    string `json:"scope" validate:"omitempty,max=20"`
}

// Validate validates the request struct using go-playground/validator
func (r *CreatePermissionRequest) Validate() error {
	return validate.Struct(r)
}
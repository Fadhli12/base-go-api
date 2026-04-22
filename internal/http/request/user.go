package request

// CreateUserRoleRequest represents a request to assign a role to a user
type CreateUserRoleRequest struct {
	RoleID string `json:"role_id" validate:"required,uuid"`
}

// CreateUserPermissionRequest represents a request to grant or deny a permission to a user
type CreateUserPermissionRequest struct {
	PermissionID string `json:"permission_id" validate:"required,uuid"`
	Effect       string `json:"effect" validate:"required,oneof=allow deny"`
}

// Validate validates the request struct using go-playground/validator
func (r *CreateUserRoleRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *CreateUserPermissionRequest) Validate() error {
	return validate.Struct(r)
}
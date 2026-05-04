package request

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Name     string `json:"name"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Name string `json:"name"`
}

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
func (r *CreateUserRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *UpdateUserRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *CreateUserRoleRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *CreateUserPermissionRequest) Validate() error {
	return validate.Struct(r)
}
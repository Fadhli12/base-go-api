package request

import (
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Validate validates the request struct using go-playground/validator
func (r *RegisterRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *LoginRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *RefreshRequest) Validate() error {
	return validate.Struct(r)
}

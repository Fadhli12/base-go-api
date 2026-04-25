package request

import (
	"unicode"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Password strength is validated by validatePasswordStrength function.
// Requirements:
// - At least 8 characters
// - At least 1 uppercase letter
// - At least 1 lowercase letter
// - At least 1 number
// - At least 1 special character
// Note: Go's regexp package doesn't support lookahead assertions,
// so validation is done manually in validatePasswordStrength().

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72,password_strength"`
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

// PasswordResetRequest represents a password reset request (request token)
type PasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmRequest represents a password reset confirmation (set new password)
type PasswordResetConfirmRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=8,max=72,password_strength"`
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

// Validate validates the request struct using go-playground/validator
func (r *PasswordResetRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the request struct using go-playground/validator
func (r *PasswordResetConfirmRequest) Validate() error {
	return validate.Struct(r)
}

// init registers custom validators
func init() {
	// Register custom password strength validator (HIGH-005)
	validate.RegisterValidation("password_strength", validatePasswordStrength)
}

// validatePasswordStrength checks password meets complexity requirements
func validatePasswordStrength(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	// Check minimum length
	if len(password) < 8 {
		return false
	}

	// Check maximum length
	if len(password) > 72 {
		return false
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Require at least: 1 uppercase, 1 lowercase, 1 number, 1 special character
	return hasUpper && hasLower && hasNumber && hasSpecial
}

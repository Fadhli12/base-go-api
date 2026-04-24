package request

import (
	"time"

	"github.com/go-playground/validator/v10"
)

// CreateAPIKeyRequest represents a request to create a new API key
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" validate:"required,max=255"`
	Scopes    []string   `json:"scopes" validate:"required,dive,min=1"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// RevokeAPIKeyRequest represents a request to revoke an API key
// Note: This is intentionally empty - revoke doesn't need a body
type RevokeAPIKeyRequest struct{}

// ListAPIKeysRequest represents query parameters for listing API keys
type ListAPIKeysRequest struct {
	Limit  int `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset int `query:"offset" validate:"omitempty,min=0"`
}

// Validate validates the CreateAPIKeyRequest struct
func (r *CreateAPIKeyRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the ListAPIKeysRequest struct
func (r *ListAPIKeysRequest) Validate() error {
	return validate.Struct(r)
}

// SetDefaults sets default values for ListAPIKeysRequest
func (r *ListAPIKeysRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
	if r.Limit > 100 {
		r.Limit = 100
	}
}

// Scope validation constants
const (
	ScopeSeparator = ":"
	ScopeWildcard  = "*"

	// Valid actions in scopes
	ScopeActionRead   = "read"
	ScopeActionWrite  = "write"
	ScopeActionCreate = "create"
	ScopeActionUpdate = "update"
	ScopeActionDelete = "delete"
	ScopeActionManage = "manage"
)

// IsValidScope checks if a scope string has valid format (resource:action or *)
func IsValidScope(scope string) bool {
	if scope == ScopeWildcard {
		return true
	}

	// Scope must have at least one ":" separator
	for i, c := range scope {
		if c == ':' {
			if i == 0 || i == len(scope)-1 {
				return false // ":" at start or end is invalid
			}
			return true
		}
	}
	return false // No ":" found
}

// ParseScope splits a scope into resource and action parts
func ParseScope(scope string) (resource, action string) {
	for i := 0; i < len(scope); i++ {
		if scope[i] == ':' {
			return scope[:i], scope[i+1:]
		}
	}
	return scope, ""
}

// ScopeValidationHelper provides custom validation for scope format
// This can be registered with validator.New() in init()
func ScopeValidationHelper(fl validator.FieldLevel) bool {
	if fl.Field().Interface() == nil {
		return false
	}
	scopes, ok := fl.Field().Interface().([]string)
	if !ok {
		return false
	}
	if len(scopes) == 0 {
		return false
	}
	for _, scope := range scopes {
		if !IsValidScope(scope) {
			return false
		}
	}
	return true
}
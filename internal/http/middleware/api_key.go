// Package middleware provides HTTP middleware components.
package middleware

import (
	"strings"

	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// APIKeyConfig holds configuration for API key middleware
type APIKeyConfig struct {
	// APIKeyService is the service for validating API keys
	APIKeyService *service.APIKeyService
	// HeaderName is the header key for the API key (default: "X-API-Key")
	HeaderName string
	// ContextKey is the key used to store user ID in context (default: "user_id")
	ContextKey string
	// Skipper is a function to skip middleware for certain routes
	Skipper func(c echo.Context) bool
}

// DefaultAPIKeyConfig returns the default API key middleware configuration
func DefaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{
		HeaderName:  "X-API-Key",
		ContextKey:  "user_id",
		Skipper:      nil,
	}
}

// APIKeyAuth returns an API key authentication middleware.
// It extracts the API key from X-API-Key header, validates it,
// and stores the user ID and scopes in the echo context.
// This middleware should be placed AFTER JWT middleware - it only
// runs if JWT authentication was not successful.
func APIKeyAuth(config APIKeyConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.HeaderName == "" {
		config.HeaderName = "X-API-Key"
	}
	if config.ContextKey == "" {
		config.ContextKey = "user_id"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip middleware if skipper function returns true
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Check if already authenticated via JWT
			// If user_id is already set in context, skip API key auth
			if c.Get(config.ContextKey) != nil {
				return next(c)
			}

			// Get API key from header
			apiKey := c.Request().Header.Get(config.HeaderName)
			if apiKey == "" {
				// No API key provided, skip to next middleware
				// This allows the request to continue without authentication
				// Permission middleware will handle unauthorized access
				return next(c)
			}

			// Validate the API key
			validatedKey, user, err := config.APIKeyService.Validate(c.Request().Context(), apiKey)
			if err != nil {
				// Return 401 for invalid API key
				if appErr := apperrors.GetAppError(err); appErr != nil {
					return echo.NewHTTPError(appErr.HTTPStatus, appErr.Message)
				}
				return echo.NewHTTPError(401, "Invalid API key")
			}

			// Store user ID in context (same as JWT middleware)
			c.Set(config.ContextKey, user.ID)

			// Store email in context for convenience
			c.Set("email", user.Email)

			// Store API key ID for audit logging
			c.Set("api_key_id", validatedKey.ID)

			// Store scopes for permission checks
			c.Set("api_key_scopes", validatedKey.GetScopes())

			// Store full API key for potential later use
			c.Set("api_key", validatedKey)

			return next(c)
		}
	}
}

// APIKeyAuthWithSkipper returns an API key middleware with a custom skipper function
func APIKeyAuthWithSkipper(apiKeyService *service.APIKeyService, skipper func(c echo.Context) bool) echo.MiddlewareFunc {
	config := DefaultAPIKeyConfig()
	config.APIKeyService = apiKeyService
	config.Skipper = skipper
	return APIKeyAuth(config)
}

// GetAPIKeyScopes retrieves API key scopes from the echo context
// Returns empty slice if no scopes are found (JWT auth or no auth)
func GetAPIKeyScopes(c echo.Context) []string {
	scopes, ok := c.Get("api_key_scopes").([]string)
	if !ok {
		return nil
	}
	return scopes
}

// GetAPIKeyID retrieves API key ID from the echo context
// Returns empty UUID if no API key was used
func GetAPIKeyID(c echo.Context) uuid.UUID {
	keyID, ok := c.Get("api_key_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return keyID
}

// IsAPIKeyAuth checks if the current request is authenticated via API key
func IsAPIKeyAuth(c echo.Context) bool {
	return c.Get("api_key_id") != nil
}

// APIKeyOrJWTAuth combines API key and JWT authentication.
// It first tries JWT, then falls back to API key if JWT fails.
// This is a convenience wrapper for the common pattern.
func APIKeyOrJWTAuth(jwtConfig JWTConfig, apiKeyConfig APIKeyConfig) echo.MiddlewareFunc {
	jwtMiddleware := JWT(jwtConfig)
	apiKeyMiddleware := APIKeyAuth(apiKeyConfig)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try JWT first
			if err := jwtMiddleware(next)(c); err != nil {
				// If JWT failed, check if there's an API key
				if c.Get(apiKeyConfig.ContextKey) == nil {
					// No JWT auth, try API key
					return apiKeyMiddleware(next)(c)
				}
				// JWT auth succeeded, continue
				return nil
			}
			return nil
		}
	}
}

// SkipAuthPaths is a helper function to create a skipper for public paths
func SkipAuthPaths(paths ...string) func(c echo.Context) bool {
	return func(c echo.Context) bool {
		path := c.Path()
		for _, p := range paths {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
		return false
	}
}

// SkipAuthMethods is a helper function to create a skipper for specific HTTP methods
func SkipAuthMethods(methods ...string) func(c echo.Context) bool {
	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[strings.ToUpper(m)] = true
	}
	return func(c echo.Context) bool {
		return methodSet[c.Request().Method]
	}
}
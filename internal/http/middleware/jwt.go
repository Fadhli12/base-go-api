// Package middleware provides HTTP middleware components.
package middleware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/example/go-api-base/internal/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// JWTConfig holds configuration for JWT middleware
type JWTConfig struct {
	// Secret is the JWT signing secret
	Secret string
	// ContextKey is the key used to store claims in context (default: "user")
	ContextKey string
	// Skipper is a function to skip middleware for certain routes
	Skipper func(c echo.Context) bool
}

// DefaultJWTConfig returns the default JWT middleware configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		ContextKey: "user",
		Skipper:     nil,
	}
}

// JWT returns a JWT authentication middleware
// It extracts the Bearer token from the Authorization header, validates it,
// and stores the claims in the echo context.
func JWT(config JWTConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip middleware if skipper function returns true
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Get Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(401, "Missing authorization header")
			}

			// Extract Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(401, "Invalid authorization header format")
			}

			tokenString := strings.TrimSpace(parts[1])
			if tokenString == "" {
				return echo.NewHTTPError(401, "Missing bearer token")
			}

			// Parse and validate token, extract claims
			claims, err := auth.GetClaims(tokenString, config.Secret)
			if err != nil {
				return echo.NewHTTPError(401, "Invalid or expired token")
			}

			// Store claims in context
			c.Set(config.ContextKey, claims)

			return next(c)
		}
	}
}

// JWTWithSkipper returns a JWT middleware with a custom skipper function
func JWTWithSkipper(secret string, skipper func(c echo.Context) bool) echo.MiddlewareFunc {
	config := DefaultJWTConfig()
	config.Secret = secret
	config.Skipper = skipper
	return JWT(config)
}

// GetClaims retrieves JWT claims from the echo context
func GetClaims(c echo.Context) *auth.Claims {
	claims, ok := c.Get("user").(*auth.Claims)
	if !ok {
		return nil
	}
	return claims
}

// Context errors
var (
	ErrNoUserInContext   = errors.New("no user found in context")
	ErrInvalidUserClaims = errors.New("invalid user claims in context")
)

// GetUserClaims extracts JWT claims from the echo context
// Returns the claims if found, or an error if claims are missing or invalid
func GetUserClaims(c echo.Context) (*auth.Claims, error) {
	val := c.Get("user")
	if val == nil {
		return nil, ErrNoUserInContext
	}

	claims, ok := val.(*auth.Claims)
	if !ok {
		return nil, ErrInvalidUserClaims
	}

	return claims, nil
}

// GetUserID extracts the user UUID from JWT claims in the context
// Returns the user ID if found, or an error if the user ID is missing or invalid
func GetUserID(c echo.Context) (uuid.UUID, error) {
	claims, err := GetUserClaims(c)
	if err != nil {
		return uuid.Nil, err
	}

	if claims.UserID == "" {
		return uuid.Nil, fmt.Errorf("user ID not found in claims")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user ID in claims: %w", err)
	}

	return userID, nil
}

// GetUserEmail extracts the user email from JWT claims in the context
// Returns the email if found, or an error if the email is missing
func GetUserEmail(c echo.Context) (string, error) {
	claims, err := GetUserClaims(c)
	if err != nil {
		return "", err
	}

	if claims.Email == "" {
		return "", fmt.Errorf("user email not found in claims")
	}

	return claims.Email, nil
}

// GetTokenID extracts the JWT token ID (jti) from the context claims.
// Returns the token ID and true if found, or uuid.Nil and false if not present.
func GetTokenID(c echo.Context) (uuid.UUID, bool) {
	claims, err := GetUserClaims(c)
	if err != nil {
		return uuid.Nil, false
	}
	if claims.ID == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(claims.ID)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}


// Package middleware provides HTTP middleware components.
package middleware

import (
	"net/http"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/permission"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// WebSocketAuth returns an Echo middleware that authenticates WebSocket connections
// by extracting a JWT token from the "token" query parameter and validating it.
// It also enforces permission checks via the Casbin enforcer.
func WebSocketAuth(secret string, enforcer *permission.Enforcer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenString := c.QueryParam("token")
			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing token")
			}

			claims, err := auth.GetClaims(tokenString, secret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid user ID")
			}

			orgDomain := "default"
			orgID, hasOrg := GetOrganizationID(c)
			if hasOrg {
				orgDomain = orgID.String()
				c.Set("ws_organization_id", orgID)
			}

			allowed, err := enforcer.Enforce(userID.String(), orgDomain, "websocket", "connect")
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "permission check failed")
			}
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "websocket access denied")
			}

			c.Set("user", claims)
			c.Set("ws_user_id", userID)

			return next(c)
		}
	}
}

// GetWSUserID retrieves the authenticated WebSocket user ID from the echo context.
// Returns uuid.Nil if the user ID is not found.
func GetWSUserID(c echo.Context) uuid.UUID {
	id, ok := c.Get("ws_user_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}

// GetWSOrgID retrieves the WebSocket organization ID from the echo context.
// Returns uuid.Nil if the organization ID is not found.
func GetWSOrgID(c echo.Context) uuid.UUID {
	id, ok := c.Get("ws_organization_id").(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}
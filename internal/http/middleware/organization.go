// Package middleware provides HTTP middleware components.
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/example/go-api-base/internal/http/response"
)

// OrganizationIDHeader is the header key for organization ID
const OrganizationIDHeader = "X-Organization-ID"

// ExtractOrganizationID extracts the organization ID from the X-Organization-ID header
// and stores it in the echo context for use by handlers.
//
// This middleware is backward compatible - if the header is missing or empty,
// it continues without error, allowing requests in global context (no organization).
//
// If the header is present, it validates the UUID format and returns an error
// response if the format is invalid.
func ExtractOrganizationID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get organization ID from header
			orgIDStr := c.Request().Header.Get(OrganizationIDHeader)
			if orgIDStr == "" {
				// Backward compatible - no organization context
				return next(c)
			}

			// Validate UUID format
			orgID, err := uuid.Parse(orgIDStr)
			if err != nil {
				return c.JSON(
					http.StatusBadRequest,
					response.ErrorWithContext(c, "INVALID_ORG_ID", "Invalid organization ID format"),
				)
			}

			// Store organization ID in context
			c.Set("organization_id", orgID)

			return next(c)
		}
	}
}

// GetOrganizationID retrieves the organization ID from the echo context.
// Returns the UUID and a boolean indicating if the organization ID was present.
//
// Usage:
//
//	orgID, ok := middleware.GetOrganizationID(c)
//	if !ok {
//	    // No organization context - global scope
//	}
func GetOrganizationID(c echo.Context) (uuid.UUID, bool) {
	orgID, ok := c.Get("organization_id").(uuid.UUID)
	return orgID, ok
}

// GetOrganizationIDOrGlobal retrieves the organization ID from context, or returns
// an empty UUID for global context.
//
// Usage:
//
//	orgID := middleware.GetOrganizationIDOrGlobal(c)
//	// orgID may be uuid.Nil if no organization context
func GetOrganizationIDOrGlobal(c echo.Context) uuid.UUID {
	orgID, ok := GetOrganizationID(c)
	if !ok {
		return uuid.Nil
	}
	return orgID
}
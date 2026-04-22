// Package middleware provides HTTP middleware components.
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AuditMiddlewareConfig holds configuration for the audit middleware
type AuditMiddlewareConfig struct {
	// Skipper is a function to skip middleware for certain routes (e.g., health checks)
	Skipper func(c echo.Context) bool
	// AuditService is the service for logging audit entries
	AuditService *service.AuditService
}

// DefaultAuditSkipper returns routes that should be skipped from auditing
func DefaultAuditSkipper() func(c echo.Context) bool {
	return func(c echo.Context) bool {
		path := c.Path()
		// Skip health checks and other non-audit routes
		skipPaths := []string{
			"/health",
			"/ready",
			"/metrics",
			"/api/v1/auth/login",
			"/api/v1/auth/refresh",
		}
		for _, skipPath := range skipPaths {
			if strings.HasPrefix(path, skipPath) {
				return true
			}
		}
		return false
	}
}

// responseWriter wraps http.ResponseWriter to capture response body
type responseWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

// Write captures response body for audit logging
func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Audit returns an audit logging middleware.
// It captures before/after states for mutating operations (POST, PUT, PATCH, DELETE)
// and logs them asynchronously to the audit service.
func Audit(config AuditMiddlewareConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultAuditSkipper()
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if configured to skip
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Only audit mutating operations
			method := c.Request().Method
			if method != http.MethodPost && method != http.MethodPut && 
			   method != http.MethodPatch && method != http.MethodDelete {
				return next(c)
			}

			// Capture request body for before state
			var beforeState interface{}
			if method == http.MethodPost {
				// For POST, the request body IS the before state (what's being created)
				bodyBytes, _ := io.ReadAll(c.Request().Body)
				c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				if len(bodyBytes) > 0 {
					_ = json.Unmarshal(bodyBytes, &beforeState)
				}
			}

			// Capture response body using a wrapper
			bodyBuffer := &bytes.Buffer{}
			origWriter := c.Response().Writer
			wrappedWriter := &responseWriter{
				ResponseWriter: origWriter,
				body:          bodyBuffer,
			}
			c.Response().Writer = wrappedWriter

			// Execute handler
			handlerErr := next(c)

			// Restore original writer
			c.Response().Writer = origWriter

			// Determine action based on HTTP method
			action := methodToAction(method)
			resource := extractResource(c.Path())
			resourceID := extractResourceID(c.Param("id"), c.Response().Status, bodyBuffer.Bytes())

			// Extract actor information from context
			actorID := extractActorID(c)
			ipAddress := c.RealIP()
			userAgent := c.Request().Header.Get("User-Agent")

			// Capture after state from response
			var afterState interface{}
			if bodyBuffer.Len() > 0 {
				// Try to parse the response
				var responseWrapper struct {
					Data interface{} `json:"data"`
				}
				if err := json.Unmarshal(bodyBuffer.Bytes(), &responseWrapper); err == nil {
					afterState = responseWrapper.Data
				} else {
					// If response wrapper parsing fails, try direct parse
					_ = json.Unmarshal(bodyBuffer.Bytes(), &afterState)
				}
			}

			// Map before/after based on action type
			// For POST: before=nil, after=response data
			// For PUT/PATCH: before=beforeState (request body), after=response data
			// For DELETE: before=nil, after=nil
			var before, after interface{}
			switch action {
			case domain.AuditActionCreate:
				before = nil // No state before creation
				after = afterState
			case domain.AuditActionUpdate:
				// For updates, the request body represents the changes
				before = beforeState
				after = afterState
			case domain.AuditActionDelete:
				before = nil
				after = nil
			default:
				before = beforeState
				after = afterState
			}

			// Log asynchronously (don't block on errors)
			if config.AuditService != nil {
				go func() {
					auditErr := config.AuditService.LogAction(
						c.Request().Context(),
						actorID,
						action,
						resource,
						resourceID,
						before,
						after,
						ipAddress,
						userAgent,
					)
					if auditErr != nil {
						slog.Warn("failed to create audit log",
							slog.String("error", auditErr.Error()),
							slog.String("path", c.Path()),
							slog.String("method", method),
						)
					}
				}()
			}

			return handlerErr
		}
	}
}

// methodToAction converts HTTP method to audit action
func methodToAction(method string) string {
	switch method {
	case http.MethodPost:
		return domain.AuditActionCreate
	case http.MethodPut, http.MethodPatch:
		return domain.AuditActionUpdate
	case http.MethodDelete:
		return domain.AuditActionDelete
	default:
		return strings.ToLower(method)
	}
}

// extractResource extracts the resource name from the path
// e.g., "/api/v1/users/:id" -> "user"
func extractResource(path string) string {
	// Remove API prefix and extract resource
	path = strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		resource := parts[0]
		// Convert plural to singular (simple heuristic)
		if strings.HasSuffix(resource, "s") && len(resource) > 1 {
			resource = resource[:len(resource)-1]
		}
		return resource
	}
	return "unknown"
}

// extractResourceID extracts the resource ID from path params or response
func extractResourceID(paramID string, statusCode int, responseBody []byte) string {
	// Try to get ID from path param first
	if paramID != "" {
		return paramID
	}

	// For successful POST requests, try to extract ID from response
	if statusCode >= 200 && statusCode < 300 && len(responseBody) > 0 {
		var response struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(responseBody, &response); err == nil && response.Data.ID != "" {
			return response.Data.ID
		}
	}

	return ""
}

// extractActorID extracts the actor ID from the echo context
func extractActorID(c echo.Context) uuid.UUID {
	claims, err := GetUserClaims(c)
	if err != nil || claims == nil {
		return uuid.Nil
	}

	if claims.UserID == "" {
		return uuid.Nil
	}

	actorID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil
	}

	return actorID
}

// AuditWithResource returns an audit middleware configured for a specific resource type.
// Use this when you need custom handling for specific resource types.
func AuditWithResource(auditService *service.AuditService, resource string, skipper func(c echo.Context) bool) echo.MiddlewareFunc {
	config := AuditMiddlewareConfig{
		Skipper:      skipper,
		AuditService: auditService,
	}
	
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		handler := Audit(config)(next)
		return func(c echo.Context) error {
			// Store custom resource in context
			c.Set("_audit_resource", resource)
			return handler(c)
		}
	}
}
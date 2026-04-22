// Package middleware provides HTTP middleware components.
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/example/go-api-base/internal/permission"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// PermissionConfig holds configuration for permission middleware.
type PermissionConfig struct {
	// Enforcer is the Casbin permission enforcer
	Enforcer *permission.Enforcer
	// Resource is the resource to check permission for (e.g., "users", "roles")
	Resource string
	// Action is the action to check permission for (e.g., "read", "write", "delete")
	Action string
	// DomainExtractor extracts the domain/tenant from the request (optional)
	DomainExtractor func(c echo.Context) string
	// DefaultDomain is used when no domain extractor is provided
	DefaultDomain string
	// Skipper is a function to skip middleware for certain routes
	Skipper func(c echo.Context) bool
}

// DefaultDomainExtractor returns "default" for all requests.
func DefaultDomainExtractor(c echo.Context) string {
	return "default"
}

// jsonErrorResponse sends a JSON error response.
func jsonErrorResponse(c echo.Context, statusCode int, code, message string) error {
	return c.JSON(statusCode, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
		"meta": map[string]string{
			"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Permission returns a middleware that checks if the user has permission to access a resource.
// It extracts the user ID from JWT claims, the domain from the request (or uses default),
// and checks permission via the Casbin enforcer with Redis cache.
func Permission(config PermissionConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.DomainExtractor == nil {
		config.DomainExtractor = DefaultDomainExtractor
	}
	if config.DefaultDomain == "" {
		config.DefaultDomain = "default"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip middleware if skipper function returns true
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Get user claims from context (set by JWT middleware)
			claims, err := GetUserClaims(c)
			if err != nil {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required")
			}

			// Extract domain from request
			domain := config.DomainExtractor(c)
			if domain == "" {
				domain = config.DefaultDomain
			}

			// Get user ID (subject)
			userID := claims.UserID
			if userID == "" {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User ID not found in claims")
			}

			// Check permission with cache
			ctx := c.Request().Context()
			allowed, err := config.Enforcer.EnforceWithCache(ctx, userID, domain, config.Resource, config.Action)
			if err != nil {
				return jsonErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check permission")
			}

			if !allowed {
				return jsonErrorResponse(c, http.StatusForbidden, "FORBIDDEN", "You don't have permission to access this resource")
			}

			return next(c)
		}
	}
}

// PermissionWithExtractor returns a middleware that checks permission using custom extractors.
// This allows more flexible permission checks where resource and action are extracted from the request.
func PermissionWithExtractor(
	enforcer *permission.Enforcer,
	resourceExtractor func(c echo.Context) string,
	actionExtractor func(c echo.Context) string,
	domainExtractor func(c echo.Context) string,
) echo.MiddlewareFunc {
	if domainExtractor == nil {
		domainExtractor = DefaultDomainExtractor
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user claims from context
			claims, err := GetUserClaims(c)
			if err != nil {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required")
			}

			// Extract resource, action, and domain from request
			resource := resourceExtractor(c)
			action := actionExtractor(c)
			domain := domainExtractor(c)
			if domain == "" {
				domain = "default"
			}

			// Get user ID
			userID := claims.UserID
			if userID == "" {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User ID not found in claims")
			}

			// Check permission
			ctx := c.Request().Context()
			allowed, err := enforcer.EnforceWithCache(ctx, userID, domain, resource, action)
			if err != nil {
				return jsonErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check permission")
			}

			if !allowed {
				return jsonErrorResponse(c, http.StatusForbidden, "FORBIDDEN", "You don't have permission to perform this action")
			}

			return next(c)
		}
	}
}

// TenantDomainExtractor extracts the domain/tenant from the X-Tenant-ID header.
func TenantDomainExtractor(c echo.Context) string {
	tenantID := c.Request().Header.Get("X-Tenant-ID")
	if tenantID == "" {
		return "default"
	}
	return tenantID
}

// RequirePermission is a helper function to create permission middleware for specific resource/action.
func RequirePermission(enforcer *permission.Enforcer, resource, action string) echo.MiddlewareFunc {
	return Permission(PermissionConfig{
		Enforcer:        enforcer,
		Resource:        resource,
		Action:          action,
		DomainExtractor: TenantDomainExtractor,
	})
}

// RequirePermissionWithDomain is a helper function to create permission middleware with custom domain extractor.
func RequirePermissionWithDomain(enforcer *permission.Enforcer, resource, action string, domainExtractor func(c echo.Context) string) echo.MiddlewareFunc {
	return Permission(PermissionConfig{
		Enforcer:        enforcer,
		Resource:        resource,
		Action:          action,
		DomainExtractor: domainExtractor,
	})
}

// RequireRole is a helper function to create permission middleware for role-based access.
// This checks if the user has a specific role in the given domain.
func RequireRole(enforcer *permission.Enforcer, role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user claims from context
			claims, err := GetUserClaims(c)
			if err != nil {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User authentication required")
			}

			// Extract domain from request
			domain := TenantDomainExtractor(c)

			// Get user ID
			userID := claims.UserID
			if userID == "" {
				return jsonErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User ID not found in claims")
			}

			// Check if user has role (ignoring context since not needed)
			hasRole, err := enforcer.HasRoleForUser(userID, role, domain)
			if err != nil {
				return jsonErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to check role")
			}

			if !hasRole {
				return jsonErrorResponse(c, http.StatusForbidden, "FORBIDDEN", "You don't have the required role")
			}

			return next(c)
		}
	}
}

// RequireAdmin is a convenience middleware that requires the admin role.
func RequireAdmin(enforcer *permission.Enforcer) echo.MiddlewareFunc {
	return RequireRole(enforcer, "admin")
}

// Context key for storing permission check results
type permissionCtxKey struct{}

// PermissionResultKey is the context key for permission check results.
var PermissionResultKey = permissionCtxKey{}

// PermissionResult stores the result of a permission check.
type PermissionResult struct {
	UserID   string
	Domain   string
	Resource string
	Action   string
	Allowed  bool
}

// SetPermissionResult stores the permission check result in the echo context.
func SetPermissionResult(c echo.Context, result PermissionResult) {
	c.Set("permission_result", result)
}

// GetPermissionResult retrieves the permission check result from the echo context.
func GetPermissionResult(c echo.Context) *PermissionResult {
	val := c.Get("permission_result")
	if val == nil {
		return nil
	}
	result, ok := val.(PermissionResult)
	if !ok {
		return nil
	}
	return &result
}

// GetUserIDFromContext extracts user ID from context for permission checks.
func GetUserIDFromContext(c echo.Context) (uuid.UUID, error) {
	userID, err := GetUserID(c)
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// WithPermissionContext wraps a context with permission information.
func WithPermissionContext(ctx context.Context, userID, domain, resource, action string, allowed bool) context.Context {
	return context.WithValue(ctx, PermissionResultKey, PermissionResult{
		UserID:   userID,
		Domain:   domain,
		Resource: resource,
		Action:   action,
		Allowed:  allowed,
	})
}
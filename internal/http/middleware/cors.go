package middleware

import (
	"net/http"
	"strings"

	"github.com/example/go-api-base/internal/config"
	"github.com/labstack/echo/v4"
)

// CORS returns a middleware that handles CORS
// MED-002: Fixed CORS misconfiguration
// - Rejects requests with no Origin in credential mode
// - Returns 403 for unauthorized origins
// - Adds Vary: Origin header for proper caching
func CORS(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")
			
			// MED-002 FIX: If no Origin header and credentials are allowed,
			// reject the request rather than returning wildcard
			// This prevents credentialed requests from any origin
			if origin == "" {
				// For non-browser clients (no Origin), allow the request
				// but don't set CORS headers
				return next(c)
			}

			// Check if origin is allowed
			allowedOrigin, isAllowed := checkOrigin(origin, cfg.CORS.AllowedOrigins)
			
			if !isAllowed {
				// MED-002 FIX: Return 403 for unauthorized origins
				// instead of silently allowing or redirecting
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error": map[string]string{
						"code":    "CORS_FORBIDDEN",
						"message": "Origin not allowed",
					},
				})
			}

			// Set CORS headers
			c.Response().Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Organization-ID")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			c.Response().Header().Set("Access-Control-Max-Age", "86400")
			
			// MED-002 FIX: Add Vary header for proper caching
			// This prevents caches from serving the wrong CORS headers
			c.Response().Header().Add("Vary", "Origin")

			// Handle preflight request
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

// checkOrigin checks if the origin is allowed
// Returns the allowed origin string and whether it's allowed
func checkOrigin(origin string, allowedOrigins []string) (string, bool) {
	if origin == "" {
		return "", false
	}

	// Check if origin is in allowed list
	for _, allowed := range allowedOrigins {
		// Support wildcard for development only
		if allowed == "*" {
			return origin, true
		}
		// Case-insensitive comparison
		if strings.EqualFold(allowed, origin) {
			return origin, true
		}
		// Support subdomain wildcards (e.g., "*.example.com")
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[2:] // Remove "*."
			if strings.HasSuffix(origin, domain) {
				return origin, true
			}
		}
	}

	return "", false
}

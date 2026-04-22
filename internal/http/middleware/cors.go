package middleware

import (
	"net/http"
	"strings"

	"github.com/example/go-api-base/internal/config"
	"github.com/labstack/echo/v4"
)

// CORS returns a middleware that handles CORS
func CORS(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")
			allowedOrigin := getAllowedOrigin(origin, cfg.CORS.AllowedOrigins)

			// Set CORS headers
			c.Response().Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			c.Response().Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight request
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

// getAllowedOrigin checks if the origin is allowed and returns the appropriate origin value
func getAllowedOrigin(origin string, allowedOrigins []string) string {
	if origin == "" {
		return "*"
	}

	// Check if origin is in allowed list
	for _, allowed := range allowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return origin
		}
	}

	// If origin is not allowed, return the first allowed origin
	// This prevents exposing origins that aren't explicitly allowed
	if len(allowedOrigins) > 0 {
		return allowedOrigins[0]
	}

	return "*"
}

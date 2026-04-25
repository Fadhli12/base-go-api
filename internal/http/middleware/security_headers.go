package middleware

import (
	"github.com/labstack/echo/v4"
)

// SecurityHeadersConfig holds configuration for security headers middleware
type SecurityHeadersConfig struct {
	// X-Frame-Options header value (default: "DENY")
	// Prevents clickjacking attacks
	XFrameOptions string
	
	// X-Content-Type-Options header value (default: "nosniff")
	// Prevents MIME type sniffing
	XContentTypeOptions string
	
	// X-XSS-Protection header value (default: "1; mode=block")
	// Enables browser XSS filter
	XXSSProtection string
	
	// Strict-Transport-Security header value
	// Enables HSTS (should be configured for production)
	// Example: "max-age=31536000; includeSubDomains"
	HSTS string
	
	// Content-Security-Policy header value
	// Example: "default-src 'self'; script-src 'self' 'unsafe-inline'"
	CSP string
	
	// Referrer-Policy header value (default: "strict-origin-when-cross-origin")
	// Controls referrer information
	ReferrerPolicy string
	
	// Permissions-Policy header value
	// Example: "geolocation=(), microphone=()"
	PermissionsPolicy string
}

// DefaultSecurityHeadersConfig returns sensible defaults for security headers
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		XFrameOptions:       "DENY",
		XContentTypeOptions: "nosniff",
		XXSSProtection:      "1; mode=block",
		// HSTS should be configured with actual values in production
		HSTS: "",
		// CSP should be configured based on application needs
		CSP: "",
		ReferrerPolicy:   "strict-origin-when-cross-origin",
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders returns middleware that adds security headers to all responses
// MED-003: Addresses missing security headers vulnerability
func SecurityHeaders(config SecurityHeadersConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// X-Frame-Options: Prevents clickjacking
			// DENY: Completely blocks framing
			// SAMEORIGIN: Allows framing from same origin
			// ALLOW-FROM uri: Allows framing from specific origin (deprecated)
			if config.XFrameOptions != "" {
				c.Response().Header().Set("X-Frame-Options", config.XFrameOptions)
			}
			
			// X-Content-Type-Options: Prevents MIME type sniffing
			// nosniff: Browser will not MIME-sniff the content-type
			if config.XContentTypeOptions != "" {
				c.Response().Header().Set("X-Content-Type-Options", config.XContentTypeOptions)
			}
			
			// X-XSS-Protection: Enables browser XSS filter
			// 1; mode=block: Enable protection and block page on detection
			if config.XXSSProtection != "" {
				c.Response().Header().Set("X-XSS-Protection", config.XXSSProtection)
			}
			
			// Strict-Transport-Security: Enforces HTTPS connections
			// Should be configured in production with appropriate max-age
			if config.HSTS != "" {
				c.Response().Header().Set("Strict-Transport-Security", config.HSTS)
			}
			
			// Content-Security-Policy: Prevents XSS, clickjacking, and other injection attacks
			// Should be carefully configured based on application needs
			if config.CSP != "" {
				c.Response().Header().Set("Content-Security-Policy", config.CSP)
			}
			
			// Referrer-Policy: Controls referrer information
			// strict-origin-when-cross-origin: Recommended default
			if config.ReferrerPolicy != "" {
				c.Response().Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}
			
			// Permissions-Policy: Controls browser features
			// Disables potentially sensitive features by default
			if config.PermissionsPolicy != "" {
				c.Response().Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}
			
			return next(c)
		}
	}
}

// SecurityHeadersWithDefaults returns security headers middleware with sensible defaults
// Suitable for development and as a baseline for production
func SecurityHeadersWithDefaults() echo.MiddlewareFunc {
	return SecurityHeaders(DefaultSecurityHeadersConfig())
}

// ProductionSecurityHeaders returns security headers middleware configured for production
// Includes HSTS and recommended CSP settings
func ProductionSecurityHeaders(domain string) echo.MiddlewareFunc {
	config := SecurityHeadersConfig{
		XFrameOptions:        "DENY",
		XContentTypeOptions: "nosniff",
		XXSSProtection:       "1; mode=block",
		HSTS:                "max-age=31536000; includeSubDomains",
		CSP:                 "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'",
		ReferrerPolicy:      "strict-origin-when-cross-origin",
		PermissionsPolicy:   "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()",
	}
	return SecurityHeaders(config)
}
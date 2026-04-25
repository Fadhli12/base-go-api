// Package middleware provides HTTP middleware components.
package middleware

import (
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/logger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// LoggerContextKey is the key used to store logger in echo context
const LoggerContextKey = "logger"

// TraceIDHeader is the HTTP header for trace ID
const TraceIDHeader = "X-Trace-ID"

// LoggingMiddlewareConfig holds configuration for structured logging middleware
type LoggingMiddlewareConfig struct {
	// Logger is the logger instance to use
	Logger logger.Logger
	// Skipper is a function to skip middleware for certain routes
	Skipper func(c echo.Context) bool
	// LogRequestBody determines if request body should be logged (disabled by default for security)
	LogRequestBody bool
	// LogResponseBody determines if response body should be logged (disabled by default for security)
	LogResponseBody bool
	// BodyMaxSize is the maximum body size to log (default: 1KB)
	BodyMaxSize int
}

// DefaultLoggingSkipper returns a skipper function that skips health check and metrics endpoints
func DefaultLoggingSkipper() func(echo.Context) bool {
	return func(c echo.Context) bool {
		path := c.Request().URL.Path
		switch path {
		case "/healthz", "/readyz", "/metrics":
			return true
		default:
			return false
		}
	}
}

// DefaultLoggingMiddlewareConfig returns a default configuration for the logging middleware
func DefaultLoggingMiddlewareConfig(log logger.Logger) LoggingMiddlewareConfig {
	if log == nil {
		log = logger.NewNopLogger()
	}
	return LoggingMiddlewareConfig{
		Logger:          log,
		Skipper:         DefaultLoggingSkipper(),
		LogRequestBody:  false,
		LogResponseBody: false,
		BodyMaxSize:     1024,
	}
}

// StructuredLogging returns a middleware that provides structured logging for HTTP requests
// It extracts request context (request_id, user_id, org_id, trace_id) and injects a logger
// into the echo context for use by handlers.
//
// The middleware logs:
//   - Request start (DEBUG level) with method, path, query params
//   - Request completion (INFO level) with status, duration, response size
//   - Errors (ERROR level) with error details and stack trace
//
// Usage:
//
//	e.Use(middleware.StructuredLogging(middleware.DefaultLoggingMiddlewareConfig(logger)))
//
//	// In handler:
//	log := middleware.GetLogger(c)
//	log.Info(c.Request().Context(), "processing request")
func StructuredLogging(config LoggingMiddlewareConfig) echo.MiddlewareFunc {
	// Use default config if logger is nil
	if config.Logger == nil {
		config.Logger = logger.NewNopLogger()
	}

	// Use default skipper if not provided
	if config.Skipper == nil {
		config.Skipper = DefaultLoggingSkipper()
	}

	// Set default body max size
	if config.BodyMaxSize <= 0 {
		config.BodyMaxSize = 1024
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if skipper returns true
			if config.Skipper(c) {
				return next(c)
			}

			// Start timing
			start := time.Now()

			// Extract request context
			requestID := extractRequestID(c)
			userID := extractUserID(c)
			orgID := extractOrgID(c)
			traceID := extractTraceID(c)

			// Create logger with request context fields
			requestLog := config.Logger.WithFields(
				config.Logger.String("request_id", requestID),
				config.Logger.String("trace_id", traceID),
			)

			// Add user_id if present
			if userID != "" {
				requestLog = requestLog.WithFields(
					config.Logger.String("user_id", userID),
				)
			}

			// Add org_id if present
			if orgID != "" {
				requestLog = requestLog.WithFields(
					config.Logger.String("org_id", orgID),
				)
			}

			// Inject logger into echo context
			c.Set(LoggerContextKey, requestLog)

			// Log request start
			method := c.Request().Method
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery
			remoteAddr := c.RealIP()

			startFields := []logger.Field{
				config.Logger.String("method", method),
				config.Logger.String("path", path),
				config.Logger.String("remote_addr", remoteAddr),
			}

			if query != "" {
				startFields = append(startFields, config.Logger.String("query", query))
			}

			requestLog.Debug(c.Request().Context(), "request started", startFields...)

			// Process request and capture any error
			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Get status code
			status := c.Response().Status
			if err != nil {
				// Handle echo.HTTPError
				if httpErr, ok := err.(*echo.HTTPError); ok {
					status = httpErr.Code
				} else {
					status = 500
				}
			}

			// Build completion log fields
			completeFields := []logger.Field{
				config.Logger.String("method", method),
				config.Logger.String("path", path),
				config.Logger.Int("status", status),
				config.Logger.Duration("duration", duration),
				config.Logger.Int64("response_size", c.Response().Size),
			}

			// Log based on status code
			switch {
			case status >= 500:
				// Server errors - log at ERROR level
				if err != nil {
					requestLog.WithError(err).Error(c.Request().Context(), "request failed with server error", completeFields...)
				} else {
					requestLog.Error(c.Request().Context(), "request completed with server error", completeFields...)
				}
			case status >= 400:
				// Client errors - log at WARN level
				requestLog.Warn(c.Request().Context(), "request completed with client error", completeFields...)
			default:
				// Success - log at INFO level
				requestLog.Info(c.Request().Context(), "request completed", completeFields...)
			}

			return err
		}
	}
}

// GetLogger retrieves the logger from the echo context
// Returns a NopLogger if no logger is found in context
//
// Usage:
//
//	func (h *Handler) GetUser(c echo.Context) error {
//	    log := middleware.GetLogger(c)
//	    log.Info(c.Request().Context(), "fetching user")
//	    // ...
//	}
func GetLogger(c echo.Context) logger.Logger {
	if c == nil {
		return logger.NewNopLogger()
	}

	log, ok := c.Get(LoggerContextKey).(logger.Logger)
	if !ok || log == nil {
		return logger.NewNopLogger()
	}

	return log
}

// SetLogger sets the logger in the echo context
// Useful for creating scoped loggers in handlers
//
// Usage:
//
//	func (h *Handler) ProcessOrder(c echo.Context) error {
//	    log := middleware.GetLogger(c)
//	    scopedLog := log.WithFields(log.String("order_id", orderID))
//	    middleware.SetLogger(c, scopedLog)
//	    // Subsequent calls to GetLogger will return the scoped logger
//	}
func SetLogger(c echo.Context, log logger.Logger) {
	if c == nil || log == nil {
		return
	}
	c.Set(LoggerContextKey, log)
}

// extractRequestID extracts request ID from echo context or generates a new one
func extractRequestID(c echo.Context) string {
	requestID := GetRequestID(c)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	return requestID
}

// extractUserID extracts user ID from JWT claims in echo context
func extractUserID(c echo.Context) string {
	// Try to get claims from context (set by JWT middleware)
	val := c.Get("user")
	if val == nil {
		return ""
	}

	// Type assert to auth.Claims
	claims, ok := val.(*auth.Claims)
	if !ok || claims == nil {
		return ""
	}

	return claims.UserID
}

// extractOrgID extracts organization ID from echo context
func extractOrgID(c echo.Context) string {
	orgID, ok := GetOrganizationID(c)
	if !ok {
		return ""
	}
	return orgID.String()
}

// extractTraceID extracts trace ID from header or generates a new one
func extractTraceID(c echo.Context) string {
	traceID := c.Request().Header.Get(TraceIDHeader)
	if traceID == "" {
		traceID = uuid.New().String()
	}
	return traceID
}

// recoverFromPanic recovers from panics and logs them
// This should be used in combination with the Recover middleware
func recoverFromPanic(c echo.Context, log logger.Logger) {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("panic: %v", r)
		}

		log.Error(c.Request().Context(), "panic recovered",
			log.String("error", err.Error()),
		)

		// Re-panic so Recover middleware can handle it
		panic(r)
	}
}

package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// Logging returns a middleware that logs HTTP requests and responses.
// It logs request start/end with method, path, status, duration, and request ID.
// Health check endpoints (/healthz, /readyz) are skipped to reduce log noise.
func Logging() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip logging for health check endpoints
			path := c.Request().URL.Path
			if path == "/healthz" || path == "/readyz" {
				return next(c)
			}

			requestID := GetRequestID(c)
			method := c.Request().Method
			start := time.Now()

			// Log request start
			slog.Debug("request started",
				slog.String("request_id", requestID),
				slog.String("method", method),
				slog.String("path", path),
				slog.String("remote_addr", c.RealIP()),
			)

			// Process request
			err := next(c)

			// Calculate duration
			duration := time.Since(start)
			durationMs := duration.Milliseconds()

			// Get status code
			status := c.Response().Status
			if err != nil {
				if httpErr, ok := err.(*echo.HTTPError); ok {
					status = httpErr.Code
				} else {
					status = 500
				}
			}

			// Log request completed
			slog.Info("request completed",
				slog.String("request_id", requestID),
				slog.String("method", method),
				slog.String("path", path),
				slog.Int("status", status),
				slog.Int64("duration_ms", durationMs),
				slog.Int64("response_size", c.Response().Size),
			)

			return err
		}
	}
}
package logger

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// EchoMiddleware creates an Echo middleware for request/response logging
func EchoMiddleware(log *Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			// Get or generate request ID
			requestID := req.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = GetRequestID(req.Context())
			}
			res.Header().Set("X-Request-ID", requestID)

			// Create context with request ID and other tracking info
			ctx := WithRequestID(req.Context(), requestID)
			ctx = WithStartTime(ctx, time.Now())
			c.SetRequest(req.WithContext(ctx))

			// Extract user_id and org_id from claims if available (optional)
			// This assumes claims are available in context after auth middleware
			if user, ok := c.Get("user").(interface{ GetUserID() string }); ok {
				userIDStr := user.GetUserID()
				if userID, err := uuid.Parse(userIDStr); err == nil {
					ctx = WithUserID(ctx, userID)
					c.SetRequest(req.WithContext(ctx))
				}
			}
			if org, ok := c.Get("org").(interface{ GetOrgID() string }); ok {
				orgIDStr := org.GetOrgID()
				if orgID, err := uuid.Parse(orgIDStr); err == nil {
					ctx = WithOrgID(ctx, orgID)
					c.SetRequest(req.WithContext(ctx))
				}
			}

			// Process request
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			// Build log attributes
			status := res.Status
			method := req.Method
			path := req.URL.Path
			query := req.URL.RawQuery

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", method),
				slog.String("path", path),
				slog.Int("status", status),
				slog.Int64("duration_ms", duration.Milliseconds()),
			}

			if query != "" {
				attrs = append(attrs, slog.String("query", query))
			}

			userID := GetUserID(ctx)
			if userID != "" {
				attrs = append(attrs, slog.String("user_id", userID))
			}

			orgID := GetOrgID(ctx)
			if orgID != "" {
				attrs = append(attrs, slog.String("org_id", orgID))
			}

			// Log based on status - convert attrs to any
			args := make([]any, len(attrs))
			for i, attr := range attrs {
				args[i] = attr
			}

			if err != nil {
				args = append(args, slog.String("error", err.Error()))
				log.Error("http_request_failed", args...)
			} else if status >= 400 {
				log.Warn("http_request_warning", args...)
			} else {
				log.Info("http_request_completed", args...)
			}

			return err
		}
	}
}

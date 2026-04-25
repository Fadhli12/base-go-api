package logger

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions with
// other packages using the same string values.
type contextKey string

// Context keys for logger and request-scoped fields.
// Keys are private to prevent external packages from directly modifying context.
const (
	// LoggerContextKey stores the Logger instance in context.
	LoggerContextKey contextKey = "logger"

	// RequestIDContextKey stores the request ID for tracing.
	RequestIDContextKey contextKey = "request_id"

	// UserIDContextKey stores the authenticated user ID.
	UserIDContextKey contextKey = "user_id"

	// OrgIDContextKey stores the organization ID for multi-tenant contexts.
	OrgIDContextKey contextKey = "org_id"

	// TraceIDContextKey stores the distributed trace ID.
	TraceIDContextKey contextKey = "trace_id"

	// StartTimeContextKey stores the request start time for duration calculation.
	StartTimeContextKey contextKey = "start_time"
)

// Header constants for HTTP header names.
const (
	// RequestIDHeader is the HTTP header for request ID.
	RequestIDHeader = "X-Request-ID"

	// TraceIDHeader is the HTTP header for distributed trace ID.
	TraceIDHeader = "X-Trace-ID"

	// OrgIDHeader is the HTTP header for organization ID.
	OrgIDHeader = "X-Organization-ID"
)

// WithLogger injects a Logger into the context.
// Returns a new context with the logger stored; original context is unchanged.
//
// Usage:
//
//	ctx = logger.WithLogger(ctx, myLogger)
//	l := logger.FromContext(ctx) // Returns myLogger
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, LoggerContextKey, logger)
}

// FromContext extracts a Logger from the context.
// Never returns nil; returns NopLogger if no logger is found in context.
//
// Usage:
//
//	logger := logger.FromContext(ctx)
//	logger.Info(ctx, "message")
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return NewNopLogger()
	}

	if logger, ok := ctx.Value(LoggerContextKey).(Logger); ok && logger != nil {
		return logger
	}

	return NewNopLogger()
}

// WithRequestID stores a request ID in the context.
// Returns a new context with the request ID stored.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, requestID)
}

// GetRequestID extracts the request ID from the context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return id
	}

	return ""
}

// WithUserID stores a user ID in the context.
// Returns a new context with the user ID stored.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDContextKey, userID)
}

// GetUserID extracts the user ID from the context.
// Returns empty string if not found.
func GetUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(UserIDContextKey).(string); ok {
		return id
	}

	return ""
}

// WithOrgID stores an organization ID in the context.
// Returns a new context with the org ID stored.
func WithOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, OrgIDContextKey, orgID)
}

// GetOrgID extracts the organization ID from the context.
// Returns empty string if not found.
func GetOrgID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(OrgIDContextKey).(string); ok {
		return id
	}

	return ""
}

// WithTraceID stores a trace ID in the context.
// Returns a new context with the trace ID stored.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, traceID)
}

// GetTraceID extracts the trace ID from the context.
// Returns empty string if not found.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(TraceIDContextKey).(string); ok {
		return id
	}

	return ""
}

// WithStartTime stores the request start time in the context.
// Returns a new context with the start time stored.
func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, StartTimeContextKey, startTime)
}

// GetStartTime extracts the start time from the context.
// Returns zero time if not found.
func GetStartTime(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Time{}
	}

	if t, ok := ctx.Value(StartTimeContextKey).(time.Time); ok {
		return t
	}

	return time.Time{}
}

// ExtractContextFields gathers all context fields into a slice of Field.
// Returns an empty slice (never nil) with all set context values.
//
// Fields are returned in order: request_id, user_id, org_id, trace_id
func ExtractContextFields(ctx context.Context) []Field {
	if ctx == nil {
		return []Field{}
	}

	fields := make([]Field, 0, 4)

	if requestID := GetRequestID(ctx); requestID != "" {
		fields = append(fields, String("request_id", requestID))
	}

	if userID := GetUserID(ctx); userID != "" {
		fields = append(fields, String("user_id", userID))
	}

	if orgID := GetOrgID(ctx); orgID != "" {
		fields = append(fields, String("org_id", orgID))
	}

	if traceID := GetTraceID(ctx); traceID != "" {
		fields = append(fields, String("trace_id", traceID))
	}

	return fields
}

// GenerateRequestID generates a new unique request ID using UUID v4.
// Returns a string in standard UUID format.
func GenerateRequestID() string {
	return uuid.New().String()
}

// GenerateTraceID generates a new unique trace ID using UUID v4.
// Returns a string in standard UUID format.
func GenerateTraceID() string {
	return uuid.New().String()
}

// IsNopLogger returns true if the logger is a no-op logger.
// Used internally to check if FromContext returned a default NopLogger.
func IsNopLogger(logger Logger) bool {
	_, ok := logger.(*nopLogger)
	return ok
}

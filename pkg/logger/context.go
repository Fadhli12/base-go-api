package logger

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey  contextKey = "request_id"
	userIDKey     contextKey = "user_id"
	orgIDKey      contextKey = "org_id"
	startTimeKey  contextKey = "request_start"
)

// WithRequestID stores request_id in context, generates one if empty
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// GetRequestID retrieves request_id from context, generates one if not found
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		return id
	}
	return uuid.New().String()
}

// WithUserID stores user_id in context
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id.String())
}

// GetUserID retrieves user_id from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(userIDKey).(string); ok {
		return id
	}
	return ""
}

// WithOrgID stores org_id in context
func WithOrgID(ctx context.Context, id uuid.UUID) context.Context {
	if id == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, orgIDKey, id.String())
}

// GetOrgID retrieves org_id from context
func GetOrgID(ctx context.Context) string {
	if id, ok := ctx.Value(orgIDKey).(string); ok {
		return id
	}
	return ""
}

// WithStartTime stores request start time in context
func WithStartTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startTimeKey, t)
}

// GetStartTime retrieves request start time from context
func GetStartTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(startTimeKey).(time.Time); ok {
		return t
	}
	return time.Now()
}

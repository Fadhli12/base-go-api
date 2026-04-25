package logger

import (
	"context"
	"time"
)

// nopLogger is a no-op implementation of Logger that discards all output.
// Used as a safe default when no logger is configured.
type nopLogger struct{}

// NewNopLogger returns a no-op logger that discards all output.
// Safe to use when logging is not configured or for testing.
func NewNopLogger() Logger {
	return &nopLogger{}
}

// Debug is a no-op.
func (n *nopLogger) Debug(ctx context.Context, msg string, fields ...Field) {}

// Info is a no-op.
func (n *nopLogger) Info(ctx context.Context, msg string, fields ...Field) {}

// Warn is a no-op.
func (n *nopLogger) Warn(ctx context.Context, msg string, fields ...Field) {}

// Error is a no-op.
func (n *nopLogger) Error(ctx context.Context, msg string, fields ...Field) {}

// WithFields returns the same nopLogger (immutable no-op).
func (n *nopLogger) WithFields(fields ...Field) Logger {
	return n
}

// WithError returns the same nopLogger (immutable no-op).
func (n *nopLogger) WithError(err error) Logger {
	return n
}

// String returns an empty Field (no-op factory method).
func (n *nopLogger) String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int returns an empty Field (no-op factory method).
func (n *nopLogger) Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 returns an empty Field (no-op factory method).
func (n *nopLogger) Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 returns an empty Field (no-op factory method).
func (n *nopLogger) Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool returns an empty Field (no-op factory method).
func (n *nopLogger) Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration returns an empty Field (no-op factory method).
func (n *nopLogger) Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time returns an empty Field (no-op factory method).
func (n *nopLogger) Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Any returns an empty Field (no-op factory method).
func (n *nopLogger) Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

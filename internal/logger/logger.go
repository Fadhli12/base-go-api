// Package logger provides a structured logging abstraction layer over log/slog.
//
// The logger package offers:
//   - Logger interface for swappable implementations
//   - Structured fields with type-safe constructors
//   - Context propagation for request-scoped fields
//   - Multiple output writers (stdout, file, syslog)
//   - Level filtering and routing
//
// Basic Usage:
//
//	logger := logger.FromContext(ctx)
//	logger.Info(ctx, "request processed",
//	    logger.String("method", "GET"),
//	    logger.Duration("elapsed", time.Since(start)),
//	)
//
// Field Chaining:
//
//	scoped := logger.WithFields(
//	    logger.String("user_id", userID),
//	    logger.String("org_id", orgID),
//	)
//	scoped.Info(ctx, "operation completed")
//	scoped.Error(ctx, "operation failed", logger.Err(err))
//
// Error Handling:
//
//	logger.WithError(err).Error(ctx, "database query failed",
//	    logger.String("query", "SELECT ..."),
//	)
package logger

import (
	"context"
	"fmt"
	"time"
)

// Level represents log severity levels.
type Level int

const (
	// LevelDebug is for development-only verbose logging.
	// May impact performance; use sparingly in production.
	LevelDebug Level = iota

	// LevelInfo is for significant state changes and events.
	// Default level for production.
	LevelInfo

	// LevelWarn is for degraded conditions or suspicious behavior.
	// Requests succeed but with caveats.
	LevelWarn

	// LevelError is for failures affecting request processing.
	// Always logged regardless of level filter.
	LevelError
)

// String returns the string representation of the level.
// Returns uppercase: DEBUG, INFO, WARN, ERROR
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("Level(%d)", l)
	}
}

// ParseLevel converts a string level to Level.
// Case-insensitive. Returns LevelInfo for unknown values.
//
// Supported values:
//   - "debug", "DEBUG" -> LevelDebug
//   - "info", "INFO" -> LevelInfo (default for unknown)
//   - "warn", "WARN", "warning", "WARNING" -> LevelWarn
//   - "error", "ERROR" -> LevelError
func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger is the primary interface for structured logging.
//
// CONTRACT:
//   - All methods accept context.Context as first parameter
//   - Field methods return Field by value (not pointer) for stack allocation
//   - WithFields returns a NEW logger instance (immutable)
//   - WithError returns a NEW logger with error field attached
//   - Implementations must be goroutine-safe
//   - Implementations must never panic on nil context (use background context)
//   - Implementations must handle missing context fields gracefully
//
// Usage:
//
//	logger := logger.FromContext(ctx)
//	logger.Info(ctx, "request processed",
//	    logger.String("method", "GET"),
//	    logger.Duration("duration", elapsed),
//	)
type Logger interface {
	// Debug logs a message at DEBUG level.
	// Messages at this level are development-only and may be expensive.
	// CONTRACT: No-op if level > DEBUG
	Debug(ctx context.Context, msg string, fields ...Field)

	// Info logs a message at INFO level.
	// Use for state changes and significant events.
	// CONTRACT: No-op if level > INFO
	Info(ctx context.Context, msg string, fields ...Field)

	// Warn logs a message at WARN level.
	// Use for degraded conditions or suspicious behavior.
	// CONTRACT: No-op if level > WARN
	Warn(ctx context.Context, msg string, fields ...Field)

	// Error logs a message at ERROR level.
	// Use for failures that affect request processing.
	// CONTRACT: Always logs (unless level > ERROR)
	Error(ctx context.Context, msg string, fields ...Field)

	// WithFields returns a new Logger with additional fields.
	// Fields are included in all subsequent log calls.
	// CONTRACT: Returns new instance, original logger unchanged
	WithFields(fields ...Field) Logger

	// WithError returns a new Logger with error context.
	// Error is added to fields with key "error".
	// CONTRACT: Returns new instance, original logger unchanged
	WithError(err error) Logger

	// Field factory methods
	// CONTRACT: All return Field by value (not pointer)
	String(key, value string) Field
	Int(key string, value int) Field
	Int64(key string, value int64) Field
	Float64(key string, value float64) Field
	Bool(key string, value bool) Field
	Duration(key string, value time.Duration) Field
	Time(key string, value time.Time) Field
	Any(key string, value interface{}) Field
}

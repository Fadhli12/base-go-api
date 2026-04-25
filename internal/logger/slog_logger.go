package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// SlogLogger wraps slog.Handler to implement the Logger interface.
// Provides structured logging with context propagation and level filtering.
//
// Thread-safety is provided by the underlying slog.Handler which is goroutine-safe.
type SlogLogger struct {
	handler slog.Handler
	level   Level
	fields  []Field
}

// NewSlogLogger creates a new SlogLogger with the given handler and minimum level.
// The handler must be initialized with the desired output destination.
//
// Usage:
//
//	handler := slog.NewJSONHandler(os.Stdout, nil)
//	logger := logger.NewSlogLogger(handler, logger.LevelInfo)
//	logger.Info(ctx, "application started")
func NewSlogLogger(handler slog.Handler, level Level) *SlogLogger {
	return &SlogLogger{
		handler: handler,
		level:   level,
		fields:  make([]Field, 0),
	}
}

// NewDefaultSlogLogger creates a SlogLogger with sensible defaults.
// Outputs JSON to stdout at INFO level.
func NewDefaultSlogLogger() *SlogLogger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return NewSlogLogger(handler, LevelInfo)
}

// Debug logs a message at DEBUG level.
// No-op if logger level > DEBUG.
func (l *SlogLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	if l.level > LevelDebug {
		return
	}
	l.log(ctx, slog.LevelDebug, msg, fields)
}

// Info logs a message at INFO level.
// No-op if logger level > INFO.
func (l *SlogLogger) Info(ctx context.Context, msg string, fields ...Field) {
	if l.level > LevelInfo {
		return
	}
	l.log(ctx, slog.LevelInfo, msg, fields)
}

// Warn logs a message at WARN level.
// No-op if logger level > WARN.
func (l *SlogLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	if l.level > LevelWarn {
		return
	}
	l.log(ctx, slog.LevelWarn, msg, fields)
}

// Error logs a message at ERROR level.
// Always logs unless level > ERROR (which is never the case for standard levels).
func (l *SlogLogger) Error(ctx context.Context, msg string, fields ...Field) {
	if l.level > LevelError {
		return
	}
	l.log(ctx, slog.LevelError, msg, fields)
}

// log is the internal logging implementation.
func (l *SlogLogger) log(ctx context.Context, level slog.Level, msg string, fields []Field) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Merge pre-attached fields with new fields
	allFields := l.fields
	if len(fields) > 0 {
		allFields = mergeFields(l.fields, fields)
	}

	// Extract context fields (request_id, user_id, org_id, trace_id)
	ctxFields := ExtractContextFields(ctx)
	if len(ctxFields) > 0 {
		allFields = mergeFields(ctxFields, allFields)
	}

	// Convert fields to slog.Attr
	attrs := fieldsToSlogAttrs(allFields)

	// Create the record
	record := slog.Record{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
	}

	// Add attributes
	for _, attr := range attrs {
		record.AddAttrs(attr)
	}

	// Handle the record
	_ = l.handler.Handle(ctx, record)
}

// WithFields returns a new Logger with additional fields.
// The original logger is unchanged (immutable pattern).
func (l *SlogLogger) WithFields(fields ...Field) Logger {
	if len(fields) == 0 {
		return l
	}

	return &SlogLogger{
		handler: l.handler,
		level:   l.level,
		fields:  mergeFields(l.fields, fields),
	}
}

// WithError returns a new Logger with an error field attached.
// The error is stored with key "error".
func (l *SlogLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}

	return l.WithFields(Field{Key: "error", Value: err})
}

// String creates a string field.
func (l *SlogLogger) String(key, value string) Field {
	return String(key, value)
}

// Int creates an int field.
func (l *SlogLogger) Int(key string, value int) Field {
	return Int(key, value)
}

// Int64 creates an int64 field.
func (l *SlogLogger) Int64(key string, value int64) Field {
	return Int64(key, value)
}

// Float64 creates a float64 field.
func (l *SlogLogger) Float64(key string, value float64) Field {
	return Float64(key, value)
}

// Bool creates a bool field.
func (l *SlogLogger) Bool(key string, value bool) Field {
	return Bool(key, value)
}

// Duration creates a time.Duration field.
func (l *SlogLogger) Duration(key string, value time.Duration) Field {
	return Duration(key, value)
}

// Time creates a time.Time field.
func (l *SlogLogger) Time(key string, value time.Time) Field {
	return Time(key, value)
}

// Any creates a field with any value.
func (l *SlogLogger) Any(key string, value interface{}) Field {
	return Any(key, value)
}

// Level returns the current log level.
func (l *SlogLogger) Level() Level {
	return l.level
}

// SetLevel changes the log level.
// This affects all future log calls.
func (l *SlogLogger) SetLevel(level Level) {
	l.level = level
}

// clone creates a deep copy of the logger for WithFields operations.
// Returns a new SlogLogger with copied fields slice.
func (l *SlogLogger) clone() *SlogLogger {
	clonedFields := make([]Field, len(l.fields))
	copy(clonedFields, l.fields)

	return &SlogLogger{
		handler: l.handler,
		level:   l.level,
		fields:  clonedFields,
	}
}

// toSlogLevel converts our Level to slog.Level.
func toSlogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fromSlogLevel converts slog.Level to our Level.
func fromSlogLevel(level slog.Level) Level {
	switch level {
	case slog.LevelDebug:
		return LevelDebug
	case slog.LevelInfo:
		return LevelInfo
	case slog.LevelWarn:
		return LevelWarn
	case slog.LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

// newSlogHandler creates a new slog handler with our configuration.
func newSlogHandler(output *os.File, level Level, addSource bool) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     toSlogLevel(level),
		AddSource: addSource,
	}
	return slog.NewJSONHandler(output, opts)
}

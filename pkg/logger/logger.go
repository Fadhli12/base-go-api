package logger

import (
	"context"
	"log/slog"
)

// Logger wraps slog.Logger with context awareness for structured logging
type Logger struct {
	*slog.Logger
	ctx context.Context
}

// New creates a new Logger with the given handler
func New(handler slog.Handler) *Logger {
	return &Logger{
		Logger: slog.New(handler),
		ctx:    context.Background(),
	}
}

// WithContext returns a new Logger with the given context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: l.Logger,
		ctx:    ctx,
	}
}

// WithRequest returns a new Logger with request_id field
func (l *Logger) WithRequest(requestID string) *Logger {
	return l.with(slog.String("request_id", requestID))
}

// WithUser returns a new Logger with user_id field
func (l *Logger) WithUser(userID string) *Logger {
	return l.with(slog.String("user_id", userID))
}

// WithOrg returns a new Logger with org_id field
func (l *Logger) WithOrg(orgID string) *Logger {
	return l.with(slog.String("org_id", orgID))
}

// WithFields returns a new Logger with additional attributes
func (l *Logger) WithFields(attrs ...slog.Attr) *Logger {
	return l.with(attrs...)
}

// with is the internal method that creates a new Logger with attributes
func (l *Logger) with(attrs ...slog.Attr) *Logger {
	// Convert slog.Attr to any for Logger.With()
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return &Logger{
		Logger: l.Logger.With(args...),
		ctx:    l.ctx,
	}
}

// FromContext retrieves the Logger from context, or returns the default logger
func FromContext(ctx context.Context) *Logger {
	if log, ok := ctx.Value(loggerContextKey).(*Logger); ok {
		return log
	}
	return defaultLogger
}

// WithLoggerContext stores the Logger in the context
func WithLoggerContext(ctx context.Context, log *Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, log)
}

var (
	defaultLogger     *Logger
	loggerContextKey  contextKey = "logger"
)

// Init initializes the default logger - call this once at startup
func Init(log *Logger) {
	defaultLogger = log
}

// Package logger provides structured logging configuration using slog.
package logger

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// NewLogger creates a new structured logger based on the log level.
// In development, it uses tint handler for colored, readable output.
// In production, it uses JSON handler for log aggregation systems.
func NewLogger(level string) *slog.Logger {
	return slog.New(NewHandler(level))
}

// NewHandler creates a slog.Handler based on the log level.
// Uses tint for colored output in development, JSON for production.
func NewHandler(level string) slog.Handler {
	logLevel := parseLevel(level)

	// Check if we're in production mode
	if isProduction() {
		return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	// Use tint for development - colored, readable output
	return tint.NewHandler(os.Stdout, &tint.Options{
		Level:      logLevel,
		TimeFormat: "15:04:05.000",
	})
}

// parseLevel converts a log level string to slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// isProduction checks if we're running in production mode.
// This can be determined by LOG_FORMAT=json or an environment variable.
func isProduction() bool {
	format := os.Getenv("LOG_FORMAT")
	return format == "json" || format == "JSON"
}

// SetDefault sets the default logger for the application.
func SetDefault(level string) {
	slog.SetDefault(NewLogger(level))
}

// With returns a logger with additional context.
func With(args ...any) *slog.Logger {
	return slog.With(args...)
}

// Debug logs a debug message.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs an info message.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs an error message.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
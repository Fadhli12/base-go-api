package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogOutput captures slog output to a buffer for testing.
func captureLogOutput(level Level, fn func(Logger)) map[string]interface{} {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewSlogLogger(handler, level)

	fn(logger)

	// Parse the output
	var result map[string]interface{}
	if buf.Len() > 0 {
		_ = json.Unmarshal(buf.Bytes(), &result)
	}
	return result
}

func TestNewSlogLogger(t *testing.T) {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	logger := NewSlogLogger(handler, LevelInfo)

	assert.NotNil(t, logger)
	assert.Equal(t, LevelInfo, logger.Level())
}

func TestNewDefaultSlogLogger(t *testing.T) {
	logger := NewDefaultSlogLogger()
	assert.NotNil(t, logger)
	assert.Equal(t, LevelInfo, logger.Level())
}

func TestSlogLoggerLevelFiltering(t *testing.T) {
	t.Run("Debug filtered at Info level", func(t *testing.T) {
		output := captureLogOutput(LevelInfo, func(l Logger) {
			l.Debug(context.Background(), "debug message")
		})
		assert.Empty(t, output)
	})

	t.Run("Info allowed at Info level", func(t *testing.T) {
		output := captureLogOutput(LevelInfo, func(l Logger) {
			l.Info(context.Background(), "info message")
		})
		assert.Equal(t, "info message", output["msg"])
		assert.Equal(t, "INFO", output["level"])
	})

	t.Run("Warn allowed at Info level", func(t *testing.T) {
		output := captureLogOutput(LevelInfo, func(l Logger) {
			l.Warn(context.Background(), "warn message")
		})
		assert.Equal(t, "warn message", output["msg"])
		assert.Equal(t, "WARN", output["level"])
	})

	t.Run("Error allowed at Info level", func(t *testing.T) {
		output := captureLogOutput(LevelInfo, func(l Logger) {
			l.Error(context.Background(), "error message")
		})
		assert.Equal(t, "error message", output["msg"])
		assert.Equal(t, "ERROR", output["level"])
	})

	t.Run("All levels at Debug level", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := NewSlogLogger(handler, LevelDebug)

		logger.Debug(context.Background(), "debug")
		logger.Info(context.Background(), "info")
		logger.Warn(context.Background(), "warn")
		logger.Error(context.Background(), "error")

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Len(t, lines, 4)
	})

	t.Run("Error filtered at Error level", func(t *testing.T) {
		output := captureLogOutput(LevelError, func(l Logger) {
			l.Warn(context.Background(), "warn message")
		})
		assert.Empty(t, output)
	})
}

func TestSlogLoggerWithFields(t *testing.T) {
	t.Run("adds fields to log", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := NewSlogLogger(handler, LevelInfo)

		logger.Info(context.Background(), "test message",
			String("key1", "value1"),
			Int("key2", 42))

		var result map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)

		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, float64(42), result["key2"])
	})

	t.Run("WithFields creates new logger", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		base := NewSlogLogger(handler, LevelInfo)

		// Create scoped logger with pre-attached fields
		scoped := base.WithFields(String("user_id", "123"))

		// Log with scoped logger
		scoped.Info(context.Background(), "message", String("action", "create"))

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		// Both fields should be present
		assert.Equal(t, "123", result["user_id"])
		assert.Equal(t, "create", result["action"])
	})

	t.Run("WithFields preserves original logger", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		base := NewSlogLogger(handler, LevelInfo)

		_ = base.WithFields(String("extra", "data"))

		// Original logger should not have the field
		base.Info(context.Background(), "test")

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		_, exists := result["extra"]
		assert.False(t, exists, "Original logger should not have extra field")
	})

	t.Run("empty fields returns same logger", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		base := NewSlogLogger(handler, LevelInfo)

		// Should return the same logger
		result := base.WithFields()
		assert.Equal(t, base, result)
	})
}

func TestSlogLoggerWithError(t *testing.T) {
	t.Run("adds error field", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := NewSlogLogger(handler, LevelInfo)

		err := errors.New("something went wrong")
		logger.WithError(err).Error(context.Background(), "operation failed")

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		assert.Equal(t, "something went wrong", result["error"])
	})

	t.Run("nil error returns same logger", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		base := NewSlogLogger(handler, LevelInfo)

		result := base.WithError(nil)
		assert.Equal(t, base, result)
	})

	t.Run("chaining WithError and WithFields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := NewSlogLogger(handler, LevelInfo)

		err := errors.New("database error")
		logger.WithError(err).
			WithFields(String("query", "SELECT *")).
			Error(context.Background(), "query failed")

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		assert.Equal(t, "database error", result["error"])
		assert.Equal(t, "SELECT *", result["query"])
	})
}

func TestSlogLoggerWithContext(t *testing.T) {
	t.Run("extracts context fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := NewSlogLogger(handler, LevelInfo)

		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithUserID(ctx, "user-456")
		ctx = WithOrgID(ctx, "org-789")
		ctx = WithTraceID(ctx, "trace-abc")

		logger.Info(ctx, "test message")

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		assert.Equal(t, "req-123", result["request_id"])
		assert.Equal(t, "user-456", result["user_id"])
		assert.Equal(t, "org-789", result["org_id"])
		assert.Equal(t, "trace-abc", result["trace_id"])
	})

	t.Run("nil context handled gracefully", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		logger := NewSlogLogger(handler, LevelInfo)

		// Should not panic
		logger.Info(nil, "test message")

		var result map[string]interface{}
		_ = json.Unmarshal(buf.Bytes(), &result)

		assert.Equal(t, "test message", result["msg"])
	})
}

func TestSlogLoggerFieldMethods(t *testing.T) {
	logger := NewDefaultSlogLogger()

	t.Run("String", func(t *testing.T) {
		f := logger.String("key", "value")
		assert.Equal(t, "key", f.Key)
		assert.Equal(t, "value", f.Value)
	})

	t.Run("Int", func(t *testing.T) {
		f := logger.Int("count", 42)
		assert.Equal(t, "count", f.Key)
		assert.Equal(t, 42, f.Value)
	})

	t.Run("Int64", func(t *testing.T) {
		f := logger.Int64("ts", 123)
		assert.Equal(t, "ts", f.Key)
		assert.Equal(t, int64(123), f.Value)
	})

	t.Run("Float64", func(t *testing.T) {
		f := logger.Float64("pi", 3.14)
		assert.Equal(t, "pi", f.Key)
		assert.Equal(t, 3.14, f.Value)
	})

	t.Run("Bool", func(t *testing.T) {
		f := logger.Bool("active", true)
		assert.Equal(t, "active", f.Key)
		assert.Equal(t, true, f.Value)
	})

	t.Run("Duration", func(t *testing.T) {
		f := logger.Duration("elapsed", time.Second)
		assert.Equal(t, "elapsed", f.Key)
		assert.Equal(t, time.Second, f.Value)
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()
		f := logger.Time("created", now)
		assert.Equal(t, "created", f.Key)
		assert.Equal(t, now, f.Value)
	})

	t.Run("Any", func(t *testing.T) {
		data := map[string]int{"a": 1}
		f := logger.Any("data", data)
		assert.Equal(t, "data", f.Key)
		assert.Equal(t, data, f.Value)
	})
}

func TestSlogLoggerSetLevel(t *testing.T) {
	logger := NewDefaultSlogLogger()
	assert.Equal(t, LevelInfo, logger.Level())

	logger.SetLevel(LevelDebug)
	assert.Equal(t, LevelDebug, logger.Level())

	logger.SetLevel(LevelError)
	assert.Equal(t, LevelError, logger.Level())
}

func TestSlogLoggerClone(t *testing.T) {
	logger := NewDefaultSlogLogger()
	logger.fields = []Field{String("base", "field")}

	cloned := logger.clone()

	// Clone should have same fields
	assert.Equal(t, len(logger.fields), len(cloned.fields))
	assert.Equal(t, logger.fields[0].Key, cloned.fields[0].Key)

	// Modifying clone should not affect original
	cloned.fields = append(cloned.fields, String("new", "field"))
	assert.Len(t, logger.fields, 1)
	assert.Len(t, cloned.fields, 2)
}

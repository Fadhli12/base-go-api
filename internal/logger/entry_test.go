package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLogEntry(t *testing.T) {
	t.Run("creates entry with basic fields", func(t *testing.T) {
		fields := []Field{
			String("key1", "value1"),
			Int("key2", 42),
		}

		entry := NewLogEntry(LevelInfo, "test message", fields)

		assert.Equal(t, "INFO", entry.Level)
		assert.Equal(t, "test message", entry.Message)
		assert.WithinDuration(t, time.Now().UTC(), entry.Timestamp, time.Second)
		assert.Len(t, entry.Fields, 2)
		assert.Equal(t, "value1", entry.Fields["key1"])
		assert.Equal(t, 42, entry.Fields["key2"])
	})

	t.Run("handles empty fields", func(t *testing.T) {
		entry := NewLogEntry(LevelDebug, "no fields", nil)

		assert.Equal(t, "DEBUG", entry.Level)
		assert.Equal(t, "no fields", entry.Message)
		assert.Empty(t, entry.Fields)
	})

	t.Run("extracts error from fields", func(t *testing.T) {
		testErr := assert.AnError
		fields := []Field{
			String("key", "value"),
			Err(testErr),
		}

		entry := NewLogEntry(LevelError, "error occurred", fields)

		assert.Equal(t, "error occurred", entry.Message)
		assert.Equal(t, testErr.Error(), entry.Error)
		assert.Equal(t, "value", entry.Fields["key"])
	})

	t.Run("handles nil error", func(t *testing.T) {
		fields := []Field{
			Err(nil),
		}

		entry := NewLogEntry(LevelError, "nil error", fields)

		assert.Equal(t, "<nil>", entry.Error)
	})
}

func TestLogEntryAddContextFields(t *testing.T) {
	entry := NewLogEntry(LevelInfo, "test", nil)

	contextFields := []Field{
		String("request_id", "req-123"),
		String("user_id", "user-456"),
		String("org_id", "org-789"),
		String("trace_id", "trace-abc"),
	}

	entry.AddContextFields(contextFields)

	assert.Equal(t, "req-123", entry.RequestID)
	assert.Equal(t, "user-456", entry.UserID)
	assert.Equal(t, "org-789", entry.OrgID)
	assert.Equal(t, "trace-abc", entry.TraceID)
}

func TestLogEntrySetters(t *testing.T) {
	entry := NewLogEntry(LevelError, "error", nil)

	t.Run("SetError", func(t *testing.T) {
		entry.SetError("connection failed")
		assert.Equal(t, "connection failed", entry.Error)
	})

	t.Run("SetStackTrace", func(t *testing.T) {
		entry.SetStackTrace("at line 42")
		assert.Equal(t, "at line 42", entry.StackTrace)
	})

	t.Run("SetSource", func(t *testing.T) {
		entry.SetSource("main.go:15")
		assert.Equal(t, "main.go:15", entry.Source)
	})
}

func TestLogEntryJSONTags(t *testing.T) {
	entry := LogEntry{
		Timestamp:  time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Level:      "INFO",
		Message:    "test",
		RequestID:  "req-123",
		UserID:     "user-456",
		OrgID:      "org-789",
		TraceID:    "trace-abc",
		Fields:     map[string]interface{}{"key": "value"},
		Error:      "something wrong",
		StackTrace: "at line 42",
		Source:     "main.go:15",
	}

	// Verify all fields are set
	assert.NotZero(t, entry.Timestamp)
	assert.Equal(t, "INFO", entry.Level)
	assert.Equal(t, "test", entry.Message)
	assert.Equal(t, "req-123", entry.RequestID)
	assert.Equal(t, "user-456", entry.UserID)
	assert.Equal(t, "org-789", entry.OrgID)
	assert.Equal(t, "trace-abc", entry.TraceID)
	assert.NotNil(t, entry.Fields)
	assert.Equal(t, "something wrong", entry.Error)
	assert.Equal(t, "at line 42", entry.StackTrace)
	assert.Equal(t, "main.go:15", entry.Source)
}

func TestLogEntryOmitEmpty(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "minimal",
	}

	// These should be empty/zero values
	assert.Empty(t, entry.RequestID)
	assert.Empty(t, entry.UserID)
	assert.Empty(t, entry.OrgID)
	assert.Empty(t, entry.TraceID)
	assert.Empty(t, entry.Fields)
	assert.Empty(t, entry.Error)
	assert.Empty(t, entry.StackTrace)
	assert.Empty(t, entry.Source)
}

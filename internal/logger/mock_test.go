package logger

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMockLogger(t *testing.T) {
	mock := NewMockLogger()
	assert.NotNil(t, mock)
	assert.Empty(t, mock.GetMessages())
	assert.Equal(t, 0, mock.GetMessagesCount())
}

func TestMockLogger_LoggingMethods(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	t.Run("Debug logs message", func(t *testing.T) {
		mock.Reset()
		mock.Debug(ctx, "debug message", mock.String("key", "value"))

		assert.True(t, mock.AssertLogged(LevelDebug, "debug message"))
		assert.True(t, mock.AssertField("key", "value"))
		assert.Equal(t, 1, mock.GetMessagesCount())
	})

	t.Run("Info logs message", func(t *testing.T) {
		mock.Reset()
		mock.Info(ctx, "info message", mock.String("key", "value"))

		assert.True(t, mock.AssertLogged(LevelInfo, "info message"))
		assert.True(t, mock.AssertField("key", "value"))
		assert.Equal(t, 1, mock.GetMessagesCount())
	})

	t.Run("Warn logs message", func(t *testing.T) {
		mock.Reset()
		mock.Warn(ctx, "warn message", mock.String("key", "value"))

		assert.True(t, mock.AssertLogged(LevelWarn, "warn message"))
		assert.True(t, mock.AssertField("key", "value"))
		assert.Equal(t, 1, mock.GetMessagesCount())
	})

	t.Run("Error logs message", func(t *testing.T) {
		mock.Reset()
		mock.Error(ctx, "error message", mock.String("key", "value"))

		assert.True(t, mock.AssertLogged(LevelError, "error message"))
		assert.True(t, mock.AssertField("key", "value"))
		assert.Equal(t, 1, mock.GetMessagesCount())
	})
}

func TestMockLogger_WithFields(t *testing.T) {
	t.Run("WithFields creates scoped logger", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		scoped := mock.WithFields(String("user_id", "123"))
		scoped.Info(ctx, "operation")

		assert.True(t, mock.AssertField("user_id", "123"))
	})

	t.Run("WithFields does not affect original", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		_ = mock.WithFields(String("user_id", "123"))
		mock.Info(ctx, "operation")

		// Original logger should not have user_id field
		assert.False(t, mock.AssertField("user_id", "123"))
	})

	t.Run("Chained WithFields accumulates fields", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		scoped := mock.WithFields(String("user_id", "123")).WithFields(String("org_id", "456"))
		scoped.Info(ctx, "operation")

		assert.True(t, mock.AssertField("user_id", "123"))
		assert.True(t, mock.AssertField("org_id", "456"))
	})
}

func TestMockLogger_WithError(t *testing.T) {
	t.Run("WithError attaches error", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		err := errors.New("test error")
		withErr := mock.WithError(err)
		withErr.Error(ctx, "failed")

		// Check message was recorded (shared message slice)
		msg := mock.FindMessage(LevelError, "failed")
		assert.NotNil(t, msg)

		// Check error field exists
		found := false
		for _, field := range msg.Fields {
			if field.Key == "error" {
				found = true
				break
			}
		}
		assert.True(t, found, "error field should be present")
	})

	t.Run("WithError nil does not panic", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		mock.WithError(nil).Info(ctx, "no error")
		assert.True(t, mock.AssertLogged(LevelInfo, "no error"))
	})
}

func TestMockLogger_AssertLogged(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "user created")
	mock.Warn(ctx, "suspicious activity")
	mock.Error(ctx, "database failure")

	t.Run("finds matching message", func(t *testing.T) {
		assert.True(t, mock.AssertLogged(LevelInfo, "user created"))
		assert.True(t, mock.AssertLogged(LevelWarn, "suspicious"))
		assert.True(t, mock.AssertLogged(LevelError, "failure"))
	})

	t.Run("returns false for non-matching level", func(t *testing.T) {
		assert.False(t, mock.AssertLogged(LevelDebug, "user created"))
	})

	t.Run("returns false for non-matching message", func(t *testing.T) {
		assert.False(t, mock.AssertLogged(LevelInfo, "nonexistent"))
	})

	t.Run("empty substring matches any", func(t *testing.T) {
		assert.True(t, mock.AssertLogged(LevelInfo, ""))
	})
}

func TestMockLogger_AssertLoggedExactly(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "exact match")

	t.Run("matches exact message", func(t *testing.T) {
		assert.True(t, mock.AssertLoggedExactly(LevelInfo, "exact match"))
	})

	t.Run("substring does not match", func(t *testing.T) {
		assert.False(t, mock.AssertLoggedExactly(LevelInfo, "exact"))
	})
}

func TestMockLogger_AssertField(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "message", mock.String("user_id", "123"), mock.Int("count", 42))

	t.Run("finds string field", func(t *testing.T) {
		assert.True(t, mock.AssertField("user_id", "123"))
	})

	t.Run("finds int field", func(t *testing.T) {
		assert.True(t, mock.AssertField("count", 42))
	})

	t.Run("returns false for missing field", func(t *testing.T) {
		assert.False(t, mock.AssertField("missing", "value"))
	})

	t.Run("returns false for wrong value", func(t *testing.T) {
		assert.False(t, mock.AssertField("user_id", "456"))
	})
}

func TestMockLogger_AssertFieldInMessage(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "user action", mock.String("user_id", "123"))
	mock.Error(ctx, "database error", mock.String("query", "SELECT *"))

	t.Run("finds field in specific message level", func(t *testing.T) {
		assert.True(t, mock.AssertFieldInMessage(LevelInfo, "user action", "user_id", "123"))
	})

	t.Run("returns false if field in wrong level", func(t *testing.T) {
		assert.False(t, mock.AssertFieldInMessage(LevelError, "user action", "user_id", "123"))
	})

	t.Run("returns false if message not found", func(t *testing.T) {
		assert.False(t, mock.AssertFieldInMessage(LevelInfo, "nonexistent", "user_id", "123"))
	})
}

func TestMockLogger_GetMessages(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "message 1")
	mock.Info(ctx, "message 2")

	messages := mock.GetMessages()
	assert.Len(t, messages, 2)
	assert.Equal(t, "message 1", messages[0].Message)
	assert.Equal(t, "message 2", messages[1].Message)

	// Verify it's a copy
	messages[0].Message = "modified"
	assert.Equal(t, "message 1", mock.GetMessages()[0].Message)
}

func TestMockLogger_GetMessagesCount(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	assert.Equal(t, 0, mock.GetMessagesCount())

	mock.Info(ctx, "message 1")
	assert.Equal(t, 1, mock.GetMessagesCount())

	mock.Info(ctx, "message 2")
	assert.Equal(t, 2, mock.GetMessagesCount())
}

func TestMockLogger_GetMessagesAtLevel(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Debug(ctx, "debug 1")
	mock.Info(ctx, "info 1")
	mock.Info(ctx, "info 2")
	mock.Error(ctx, "error 1")

	debugMsgs := mock.GetMessagesAtLevel(LevelDebug)
	assert.Len(t, debugMsgs, 1)
	assert.Equal(t, "debug 1", debugMsgs[0].Message)

	infoMsgs := mock.GetMessagesAtLevel(LevelInfo)
	assert.Len(t, infoMsgs, 2)

	errorMsgs := mock.GetMessagesAtLevel(LevelError)
	assert.Len(t, errorMsgs, 1)

	warnMsgs := mock.GetMessagesAtLevel(LevelWarn)
	assert.Empty(t, warnMsgs)
}

func TestMockLogger_Reset(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "message")
	assert.Equal(t, 1, mock.GetMessagesCount())

	mock.Reset()
	assert.Equal(t, 0, mock.GetMessagesCount())
	assert.Empty(t, mock.GetMessages())
}

func TestMockLogger_FindMessage(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	mock.Info(ctx, "user created", mock.String("user_id", "123"))

	t.Run("finds existing message", func(t *testing.T) {
		msg := mock.FindMessage(LevelInfo, "user created")
		assert.NotNil(t, msg)
		assert.Equal(t, "user created", msg.Message)
	})

	t.Run("returns nil for non-existing", func(t *testing.T) {
		msg := mock.FindMessage(LevelError, "user created")
		assert.Nil(t, msg)
	})

	t.Run("returns nil for non-matching substring", func(t *testing.T) {
		msg := mock.FindMessage(LevelInfo, "nonexistent")
		assert.Nil(t, msg)
	})
}

func TestMockLogger_HasError(t *testing.T) {
	t.Run("true when error logged", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		mock.Error(ctx, "failed", mock.String("error", "connection refused"))
		assert.True(t, mock.HasError())
	})

	t.Run("false when no error", func(t *testing.T) {
		mock := NewMockLogger()
		ctx := context.Background()

		mock.Info(ctx, "success")
		assert.False(t, mock.HasError())
	})
}

func TestMockLogger_FieldFactoryMethods(t *testing.T) {
	mock := NewMockLogger()

	tests := []struct {
		name     string
		field    Field
		wantKey  string
		wantValue interface{}
	}{
		{"String", mock.String("key", "value"), "key", "value"},
		{"Int", mock.Int("count", 42), "count", 42},
		{"Int64", mock.Int64("ts", int64(123)), "ts", int64(123)},
		{"Float64", mock.Float64("pi", 3.14), "pi", 3.14},
		{"Bool", mock.Bool("active", true), "active", true},
		{"Duration", mock.Duration("elapsed", time.Second), "elapsed", time.Second},
		{"Time", mock.Time("created", time.Unix(1234567890, 0)), "created", time.Unix(1234567890, 0)},
		{"Any", mock.Any("data", map[string]int{"a": 1}), "data", map[string]int{"a": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantKey, tt.field.Key)
			assert.Equal(t, tt.wantValue, tt.field.Value)
		})
	}
}

func TestMockLogger_Concurrency(t *testing.T) {
	mock := NewMockLogger()
	ctx := context.Background()

	// Run concurrent logging operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mock.Info(ctx, "message", mock.Int("id", id))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, mock.GetMessagesCount())

	// Verify all messages were captured
	for i := 0; i < 100; i++ {
		found := false
		for _, msg := range mock.GetMessages() {
			for _, field := range msg.Fields {
				if field.Key == "id" && field.Value == i {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		assert.True(t, found, "should find message with id %d", i)
	}
}

func TestMockLogger_ImplementsLogger(t *testing.T) {
	// Verify MockLogger implements Logger interface
	var _ Logger = NewMockLogger()
}

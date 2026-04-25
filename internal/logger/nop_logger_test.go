package logger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	assert.NotNil(t, logger)
	assert.IsType(t, &nopLogger{}, logger)
}

func TestNopLoggerMethods(t *testing.T) {
	logger := NewNopLogger()
	ctx := context.Background()

	// All methods should be no-ops and not panic
	t.Run("Debug", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Debug(ctx, "debug message")
		})
	})

	t.Run("Info", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Info(ctx, "info message")
		})
	})

	t.Run("Warn", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Warn(ctx, "warn message")
		})
	})

	t.Run("Error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Error(ctx, "error message")
		})
	})
}

func TestNopLoggerWithFields(t *testing.T) {
	logger := NewNopLogger()
	ctx := context.Background()

	// WithFields should return the same nopLogger instance
	t.Run("WithFields returns same instance", func(t *testing.T) {
		result := logger.WithFields(String("key", "value"))
		assert.Equal(t, logger, result)
	})

	// Chained calls should work
	t.Run("chained calls", func(t *testing.T) {
		scoped := logger.WithFields(String("user_id", "123"))
		assert.NotPanics(t, func() {
			scoped.Info(ctx, "message")
		})
	})
}

func TestNopLoggerWithError(t *testing.T) {
	logger := NewNopLogger()

	t.Run("WithError returns same instance", func(t *testing.T) {
		err := errors.New("test error")
		result := logger.WithError(err)
		assert.Equal(t, logger, result)
	})

	t.Run("WithError nil", func(t *testing.T) {
		result := logger.WithError(nil)
		assert.Equal(t, logger, result)
	})
}

func TestNopLoggerFieldMethods(t *testing.T) {
	logger := NewNopLogger()

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
		f := logger.Int64("ts", int64(123))
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

func TestIsNopLogger(t *testing.T) {
	t.Run("returns true for NopLogger", func(t *testing.T) {
		logger := NewNopLogger()
		assert.True(t, IsNopLogger(logger))
	})

	t.Run("returns false for SlogLogger", func(t *testing.T) {
		logger := NewDefaultSlogLogger()
		assert.False(t, IsNopLogger(logger))
	})
}

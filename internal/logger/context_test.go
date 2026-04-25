package logger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithLogger(t *testing.T) {
	t.Run("store and retrieve logger", func(t *testing.T) {
		ctx := context.Background()
		logger := NewDefaultSlogLogger()

		ctx = WithLogger(ctx, logger)
		retrieved := FromContext(ctx)

		assert.NotNil(t, retrieved)
		assert.IsType(t, &SlogLogger{}, retrieved)
	})

	t.Run("returns NopLogger for nil context", func(t *testing.T) {
		logger := FromContext(nil)
		assert.NotNil(t, logger)
		assert.True(t, IsNopLogger(logger))
	})

	t.Run("returns NopLogger when not set", func(t *testing.T) {
		ctx := context.Background()
		logger := FromContext(ctx)
		assert.NotNil(t, logger)
		assert.True(t, IsNopLogger(logger))
	})

	t.Run("context is immutable", func(t *testing.T) {
		ctx1 := context.Background()
		logger := NewDefaultSlogLogger()

		ctx2 := WithLogger(ctx1, logger)

		// Original context should not have logger
		l1 := FromContext(ctx1)
		assert.True(t, IsNopLogger(l1))

		// New context should have logger
		l2 := FromContext(ctx2)
		assert.False(t, IsNopLogger(l2))
	})
}

func TestRequestID(t *testing.T) {
	t.Run("store and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")

		id := GetRequestID(ctx)
		assert.Equal(t, "req-123", id)
	})

	t.Run("returns empty for nil context", func(t *testing.T) {
		id := GetRequestID(nil)
		assert.Empty(t, id)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		ctx := context.Background()
		id := GetRequestID(ctx)
		assert.Empty(t, id)
	})
}

func TestUserID(t *testing.T) {
	t.Run("store and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "user-456")

		id := GetUserID(ctx)
		assert.Equal(t, "user-456", id)
	})

	t.Run("returns empty for nil context", func(t *testing.T) {
		id := GetUserID(nil)
		assert.Empty(t, id)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		ctx := context.Background()
		id := GetUserID(ctx)
		assert.Empty(t, id)
	})
}

func TestOrgID(t *testing.T) {
	t.Run("store and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithOrgID(ctx, "org-789")

		id := GetOrgID(ctx)
		assert.Equal(t, "org-789", id)
	})

	t.Run("returns empty for nil context", func(t *testing.T) {
		id := GetOrgID(nil)
		assert.Empty(t, id)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		ctx := context.Background()
		id := GetOrgID(ctx)
		assert.Empty(t, id)
	})
}

func TestTraceID(t *testing.T) {
	t.Run("store and retrieve", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithTraceID(ctx, "trace-abc")

		id := GetTraceID(ctx)
		assert.Equal(t, "trace-abc", id)
	})

	t.Run("returns empty for nil context", func(t *testing.T) {
		id := GetTraceID(nil)
		assert.Empty(t, id)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		ctx := context.Background()
		id := GetTraceID(ctx)
		assert.Empty(t, id)
	})
}

func TestStartTime(t *testing.T) {
	t.Run("store and retrieve", func(t *testing.T) {
		ctx := context.Background()
		startTime := time.Now()
		ctx = WithStartTime(ctx, startTime)

		retrieved := GetStartTime(ctx)
		assert.Equal(t, startTime, retrieved)
	})

	t.Run("returns zero time for nil context", func(t *testing.T) {
		time := GetStartTime(nil)
		assert.True(t, time.IsZero())
	})

	t.Run("returns zero time when not set", func(t *testing.T) {
		ctx := context.Background()
		time := GetStartTime(ctx)
		assert.True(t, time.IsZero())
	})
}

func TestExtractContextFields(t *testing.T) {
	t.Run("empty context", func(t *testing.T) {
		ctx := context.Background()
		fields := ExtractContextFields(ctx)
		assert.Empty(t, fields)
		assert.NotNil(t, fields)
	})

	t.Run("nil context", func(t *testing.T) {
		fields := ExtractContextFields(nil)
		assert.Empty(t, fields)
		assert.NotNil(t, fields)
	})

	t.Run("single field", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")

		fields := ExtractContextFields(ctx)
		assert.Len(t, fields, 1)
		assert.Equal(t, "request_id", fields[0].Key)
		assert.Equal(t, "req-123", fields[0].Value)
	})

	t.Run("all fields", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithUserID(ctx, "user-456")
		ctx = WithOrgID(ctx, "org-789")
		ctx = WithTraceID(ctx, "trace-abc")

		fields := ExtractContextFields(ctx)
		assert.Len(t, fields, 4)

		// Check fields are in expected order
		assert.Equal(t, "request_id", fields[0].Key)
		assert.Equal(t, "user_id", fields[1].Key)
		assert.Equal(t, "org_id", fields[2].Key)
		assert.Equal(t, "trace_id", fields[3].Key)
	})

	t.Run("partial fields", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "user-456")
		ctx = WithTraceID(ctx, "trace-abc")

		fields := ExtractContextFields(ctx)
		assert.Len(t, fields, 2)
		assert.Equal(t, "user_id", fields[0].Key)
		assert.Equal(t, "trace_id", fields[1].Key)
	})

	t.Run("empty values are skipped", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "")
		ctx = WithUserID(ctx, "user-456")

		fields := ExtractContextFields(ctx)
		assert.Len(t, fields, 1)
		assert.Equal(t, "user_id", fields[0].Key)
	})
}

func TestGenerateIDs(t *testing.T) {
	t.Run("GenerateRequestID creates unique IDs", func(t *testing.T) {
		id1 := GenerateRequestID()
		id2 := GenerateRequestID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
		assert.Len(t, id1, 36) // UUID format
	})

	t.Run("GenerateTraceID creates unique IDs", func(t *testing.T) {
		id1 := GenerateTraceID()
		id2 := GenerateTraceID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
		assert.Len(t, id1, 36) // UUID format
	})
}

func TestHeaderConstants(t *testing.T) {
	assert.Equal(t, "X-Request-ID", RequestIDHeader)
	assert.Equal(t, "X-Trace-ID", TraceIDHeader)
	assert.Equal(t, "X-Organization-ID", OrgIDHeader)
}

func TestContextKeyType(t *testing.T) {
	// Ensure context keys don't collide
	keys := map[contextKey]bool{
		LoggerContextKey:    true,
		RequestIDContextKey: true,
		UserIDContextKey:    true,
		OrgIDContextKey:     true,
		TraceIDContextKey:   true,
		StartTimeContextKey: true,
	}

	// Should have 6 unique keys
	assert.Len(t, keys, 6)
}

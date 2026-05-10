package unit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
)

// mockWebhookRateLimiter implements service.WebhookRateLimiterInterface for testing.
type mockWebhookRateLimiter struct {
	allowFunc func(ctx context.Context, webhookID string) (bool, int, time.Duration)
	callCount atomic.Int64
}

func newMockWebhookRateLimiter() *mockWebhookRateLimiter {
	return &mockWebhookRateLimiter{}
}

func (m *mockWebhookRateLimiter) Allow(ctx context.Context, webhookID string) (bool, int, time.Duration) {
	m.callCount.Add(1)
	if m.allowFunc != nil {
		return m.allowFunc(ctx, webhookID)
	}
	return true, 100, 0
}

// TestMockWebhookRateLimiter_AllowDefault tests the default allow behavior (always allow).
func TestMockWebhookRateLimiter_AllowDefault(t *testing.T) {
	mock := newMockWebhookRateLimiter()
	ctx := context.Background()

	allowed, remaining, retryAfter := mock.Allow(ctx, "wh_123")
	assert.True(t, allowed)
	assert.Equal(t, 100, remaining)
	assert.Zero(t, retryAfter)
}

// TestMockWebhookRateLimiter_AllowDeny tests forced deny behavior.
func TestMockWebhookRateLimiter_AllowDeny(t *testing.T) {
	mock := newMockWebhookRateLimiter()
	mock.allowFunc = func(ctx context.Context, webhookID string) (bool, int, time.Duration) {
		return false, 0, 30 * time.Second
	}
	ctx := context.Background()

	allowed, remaining, retryAfter := mock.Allow(ctx, "wh_456")
	assert.False(t, allowed)
	assert.Zero(t, remaining)
	assert.Equal(t, 30*time.Second, retryAfter)
}

// TestMockWebhookRateLimiter_AllowCustomRemaining tests custom remaining count.
func TestMockWebhookRateLimiter_AllowCustomRemaining(t *testing.T) {
	mock := newMockWebhookRateLimiter()
	mock.allowFunc = func(ctx context.Context, webhookID string) (bool, int, time.Duration) {
		return true, 42, 0
	}
	ctx := context.Background()

	allowed, remaining, retryAfter := mock.Allow(ctx, "wh_789")
	assert.True(t, allowed)
	assert.Equal(t, 42, remaining)
	assert.Zero(t, retryAfter)
}

// TestMockWebhookRateLimiter_CallTracking tests that calls are tracked.
func TestMockWebhookRateLimiter_CallTracking(t *testing.T) {
	mock := newMockWebhookRateLimiter()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		mock.Allow(ctx, "wh_test")
	}

	assert.Equal(t, int64(5), mock.callCount.Load())
}

// TestMockWebhookRateLimiter_WebhookIDPassedCorrectly tests that webhook ID is received.
func TestMockWebhookRateLimiter_WebhookIDPassedCorrectly(t *testing.T) {
	var capturedID string
	mock := newMockWebhookRateLimiter()
	mock.allowFunc = func(ctx context.Context, webhookID string) (bool, int, time.Duration) {
		capturedID = webhookID
		return true, 0, 0
	}
	ctx := context.Background()

	mock.Allow(ctx, "wh_specific_123")
	assert.Equal(t, "wh_specific_123", capturedID)
}

// TestMockWebhookRateLimiter_ConcurrentSafety tests concurrent access.
func TestMockWebhookRateLimiter_ConcurrentSafety(t *testing.T) {
	mock := newMockWebhookRateLimiter()
	ctx := context.Background()
	const goroutines = 100

	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			mock.Allow(ctx, "wh_concurrent")
			done <- struct{}{}
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}

	assert.Equal(t, int64(goroutines), mock.callCount.Load())
}

// Verify mockWebhookRateLimiter implements service.WebhookRateLimiterInterface.
var _ service.WebhookRateLimiterInterface = (*mockWebhookRateLimiter)(nil)

package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRateLimiter(exportLimit, importLimit, recordLimit int) (*DataPortabilityRateLimiterImpl, *miniredis.Miniredis, error) {
	mr, err := miniredis.Run()
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	limiter := NewDataPortabilityRateLimiter(client, exportLimit, importLimit, recordLimit)
	return limiter, mr, nil
}

func TestDataPortabilityRateLimiter_Allow_UnderLimit(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(3, 2, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name   string
		userID string
		action string
		allowed bool
	}{
		{"export under limit", "user1", ActionExport, true},
		{"import under limit", "user1", ActionImport, true},
		{"different user export", "user2", ActionExport, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := limiter.Allow(ctx, tt.userID, tt.action)
			assert.NoError(t, err)
			assert.Equal(t, tt.allowed, allowed)
		})
	}
}

func TestDataPortabilityRateLimiter_Allow_OverLimit(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(2, 1, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.False(t, allowed, "should be rate limited after exceeding export limit")
}

func TestDataPortabilityRateLimiter_Allow_SeperateActions(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(2, 1, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1", ActionImport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user1", ActionImport)
	assert.NoError(t, err)
	assert.False(t, allowed, "import should be rate limited after 1 use")

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed, "export should still be allowed (separate action key)")
}

func TestDataPortabilityRateLimiter_Allow_DifferentUsers(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(1, 1, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user2", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed, "different user should have separate rate limit")
}

func TestDataPortabilityRateLimiter_Allow_RedisFailure(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(1, 1, 50000)
	require.NoError(t, err)
	mr.Close()

	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed, "should fail open on Redis error")
}

func TestDataPortabilityRateLimiter_AllowRecords_UnderLimit(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		orgID   string
		count   int
		allowed bool
	}{
		{"100 records under limit", "org1", 100, true},
		{"another 100 records under limit", "org1", 100, true},
		{"different org under limit", "org2", 500, true},
	}

	var totalChecked int
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := limiter.AllowRecords(ctx, tt.orgID, tt.count)
			assert.NoError(t, err)
			assert.Equal(t, tt.allowed, allowed)
		})
		totalChecked += tt.count
	}
	_ = totalChecked
}

func TestDataPortabilityRateLimiter_AllowRecords_OverLimit(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 100)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.AllowRecords(ctx, "org1", 80)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.AllowRecords(ctx, "org1", 30)
	assert.NoError(t, err)
	assert.False(t, allowed, "should reject when total would exceed record limit")
}

func TestDataPortabilityRateLimiter_AllowRecords_ExactLimit(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 100)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.AllowRecords(ctx, "org1", 100)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow exactly at limit")

	allowed, err = limiter.AllowRecords(ctx, "org1", 1)
	assert.NoError(t, err)
	assert.False(t, allowed, "should reject even 1 more over limit")
}

func TestDataPortabilityRateLimiter_AllowRecords_DifferentOrgs(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 100)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.AllowRecords(ctx, "org1", 80)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.AllowRecords(ctx, "org2", 80)
	assert.NoError(t, err)
	assert.True(t, allowed, "different org should have separate count")
}

func TestDataPortabilityRateLimiter_AllowRecords_RedisFailure(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 100)
	require.NoError(t, err)
	mr.Close()

	ctx := context.Background()

	allowed, err := limiter.AllowRecords(ctx, "org1", 50)
	assert.NoError(t, err)
	assert.True(t, allowed, "should fail open on Redis error")
}

func TestDataPortabilityRateLimiter_Allow_SlidingWindow(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(2, 1, 50000)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.False(t, allowed, "should be rate limited")

	mr.FastForward(2 * time.Hour)

	allowed, err = limiter.Allow(ctx, "user1", ActionExport)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow after window expires")
}

func TestDataPortabilityRateLimiter_AllowRecords_SlidingWindow(t *testing.T) {
	limiter, mr, err := newTestRateLimiter(10, 5, 100)
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	allowed, err := limiter.AllowRecords(ctx, "org1", 100)
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.AllowRecords(ctx, "org1", 1)
	assert.NoError(t, err)
	assert.False(t, allowed, "should be at limit")

	mr.FastForward(2 * time.Hour)

	allowed, err = limiter.AllowRecords(ctx, "org1", 50)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow after window expires")
}
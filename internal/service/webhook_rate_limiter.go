package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	WebhookRateLimitKeyPrefix = "webhook:ratelimit:"
	WebhookRateLimitWindow    = 60 * time.Second // 1-minute sliding window
)

// WebhookRateLimiter provides per-webhook rate limiting using a Redis sliding window.
type WebhookRateLimiter struct {
	client    *redis.Client
	rateLimit int // max requests per window
}

// NewWebhookRateLimiter creates a new WebhookRateLimiter instance.
func NewWebhookRateLimiter(client *redis.Client, rateLimit int) *WebhookRateLimiter {
	return &WebhookRateLimiter{
		client:    client,
		rateLimit: rateLimit,
	}
}

// Allow checks whether a delivery attempt for the given webhook is allowed.
// It returns whether the attempt is allowed, the remaining quota, and the
// duration to wait before retrying if the limit has been exceeded.
//
// This method uses a Redis sorted set (ZSET) to implement a sliding window.
// On any Redis error the limiter fails open (allows the request) so that
// a Redis outage does not block webhook delivery.
func (r *WebhookRateLimiter) Allow(ctx context.Context, webhookID string) (bool, int, time.Duration) {
	key := fmt.Sprintf("%s%s", WebhookRateLimitKeyPrefix, webhookID)
	now := time.Now()
	windowStart := now.Add(-WebhookRateLimitWindow)

	// 1. Remove expired entries outside the sliding window.
	r.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))

	// 2. Count current entries within the window.
	count, err := r.client.ZCard(ctx, key).Result()
	if err != nil {
		// Fail open on Redis errors.
		return true, r.rateLimit, 0
	}

	// 3. Check if the rate limit has been exceeded.
	if count >= int64(r.rateLimit) {
		oldest, err := r.client.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
			Min:   "-inf",
			Max:   "+inf",
			Count: 1,
		}).Result()

		retryAfter := WebhookRateLimitWindow
		if err == nil && len(oldest) > 0 {
			oldestTime := int64(oldest[0].Score)
			expiry := time.UnixMilli(oldestTime).Add(WebhookRateLimitWindow)
			retryAfter = time.Until(expiry)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}

		return false, 0, retryAfter
	}

	// 4. Add a new entry with a unique member ID.
	entryID := fmt.Sprintf("%d-%d", now.UnixMilli(), rand.Intn(10000))
	r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: entryID,
	})

	// 5. Refresh the key expiry.
	r.client.Expire(ctx, key, WebhookRateLimitWindow)

	remaining := r.rateLimit - int(count) - 1
	if remaining < 0 {
		remaining = 0
	}

	return true, remaining, 0
}

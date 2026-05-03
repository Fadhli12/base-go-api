package service

import (
	"context"
	"time"
)

// WebhookRateLimiterInterface defines the interface for per-webhook rate limiting.
// Implementations must be safe for concurrent use.
type WebhookRateLimiterInterface interface {
	// Allow checks whether a webhook is within its rate limit.
	// Returns (allowed, remaining, retryAfter) where:
	//   - allowed: true if the request is permitted
	//   - remaining: number of requests still allowed in the current window
	//   - retryAfter: duration until the next request will be allowed (0 if allowed)
	Allow(ctx context.Context, webhookID string) (allowed bool, remaining int, retryAfter time.Duration)
}
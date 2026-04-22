package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultRequestsPerMinute is the default rate limit
	DefaultRequestsPerMinute = 100
	// RateLimitWindow is the time window for rate limiting
	RateLimitWindow = 60 * time.Second
	// RateLimitKeyPrefix is the Redis key prefix for rate limiting
	RateLimitKeyPrefix = "ratelimit:"
	// RateLimitRemainingHeader is the header for remaining requests
	RateLimitRemainingHeader = "X-RateLimit-Remaining"
)

// RateLimiter holds the Redis client and rate limit settings
type RateLimiter struct {
	redisClient        *redis.Client
	requestsPerMinute  int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, requestsPerMinute int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = DefaultRequestsPerMinute
	}

	return &RateLimiter{
		redisClient:       redisClient,
		requestsPerMinute: requestsPerMinute,
	}
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := c.RealIP()
			key := fmt.Sprintf("%s%s", RateLimitKeyPrefix, clientIP)

			// Get current count
			ctx := c.Request().Context()
			count, err := rl.redisClient.Get(ctx, key).Int()
			if err == redis.Nil {
				count = 0
			} else if err != nil {
				slog.Error("rate limit redis error", slog.String("error", err.Error()))
				// Fail open - allow request through on Redis error
				return next(c)
			}

			// Check if limit exceeded
			if count >= rl.requestsPerMinute {
				remaining := 0
				c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Rate limit exceeded. Please try again later.",
					},
				})
			}

			// Increment counter and set TTL if new
			pipe := rl.redisClient.Pipeline()
			pipe.Incr(ctx, key)
			pipe.TTL(ctx, key)
			results, err := pipe.Exec(ctx)
			if err != nil {
				slog.Error("rate limit pipeline error", slog.String("error", err.Error()))
			} else {
				// Set expiry if this is the first request (TTL will be -1)
				ttlCmd := results[1].(*redis.Cmd)
				ttl, _ := ttlCmd.Result()
				if ttl == -1 {
					rl.redisClient.Expire(ctx, key, RateLimitWindow)
				}
			}

			// Calculate remaining
			remaining := rl.requestsPerMinute - count - 1
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))

			return next(c)
		}
	}
}

// RateLimit returns a rate limiting middleware with default settings
func RateLimit(redisClient *redis.Client) echo.MiddlewareFunc {
	return NewRateLimiter(redisClient, DefaultRequestsPerMinute).Middleware()
}

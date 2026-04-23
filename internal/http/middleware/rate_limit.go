package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/cache"
	"github.com/labstack/echo/v4"
)

const (
	// DefaultRequestsPerMinute is the default rate limit
	DefaultRequestsPerMinute = 100
	// RateLimitWindow is the time window for rate limiting in seconds
	RateLimitWindow = 60
	// RateLimitKeyPrefix is the cache key prefix for rate limiting
	RateLimitKeyPrefix = "ratelimit:"
	// RateLimitRemainingHeader is the header for remaining requests
	RateLimitRemainingHeader = "X-RateLimit-Remaining"
)

// RateLimiter holds the cache driver and rate limit settings
type RateLimiter struct {
	driver            cache.Driver
	requestsPerMinute int
	ttlSeconds        int
}

// NewRateLimiter creates a new rate limiter with the given cache driver
func NewRateLimiter(driver cache.Driver, requestsPerMinute int, ttlSeconds int) *RateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = DefaultRequestsPerMinute
	}
	if ttlSeconds <= 0 {
		ttlSeconds = RateLimitWindow
	}

	return &RateLimiter{
		driver:            driver,
		requestsPerMinute: requestsPerMinute,
		ttlSeconds:        ttlSeconds,
	}
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// For no-op cache, allow all requests through (fail open)
			if rl.driver == nil {
				return next(c)
			}

			clientIP := c.RealIP()
			key := fmt.Sprintf("%s%s", RateLimitKeyPrefix, clientIP)
			ctx := c.Request().Context()

			// Get current count
			count := 0
			val, err := rl.driver.Get(ctx, key)
			if err != nil {
				slog.Error("rate limit cache error", slog.String("error", err.Error()))
				// Fail open - allow request through on cache error
				return next(c)
			}

			if val != nil {
				parsed, err := strconv.Atoi(string(val))
				if err == nil {
					count = parsed
				}
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

			// Increment counter
			newCount := count + 1
			if err := rl.driver.Set(ctx, key, []byte(strconv.Itoa(newCount)), rl.ttlSeconds); err != nil {
				slog.Error("rate limit set error", slog.String("error", err.Error()))
				// Fail open - allow request through
				return next(c)
			}

			// Calculate remaining
			remaining := rl.requestsPerMinute - newCount
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))

			return next(c)
		}
	}
}

// RateLimit returns a rate limiting middleware with default settings
// This function is kept for backward compatibility but now requires a cache driver
func RateLimit(driver cache.Driver) echo.MiddlewareFunc {
	return NewRateLimiter(driver, DefaultRequestsPerMinute, RateLimitWindow).Middleware()
}

// RateLimitWithConfig returns a rate limiting middleware with custom configuration
func RateLimitWithConfig(driver cache.Driver, requestsPerMinute int, ttlSeconds int) echo.MiddlewareFunc {
	return NewRateLimiter(driver, requestsPerMinute, ttlSeconds).Middleware()
}

package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/cache"
	"github.com/labstack/echo/v4"
)

// AuthRateLimiter provides stricter rate limiting for authentication endpoints
// to prevent brute force attacks, credential stuffing, and account enumeration.
type AuthRateLimiter struct {
	driver            cache.Driver
	loginLimit        int // Max login attempts per window per IP+email
	loginWindow        int // Window in seconds for login attempts
	registerLimit      int // Max registrations per window per IP
	registerWindow     int // Window in seconds for registrations
	passwordResetLimit int // Max password reset requests per window per email
	passwordResetWindow int // Window in seconds for password resets
}

// AuthRateLimiterConfig holds configuration for auth-specific rate limiting
type AuthRateLimiterConfig struct {
	LoginAttemptsPerWindow      int // Default: 5 attempts per 15 min per IP+email
	LoginWindowSeconds          int // Default: 900 (15 min)
	RegisterAttemptsPerWindow   int // Default: 5 per hour per IP
	RegisterWindowSeconds       int // Default: 3600 (1 hour)
	PasswordResetAttemptsPerWindow int // Default: 3 per hour per email
	PasswordResetWindowSeconds     int // Default: 3600 (1 hour)
}

// DefaultAuthRateLimiterConfig returns sensible defaults for auth rate limiting
func DefaultAuthRateLimiterConfig() AuthRateLimiterConfig {
	return AuthRateLimiterConfig{
		LoginAttemptsPerWindow:      5,    // 5 attempts
		LoginWindowSeconds:         900,  // per 15 minutes
		RegisterAttemptsPerWindow:   5,    // 5 registrations
		RegisterWindowSeconds:      3600, // per hour
		PasswordResetAttemptsPerWindow: 3, // 3 resets
		PasswordResetWindowSeconds:     3600, // per hour
	}
}

// NewAuthRateLimiter creates a new auth rate limiter
func NewAuthRateLimiter(driver cache.Driver, config AuthRateLimiterConfig) *AuthRateLimiter {
	return &AuthRateLimiter{
		driver:            driver,
		loginLimit:        config.LoginAttemptsPerWindow,
		loginWindow:       config.LoginWindowSeconds,
		registerLimit:     config.RegisterAttemptsPerWindow,
		registerWindow:    config.RegisterWindowSeconds,
		passwordResetLimit: config.PasswordResetAttemptsPerWindow,
		passwordResetWindow: config.PasswordResetWindowSeconds,
	}
}

// LoginRateLimit returns middleware for login endpoint rate limiting
// Uses both IP and email for rate limiting (prevents distributed attacks)
func (rl *AuthRateLimiter) LoginRateLimit() echo.MiddlewareFunc {
	return rl.rateLimitByIPAndEmail("login", rl.loginLimit, rl.loginWindow)
}

// RegisterRateLimit returns middleware for registration endpoint rate limiting
// Uses IP-based rate limiting (prevents mass registration)
func (rl *AuthRateLimiter) RegisterRateLimit() echo.MiddlewareFunc {
	return rl.rateLimitByIP("register", rl.registerLimit, rl.registerWindow)
}

// PasswordResetRateLimit returns middleware for password reset endpoint rate limiting
// Uses email-based rate limiting (prevents abuse while allowing legitimate users)
func (rl *AuthRateLimiter) PasswordResetRateLimit() echo.MiddlewareFunc {
	return rl.rateLimitByEmail("password_reset", rl.passwordResetLimit, rl.passwordResetWindow)
}

// rateLimitByIP creates a rate limiter based on IP address
func (rl *AuthRateLimiter) rateLimitByIP(endpoint string, limit int, windowSeconds int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if rl.driver == nil {
				return next(c)
			}

			clientIP := c.RealIP()
			key := fmt.Sprintf("auth_rate_limit:%s:ip:%s", endpoint, clientIP)
			ctx := c.Request().Context()

			count, err := rl.incrementCounter(ctx, key, windowSeconds)
			if err != nil {
				slog.Error("auth rate limit cache error", "error", err, "endpoint", endpoint)
				// Fail open on cache error
				return next(c)
			}

			if count > limit {
				remaining := 0
				c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many requests. Please try again later.",
					},
				})
			}

			remaining := limit - count
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))

			return next(c)
		}
	}
}

// rateLimitByIPAndEmail creates a rate limiter based on both IP and email
// This prevents distributed brute force attacks
func (rl *AuthRateLimiter) rateLimitByIPAndEmail(endpoint string, limit int, windowSeconds int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if rl.driver == nil {
				return next(c)
			}

			// Parse email from request body for rate limiting
			// Note: We only check IP-based limit before the handler processes the body
			// The email-based limit is enforced after validating credentials
			clientIP := c.RealIP()
			ipKey := fmt.Sprintf("auth_rate_limit:%s:ip:%s", endpoint, clientIP)
			ctx := c.Request().Context()

			count, err := rl.incrementCounter(ctx, ipKey, windowSeconds)
			if err != nil {
				slog.Error("auth rate limit cache error", "error", err, "endpoint", endpoint)
				return next(c)
			}

			if count > limit {
				remaining := 0
				c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many login attempts from this IP. Please try again later.",
					},
				})
			}

			remaining := limit - count
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))

			return next(c)
		}
	}
}

// rateLimitByEmail creates a rate limiter based on email address
func (rl *AuthRateLimiter) rateLimitByEmail(endpoint string, limit int, windowSeconds int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// For password reset, the check happens in the handler after reading email
			// This middleware provides IP-based protection only
			if rl.driver == nil {
				return next(c)
			}

			clientIP := c.RealIP()
			key := fmt.Sprintf("auth_rate_limit:%s:ip:%s", endpoint, clientIP)
			ctx := c.Request().Context()

			count, err := rl.incrementCounter(ctx, key, windowSeconds)
			if err != nil {
				slog.Error("auth rate limit cache error", "error", err, "endpoint", endpoint)
				return next(c)
			}

			if count > limit {
				remaining := 0
				c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many password reset requests. Please try again later.",
					},
				})
			}

			remaining := limit - count
			if remaining < 0 {
				remaining = 0
			}
			c.Response().Header().Set(RateLimitRemainingHeader, strconv.Itoa(remaining))

			return next(c)
		}
	}
}

// incrementCounter increments the counter for a key and returns the new count
func (rl *AuthRateLimiter) incrementCounter(ctx context.Context, key string, windowSeconds int) (int, error) {
	var count int
	
	val, err := rl.driver.Get(ctx, key)
	if err != nil {
		// Key doesn't exist, start fresh
		count = 1
	} else {
		if val != nil {
			parsed, parseErr := strconv.Atoi(string(val))
			if parseErr != nil {
				count = 1
			} else {
				count = parsed + 1
			}
		} else {
			count = 1
		}
	}

	// Set the new count with TTL
	if err := rl.driver.Set(ctx, key, []byte(strconv.Itoa(count)), windowSeconds); err != nil {
		return 0, err
	}

	return count, nil
}

// CheckEmailRateLimit checks rate limit for a specific email (for use in handlers)
func (rl *AuthRateLimiter) CheckEmailRateLimit(ctx context.Context, endpoint, email string, limit int) (bool, error) {
	if rl.driver == nil {
		return true, nil
	}

	key := fmt.Sprintf("auth_rate_limit:%s:email:%s", endpoint, email)
	
	val, err := rl.driver.Get(ctx, key)
	if err != nil {
		// Key doesn't exist, allow
		return true, nil
	}

	if val == nil {
		return true, nil
	}

	count, parseErr := strconv.Atoi(string(val))
	if parseErr != nil {
		return true, nil
	}

	return count < limit, nil
}

// IncrementEmailRateLimit increments the rate limit counter for an email
func (rl *AuthRateLimiter) IncrementEmailRateLimit(ctx context.Context, endpoint, email string, windowSeconds int) error {
	if rl.driver == nil {
		return nil
	}

	key := fmt.Sprintf("auth_rate_limit:%s:email:%s", endpoint, email)
	
	var count int
	val, err := rl.driver.Get(ctx, key)
	if err != nil || val == nil {
		count = 1
	} else {
		parsed, parseErr := strconv.Atoi(string(val))
		if parseErr != nil {
			count = 1
		} else {
			count = parsed + 1
		}
	}

	return rl.driver.Set(ctx, key, []byte(strconv.Itoa(count)), windowSeconds)
}
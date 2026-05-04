// Package permission provides permission caching functionality.
package permission

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/example/go-api-base/internal/cache"
)

// CacheKeyPrefix is the prefix for all permission cache keys.
const CacheKeyPrefix = "perm:"

// ErrCacheMiss indicates that the requested key was not found in the cache.
var ErrCacheMiss = fmt.Errorf("cache miss")

// Cache provides cache-based caching for permission decisions.
type Cache struct {
	driver cache.Driver
	ttl    time.Duration
}

// NewCache creates a new permission cache with the specified TTL.
// Default TTL is 5 minutes if not specified.
//
// Performance Note:
//   - Expected latency: <10ms p99 for permission check operations
//   - Cache GET/SET operations are O(1) time complexity
//   - Cache invalidation uses driver-specific Clear for bulk operations
//   - Recommended TTL: 5 minutes (default) - balances security and performance
//   - Shorter TTL = more DB queries but faster permission propagation
//   - Longer TTL = fewer DB queries but slower permission propagation
func NewCache(driver cache.Driver, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Cache{
		driver: driver,
		ttl:    ttl,
	}
}

// Get retrieves a cached permission decision.
// Returns (allowed, ErrCacheMiss) on cache miss, (allowed, nil) on cache hit.
func (c *Cache) Get(ctx context.Context, key string) (bool, error) {
	result, err := c.driver.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get cached permission: %w", err)
	}

	if result == nil {
		// Cache miss
		return false, ErrCacheMiss
	}

	// Parse the result
	allowed := string(result) == "1"
	return allowed, nil
}

// Set stores a permission decision in the cache.
func (c *Cache) Set(ctx context.Context, key string, allowed bool) error {
	var value string
	if allowed {
		value = "1"
	} else {
		value = "0"
	}

	ttlSeconds := int(c.ttl.Seconds())
	if err := c.driver.Set(ctx, key, []byte(value), ttlSeconds); err != nil {
		return fmt.Errorf("failed to cache permission: %w", err)
	}

	slog.Debug("Permission cached",
		"key", key,
		"allowed", allowed,
		"ttl", c.ttl.String())

	return nil
}

// Invalidate removes a specific cached permission decision.
func (c *Cache) Invalidate(ctx context.Context, key string) error {
	if err := c.driver.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to invalidate cached permission: %w", err)
	}
	slog.Debug("Permission cache invalidated", "key", key)
	return nil
}

// InvalidateAll removes all permission cache entries.
// Uses driver-specific Clear method for bulk deletion.
func (c *Cache) InvalidateAll(ctx context.Context) error {
	pattern := CacheKeyPrefix + "*"

	if err := c.driver.Clear(ctx, pattern); err != nil {
		return fmt.Errorf("failed to clear permission cache: %w", err)
	}

	slog.Info("Permission cache invalidated",
		"pattern", pattern)

	return nil
}

// InvalidateForUser invalidates all cached permissions for a specific user.
// This is useful when user roles or permissions are changed.
func (c *Cache) InvalidateForUser(ctx context.Context, userID string) error {
	pattern := CacheKeyPrefix + userID + ":*"

	if err := c.driver.Clear(ctx, pattern); err != nil {
		return fmt.Errorf("failed to clear user permission cache: %w", err)
	}

	slog.Debug("User permission cache invalidated",
		"user_id", userID,
		"pattern", pattern)

	return nil
}

// InvalidateForDomain invalidates all cached permissions for a specific domain/tenant.
func (c *Cache) InvalidateForDomain(ctx context.Context, domain string) error {
	pattern := CacheKeyPrefix + "*:" + domain + ":*"

	if err := c.driver.Clear(ctx, pattern); err != nil {
		return fmt.Errorf("failed to clear domain permission cache: %w", err)
	}

	slog.Debug("Domain permission cache invalidated",
		"domain", domain,
		"pattern", pattern)

	return nil
}

// TTL returns the current cache TTL.
// Performance expectation: Permission checks with cache hit should complete <10ms p99.
func (c *Cache) TTL() time.Duration {
	return c.ttl
}

// SetTTL updates the cache TTL.
// Recommended values:
//   - Development: 1-5 minutes (faster testing cycles)
//   - Production: 5 minutes (default, balances security and performance)
//   - High-security: 1-2 minutes (faster permission propagation)
func (c *Cache) SetTTL(ttl time.Duration) {
	if ttl > 0 {
		c.ttl = ttl
	}
}

// Driver returns the underlying cache driver.
func (c *Cache) Driver() cache.Driver {
	return c.driver
}

// Helper function to parse TTL from Redis driver
func parseTTL(val []byte) (time.Duration, error) {
	seconds, err := strconv.Atoi(string(val))
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

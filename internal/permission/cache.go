// Package permission provides permission caching functionality.
package permission

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheKeyPrefix is the prefix for all permission cache keys.
const CacheKeyPrefix = "perm:"

// Cache provides Redis-based caching for permission decisions.
type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCache creates a new permission cache with the specified TTL.
// Default TTL is 5 minutes if not specified.
//
// Performance Note:
//   - Expected latency: <10ms p99 for permission check operations
//   - Redis GET/SET operations are O(1) time complexity
//   - Cache invalidation uses SCAN (non-blocking) to avoid production impact
//   - Recommended TTL: 5 minutes (default) - balances security and performance
//     - Shorter TTL = more DB queries but faster permission propagation
//     - Longer TTL = fewer DB queries but slower permission propagation
func NewCache(client *redis.Client, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Cache{
		client: client,
		ttl:    ttl,
	}
}

// Get retrieves a cached permission decision.
// Returns the decision and true if found, false and error if not found or on error.
func (c *Cache) Get(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss
			return false, nil
		}
		return false, fmt.Errorf("failed to get cached permission: %w", err)
	}

	// Parse the result
	allowed := result == "1"
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

	if err := c.client.Set(ctx, key, value, c.ttl).Err(); err != nil {
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
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to invalidate cached permission: %w", err)
	}
	slog.Debug("Permission cache invalidated", "key", key)
	return nil
}

// InvalidateAll removes all permission cache entries.
// Uses SCAN to avoid blocking on large keyspaces.
func (c *Cache) InvalidateAll(ctx context.Context) error {
	pattern := CacheKeyPrefix + "*"
	var cursor uint64
	var deletedCount int64

	for {
		// Scan for keys matching the pattern
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan permission cache keys: %w", err)
		}

		// Delete found keys
		if len(keys) > 0 {
			deleted, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				return fmt.Errorf("failed to delete permission cache keys: %w", err)
			}
			deletedCount += deleted
		}

		// Move to next cursor
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slog.Info("Permission cache invalidated",
		"deleted_count", deletedCount,
		"pattern", pattern)

	return nil
}

// InvalidateForUser invalidates all cached permissions for a specific user.
// This is useful when user roles or permissions are changed.
func (c *Cache) InvalidateForUser(ctx context.Context, userID string) error {
	pattern := CacheKeyPrefix + userID + ":*"
	var cursor uint64
	var deletedCount int64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan user permission cache keys: %w", err)
		}

		if len(keys) > 0 {
			deleted, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				return fmt.Errorf("failed to delete user permission cache keys: %w", err)
			}
			deletedCount += deleted
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slog.Debug("User permission cache invalidated",
		"user_id", userID,
		"deleted_count", deletedCount)

	return nil
}

// InvalidateForDomain invalidates all cached permissions for a specific domain/tenant.
func (c *Cache) InvalidateForDomain(ctx context.Context, domain string) error {
	pattern := CacheKeyPrefix + "*:" + domain + ":*"
	var cursor uint64
	var deletedCount int64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan domain permission cache keys: %w", err)
		}

		if len(keys) > 0 {
			deleted, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				return fmt.Errorf("failed to delete domain permission cache keys: %w", err)
			}
			deletedCount += deleted
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	slog.Debug("Domain permission cache invalidated",
		"domain", domain,
		"deleted_count", deletedCount)

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

// Client returns the underlying Redis client.
func (c *Cache) Client() *redis.Client {
	return c.client
}
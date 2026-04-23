package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache implements Driver using Redis
type redisCache struct {
	client *redis.Client
}

// newRedisCacheFromClient creates a Redis cache driver from an existing client
func newRedisCacheFromClient(client interface{}) *redisCache {
	if rc, ok := client.(*redis.Client); ok {
		return &redisCache{client: rc}
	}
	return &redisCache{client: nil}
}

// Get retrieves a value from Redis by key
func (c *redisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, errors.New("redis client not initialized")
	}

	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss - return nil without error
			return nil, nil
		}
		return nil, err
	}

	return []byte(result), nil
}

// Set stores a value in Redis with expiration
func (c *redisCache) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	if c.client == nil {
		return errors.New("redis client not initialized")
	}

	expiration := time.Duration(ttlSeconds) * time.Second
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Delete removes a key from Redis
func (c *redisCache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return errors.New("redis client not initialized")
	}

	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in Redis
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	if c.client == nil {
		return false, errors.New("redis client not initialized")
	}

	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Close is a no-op for Redis as the client should be managed externally
func (c *redisCache) Close() error {
	// Don't close the Redis client - it should be managed by the caller
	return nil
}

// Clear removes all keys matching the given pattern using SCAN
func (c *redisCache) Clear(ctx context.Context, pattern string) error {
	if c.client == nil {
		return errors.New("redis client not initialized")
	}

	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

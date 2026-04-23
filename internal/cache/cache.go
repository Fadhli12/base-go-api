package cache

import (
	"context"
	"fmt"
)

// Driver interface for cache backends
type Driver interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context, pattern string) error // Clear all keys matching pattern
	Close() error
}

// Config holds cache driver configuration
type Config struct {
	Driver        string // redis, memory, none
	DefaultTTL    int    // seconds
	PermissionTTL int    // seconds
	RateLimitTTL  int    // seconds
}

// NewDriver creates a cache driver based on configuration
func NewDriver(cfg Config, redisClient interface{}) (Driver, error) {
	switch cfg.Driver {
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis driver")
		}
		return newRedisCacheFromClient(redisClient), nil
	case "memory":
		return newMemoryCache(), nil
	case "none":
		return newNoopCache(), nil
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", cfg.Driver)
	}
}

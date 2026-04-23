// Package config provides application configuration management.
package config

// CacheConfig holds cache configuration.
// Supports Redis, in-memory, and disabled caching strategies.
type CacheConfig struct {
	Driver       string `mapstructure:"driver"`        // redis, memory, none
	DefaultTTL   int    `mapstructure:"default_ttl"`   // seconds
	PermissionTTL int   `mapstructure:"permission_ttl"` // seconds
	RateLimitTTL  int   `mapstructure:"rate_limit_ttl"` // seconds
}

// DefaultCacheConfig returns a CacheConfig with sensible defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Driver:        "redis",
		DefaultTTL:    300,
		PermissionTTL: 300,
		RateLimitTTL:  60,
	}
}
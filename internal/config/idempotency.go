// Package config provides application configuration management.
package config

import "time"

// IdempotencyConfig holds idempotency key system configuration.
type IdempotencyConfig struct {
	Enabled                bool          `mapstructure:"enabled"`
	MaxKeyLength           int           `mapstructure:"max_key_length"`
	KeyPattern            string        `mapstructure:"key_pattern"`
	DefaultTTL            time.Duration `mapstructure:"default_ttl"`
	MinTTL                time.Duration `mapstructure:"min_ttl"`
	MaxTTL                time.Duration `mapstructure:"max_ttl"`
	GuardTTL              time.Duration `mapstructure:"guard_ttl"`
	MaxCachedResponseSize int           `mapstructure:"max_cached_response_size"`
	ReaperInterval        time.Duration `mapstructure:"reaper_interval"`
	RetentionDays         int           `mapstructure:"retention_days"`
	DefaultPageSize       int           `mapstructure:"default_page_size"`
	MaxPageSize            int           `mapstructure:"max_page_size"`
}

// DefaultIdempotencyConfig returns an IdempotencyConfig with sensible defaults.
func DefaultIdempotencyConfig() IdempotencyConfig {
	return IdempotencyConfig{
		Enabled:                true,
		MaxKeyLength:           128,
		KeyPattern:            `^[a-zA-Z0-9_-]+$`,
		DefaultTTL:            24 * time.Hour,
		MinTTL:                1 * time.Hour,
		MaxTTL:                72 * time.Hour,
		GuardTTL:              5 * time.Minute,
		MaxCachedResponseSize: 4096, // 4KB
		ReaperInterval:        60 * time.Second,
		RetentionDays:        30,
		DefaultPageSize:      20,
		MaxPageSize:          100,
	}
}
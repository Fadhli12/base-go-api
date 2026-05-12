// Package config provides application configuration management.
package config

import "time"

// ActivityConfig holds activity feed system configuration.
type ActivityConfig struct {
	RetentionDays    int           `mapstructure:"retention_days"`
	DefaultPageSize  int           `mapstructure:"default_page_size"`
	MaxPageSize      int           `mapstructure:"max_page_size"`
	ReaperInterval   time.Duration `mapstructure:"reaper_interval"`
}

// DefaultActivityConfig returns an ActivityConfig with sensible defaults.
func DefaultActivityConfig() ActivityConfig {
	return ActivityConfig{
		RetentionDays:    90,
		DefaultPageSize:  20,
		MaxPageSize:      100,
		ReaperInterval:   60 * time.Second,
	}
}

package config

import "time"

// AnalyticsConfig holds configuration for the analytics dashboard feature.
type AnalyticsConfig struct {
	RetentionDays       int           `mapstructure:"retention_days"`
	AggregationInterval time.Duration `mapstructure:"aggregation_interval"`
	ReaperInterval      time.Duration `mapstructure:"reaper_interval"`
	DefaultPageSize    int           `mapstructure:"default_page_size"`
	MaxPageSize        int           `mapstructure:"max_page_size"`
}

// DefaultAnalyticsConfig returns the default analytics configuration.
func DefaultAnalyticsConfig() AnalyticsConfig {
	return AnalyticsConfig{
		RetentionDays:       90,
		AggregationInterval: 60 * time.Second,
		ReaperInterval:      60 * time.Second,
		DefaultPageSize:    20,
		MaxPageSize:        100,
	}
}
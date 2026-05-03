// Package config provides application configuration management.
package config

import "time"

// WebhookConfig holds webhook system configuration.
type WebhookConfig struct {
	WorkerConcurrency     int           `mapstructure:"worker_concurrency"`
	RetryMax              int           `mapstructure:"retry_max"`
	RateLimit             int           `mapstructure:"rate_limit"`
	AllowHTTP             bool          `mapstructure:"allow_http"`
	DeliveryTimeout       time.Duration `mapstructure:"delivery_timeout"`
	DeliveryRetentionDays int           `mapstructure:"delivery_retention_days"`
	MaxPayloadSize        int           `mapstructure:"max_payload_size"`
}

// DefaultWebhookConfig returns a WebhookConfig with sensible defaults.
func DefaultWebhookConfig() WebhookConfig {
	return WebhookConfig{
		WorkerConcurrency:     5,
		RetryMax:                3,
		RateLimit:               100,
		AllowHTTP:               false,
		DeliveryTimeout:         10 * time.Second,
		DeliveryRetentionDays:   90,
		MaxPayloadSize:          1048576,
	}
}

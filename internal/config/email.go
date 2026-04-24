// Package config provides application configuration management.
package config

// EmailConfig holds email service configuration.
// Supports SMTP (baseline), SendGrid, and SES providers.
type EmailConfig struct {
	Provider          string `mapstructure:"provider"`           // smtp, sendgrid, ses
	SMTPHost          string `mapstructure:"smtp_host"`          // SMTP server host
	SMTPPort          int    `mapstructure:"smtp_port"`          // SMTP server port
	SMTPUser          string `mapstructure:"smtp_user"`          // SMTP authentication user
	SMTPPassword      string `mapstructure:"smtp_password"`      // SMTP authentication password
	SMTPFromAddress   string `mapstructure:"smtp_from_address"`   // Default from address
	SMTPFromName      string `mapstructure:"smtp_from_name"`      // Default from name
	WorkerConcurrency int    `mapstructure:"worker_concurrency"` // Email queue workers (default: 10)
	RetryMax          int    `mapstructure:"retry_max"`          // Max retry attempts (default: 5)
	RateLimitPerHour  int    `mapstructure:"rate_limit_per_hour"` // Emails per hour per user (default: 100)
}

// DefaultEmailConfig returns an EmailConfig with sensible defaults.
func DefaultEmailConfig() EmailConfig {
	return EmailConfig{
		Provider:          "smtp",
		SMTPHost:          "localhost",
		SMTPPort:          587,
		WorkerConcurrency: 10,
		RetryMax:          5,
		RateLimitPerHour:  100,
	}
}
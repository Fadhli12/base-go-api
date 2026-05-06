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
	// SendGrid provider config (env: SENDGRID_API_KEY, SENDGRID_FROM_ADDRESS, SENDGRID_FROM_NAME)
	SendGridAPIKey      string `mapstructure:"sendgrid_api_key"`      // SendGrid API key
	SendGridFromAddress string `mapstructure:"sendgrid_from_address"` // SendGrid sender email
	SendGridFromName    string `mapstructure:"sendgrid_from_name"`    // SendGrid sender name
	// SES provider config (env: AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, SES_FROM_ADDRESS, SES_FROM_NAME)
	AWSRegion          string `mapstructure:"aws_region"`           // AWS region (e.g., us-east-1)
	AWSAccessKeyID     string `mapstructure:"aws_access_key_id"`    // AWS IAM access key ID
	AWSSecretAccessKey string `mapstructure:"aws_secret_access_key"` // AWS IAM secret access key
	SESFromAddress     string `mapstructure:"ses_from_address"`     // SES verified sender email
	SESFromName        string `mapstructure:"ses_from_name"`        // SES sender display name
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
		SendGridAPIKey:    "",
		SendGridFromAddress: "",
		SendGridFromName:    "",
		AWSRegion:           "",
		AWSAccessKeyID:      "",
		AWSSecretAccessKey:  "",
		SESFromAddress:      "",
		SESFromName:         "",
	}
}
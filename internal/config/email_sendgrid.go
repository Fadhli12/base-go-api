// Package config provides application configuration management.
package config

// SendGridConfig holds SendGrid email provider configuration.
// Fields map to SENDGRID_* environment variables via Viper.
type SendGridConfig struct {
	APIKey      string `mapstructure:"api_key"`      // SENDGRID_API_KEY - SendGrid API key for authentication
	FromAddress string `mapstructure:"from_address"`  // SENDGRID_FROM_ADDRESS - Default sender email address
	FromName    string `mapstructure:"from_name"`     // SENDGRID_FROM_NAME - Default sender display name
}

// DefaultSendGridConfig returns a SendGridConfig with sensible defaults.
func DefaultSendGridConfig() SendGridConfig {
	return SendGridConfig{
		APIKey:      "",
		FromAddress: "",
		FromName:    "",
	}
}

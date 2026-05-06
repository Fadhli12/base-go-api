// Package config provides application configuration management.
package config

// SESConfig holds AWS SES email provider configuration.
// Fields map to SES_* and AWS_* environment variables via Viper.
type SESConfig struct {
	Region          string `mapstructure:"region"`           // AWS_REGION - AWS region for SES (e.g., us-east-1)
	AccessKeyID     string `mapstructure:"access_key_id"`    // AWS_ACCESS_KEY_ID - AWS IAM access key
	SecretAccessKey string `mapstructure:"secret_access_key"` // AWS_SECRET_ACCESS_KEY - AWS IAM secret key
	FromAddress     string `mapstructure:"from_address"`     // SES_FROM_ADDRESS - Default sender email address (must be verified in SES)
	FromName        string `mapstructure:"from_name"`        // SES_FROM_NAME - Default sender display name
}

// DefaultSESConfig returns an SESConfig with sensible defaults.
func DefaultSESConfig() SESConfig {
	return SESConfig{
		Region:          "",
		AccessKeyID:     "",
		SecretAccessKey: "",
		FromAddress:     "",
		FromName:        "",
	}
}

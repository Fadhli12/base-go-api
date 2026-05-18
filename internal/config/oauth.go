// Package config provides application configuration management.
package config

import "time"

// OAuthConfig holds OAuth-related configuration.
type OAuthConfig struct {
	// EncryptionKey is the master key for encrypting OAuth client secrets.
	// If empty, derives from JWT_SECRET using HKDF-SHA256 (info: "oauth-encryption-v1").
	EncryptionKey string `mapstructure:"encryption_key"`

	// AllowHTTP controls whether HTTP URLs are allowed for redirect URLs.
	// Default: false (HTTPS only). Set to true for local development only.
	AllowHTTP bool `mapstructure:"allow_http"`

	// FrontendCallbackURL is the default frontend URL for OAuth callback redirects.
	// Per-provider redirect_url takes precedence if set.
	FrontendCallbackURL string `mapstructure:"frontend_callback_url"`

	// StateTTL is the time-to-live for OAuth state parameters stored in Redis.
	// Default: 600 seconds (10 minutes).
	StateTTL time.Duration `mapstructure:"state_ttl"`

	// StateTTLSeconds is StateTTL in seconds (for Redis TTL integer).
	StateTTLSeconds int `mapstructure:"state_ttl_seconds"`
}

// DefaultOAuthConfig returns an OAuthConfig with sensible defaults.
func DefaultOAuthConfig() OAuthConfig {
	return OAuthConfig{
		AllowHTTP:           false,
		FrontendCallbackURL: "http://localhost:3000/auth/callback",
		StateTTLSeconds:    600,
		StateTTL:           600 * time.Second,
	}
}
// Package config provides application configuration management.
package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/ssrf"
	"github.com/spf13/viper"
)

// SSRFConfig holds SSRF protection configuration.
// Maps environment variables (SSRF_*) to the internal ssrf.SSRFConfig struct.
type SSRFConfig struct {
	AllowPrivateIPs bool          `mapstructure:"allow_private_ips"`
	AllowedSchemes  []string      `mapstructure:"allowed_schemes"`
	BlockedHosts    []string      `mapstructure:"blocked_hosts"`
	BlockedCIDRs    []string      `mapstructure:"blocked_cidrs"`
	AllowedCIDRs    []string      `mapstructure:"allowed_cidrs"`
	Timeout         time.Duration `mapstructure:"timeout"`
}

// DefaultSSRFConfig returns a production-secure SSRFConfig with sensible defaults.
// These defaults match the internal ssrf.DefaultSSRFConfig() values.
func DefaultSSRFConfig() SSRFConfig {
	return SSRFConfig{
		AllowPrivateIPs: false,
		AllowedSchemes:  []string{"https"},
		BlockedHosts: []string{
			"metadata.internal",
			"metadata.google.internal",
			"instance-data.ec2.internal",
		},
		BlockedCIDRs: []string{
			"127.0.0.0/8",
			"::1/128",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"169.254.0.0/16",
			"fe80::/10",
			"169.254.169.254/32",
			"100.64.0.0/10",
			"0.0.0.0/8",
			"::/128",
		},
		AllowedCIDRs: []string{},
		Timeout:      30 * time.Second,
	}
}

// ToInternal converts the config SSRFConfig to the internal ssrf.SSRFConfig.
func (c SSRFConfig) ToInternal() ssrf.SSRFConfig {
	return ssrf.SSRFConfig{
		AllowPrivateIPs: c.AllowPrivateIPs,
		AllowedSchemes:   c.AllowedSchemes,
		BlockedHosts:     c.BlockedHosts,
		BlockedCIDRs:     c.BlockedCIDRs,
		AllowedCIDRs:     c.AllowedCIDRs,
		Timeout:          c.Timeout,
	}
}

// parseSSRFConfig parses SSRF configuration from environment variables via Viper.
func parseSSRFConfig(v *viper.Viper, cfg *Config) error {
	cfg.SSRF = DefaultSSRFConfig()

	// AllowPrivateIPs
	cfg.SSRF.AllowPrivateIPs = v.GetBool("ssrf.allow_private_ips")
	if cfg.SSRF.AllowPrivateIPs {
		slog.Warn("SSRF protection: private IPs are ALLOWED via configuration",
			"allow_private_ips", true,
			"component", "ssrf",
		)
	}

	// Map WebhookConfig.AllowHTTP to SSRFConfig.AllowedSchemes
	// If webhook allows HTTP, the SSRF schemes must include both http and https.
	// If not, only https is allowed (default).
	if v.GetBool("webhook.allow_http") {
		cfg.SSRF.AllowedSchemes = []string{"https", "http"}
	}

	// AllowedSchemes (comma-separated) — overrides the AllowHTTP default if explicitly set
	if schemes := v.GetString("ssrf.allowed_schemes"); schemes != "" {
		cfg.SSRF.AllowedSchemes = strings.Split(schemes, ",")
		for i := range cfg.SSRF.AllowedSchemes {
			cfg.SSRF.AllowedSchemes[i] = strings.TrimSpace(cfg.SSRF.AllowedSchemes[i])
		}
	}

	// BlockedHosts (comma-separated)
	if hosts := v.GetString("ssrf.blocked_hosts"); hosts != "" {
		cfg.SSRF.BlockedHosts = strings.Split(hosts, ",")
		for i := range cfg.SSRF.BlockedHosts {
			cfg.SSRF.BlockedHosts[i] = strings.TrimSpace(cfg.SSRF.BlockedHosts[i])
		}
	}

	// BlockedCIDRs (comma-separated)
	if cidrs := v.GetString("ssrf.blocked_cidrs"); cidrs != "" {
		cfg.SSRF.BlockedCIDRs = strings.Split(cidrs, ",")
		for i := range cfg.SSRF.BlockedCIDRs {
			cfg.SSRF.BlockedCIDRs[i] = strings.TrimSpace(cfg.SSRF.BlockedCIDRs[i])
		}
	}

	// AllowedCIDRs (comma-separated)
	if cidrs := v.GetString("ssrf.allowed_cidrs"); cidrs != "" {
		cfg.SSRF.AllowedCIDRs = strings.Split(cidrs, ",")
		for i := range cfg.SSRF.AllowedCIDRs {
			cfg.SSRF.AllowedCIDRs[i] = strings.TrimSpace(cfg.SSRF.AllowedCIDRs[i])
		}
	}

	// Timeout
	if timeout := v.GetString("ssrf.timeout"); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid ssrf.timeout: %w", err)
		}
		cfg.SSRF.Timeout = d
	}

	// Validate CIDR formats
	internal := cfg.SSRF.ToInternal()
	if err := internal.ParseCIDRs(); err != nil {
		return fmt.Errorf("invalid SSRF CIDR configuration: %w", err)
	}

	return nil
}
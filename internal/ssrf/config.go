// Package ssrf provides Server-Side Request Forgery (SSRF) protection for outbound HTTP requests.
//
// It implements a two-layer defense:
// 1. URL validation at registration time (early rejection of obviously invalid URLs)
// 2. Dial-time IP validation in custom HTTP Transport (DNS rebinding prevention)
//
// The package is designed to be used as a standalone dependency with zero service-layer
// imports. Create a validator with NewValidator() and an HTTP client with NewClient().
package ssrf

import (
	"net"
	"strings"
	"time"
)

// SSRFConfig holds immutable SSRF protection configuration.
// Once created via DefaultSSRFConfig() or explicit construction, it must not be mutated.
// Thread-safe by design (no setters, no shared mutable state).
type SSRFConfig struct {
	// AllowPrivateIPs enables development mode where private/internal IPs are allowed.
	// When true, a warning is logged on validator creation. Default: false.
	AllowPrivateIPs bool `mapstructure:"allow_private_ips"`

	// AllowedSchemes is the list of URL schemes permitted (e.g., ["https"] or ["https", "http"]).
	// Default: ["https"] (HTTPS-only).
	AllowedSchemes []string `mapstructure:"allowed_schemes"`

	// BlockedHosts is a list of hostnames that are always blocked, regardless of
	// DNS resolution. Matching is case-insensitive and includes subdomain matching
	// (e.g., blocking "metadata.internal" also blocks "sub.metadata.internal").
	// Default: ["metadata.internal", "metadata.google.internal", "instance-data.ec2.internal"]
	BlockedHosts []string `mapstructure:"blocked_hosts"`

	// BlockedCIDRs is a list of CIDR ranges that are always blocked.
	// Parsed from string format (e.g., "10.0.0.0/8") into net.IPNet at construction time.
	// Default: RFC1918 private ranges + loopback + link-local + cloud metadata + CGNAT + unspecified.
	BlockedCIDRs []string `mapstructure:"blocked_cidrs"`

	// AllowedCIDRs is a list of CIDR ranges that are allowed even if they fall within blocked ranges.
	// This enables whitelisting specific IPs within otherwise-blocked CIDRs.
	// AllowedCIDRs override ALL blocking, including Go's built-in IsLoopback/IsPrivate/IsLinkLocal checks.
	// For example, AllowedCIDRs: ["127.0.0.1/32"] will allow loopback access.
	// Parsed from string format at construction time.
	// Default: empty (no overrides).
	AllowedCIDRs []string `mapstructure:"allowed_cidrs"`

	// Timeout is the HTTP client timeout for requests made through the SSRF-safe client.
	// Default: 30 seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// Parsed blocked CIDRs (internal, not serialized).
	blockedNets []*net.IPNet

	// Parsed allowed CIDRs (internal, not serialized).
	allowedNets []*net.IPNet

	// Normalized blocked hosts (lowercase, for case-insensitive matching).
	blockedHostsNormalized []string
}

// DefaultSSRFConfig returns a production-secure SSRFConfig with sensible defaults.
//
// Defaults:
//   - AllowPrivateIPs: false (secure by default)
//   - AllowedSchemes: ["https"] (HTTPS-only)
//   - BlockedHosts: cloud metadata endpoints
//   - BlockedCIDRs: RFC1918 + loopback + link-local + cloud metadata + CGNAT + unspecified
//   - AllowedCIDRs: empty (no overrides)
//   - Timeout: 30 seconds
func DefaultSSRFConfig() SSRFConfig {
	cfg := SSRFConfig{
		AllowPrivateIPs: false,
		AllowedSchemes:  []string{"https"},
		BlockedHosts: []string{
			"metadata.internal",
			"metadata.google.internal",
			"instance-data.ec2.internal",
		},
		BlockedCIDRs: []string{
			// Loopback
			"127.0.0.0/8",
			"::1/128",
			// RFC1918 private
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			// Link-local
			"169.254.0.0/16",
			"fe80::/10",
			// Cloud metadata (AWS, GCP, Azure)
			"169.254.169.254/32",
			// Carrier-grade NAT (CGNAT)
			"100.64.0.0/10",
			// Unspecified addresses
			"0.0.0.0/8",
			"::/128",
		},
		AllowedCIDRs: []string{},
		Timeout:      30 * time.Second,
	}

	// Must parse before use — errors in defaults are fatal.
	if err := cfg.parseCIDRs(); err != nil {
		panic("ssrf: failed to parse default CIDRs: " + err.Error())
	}

	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)

	return cfg
}

// ParseCIDRs parses BlockedCIDRs and AllowedCIDRs string slices into net.IPNet slices.
// This is the exported version of parseCIDRs for external use (e.g., config validation).
// Returns an error if any CIDR string is malformed.
func (c *SSRFConfig) ParseCIDRs() error {
	return c.parseCIDRs()
}

// parseCIDRs parses BlockedCIDRs and AllowedCIDRs string slices into net.IPNet slices.
// Returns an error if any CIDR string is malformed.
func (c *SSRFConfig) parseCIDRs() error {
	var err error

	c.blockedNets, err = parseCIDRList(c.BlockedCIDRs)
	if err != nil {
		return err
	}

	c.allowedNets, err = parseCIDRList(c.AllowedCIDRs)
	if err != nil {
		return err
	}

	return nil
}

// normalizeHosts converts hostnames to lowercase for case-insensitive matching.
func normalizeHosts(hosts []string) []string {
	normalized := make([]string, len(hosts))
	for i, h := range hosts {
		normalized[i] = strings.ToLower(strings.TrimSpace(h))
	}
	return normalized
}

// parseCIDRList parses a list of CIDR strings into net.IPNet slices.
func parseCIDRList(cidrs []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		nets = append(nets, ipNet)
	}
	return nets, nil
}
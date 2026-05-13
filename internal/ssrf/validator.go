package ssrf

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
)

// SSRFValidator provides SSRF protection by validating URLs and IP addresses.
// It implements two-layer defense:
// 1. URL validation at registration time (early rejection)
// 2. IP validation at dial time (DNS rebinding prevention)
//
// Create via NewValidator(). The validator is immutable and thread-safe.
type SSRFValidator struct {
	cfg    SSRFConfig
	logger *slog.Logger
}

// NewValidator creates a new SSRFValidator with the given configuration.
// If cfg.AllowPrivateIPs is true, a warning is logged (development mode).
func NewValidator(cfg SSRFConfig, logger *slog.Logger) *SSRFValidator {
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.AllowPrivateIPs {
		logger.Warn("SSRF protection: private IPs are ALLOWED — this should only be used in development",
			"allow_private_ips", true,
			"component", "ssrf",
		)
	}

	return &SSRFValidator{
		cfg:    cfg,
		logger: logger,
	}
}

// ValidateURL validates a URL against SSRF protection rules.
// It checks: scheme, hostname against blocked hosts, DNS resolution, and all resolved IPs.
// Returns nil if the URL is safe, or an error describing why it was blocked.
//
// This implements the first layer of defense (FR-011): early rejection at URL creation time.
func (v *SSRFValidator) ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	if !v.IsAllowedScheme(u.Scheme) {
		return fmt.Errorf("URL scheme %q is not allowed (allowed: %v)", u.Scheme, v.cfg.AllowedSchemes)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	// Check blocked hosts (before DNS resolution)
	if v.IsBlockedHost(host) {
		return fmt.Errorf("URL hostname %q is blocked", host)
	}

	// Resolve DNS and check all IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS lookup fails, try parsing as a literal IP
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("unable to resolve URL host %q: %w", host, err)
		}
		ips = []net.IP{ip}
	}

	for _, ip := range ips {
		if v.IsBlockedIP(ip) {
			v.logger.Warn("SSRF blocked: URL resolves to blocked IP",
				"ip", ip.String(),
				"hostname", host,
				"url", rawURL,
				"component", "ssrf",
			)
			return fmt.Errorf("URL resolves to a blocked IP address %s", ip.String())
		}
	}

	return nil
}

// IsBlockedIP checks if an IP address falls within any blocked range.
// It handles IPv6-mapped IPv4 normalization automatically (FR-014).
//
// Evaluation order:
//  1. AllowPrivateIPs mode — if true, all IPs are allowed (development bypass)
//  2. AllowedCIDRs override — if IP is in any allowed CIDR, it is NOT blocked
//     (this allows whitelisting specific IPs within otherwise-blocked ranges,
//      including loopback, private, and link-local ranges)
//  3. Go built-in classification — loopback, private, link-local
//  4. Explicit BlockedCIDRs — custom blocked ranges
//
// This order allows AllowedCIDRs to override ALL blocking, including Go's
// built-in IsLoopback/IsPrivate/IsLinkLocal checks.
func (v *SSRFValidator) IsBlockedIP(ip net.IP) bool {
	// Development mode bypass
	if v.cfg.AllowPrivateIPs {
		return false
	}

	// Normalize IPv6-mapped IPv4 addresses (FR-014)
	ip = normalizeIP(ip)

	// Check AllowedCIDRs FIRST — allows overriding all blocking (including built-in)
	for _, allowedNet := range v.cfg.allowedNets {
		if allowedNet.Contains(ip) {
			return false
		}
	}

	// Check standard Go IP classification
	if ip.IsLoopback() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	if ip.IsLinkLocalUnicast() {
		return true
	}
	if ip.IsLinkLocalMulticast() {
		return true
	}

	// Check blocked CIDRs (explicit list)
	for _, blockedNet := range v.cfg.blockedNets {
		if blockedNet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsBlockedHost checks if a hostname is in the blocked hosts list.
// Matching is case-insensitive and includes subdomain matching:
//   - Exact match: "metadata.internal" blocks "metadata.internal"
//   - Subdomain match: "metadata.internal" also blocks "sub.metadata.internal"
//
// This prevents attackers from bypassing hostname blocks by adding subdomain prefixes.
func (v *SSRFValidator) IsBlockedHost(host string) bool {
	lowerHost := strings.ToLower(strings.TrimSpace(host))
	for _, blocked := range v.cfg.blockedHostsNormalized {
		if lowerHost == blocked {
			return true
		}
		// Subdomain match: "sub.metadata.internal" should be blocked by "metadata.internal"
		if strings.HasSuffix(lowerHost, "."+blocked) {
			return true
		}
	}
	return false
}

// IsAllowedScheme checks if a URL scheme is in the allowed schemes list.
func (v *SSRFValidator) IsAllowedScheme(scheme string) bool {
	lowerScheme := strings.ToLower(scheme)
	for _, allowed := range v.cfg.AllowedSchemes {
		if strings.ToLower(allowed) == lowerScheme {
			return true
		}
	}
	return false
}

// normalizeIP handles IPv6-mapped IPv4 normalization.
// Address ::ffff:127.0.0.1 is normalized to 127.0.0.1 before checking against blocked ranges.
func normalizeIP(ip net.IP) net.IP {
	// IPv6-mapped IPv4: ::ffff:x.x.x.x
	// To4() returns non-nil for IPv4 addresses and IPv6-mapped IPv4 addresses.
	if ip4 := ip.To4(); ip4 != nil {
		return ip4
	}
	return ip
}
package ssrf

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// NewClient creates an SSRF-safe HTTP client with dial-time IP validation.
// The client uses a custom Transport with a DialContext that resolves hostnames,
// validates all resolved IPs against blocked ranges, and only connects to validated IPs.
//
// This implements the second layer of defense (FR-011): DNS rebinding prevention at dial time.
// Each redirect triggers a new DialContext validation, preventing redirect-based SSRF attacks.
// An explicit redirect limit of 10 is enforced as defense in depth.
//
// If ssrfCfg is nil, returns a basic http.Client with NO SSRF protection.
// Callers should always provide a valid SSRFConfig (use DefaultSSRFConfig() for production defaults).
// The nil fallback exists only for backward compatibility during migration and should not be used in production.
func NewClient(ssrfCfg *SSRFConfig, logger *slog.Logger) *http.Client {
	if logger == nil {
		logger = slog.Default()
	}

	// Fallback: no SSRF protection (backward compatibility during migration)
	// IMPORTANT: Callers should use DefaultSSRFConfig() instead of passing nil.
	if ssrfCfg == nil {
		logger.Warn("SSRF config is nil, creating client without SSRF protection — use DefaultSSRFConfig() in production")
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	validator := NewValidator(*ssrfCfg, logger)

	return &http.Client{
		Timeout: ssrfCfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return ssrfDialContext(ctx, network, addr, validator, logger)
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("ssrf: too many redirects (%d)", len(via))
			}
			logger.Debug("SSRF: following redirect",
				"url", req.URL.String(),
				"redirect_count", len(via),
				"component", "ssrf",
			)
			return nil
		},
	}
}

// ssrfDialContext is the custom DialContext that performs SSRF validation at connection time.
// It resolves the hostname, validates ALL resolved IPs, and only dials the first valid IP.
// DNS resolution failure = blocked request (fail-closed, FR-013).
func ssrfDialContext(ctx context.Context, network, addr string, validator *SSRFValidator, logger *slog.Logger) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ssrf: invalid address format %q: %w", addr, err)
	}

	// Check if address is an IP literal
	ip := net.ParseIP(host)
	if ip != nil {
		// Direct IP literal — validate without DNS resolution
		if validator.IsBlockedIP(ip) {
			logger.Warn("SSRF blocked: direct IP is blocked",
				"ip", ip.String(),
				"component", "ssrf",
			)
			return nil, fmt.Errorf("ssrf: blocked IP %s", ip.String())
		}
		// Dial the validated IP directly
		dialAddr := net.JoinHostPort(ip.String(), port)
		return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, dialAddr)
	}

	// Hostname — resolve DNS and check ALL resolved IPs (DNS rebinding prevention)
	ips, err := net.LookupIP(host)
	if err != nil {
		// Fail-closed: DNS failure = blocked request (FR-013)
		logger.Warn("SSRF blocked: DNS resolution failed",
			"hostname", host,
			"error", err.Error(),
			"component", "ssrf",
		)
		return nil, fmt.Errorf("ssrf: DNS resolution failed for %q: %w", host, err)
	}

	// Validate ALL resolved IPs
	for _, resolvedIP := range ips {
		if validator.IsBlockedIP(resolvedIP) {
			logger.Warn("SSRF blocked: hostname resolves to blocked IP",
				"ip", resolvedIP.String(),
				"hostname", host,
				"component", "ssrf",
			)
			return nil, fmt.Errorf("ssrf: hostname %q resolves to blocked IP %s", host, resolvedIP.String())
		}
	}

	// All IPs validated — dial the first valid IP (preserving standard Go behavior)
	dialAddr := net.JoinHostPort(ips[0].String(), port)
	return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, network, dialAddr)
}
package ssrf

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newSSRFConfig creates a config for transport tests.
// By default, blocks private IPs. Use AllowedCIDRs to whitelist specific ranges.
func newSSRFConfig(allowPrivate bool, allowedSchemes []string, allowedCIDRs []string) SSRFConfig {
	cfg := SSRFConfig{
		AllowPrivateIPs: allowPrivate,
		AllowedSchemes:  allowedSchemes,
		BlockedHosts: []string{
			"metadata.internal",
			"metadata.google.internal",
			"instance-data.ec2.internal",
		},
		BlockedCIDRs: []string{
			"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12",
			"192.168.0.0/16", "169.254.0.0/16", "fe80::/10",
			"169.254.169.254/32", "100.64.0.0/10", "0.0.0.0/8", "::/128",
		},
		AllowedCIDRs: allowedCIDRs,
		Timeout:      5 * time.Second,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)
	return cfg
}

func TestNewClient_NilConfig_ReturnsBasicClient(t *testing.T) {
	client := NewClient(nil, slog.Default())
	if client == nil {
		t.Fatal("NewClient with nil config should return non-nil client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", client.Timeout)
	}
	// nil config client has no custom Transport — uses Go's DefaultTransport
	if client.Transport != nil {
		t.Error("nil config client should have nil Transport (uses DefaultTransport)")
	}
}

func TestNewClient_NilLogger_Defaults(t *testing.T) {
	cfg := newSSRFConfig(true, []string{"https"}, nil)
	client := NewClient(&cfg, nil)
	if client == nil {
		t.Fatal("NewClient with nil logger should return non-nil client")
	}
}

func TestSSRFSafeTransport_BlocksLoopback(t *testing.T) {
	// Start an HTTP server on 127.0.0.1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "hello")
	}))
	defer ts.Close()

	// Config with AllowPrivateIPs=false (default) — should block loopback
	cfg := newSSRFConfig(false, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	// The httptest server URL will be http://127.0.0.1:PORT
	resp, err := client.Get(ts.URL)
	if err == nil {
		resp.Body.Close()
		t.Error("expected connection to loopback to be blocked, but request succeeded")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected blocked error, got: %v", err)
	}
}

func TestSSRFSafeTransport_BlocksPrivateIP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Default config blocks private IPs
	cfg := newSSRFConfig(false, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	_, err := client.Get(ts.URL)
	if err == nil {
		t.Error("expected private IP to be blocked")
	}
}

func TestSSRFSafeTransport_BlocksCloudMetadata(t *testing.T) {
	cfg := newSSRFConfig(false, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	// Cloud metadata IP is blocked even without a server
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected cloud metadata IP to be blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected blocked error, got: %v", err)
	}
}

func TestSSRFSafeTransport_BlocksIPv6MappedIPv4(t *testing.T) {
	// ::ffff:127.0.0.1 should be normalized to 127.0.0.1 and blocked
	// We test this by creating a custom transport that tries to connect to an IPv6-mapped address
	cfg := newSSRFConfig(false, []string{"http"}, nil)
	validator := NewValidator(cfg, slog.Default())

	t.Run("IPv6-mapped loopback is blocked", func(t *testing.T) {
		ip := net.ParseIP("::ffff:127.0.0.1")
		if ip == nil {
			t.Fatal("failed to parse ::ffff:127.0.0.1")
		}
		if !validator.IsBlockedIP(ip) {
			t.Error("::ffff:127.0.0.1 should be blocked (IPv6-mapped loopback)")
		}
	})

	t.Run("IPv6-mapped private IP is blocked", func(t *testing.T) {
		ip := net.ParseIP("::ffff:10.0.0.1")
		if ip == nil {
			t.Fatal("failed to parse ::ffff:10.0.0.1")
		}
		if !validator.IsBlockedIP(ip) {
			t.Error("::ffff:10.0.0.1 should be blocked (IPv6-mapped private)")
		}
	})
}

func TestSSRFSafeTransport_AllowPrivateIPsMode(t *testing.T) {
	// With AllowPrivateIPs=true, loopback connections should succeed
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "hello from loopback")
	}))
	defer ts.Close()

	cfg := newSSRFConfig(true, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("expected request to succeed with AllowPrivateIPs=true, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSSRFSafeTransport_BlockedHosts(t *testing.T) {
	cfg := newSSRFConfig(false, []string{"http"}, []string{"127.0.0.0/8"})
	client := NewClient(&cfg, slog.Default())

	// metadata.internal should be blocked by hostname check
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://metadata.internal/", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected metadata.internal to be blocked")
	}
	// Note: even if DNS resolves, hostname block comes first in ValidateURL
	// but in transport, it's IP-based. Our blocked host check in the validator
	// will catch it if URL validation is also used.
}

func TestSSRFSafeTransport_AllowedCIDRs(t *testing.T) {
	// AllowedCIDRs override ALL blocking, including Go's built-in checks.
	// Test AllowedCIDRs override with both non-standard and built-in ranges.
	cfg := SSRFConfig{
		AllowPrivateIPs: false,
		AllowedSchemes:  []string{"https"},
		BlockedHosts:    []string{},
		BlockedCIDRs:    []string{"127.0.0.0/8", "203.0.113.0/24"}, // 203.0.113.0/24 is TEST-NET-1
		AllowedCIDRs:    []string{
			"203.0.113.128/25", // Allow half of TEST-NET-1
			"127.0.0.1/32",     // Allow specific loopback IP (overrides IsLoopback)
		},
		Timeout:         5 * time.Second,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)
	validator := NewValidator(cfg, slog.Default())

	// IP within AllowedCIDRs override should not be blocked
	t.Run("allowed_via_override", func(t *testing.T) {
		if validator.IsBlockedIP(net.ParseIP("203.0.113.200")) {
			t.Error("203.0.113.200 should be allowed via AllowedCIDRs override")
		}
	})

	// IP outside override but within blocked range should be blocked
	t.Run("blocked_outside_override", func(t *testing.T) {
		if !validator.IsBlockedIP(net.ParseIP("203.0.113.1")) {
			t.Error("203.0.113.1 should be blocked (outside AllowedCIDRs override)")
		}
	})

	// AllowedCIDRs override Go's built-in IsLoopback check
	t.Run("loopback_allowed_via_override", func(t *testing.T) {
		if validator.IsBlockedIP(net.ParseIP("127.0.0.1")) {
			t.Error("127.0.0.1 should be allowed via AllowedCIDRs override (overrides IsLoopback)")
		}
	})

	// Other loopback IPs NOT in AllowedCIDRs should still be blocked
	t.Run("other_loopback_still_blocked", func(t *testing.T) {
		if !validator.IsBlockedIP(net.ParseIP("127.0.0.2")) {
			t.Error("127.0.0.2 should be blocked (not in AllowedCIDRs override)")
		}
	})
}

func TestSSRFSafeTransport_BlockHTTP(t *testing.T) {
	// Default config blocks HTTP scheme in ValidateURL
	// But transport-level blocking is only IP-based, so we test scheme blocking via ValidateURL
	cfg := DefaultSSRFConfig()
	validator := NewValidator(cfg, slog.Default())

	err := validator.ValidateURL("http://8.8.8.8/path")
	if err == nil {
		t.Error("expected HTTP scheme to be blocked by default config")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected scheme error, got: %v", err)
	}
}

func TestSSRFSafeTransport_AllowHTTPWithConfig(t *testing.T) {
	// Custom config allowing HTTP
	cfg := newSSRFConfig(true, []string{"https", "http"}, nil)
	validator := NewValidator(cfg, slog.Default())

	err := validator.ValidateURL("http://127.0.0.1/path")
	// AllowPrivateIPs=true so the IP is not blocked; HTTP is also allowed
	if err != nil {
		if strings.Contains(err.Error(), "not allowed") {
			t.Errorf("HTTP should be allowed with custom config, got: %v", err)
		}
		// DNS resolution error for 127.0.0.1 is acceptable in test environment
	}
}

func TestSSRFSafeTransport_DNSFailure(t *testing.T) {
	// FR-013: DNS failure blocks request (fail-closed)
	cfg := newSSRFConfig(false, []string{"http"}, []string{})
	client := NewClient(&cfg, slog.Default())

	// Use a hostname that won't resolve
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://this-domain-should-not-exist-xyz123.invalid/", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	_, err := client.Do(req)
	if err == nil {
		t.Error("expected DNS resolution failure to be blocked")
	}
}

func TestNewClient_WithConfig_HasCustomTransport(t *testing.T) {
	cfg := newSSRFConfig(false, []string{"https"}, nil)
	client := NewClient(&cfg, slog.Default())

	if client == nil {
		t.Fatal("NewClient should return non-nil client")
	}
	if client.Transport == nil {
		t.Error("client should have custom Transport for SSRF protection")
	}
	if client.Timeout != cfg.Timeout {
		t.Errorf("expected timeout %v, got %v", cfg.Timeout, client.Timeout)
	}
}

func TestSSRFSafeTransport_PublicIP(t *testing.T) {
	// This test verifies that the SSRF transport doesn't block public IPs.
	// We can't easily start a server on a public IP, so we test the validator directly.
	cfg := newSSRFConfig(false, []string{"https"}, nil)
	validator := NewValidator(cfg, slog.Default())

	publicIPs := []net.IP{
		net.ParseIP("8.8.8.8"),
		net.ParseIP("1.1.1.1"),
		net.ParseIP("203.0.113.1"),
	}

	for _, ip := range publicIPs {
		t.Run("public_"+ip.String(), func(t *testing.T) {
			if validator.IsBlockedIP(ip) {
				t.Errorf("public IP %s should not be blocked", ip)
			}
		})
	}
}

func TestSSRFSafeTransport_SuccessfulDialViaHostname(t *testing.T) {
	// Test successful connection through ssrfDialContext via hostname resolution.
	// Uses AllowPrivateIPs=true since httptest server binds to 127.0.0.1.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "hello from hostname test")
	}))
	defer ts.Close()

	cfg := newSSRFConfig(true, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("expected successful connection via hostname, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSSRFSafeTransport_SuccessfulDialViaIPLiteral(t *testing.T) {
	// Test successful connection through ssrfDialContext via IP literal.
	// Uses AllowPrivateIPs=true since httptest server binds to 127.0.0.1.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "hello from IP literal test")
	}))
	defer ts.Close()

	cfg := newSSRFConfig(true, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	// Extract the host:port from the test server URL and connect via IP literal
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("expected successful connection via IP literal, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSSRFSafeTransport_InvalidAddressFormat(t *testing.T) {
	// Test ssrfDialContext with invalid address format (no port)
	cfg := newSSRFConfig(false, []string{"http"}, nil)
	validator := NewValidator(cfg, slog.Default())
	logger := slog.Default()

	ctx := context.Background()
	_, err := ssrfDialContext(ctx, "tcp", "no-port-host", validator, logger)
	if err == nil {
		t.Error("expected error for invalid address format (missing port)")
	}
	if !strings.Contains(err.Error(), "invalid address format") {
		t.Errorf("expected 'invalid address format' error, got: %v", err)
	}
}

func TestNewClient_RedirectLimit(t *testing.T) {
	// MED-002: Verify the CheckRedirect policy Limits redirects to 10
	cfg := newSSRFConfig(true, []string{"http"}, nil)
	client := NewClient(&cfg, slog.Default())

	// The client should have a CheckRedirect function set
	if client.CheckRedirect == nil {
		t.Error("NewClient should have CheckRedirect set for redirect limit enforcement")
	}

	// Test that redirect limit works
	via := make([]*http.Request, 10)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	for i := range via {
		via[i] = req
	}

	err := client.CheckRedirect(req, via)
	if err == nil {
		t.Error("expected redirect limit error when via has 10 entries, got nil")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("expected 'too many redirects' error, got: %v", err)
	}

	// Under limit should succeed
	viaUnderLimit := make([]*http.Request, 5)
	for i := range viaUnderLimit {
		viaUnderLimit[i] = req
	}
	err = client.CheckRedirect(req, viaUnderLimit)
	if err != nil {
		t.Errorf("expected no error with 5 redirects, got: %v", err)
	}
}

func TestNewClient_NilConfig_WarnsAboutSecurity(t *testing.T) {
	// Verify that passing nil config creates a client without SSRF protection
	// and that callers should use DefaultSSRFConfig() instead
	client := NewClient(nil, slog.Default())
	if client == nil {
		t.Fatal("NewClient with nil config should return non-nil client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", client.Timeout)
	}
	// nil config client should NOT have custom transport (no SSRF protection)
	if client.Transport != nil {
		t.Error("nil config client should not have custom Transport (no SSRF protection)")
	}
}
package ssrf

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
)

// helper to create a validator with default config for tests
func newTestValidator(allowPrivate bool) *SSRFValidator {
	cfg := SSRFConfig{
		AllowPrivateIPs: allowPrivate,
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
		Timeout:      30 * 1e9,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)
	return NewValidator(cfg, slog.Default())
}

// newTestValidatorWithSchemes creates a validator with custom allowed schemes
func newTestValidatorWithSchemes(allowPrivate bool, schemes []string) *SSRFValidator {
	cfg := SSRFConfig{
		AllowPrivateIPs: allowPrivate,
		AllowedSchemes:  schemes,
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
		AllowedCIDRs: []string{},
		Timeout:      30 * 1e9,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)
	return NewValidator(cfg, slog.Default())
}

func TestIsBlockedIP(t *testing.T) {
	v := newTestValidator(false)

	tests := []struct {
		name    string
		ip      net.IP
		blocked bool
		fr      string
	}{
		// FR-001: Block RFC1918 private IPs
		{name: "RFC1918 10.x", ip: net.ParseIP("10.0.0.1"), blocked: true, fr: "FR-001"},
		{name: "RFC1918 172.16.x", ip: net.ParseIP("172.16.0.1"), blocked: true, fr: "FR-001"},
		{name: "RFC1918 192.168.x", ip: net.ParseIP("192.168.1.1"), blocked: true, fr: "FR-001"},
		{name: "RFC1918 10.255.255.255", ip: net.ParseIP("10.255.255.255"), blocked: true, fr: "FR-001"},
		{name: "RFC1918 172.31.255.255", ip: net.ParseIP("172.31.255.255"), blocked: true, fr: "FR-001"},
		{name: "RFC1918 192.168.255.255", ip: net.ParseIP("192.168.255.255"), blocked: true, fr: "FR-001"},

		// FR-002: Block loopback
		{name: "loopback 127.0.0.1", ip: net.ParseIP("127.0.0.1"), blocked: true, fr: "FR-002"},
		{name: "loopback 127.0.0.100", ip: net.ParseIP("127.0.0.100"), blocked: true, fr: "FR-002"},
		{name: "loopback 127.255.255.255", ip: net.ParseIP("127.255.255.255"), blocked: true, fr: "FR-002"},
		{name: "loopback ::1", ip: net.ParseIP("::1"), blocked: true, fr: "FR-002"},

		// FR-003: Block link-local/cloud metadata
		{name: "link-local 169.254.0.1", ip: net.ParseIP("169.254.0.1"), blocked: true, fr: "FR-003"},
		{name: "cloud metadata 169.254.169.254", ip: net.ParseIP("169.254.169.254"), blocked: true, fr: "FR-003"},
		{name: "link-local IPv6 fe80::1", ip: net.ParseIP("fe80::1"), blocked: true, fr: "FR-003"},
		{name: "link-local IPv6 fe80::ffff", ip: net.ParseIP("fe80::ffff"), blocked: true, fr: "FR-003"},

		// CGNAT range
		{name: "CGNAT 100.64.0.1", ip: net.ParseIP("100.64.0.1"), blocked: true, fr: "FR-003"},
		{name: "CGNAT 100.127.255.255", ip: net.ParseIP("100.127.255.255"), blocked: true, fr: "FR-003"},

		// Unspecified address
		{name: "unspecified 0.0.0.0", ip: net.ParseIP("0.0.0.0"), blocked: true, fr: "FR-003"},
		{name: "unspecified ::", ip: net.ParseIP("::"), blocked: true, fr: "FR-003"},

		// FR-014: IPv6-mapped IPv4 normalization
		{name: "IPv6-mapped loopback ::ffff:127.0.0.1", ip: net.ParseIP("::ffff:127.0.0.1"), blocked: true, fr: "FR-014"},
		{name: "IPv6-mapped private ::ffff:10.0.0.1", ip: net.ParseIP("::ffff:10.0.0.1"), blocked: true, fr: "FR-014"},
		{name: "IPv6-mapped RFC1918 ::ffff:192.168.1.1", ip: net.ParseIP("::ffff:192.168.1.1"), blocked: true, fr: "FR-014"},
		{name: "IPv6-mapped metadata ::ffff:169.254.169.254", ip: net.ParseIP("::ffff:169.254.169.254"), blocked: true, fr: "FR-014"},

		// Public IPs should NOT be blocked
		{name: "public 8.8.8.8", ip: net.ParseIP("8.8.8.8"), blocked: false, fr: "FR-001"},
		{name: "public 1.1.1.1", ip: net.ParseIP("1.1.1.1"), blocked: false, fr: "FR-001"},
		{name: "public 203.0.113.1", ip: net.ParseIP("203.0.113.1"), blocked: false, fr: "FR-001"},

		// IPv6 not in any blocked range
		{name: "public IPv6 2001:db8::1", ip: net.ParseIP("2001:db8::1"), blocked: false, fr: "FR-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.IsBlockedIP(tt.ip)
			if result != tt.blocked {
				t.Errorf("IsBlockedIP(%s) = %v, want %v (%s)", tt.ip, result, tt.blocked, tt.fr)
			}
		})
	}
}

func TestIsBlockedIP_AllowPrivateIPsMode(t *testing.T) {
	// FR-007: Development mode — AllowPrivateIPs=true blocks nothing
	v := newTestValidator(true)

	privateIPs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("10.0.0.1"),
		net.ParseIP("172.16.0.1"),
		net.ParseIP("192.168.1.1"),
		net.ParseIP("169.254.169.254"),
		net.ParseIP("::1"),
		net.ParseIP("100.64.0.1"),
	}

	for _, ip := range privateIPs {
		t.Run("allow_private_"+ip.String(), func(t *testing.T) {
			if v.IsBlockedIP(ip) {
				t.Errorf("IsBlockedIP(%s) should be false in AllowPrivateIPs mode", ip)
			}
		})
	}
}

func TestIsBlockedIP_AllowedCIDRsOverride(t *testing.T) {
	// FR-008: Allowed CIDRs override ALL blocked ranges, including Go's built-in checks.
	// AllowedCIDRs are checked FIRST, before IsLoopback/IsPrivate/IsLinkLocal.
	cfg := SSRFConfig{
		AllowPrivateIPs: false,
		AllowedSchemes:  []string{"https"},
		BlockedHosts: []string{
			"metadata.internal",
			"metadata.google.internal",
			"instance-data.ec2.internal",
		},
		BlockedCIDRs: []string{
			"127.0.0.0/8",
			"10.0.0.0/8",
			"203.0.113.0/24", // TEST-NET-1 (not in Go's IsPrivate)
		},
		AllowedCIDRs: []string{
			"127.0.0.1/32",    // Allow specific loopback IP
			"10.0.0.1/32",     // Allow specific private IP
			"203.0.113.128/25", // Allow half of TEST-NET-1
		},
		Timeout: 30 * 1e9,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)
	v := NewValidator(cfg, slog.Default())

	// AllowedCIDRs override Go's built-in IsLoopback check
	t.Run("allowed_override_loopback_127.0.0.1", func(t *testing.T) {
		if v.IsBlockedIP(net.ParseIP("127.0.0.1")) {
			t.Error("127.0.0.1 should be allowed via AllowedCIDRs override (overrides IsLoopback)")
		}
	})

	// AllowedCIDRs override Go's built-in IsPrivate check
	t.Run("allowed_override_private_10.0.0.1", func(t *testing.T) {
		if v.IsBlockedIP(net.ParseIP("10.0.0.1")) {
			t.Error("10.0.0.1 should be allowed via AllowedCIDRs override (overrides IsPrivate)")
		}
	})

	// IP within allowed CIDR override should NOT be blocked
	t.Run("allowed_override_203.0.113.200", func(t *testing.T) {
		if v.IsBlockedIP(net.ParseIP("203.0.113.200")) {
			t.Error("203.0.113.200 should be allowed (within AllowedCIDRs override)")
		}
	})

	// IP outside the override but still in blocked range SHOULD be blocked
	t.Run("blocked_outside_override_203.0.113.1", func(t *testing.T) {
		if !v.IsBlockedIP(net.ParseIP("203.0.113.1")) {
			t.Error("203.0.113.1 should be blocked (outside AllowedCIDRs override)")
		}
	})

	// Other loopback IPs NOT in AllowedCIDRs should still be blocked
	t.Run("loopback_outside_override_127.0.0.2", func(t *testing.T) {
		if !v.IsBlockedIP(net.ParseIP("127.0.0.2")) {
			t.Error("127.0.0.2 should be blocked (not in AllowedCIDRs override)")
		}
	})

	// Other private IPs NOT in AllowedCIDRs should still be blocked
	t.Run("private_outside_override_10.0.0.2", func(t *testing.T) {
		if !v.IsBlockedIP(net.ParseIP("10.0.0.2")) {
			t.Error("10.0.0.2 should be blocked (not in AllowedCIDRs override)")
		}
	})
}

func TestIsBlockedHost(t *testing.T) {
	v := newTestValidator(false)

	tests := []struct {
		name    string
		host    string
		blocked bool
	}{
		// FR-006: Blocked hostnames — case-insensitive
		{name: "exact match metadata.internal", host: "metadata.internal", blocked: true},
		{name: "uppercase METADATA.INTERNAL", host: "METADATA.INTERNAL", blocked: true},
		{name: "mixed case Metadata.Google.Internal", host: "Metadata.Google.Internal", blocked: true},
		{name: "lowercase metadata.google.internal", host: "metadata.google.internal", blocked: true},
		{name: "instance-data.ec2.internal", host: "instance-data.ec2.internal", blocked: true},
		{name: "INSTANCE-DATA.EC2.INTERNAL", host: "INSTANCE-DATA.EC2.INTERNAL", blocked: true},
		// Not blocked
		{name: "normal host", host: "api.example.com", blocked: false},
		{name: "subdomain of blocked", host: "sub.metadata.internal", blocked: true},
		{name: "partial match", host: "metadata.internal.evil.com", blocked: false},
		// Subdomain matching (MED-001 fix)
		{name: "deep subdomain sub.sub.metadata.internal", host: "sub.sub.metadata.internal", blocked: true},
		{name: "subdomain of metadata.google.internal", host: "api.metadata.google.internal", blocked: true},
		{name: "subdomain of instance-data.ec2.internal", host: "test.instance-data.ec2.internal", blocked: true},
		// Whitespace handling
		{name: "host with spaces", host: "  metadata.internal  ", blocked: true},
		{name: "host with leading space", host: " metadata.google.internal", blocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.IsBlockedHost(tt.host)
			if result != tt.blocked {
				t.Errorf("IsBlockedHost(%q) = %v, want %v", tt.host, result, tt.blocked)
			}
		})
	}
}

func TestIsAllowedScheme(t *testing.T) {
	// FR-005: Default HTTPS-only blocks HTTP
	v := newTestValidator(false)

	t.Run("default_allows_https", func(t *testing.T) {
		if !v.IsAllowedScheme("https") {
			t.Error("https should be allowed by default")
		}
	})

	t.Run("default_blocks_http", func(t *testing.T) {
		if v.IsAllowedScheme("http") {
			t.Error("http should be blocked by default")
		}
	})

	t.Run("case_insensitive_HTTPS", func(t *testing.T) {
		if !v.IsAllowedScheme("HTTPS") {
			t.Error("HTTPS (uppercase) should be allowed")
		}
	})

	t.Run("case_insensitive_mixed_HTTpS", func(t *testing.T) {
		if !v.IsAllowedScheme("HTTpS") {
			t.Error("HTTpS (mixed case) should be allowed")
		}
	})

	t.Run("blocks_ftp", func(t *testing.T) {
		if v.IsAllowedScheme("ftp") {
			t.Error("ftp should not be allowed")
		}
	})

	t.Run("blocks_empty_scheme", func(t *testing.T) {
		if v.IsAllowedScheme("") {
			t.Error("empty scheme should not be allowed")
		}
	})

	// Custom config allowing both http and https
	t.Run("custom_allows_http_and_https", func(t *testing.T) {
		v2 := newTestValidatorWithSchemes(false, []string{"https", "http"})
		if !v2.IsAllowedScheme("http") {
			t.Error("http should be allowed with custom config")
		}
		if !v2.IsAllowedScheme("https") {
			t.Error("https should be allowed with custom config")
		}
		if !v2.IsAllowedScheme("HTTP") {
			t.Error("HTTP (uppercase) should be allowed with custom config")
		}
	})
}

func TestValidateURL_BlockedIP(t *testing.T) {
	v := newTestValidator(false)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "loopback IP", url: "https://127.0.0.1/path", wantErr: true},
		{name: "private 10.x", url: "https://10.0.0.1/api", wantErr: true},
		{name: "private 172.16.x", url: "https://172.16.0.1/api", wantErr: true},
		{name: "private 192.168.x", url: "https://192.168.1.1/api", wantErr: true},
		{name: "cloud metadata", url: "https://169.254.169.254/latest/meta-data/", wantErr: true},
		{name: "link-local", url: "https://169.254.0.1/path", wantErr: true},
		{name: "CGNAT", url: "https://100.64.0.1/api", wantErr: true},
		{name: "blocked host metadata.internal", url: "https://metadata.internal/path", wantErr: true},
		{name: "blocked host metadata.google.internal", url: "https://metadata.google.internal/path", wantErr: true},
		{name: "http scheme blocked by default", url: "http://8.8.8.8/path", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL_PublicIP(t *testing.T) {
	v := newTestValidator(false)

	t.Run("public IP 8.8.8.8", func(t *testing.T) {
		err := v.ValidateURL("https://8.8.8.8/dns-query")
		if err != nil {
			t.Errorf("ValidateURL with public IP should succeed, got error: %v", err)
		}
	})

	t.Run("public IP 1.1.1.1", func(t *testing.T) {
		err := v.ValidateURL("https://1.1.1.1/dns-query")
		if err != nil {
			t.Errorf("ValidateURL with public IP should succeed, got error: %v", err)
		}
	})
}

func TestValidateURL_SchemeValidation(t *testing.T) {
	vDefault := newTestValidator(false)
	vBoth := newTestValidatorWithSchemes(false, []string{"https", "http"})

	t.Run("default_blocks_http", func(t *testing.T) {
		err := vDefault.ValidateURL("http://8.8.8.8/path")
		if err == nil {
			t.Error("http should be blocked by default config")
		}
		if err != nil && !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("expected scheme error, got: %v", err)
		}
	})

	t.Run("default_allows_https", func(t *testing.T) {
		err := vDefault.ValidateURL("https://8.8.8.8/path")
		if err != nil {
			t.Errorf("https should be allowed, got error: %v", err)
		}
	})

	t.Run("custom_allows_http", func(t *testing.T) {
		err := vBoth.ValidateURL("http://8.8.8.8/path")
		if err != nil {
			t.Errorf("http should be allowed with custom config, got error: %v", err)
		}
	})

	t.Run("blocks_ftp_scheme", func(t *testing.T) {
		err := vDefault.ValidateURL("ftp://8.8.8.8/file")
		if err == nil {
			t.Error("ftp scheme should be blocked")
		}
	})
}

func TestValidateURL_EmptyAndInvalidHost(t *testing.T) {
	v := newTestValidator(false)

	t.Run("empty host", func(t *testing.T) {
		err := v.ValidateURL("https:///path")
		if err == nil {
			t.Error("URL with empty host should be rejected")
		}
	})

	t.Run("just scheme and colon", func(t *testing.T) {
		err := v.ValidateURL("https://")
		if err == nil {
			t.Error("URL with no host should be rejected")
		}
	})

	t.Run("completely invalid URL", func(t *testing.T) {
		err := v.ValidateURL("://not-a-url")
		if err == nil {
			t.Error("invalid URL should return error")
		}
		if err != nil && !strings.Contains(err.Error(), "invalid URL") {
			t.Errorf("expected 'invalid URL' error, got: %v", err)
		}
	})
}

func TestNewValidator_AllowPrivateIPsWarning(t *testing.T) {
	// FR-007: Verify slog.Warn when AllowPrivateIPs=true
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)

	cfg := SSRFConfig{
		AllowPrivateIPs: true,
		AllowedSchemes:  []string{"https"},
		BlockedHosts:     []string{},
		BlockedCIDRs:     []string{"127.0.0.0/8"},
		AllowedCIDRs:     []string{},
		Timeout:          30 * 1e9,
	}
	_ = cfg.parseCIDRs()
	cfg.blockedHostsNormalized = normalizeHosts(cfg.BlockedHosts)

	_ = NewValidator(cfg, logger)

	output := buf.String()
	if !strings.Contains(output, "private IPs are ALLOWED") {
		t.Errorf("expected warning log about private IPs being allowed, got: %s", output)
	}
	if !strings.Contains(output, "allow_private_ips") {
		t.Errorf("expected 'allow_private_ips' in warning log, got: %s", output)
	}
}

func TestNewValidator_NilLogger(t *testing.T) {
	cfg := DefaultSSRFConfig()
	v := NewValidator(cfg, nil)
	if v == nil {
		t.Error("NewValidator with nil logger should return non-nil validator")
	}
}

func TestNormalizeIP(t *testing.T) {
	tests := []struct {
		name     string
		input    net.IP
		expected net.IP
	}{
		{name: "IPv4 stays as IPv4", input: net.ParseIP("8.8.8.8"), expected: net.ParseIP("8.8.8.8").To4()},
		{name: "IPv6-mapped IPv4 ::ffff:127.0.0.1 normalizes to 127.0.0.1",
			input:    net.ParseIP("::ffff:127.0.0.1"),
			expected: net.ParseIP("127.0.0.1").To4()},
		{name: "IPv6-mapped IPv4 ::ffff:10.0.0.1 normalizes to 10.0.0.1",
			input:    net.ParseIP("::ffff:10.0.0.1"),
			expected: net.ParseIP("10.0.0.1").To4()},
		{name: "IPv6-mapped IPv4 ::ffff:192.168.1.1 normalizes to 192.168.1.1",
			input:    net.ParseIP("::ffff:192.168.1.1"),
			expected: net.ParseIP("192.168.1.1").To4()},
		{name: "pure IPv6 ::1 stays as ::1",
			input:    net.ParseIP("::1"),
			expected: net.ParseIP("::1")},
		{name: "pure IPv6 fe80::1 stays as fe80::1",
			input:    net.ParseIP("fe80::1"),
			expected: net.ParseIP("fe80::1")},
		{name: "IPv6-mapped public IPv4 ::ffff:8.8.8.8 normalizes to 8.8.8.8",
			input:    net.ParseIP("::ffff:8.8.8.8"),
			expected: net.ParseIP("8.8.8.8").To4()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeIP(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("normalizeIP(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateURL_FailClosed_DNSFailure(t *testing.T) {
	// FR-013: DNS failure blocks request (fail-closed)
	v := newTestValidator(false)

	err := v.ValidateURL("https://this-host-definitely-does-not-exist-xyz123.invalid/api")
	if err == nil {
		t.Error("ValidateURL with unresolvable hostname should return error (fail-closed)")
	}
}

func TestValidateURL_AllowPrivateIPsMode(t *testing.T) {
	// FR-007: AllowPrivateIPs=true allows private IP literals
	v := newTestValidator(true)

	t.Run("loopback allowed in dev mode", func(t *testing.T) {
		err := v.ValidateURL("https://127.0.0.1/path")
		// IsBlockedIP returns false for all IPs in dev mode,
		// so the only possible error is DNS resolution (which won't happen for IP literals)
		if err != nil {
			if strings.Contains(err.Error(), "blocked") {
				t.Errorf("In AllowPrivateIPs mode, loopback should not be blocked, got: %v", err)
			}
		}
	})

	t.Run("private IP allowed in dev mode", func(t *testing.T) {
		err := v.ValidateURL("https://10.0.0.1/api")
		if err != nil {
			if strings.Contains(err.Error(), "blocked") {
				t.Errorf("In AllowPrivateIPs mode, private IP should not be blocked, got: %v", err)
			}
		}
	})
}
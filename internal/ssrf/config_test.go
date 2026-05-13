package ssrf

import (
	"net"
	"testing"
	"time"
)

func TestDefaultSSRFConfig(t *testing.T) {
	cfg := DefaultSSRFConfig()

	t.Run("AllowPrivateIPs defaults to false", func(t *testing.T) {
		if cfg.AllowPrivateIPs {
			t.Error("AllowPrivateIPs should be false (production-secure)")
		}
	})

	t.Run("AllowedSchemes defaults to HTTPS-only", func(t *testing.T) {
		if len(cfg.AllowedSchemes) != 1 || cfg.AllowedSchemes[0] != "https" {
			t.Errorf("AllowedSchemes should be [\"https\"], got %v", cfg.AllowedSchemes)
		}
	})

	t.Run("BlockedHosts includes metadata endpoints", func(t *testing.T) {
		expectedHosts := []string{
			"metadata.internal",
			"metadata.google.internal",
			"instance-data.ec2.internal",
		}
		if len(cfg.BlockedHosts) != len(expectedHosts) {
			t.Fatalf("BlockedHosts should have %d entries, got %d", len(expectedHosts), len(cfg.BlockedHosts))
		}
		for i, expected := range expectedHosts {
			if cfg.BlockedHosts[i] != expected {
				t.Errorf("BlockedHosts[%d]: expected %q, got %q", i, expected, cfg.BlockedHosts[i])
			}
		}
	})

	t.Run("Timeout defaults to 30s", func(t *testing.T) {
		if cfg.Timeout != 30*time.Second {
			t.Errorf("Timeout should be 30s, got %v", cfg.Timeout)
		}
	})

	t.Run("AllowedCIDRs defaults to empty", func(t *testing.T) {
		if len(cfg.AllowedCIDRs) != 0 {
			t.Errorf("AllowedCIDRs should be empty, got %v", cfg.AllowedCIDRs)
		}
	})

	t.Run("blockedHostsNormalized are lowercase", func(t *testing.T) {
		for _, h := range cfg.blockedHostsNormalized {
			for _, r := range h {
				if r >= 'A' && r <= 'Z' {
					t.Errorf("blockedHostsNormalized should be lowercase, got %q", h)
				}
			}
		}
	})
}

func TestDefaultSSRFConfigBlockedCIDRs(t *testing.T) {
	cfg := DefaultSSRFConfig()

	// Define the expected CIDR blocks
	expectedCIDRs := []struct {
		cidr   string
		desc   string
		sample net.IP
	}{
		// Loopback
		{cidr: "127.0.0.0/8", desc: "loopback IPv4", sample: net.ParseIP("127.0.0.1")},
		{cidr: "::1/128", desc: "loopback IPv6", sample: net.ParseIP("::1")},
		// RFC1918 private
		{cidr: "10.0.0.0/8", desc: "RFC1918 10.x", sample: net.ParseIP("10.0.0.1")},
		{cidr: "172.16.0.0/12", desc: "RFC1918 172.16-31", sample: net.ParseIP("172.16.0.1")},
		{cidr: "192.168.0.0/16", desc: "RFC1918 192.168", sample: net.ParseIP("192.168.1.1")},
		// Link-local
		{cidr: "169.254.0.0/16", desc: "link-local IPv4", sample: net.ParseIP("169.254.0.1")},
		{cidr: "fe80::/10", desc: "link-local IPv6", sample: net.ParseIP("fe80::1")},
		// Cloud metadata
		{cidr: "169.254.169.254/32", desc: "cloud metadata", sample: net.ParseIP("169.254.169.254")},
		// CGNAT
		{cidr: "100.64.0.0/10", desc: "CGNAT", sample: net.ParseIP("100.64.0.1")},
		// Unspecified
		{cidr: "0.0.0.0/8", desc: "unspecified IPv4", sample: net.ParseIP("0.0.0.0")},
		{cidr: "::/128", desc: "unspecified IPv6", sample: net.ParseIP("::")},
	}

	if len(cfg.blockedNets) != len(expectedCIDRs) {
		t.Fatalf("expected %d blocked nets, got %d", len(expectedCIDRs), len(cfg.blockedNets))
	}

	for _, tc := range expectedCIDRs {
		t.Run(tc.desc, func(t *testing.T) {
			found := false
			for _, n := range cfg.blockedNets {
				if n.Contains(tc.sample) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("blocked nets should contain %s (%s), sample IP %s not matched", tc.desc, tc.cidr, tc.sample)
			}
		})
	}

	// Verify public IPs are NOT blocked by default
	publicIPs := []struct {
		name string
		ip   net.IP
	}{
		{"Google DNS", net.ParseIP("8.8.8.8")},
		{"Cloudflare DNS", net.ParseIP("1.1.1.1")},
	}
	for _, tc := range publicIPs {
		t.Run("public_"+tc.name, func(t *testing.T) {
			for _, n := range cfg.blockedNets {
				if n.Contains(tc.ip) {
					t.Errorf("public IP %s should NOT be in blocked CIDR %s", tc.ip, n)
				}
			}
		})
	}
}

func TestSSRFConfigParseCIDRs(t *testing.T) {
	t.Run("valid CIDR strings parse correctly", func(t *testing.T) {
		cfg := SSRFConfig{
			BlockedCIDRs: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			AllowedCIDRs: []string{"10.10.0.0/16"},
		}
		err := cfg.parseCIDRs()
		if err != nil {
			t.Fatalf("expected no error parsing valid CIDRs, got: %v", err)
		}
		if len(cfg.blockedNets) != 3 {
			t.Errorf("expected 3 blockedNets, got %d", len(cfg.blockedNets))
		}
		if len(cfg.allowedNets) != 1 {
			t.Errorf("expected 1 allowedNet, got %d", len(cfg.allowedNets))
		}
	})

	t.Run("invalid BlockedCIDRs returns error", func(t *testing.T) {
		cfg := SSRFConfig{
			BlockedCIDRs: []string{"not-a-valid-cidr"},
		}
		err := cfg.parseCIDRs()
		if err == nil {
			t.Error("expected error for invalid CIDR, got nil")
		}
	})

	t.Run("invalid AllowedCIDRs returns error", func(t *testing.T) {
		cfg := SSRFConfig{
			BlockedCIDRs: []string{"10.0.0.0/8"},
			AllowedCIDRs: []string{"also-not-valid"},
		}
		err := cfg.parseCIDRs()
		if err == nil {
			t.Error("expected error for invalid allowed CIDR, got nil")
		}
	})

	t.Run("empty CIDR lists parse successfully", func(t *testing.T) {
		cfg := SSRFConfig{
			BlockedCIDRs: []string{},
			AllowedCIDRs: []string{},
		}
		err := cfg.parseCIDRs()
		if err != nil {
			t.Fatalf("expected no error for empty CIDR lists, got: %v", err)
		}
		if len(cfg.blockedNets) != 0 {
			t.Errorf("expected 0 blockedNets, got %d", len(cfg.blockedNets))
		}
		if len(cfg.allowedNets) != 0 {
			t.Errorf("expected 0 allowedNets, got %d", len(cfg.allowedNets))
		}
	})
}

func TestSSRFConfigCustomAllowedSchemes(t *testing.T) {
	t.Run("custom allowed schemes with http and https", func(t *testing.T) {
		cfg := SSRFConfig{
			AllowedSchemes: []string{"https", "http"},
			BlockedCIDRs:   []string{},
			AllowedCIDRs:   []string{},
		}
		if err := cfg.parseCIDRs(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.AllowedSchemes) != 2 {
			t.Errorf("expected 2 schemes, got %d", len(cfg.AllowedSchemes))
		}
	})

	t.Run("empty allowed schemes", func(t *testing.T) {
		cfg := SSRFConfig{
			AllowedSchemes: []string{},
			BlockedCIDRs:   []string{},
			AllowedCIDRs:   []string{},
		}
		if err := cfg.parseCIDRs(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.AllowedSchemes) != 0 {
			t.Errorf("expected 0 schemes, got %d", len(cfg.AllowedSchemes))
		}
	})
}

func TestSSRFConfigAllowedCIDRsOverride(t *testing.T) {
	t.Run("allowed CIDR overrides custom blocked range", func(t *testing.T) {
		// Use a custom blocked CIDR that is NOT a standard Go private/loopback range.
		// Standard Go checks (IsLoopback, IsPrivate, IsLinkLocal) always block
		// regardless of AllowedCIDRs. AllowedCIDRs only override explicit BlockedCIDRs.
		cfg := SSRFConfig{
			AllowPrivateIPs: false,
			BlockedCIDRs:    []string{"203.0.113.0/24"},         // TEST-NET-1 (not Go IsPrivate)
			AllowedCIDRs:    []string{"203.0.113.128/25"},       // Override half of it
		}
		if err := cfg.parseCIDRs(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		validator := NewValidator(cfg, nil)

		// IP within allowed override should NOT be blocked
		allowedIP := net.ParseIP("203.0.113.200")
		if validator.IsBlockedIP(allowedIP) {
			t.Error("203.0.113.200 should be allowed due to AllowedCIDRs override")
		}

		// IP outside the override but within the blocked range SHOULD be blocked
		blockedIP := net.ParseIP("203.0.113.1")
		if !validator.IsBlockedIP(blockedIP) {
			t.Error("203.0.113.1 should be blocked (outside AllowedCIDRs override)")
		}
	})
}

func TestParseCIDRList(t *testing.T) {
	t.Run("parses valid CIDRs", func(t *testing.T) {
		cidrs := []string{"10.0.0.0/8", "192.168.0.0/16", "::1/128"}
		nets, err := parseCIDRList(cidrs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(nets) != 3 {
			t.Errorf("expected 3 nets, got %d", len(nets))
		}
	})

	t.Run("returns error for invalid CIDR", func(t *testing.T) {
		cidrs := []string{"10.0.0.0/8", "not-valid"}
		_, err := parseCIDRList(cidrs)
		if err == nil {
			t.Error("expected error for invalid CIDR, got nil")
		}
	})

	t.Run("returns empty slice for empty input", func(t *testing.T) {
		nets, err := parseCIDRList([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(nets) != 0 {
			t.Errorf("expected 0 nets, got %d", len(nets))
		}
	})
}

func TestDefaultSSRFConfig_PanicOnInvalidCIDRs(t *testing.T) {
	// DefaultSSRFConfig panics if default CIDRs are invalid.
	// This test verifies that DefaultSSRFConfig does NOT panic with valid defaults.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DefaultSSRFConfig should not panic with valid defaults, got: %v", r)
		}
	}()
	cfg := DefaultSSRFConfig()
	if len(cfg.blockedNets) == 0 {
		t.Error("DefaultSSRFConfig should have parsed blocked CIDRs")
	}
}

func TestNormalizeHosts(t *testing.T) {
	t.Run("lowercases and trims whitespace", func(t *testing.T) {
		hosts := []string{"  METADATA.INTERNAL  ", "Metadata.Google.Internal", " Instance-Data.EC2.Internal "}
		result := normalizeHosts(hosts)
		expected := []string{"metadata.internal", "metadata.google.internal", "instance-data.ec2.internal"}
		for i, exp := range expected {
			if result[i] != exp {
				t.Errorf("normalizeHosts[%d]: expected %q, got %q", i, exp, result[i])
			}
		}
	})

	t.Run("empty input returns empty result", func(t *testing.T) {
		result := normalizeHosts([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d elements", len(result))
		}
	})
}
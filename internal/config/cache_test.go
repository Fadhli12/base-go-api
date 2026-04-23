package config

import (
	"testing"
)

func TestDefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()

	if cfg.Driver != "redis" {
		t.Errorf("expected Driver 'redis', got '%s'", cfg.Driver)
	}
	if cfg.DefaultTTL != 300 {
		t.Errorf("expected DefaultTTL 300, got %d", cfg.DefaultTTL)
	}
	if cfg.PermissionTTL != 300 {
		t.Errorf("expected PermissionTTL 300, got %d", cfg.PermissionTTL)
	}
	if cfg.RateLimitTTL != 60 {
		t.Errorf("expected RateLimitTTL 60, got %d", cfg.RateLimitTTL)
	}
}

func TestCacheConfigParsing(t *testing.T) {
	cfg := CacheConfig{
		Driver:        "memory",
		DefaultTTL:    600,
		PermissionTTL: 600,
		RateLimitTTL:  120,
	}
	if cfg.Driver != "memory" {
		t.Errorf("expected Driver 'memory', got '%s'", cfg.Driver)
	}
}

func TestInvalidCacheDriver(t *testing.T) {
	cfg := CacheConfig{Driver: "invalid"}
	err := validateCache(&Config{Cache: cfg})
	if err == nil {
		t.Error("expected error for invalid cache driver")
	}
}
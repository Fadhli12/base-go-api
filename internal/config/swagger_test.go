package config

import (
	"testing"
)

func TestDefaultSwaggerConfig(t *testing.T) {
	cfg := DefaultSwaggerConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled true")
	}
	if cfg.Path != "/swagger" {
		t.Errorf("expected Path '/swagger', got '%s'", cfg.Path)
	}
}

func TestSwaggerConfigDefaults(t *testing.T) {
	cfg := DefaultSwaggerConfig()

	// Enabled should be true by default
	if cfg.Enabled != true {
		t.Error("expected Enabled true by default")
	}

	// Path should start with /
	if cfg.Path == "" || cfg.Path[0] != '/' {
		t.Error("expected Path to start with /")
	}
}
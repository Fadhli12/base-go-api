package conversion_test

import (
	"testing"

	"github.com/example/go-api-base/internal/conversion"
)

func TestDefaultConversionConfig(t *testing.T) {
	cfg := conversion.DefaultConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled true by default")
	}
	if cfg.ThumbnailQuality != 85 {
		t.Errorf("expected ThumbnailQuality 85, got %d", cfg.ThumbnailQuality)
	}
	if cfg.ThumbnailWidth != 300 {
		t.Errorf("expected ThumbnailWidth 300, got %d", cfg.ThumbnailWidth)
	}
	if cfg.ThumbnailHeight != 300 {
		t.Errorf("expected ThumbnailHeight 300, got %d", cfg.ThumbnailHeight)
	}
	if cfg.PreviewQuality != 90 {
		t.Errorf("expected PreviewQuality 90, got %d", cfg.PreviewQuality)
	}
	if cfg.PreviewWidth != 800 {
		t.Errorf("expected PreviewWidth 800, got %d", cfg.PreviewWidth)
	}
	if cfg.PreviewHeight != 600 {
		t.Errorf("expected PreviewHeight 600, got %d", cfg.PreviewHeight)
	}
}

func TestCustomThumbnailQuality(t *testing.T) {
	cfg := conversion.Config{
		Enabled:          true,
		ThumbnailQuality: 75,
		ThumbnailWidth:   300,
		ThumbnailHeight:  300,
		PreviewQuality:   90,
		PreviewWidth:     800,
		PreviewHeight:    600,
	}

	def := cfg.ThumbnailDef()
	if def.Quality != 75 {
		t.Errorf("expected Quality 75, got %d", def.Quality)
	}
	if def.Width != 300 {
		t.Errorf("expected Width 300, got %d", def.Width)
	}
}

func TestCompressionDisabled(t *testing.T) {
	cfg := conversion.Config{
		Enabled: false,
	}

	defs := cfg.GetDefaultConversions()
	if len(defs) != 0 {
		t.Errorf("expected 0 conversions when disabled, got %d", len(defs))
	}
}

func TestThumbnailDef(t *testing.T) {
	cfg := conversion.DefaultConfig()
	def := cfg.ThumbnailDef()

	if def.Name != "thumbnail" {
		t.Errorf("expected Name 'thumbnail', got '%s'", def.Name)
	}
	if def.Width != 300 {
		t.Errorf("expected Width 300, got %d", def.Width)
	}
	if def.Format != "jpeg" {
		t.Errorf("expected Format 'jpeg', got '%s'", def.Format)
	}
}

func TestPreviewDef(t *testing.T) {
	cfg := conversion.DefaultConfig()
	def := cfg.PreviewDef()

	if def.Name != "preview" {
		t.Errorf("expected Name 'preview', got '%s'", def.Name)
	}
	if def.Width != 800 {
		t.Errorf("expected Width 800, got %d", def.Width)
	}
}

func TestConfigGetDefaultConversions(t *testing.T) {
	cfg := conversion.DefaultConfig()
	defs := cfg.GetDefaultConversions()

	if len(defs) != 2 {
		t.Errorf("expected 2 conversions, got %d", len(defs))
	}
	if defs[0].Name != "thumbnail" {
		t.Errorf("expected first conversion 'thumbnail', got '%s'", defs[0].Name)
	}
	if defs[1].Name != "preview" {
		t.Errorf("expected second conversion 'preview', got '%s'", defs[1].Name)
	}
}
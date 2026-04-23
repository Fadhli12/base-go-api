package config

import (
	"testing"
)

func TestDefaultImageConfig(t *testing.T) {
	cfg := DefaultImageConfig()

	if !cfg.CompressionEnabled {
		t.Error("expected CompressionEnabled true")
	}
	if cfg.ThumbnailQuality != 85 {
		t.Errorf("expected ThumbnailQuality 85, got %d", cfg.ThumbnailQuality)
	}
	if cfg.ThumbnailWidth != 300 {
		t.Errorf("expected ThumbnailWidth 300, got %d", cfg.ThumbnailWidth)
	}
}

func TestImageConfigParsing(t *testing.T) {
	cfg := ImageConfig{
		CompressionEnabled: false,
		ThumbnailQuality:   75,
		PreviewQuality:     90,
	}
	if cfg.CompressionEnabled {
		t.Error("expected CompressionEnabled false")
	}
}

func TestInvalidQuality(t *testing.T) {
	cfg := ImageConfig{
		CompressionEnabled: true,
		ThumbnailQuality:  150, // invalid
	}
	err := validateImage(&Config{Image: cfg})
	if err == nil {
		t.Error("expected error for invalid quality")
	}
}

func TestInvalidPreviewQuality(t *testing.T) {
	cfg := ImageConfig{
		CompressionEnabled: true,
		ThumbnailQuality:   85,
		PreviewQuality:     150, // invalid (> 100)
	}
	err := validateImage(&Config{Image: cfg})
	if err == nil {
		t.Error("expected error for invalid preview quality")
	}
}
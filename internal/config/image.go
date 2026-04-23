// Package config provides application configuration management.
package config

// ImageConfig holds image processing configuration.
// Controls thumbnail and preview generation settings.
type ImageConfig struct {
	CompressionEnabled bool `mapstructure:"compression_enabled"`
	ThumbnailQuality   int  `mapstructure:"thumbnail_quality"`
	ThumbnailWidth     int  `mapstructure:"thumbnail_width"`
	ThumbnailHeight    int  `mapstructure:"thumbnail_height"`
	PreviewQuality     int  `mapstructure:"preview_quality"`
	PreviewWidth       int  `mapstructure:"preview_width"`
	PreviewHeight      int  `mapstructure:"preview_height"`
}

// DefaultImageConfig returns an ImageConfig with sensible defaults.
func DefaultImageConfig() ImageConfig {
	return ImageConfig{
		CompressionEnabled: true,
		ThumbnailQuality:   85,
		ThumbnailWidth:     300,
		ThumbnailHeight:    300,
		PreviewQuality:     90,
		PreviewWidth:       800,
		PreviewHeight:      600,
	}
}
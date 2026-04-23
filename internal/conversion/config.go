// Package conversion provides image conversion and thumbnail generation functionality
package conversion

import "github.com/example/go-api-base/internal/config"

// Config holds image conversion configuration
type Config struct {
	Enabled          bool
	ThumbnailWidth   int
	ThumbnailHeight  int
	ThumbnailQuality int
	PreviewWidth     int
	PreviewHeight    int
	PreviewQuality   int
}

// DefaultConfig returns production-safe defaults
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		ThumbnailWidth:   300,
		ThumbnailHeight:  300,
		ThumbnailQuality: 85,
		PreviewWidth:     800,
		PreviewHeight:    600,
		PreviewQuality:   90,
	}
}

// ThumbnailDef returns the thumbnail conversion definition using config
func (c Config) ThumbnailDef() ConversionDef {
	return ConversionDef{
		Name:    "thumbnail",
		Width:   c.ThumbnailWidth,
		Height:  c.ThumbnailHeight,
		Quality: c.ThumbnailQuality,
		Format:  "jpeg",
	}
}

// PreviewDef returns the preview conversion definition using config
func (c Config) PreviewDef() ConversionDef {
	return ConversionDef{
		Name:    "preview",
		Width:   c.PreviewWidth,
		Height:  c.PreviewHeight,
		Quality: c.PreviewQuality,
		Format:  "jpeg",
	}
}

// GetDefaultConversions returns default conversions using config values
func (c Config) GetDefaultConversions() []ConversionDef {
	if !c.Enabled {
		return []ConversionDef{}
	}
	return []ConversionDef{
		c.ThumbnailDef(),
		c.PreviewDef(),
	}
}

// FromImageConfig creates a conversion.Config from config.ImageConfig
func FromImageConfig(imgCfg config.ImageConfig) Config {
	return Config{
		Enabled:          imgCfg.CompressionEnabled,
		ThumbnailWidth:   imgCfg.ThumbnailWidth,
		ThumbnailHeight:  imgCfg.ThumbnailHeight,
		ThumbnailQuality: imgCfg.ThumbnailQuality,
		PreviewWidth:     imgCfg.PreviewWidth,
		PreviewHeight:    imgCfg.PreviewHeight,
		PreviewQuality:   imgCfg.PreviewQuality,
	}
}
//go:build windows
// +build windows

// Package conversion provides image conversion and thumbnail generation
// This file contains the Windows stub implementation (govips requires libvips)
package conversion

import (
	"context"
	"io"
	"log/slog"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/storage"
	"github.com/example/go-api-base/pkg/errors"
)

// ImageConversionHandler handles image conversions using govips
// On Windows, this is a stub that returns an error since govips requires libvips
type ImageConversionHandler struct {
	storageDriver storage.Driver
}

// NewImageConversionHandler creates a new image conversion handler
func NewImageConversionHandler(storageDriver storage.Driver) *ImageConversionHandler {
	return &ImageConversionHandler{
		storageDriver: storageDriver,
	}
}

// CanHandle returns true if this handler can process the given MIME type
func (h *ImageConversionHandler) CanHandle(mimeType string) bool {
	// On Windows, we can't actually process images without libvips
	return false
}

// Handle generates a conversion from the source media
func (h *ImageConversionHandler) Handle(ctx context.Context, media *domain.Media, def ConversionDef, reader io.Reader) (*domain.MediaConversion, error) {
	// On Windows, return an error since govips requires libvips
	slog.Error("Image conversion not available on Windows - requires libvips installation")
	return nil, errors.NewAppError("NOT_SUPPORTED", "Image conversion requires libvips (not available on Windows)", 501)
}

// ExtractImageMetadata extracts metadata from an image
func (h *ImageConversionHandler) ExtractImageMetadata(ctx context.Context, reader io.Reader) (map[string]interface{}, error) {
	slog.Error("Image metadata extraction not available on Windows - requires libvips installation")
	return nil, errors.NewAppError("NOT_SUPPORTED", "Image metadata extraction requires libvips (not available on Windows)", 501)
}

// ProcessImageMetadata updates the media metadata with image dimensions
func (h *ImageConversionHandler) ProcessImageMetadata(ctx context.Context, media *domain.Media, reader io.Reader) error {
	// Silently skip on Windows
	return nil
}

// Shutdown is a no-op on Windows
func Shutdown() {
	// No-op on Windows
}

// GetSupportedFormats returns the list of supported image formats
func GetSupportedFormats() []string {
	// Return empty on Windows since we can't actually process
	return []string{}
}

// FitMode represents how the image should be resized
type FitMode string

const (
	// FitCover crops the image to fill the dimensions
	FitCover FitMode = "cover"
	// FitContain scales the image to fit within dimensions
	FitContain FitMode = "contain"
	// FitFill stretches the image to fill dimensions
	FitFill FitMode = "fill"
)

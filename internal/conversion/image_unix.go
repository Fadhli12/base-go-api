//go:build linux || darwin
// +build linux darwin

// Package conversion provides image conversion and thumbnail generation
// This file contains the Linux/Darwin implementation using govips
package conversion

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/storage"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// init registers the govips startup
func init() {
	// Initialize govips on package load
	vips.Startup(nil)
}

// ImageConversionHandler handles image conversions using govips
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
	return IsSupportedImageFormat(mimeType)
}

// Handle generates a conversion from the source media
func (h *ImageConversionHandler) Handle(ctx context.Context, media *domain.Media, def ConversionDef, reader io.Reader) (*domain.MediaConversion, error) {
	if !h.CanHandle(media.MimeType) {
		return nil, ErrUnsupportedFormat
	}

	// Validate dimensions
	if def.Width <= 0 || def.Height <= 0 {
		return nil, ErrInvalidDimensions
	}

	// Load image from reader using govips
	img, err := vips.NewImageFromReader(reader)
	if err != nil {
		slog.Error("Failed to load image from reader", "error", err, "media_id", media.ID)
		return nil, errors.WrapInternal(err)
	}
	defer img.Close()

	// Get original dimensions
	originalWidth := img.Width()
	originalHeight := img.Height()

	// Resize/thumbnail the image
	if err := img.Thumbnail(def.Width, def.Height, vips.InterestingCentre); err != nil {
		slog.Error("Failed to create thumbnail", "error", err, "media_id", media.ID)
		return nil, errors.WrapInternal(err)
	}

	// Get output dimensions after processing
	outputWidth := img.Width()
	outputHeight := img.Height()

	// Export to buffer
	format := strings.ToLower(def.Format)
	if format == "" {
		format = "jpeg"
	}

	var buf []byte

	switch format {
	case "jpeg", "jpg":
		exportParams := vips.NewJpegExportParams()
		exportParams.Quality = def.Quality
		exportParams.StripMetadata = true
		buf, _, err = img.ExportJpeg(exportParams)
	case "webp":
		webpParams := vips.NewWebpExportParams()
		webpParams.Quality = def.Quality
		webpParams.StripMetadata = true
		buf, _, err = img.ExportWebp(webpParams)
	case "png":
		pngParams := vips.NewPngExportParams()
		pngParams.StripMetadata = true
		buf, _, err = img.ExportPng(pngParams)
	default:
		// Default to JPEG
		exportParams := vips.NewJpegExportParams()
		exportParams.Quality = def.Quality
		exportParams.StripMetadata = true
		buf, _, err = img.ExportJpeg(exportParams)
		format = "jpeg"
	}

	if err != nil {
		slog.Error("Failed to export image", "error", err, "media_id", media.ID, "format", format)
		return nil, errors.WrapInternal(err)
	}

	// Generate storage path for conversion
	conversionFilename := fmt.Sprintf("%s_%s.%s",
		strings.TrimSuffix(media.Filename, filepath.Ext(media.Filename)),
		def.Name,
		format,
	)
	conversionPath := fmt.Sprintf("%s/%s/%s",
		media.ModelType,
		media.ModelID.String(),
		conversionFilename,
	)

	// Store the converted image
	bufReader := strings.NewReader(string(buf))
	if err := h.storageDriver.Store(ctx, conversionPath, bufReader); err != nil {
		slog.Error("Failed to store converted image", "error", err, "path", conversionPath)
		return nil, errors.WrapInternal(err)
	}

	// Get file size
	size := int64(len(buf))

	// Build metadata
	metadata := map[string]interface{}{
		"width":           outputWidth,
		"height":          outputHeight,
		"original_width":  originalWidth,
		"original_height": originalHeight,
		"quality":         def.Quality,
		"format":          format,
	}

	// Create conversion record
	conversion := &domain.MediaConversion{
		ID:        uuid.New(),
		MediaID:   media.ID,
		Name:      def.Name,
		Disk:      media.Disk,
		Path:      conversionPath,
		Size:      size,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	slog.Info("Image conversion completed",
		"media_id", media.ID,
		"conversion_name", def.Name,
		"format", format,
		"width", outputWidth,
		"height", outputHeight,
		"size", size,
	)

	return conversion, nil
}

// ExtractImageMetadata extracts metadata from an image
func (h *ImageConversionHandler) ExtractImageMetadata(ctx context.Context, reader io.Reader) (map[string]interface{}, error) {
	img, err := vips.NewImageFromReader(reader)
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	defer img.Close()

	metadata := map[string]interface{}{
		"width":  img.Width(),
		"height": img.Height(),
	}

	// Get EXIF data if available
	exifData, err := img.GetExif()
	if err == nil && exifData != nil {
		metadata["exif"] = exifData
	}

	return metadata, nil
}

// ProcessImageMetadata updates the media metadata with image dimensions
func (h *ImageConversionHandler) ProcessImageMetadata(ctx context.Context, media *domain.Media, reader io.Reader) error {
	if !IsImageMimeType(media.MimeType) {
		return nil
	}

	metadata, err := h.ExtractImageMetadata(ctx, reader)
	if err != nil {
		slog.Warn("Failed to extract image metadata", "error", err, "media_id", media.ID)
		return nil
	}

	if media.Metadata == nil {
		media.Metadata = make(map[string]interface{})
	}

	for k, v := range metadata {
		media.Metadata[k] = v
	}

	return nil
}

// Shutdown shuts down the govips library
func Shutdown() {
	vips.Shutdown()
}

// GetSupportedFormats returns the list of supported image formats
func GetSupportedFormats() []string {
	return []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
		"image/gif",
		"image/tiff",
	}
}

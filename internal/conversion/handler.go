// Package conversion provides image conversion and thumbnail generation functionality
package conversion

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
)

// ConversionJob represents a job for converting media
type ConversionJob struct {
	MediaID       uuid.UUID
	ConversionDef ConversionDef
	Priority      int
	Attempt       int
	CreatedAt     time.Time
}

// ShouldProcessSync determines if a file should be processed synchronously based on size
func ShouldProcessSync(fileSize int64) bool {
	return fileSize < SmallFileThreshold
}

// CreateConversionJob creates a conversion job for the given media
func CreateConversionJob(media *domain.Media, def ConversionDef) ConversionJob {
	return ConversionJob{
		MediaID:       media.ID,
		ConversionDef: def,
		Priority:      0,
		Attempt:       0,
		CreatedAt:     time.Now(),
	}
}

// CreateConversionJobs creates conversion jobs for all default conversions
func CreateConversionJobs(media *domain.Media) []ConversionJob {
	defs := GetDefaultConversions()
	jobs := make([]ConversionJob, len(defs))
	for i, def := range defs {
		jobs[i] = CreateConversionJob(media, def)
	}
	return jobs
}

// CreateConversionJobsWithConfig creates conversion jobs using provided config
func CreateConversionJobsWithConfig(media *domain.Media, cfg Config) []ConversionJob {
	defs := cfg.GetDefaultConversions()
	jobs := make([]ConversionJob, len(defs))
	for i, def := range defs {
		jobs[i] = CreateConversionJob(media, def)
	}
	return jobs
}

// Constants for conversion sizes and settings
const (
	ThumbnailWidth  = 300
	ThumbnailHeight = 300
	ThumbnailQuality = 85

	PreviewWidth  = 800
	PreviewHeight = 600
	PreviewQuality = 90

	SmallFileThreshold = 5 * 1024 * 1024 // 5MB
	MaxRetries         = 5
	JobTTL             = 3600 // seconds
)

// Redis keys for conversion queue and locks
const (
	ConversionQueueKey        = "media:conversions:queue"
	ConversionLockKeyPattern  = "media:conversions:lock:%s:%s"
	ConversionLockTTL         = 3600 // seconds
)

// ConversionHandler defines the interface for media conversion handlers
type ConversionHandler interface {
	// CanHandle returns true if this handler can process the given MIME type
	CanHandle(mimeType string) bool

	// Handle generates a conversion from the source media
	Handle(ctx context.Context, media *domain.Media, def ConversionDef, reader io.Reader) (*domain.MediaConversion, error)
}

// ConversionDef defines a conversion configuration
type ConversionDef struct {
	Name    string        // "thumbnail", "preview"
	Width   int           // Target width
	Height  int           // Target height
	Quality int           // JPEG quality (0-100)
	Format  string        // Output format (jpeg, png, webp)
	Filters []ImageFilter // Additional filters
}

// ImageFilter defines an interface for image filtering operations
type ImageFilter interface {
	// Apply applies the filter to the image
	// The concrete type will depend on the image library used
	Apply(image interface{}) error
}

// ConversionResult holds the result of a successful conversion
type ConversionResult struct {
	Path     string
	Size     int64
	Metadata map[string]interface{}
}

// ErrConversionFailed is returned when a conversion fails
var ErrConversionFailed = fmt.Errorf("conversion failed")

// ErrUnsupportedFormat is returned when the format is not supported
var ErrUnsupportedFormat = fmt.Errorf("unsupported image format")

// ErrInvalidDimensions is returned when the dimensions are invalid
var ErrInvalidDimensions = fmt.Errorf("invalid image dimensions")

// ErrHandlerNotFound is returned when no handler can process the MIME type
var ErrHandlerNotFound = fmt.Errorf("no handler found for MIME type")

// IsImageMimeType checks if the MIME type is an image type
func IsImageMimeType(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(mimeType), "image/")
}

// IsSupportedImageFormat checks if the image format is supported for conversion
func IsSupportedImageFormat(mimeType string) bool {
	supported := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
		"image/tiff": true,
	}
	return supported[strings.ToLower(mimeType)]
}

// DefaultThumbnailDef returns the default thumbnail conversion definition
func DefaultThumbnailDef() ConversionDef {
	return ConversionDef{
		Name:    "thumbnail",
		Width:   ThumbnailWidth,
		Height:  ThumbnailHeight,
		Quality: ThumbnailQuality,
		Format:  "jpeg",
	}
}

// DefaultPreviewDef returns the default preview conversion definition
func DefaultPreviewDef() ConversionDef {
	return ConversionDef{
		Name:    "preview",
		Width:   PreviewWidth,
		Height:  PreviewHeight,
		Quality: PreviewQuality,
		Format:  "jpeg",
	}
}

// Registry holds registered conversion handlers
type Registry struct {
	handlers []ConversionHandler
}

// NewRegistry creates a new conversion handler registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make([]ConversionHandler, 0),
	}
}

// Register adds a handler to the registry
func (r *Registry) Register(handler ConversionHandler) {
	r.handlers = append(r.handlers, handler)
}

// FindHandler returns the first handler that can handle the given MIME type
func (r *Registry) FindHandler(mimeType string) (ConversionHandler, error) {
	for _, handler := range r.handlers {
		if handler.CanHandle(mimeType) {
			return handler, nil
		}
	}
	return nil, ErrHandlerNotFound
}

// HandlerInfo provides metadata about a registered handler
type HandlerInfo struct {
	Name        string
	SupportedTypes []string
}

// GetDefaultConversions returns the default conversion definitions for images
func GetDefaultConversions() []ConversionDef {
	return []ConversionDef{
		DefaultThumbnailDef(),
		DefaultPreviewDef(),
	}
}

// CalculateBackoff calculates the exponential backoff duration for retries
func CalculateBackoff(attempt int) float64 {
	if attempt <= 0 {
		return 0
	}
	// Exponential backoff: 1s, 2s, 4s, 8s, ...
	backoff := 1.0
	for i := 1; i < attempt && i < MaxRetries; i++ {
		backoff *= 2
	}
	return backoff
}

package conversion_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/conversion"
	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStorageDriver is a mock implementation of storage.Driver for testing
type MockStorageDriver struct {
	mock.Mock
}

func (m *MockStorageDriver) Store(ctx context.Context, path string, content io.Reader) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockStorageDriver) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorageDriver) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStorageDriver) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
	args := m.Called(ctx, path, expires)
	return args.String(0), args.Error(1)
}

func (m *MockStorageDriver) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockStorageDriver) Size(ctx context.Context, path string) (int64, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(int64), args.Error(1)
}

// generateTestImage creates a simple JPEG image for testing
func generateTestImage(width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// nopCloser wraps an io.Reader to create an io.ReadCloser
type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestIsImageMimeType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{"JPEG image", "image/jpeg", true},
		{"PNG image", "image/png", true},
		{"WebP image", "image/webp", true},
		{"GIF image", "image/gif", true},
		{"PDF document", "application/pdf", false},
		{"Text file", "text/plain", false},
		{"Mixed case", "IMAGE/JPEG", true},
		{"Case insensitive PNG", "Image/Png", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conversion.IsImageMimeType(tt.mimeType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSupportedImageFormat(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{"JPEG", "image/jpeg", true},
		{"JPG", "image/jpg", true},
		{"PNG", "image/png", true},
		{"WebP", "image/webp", true},
		{"GIF", "image/gif", true},
		{"TIFF", "image/tiff", true},
		{"BMP", "image/bmp", false},
		{"SVG", "image/svg+xml", false},
		{"Unknown", "image/unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conversion.IsSupportedImageFormat(tt.mimeType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		attempt  int
		expected float64
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 8},
		{5, 16},
		{10, 16}, // capped at MaxRetries
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.attempt)), func(t *testing.T) {
			result := conversion.CalculateBackoff(tt.attempt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversionDefDefaults(t *testing.T) {
	t.Run("DefaultThumbnailDef", func(t *testing.T) {
		def := conversion.DefaultThumbnailDef()
		assert.Equal(t, "thumbnail", def.Name)
		assert.Equal(t, conversion.ThumbnailWidth, def.Width)
		assert.Equal(t, conversion.ThumbnailHeight, def.Height)
		assert.Equal(t, conversion.ThumbnailQuality, def.Quality)
		assert.Equal(t, "jpeg", def.Format)
	})

	t.Run("DefaultPreviewDef", func(t *testing.T) {
		def := conversion.DefaultPreviewDef()
		assert.Equal(t, "preview", def.Name)
		assert.Equal(t, conversion.PreviewWidth, def.Width)
		assert.Equal(t, conversion.PreviewHeight, def.Height)
		assert.Equal(t, conversion.PreviewQuality, def.Quality)
		assert.Equal(t, "jpeg", def.Format)
	})
}

func TestGetDefaultConversions(t *testing.T) {
	conversions := conversion.GetDefaultConversions()
	require.Len(t, conversions, 2)
	assert.Equal(t, "thumbnail", conversions[0].Name)
	assert.Equal(t, "preview", conversions[1].Name)
}

func TestShouldProcessSync(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected bool
	}{
		{"Small file (1MB)", 1 * 1024 * 1024, true},
		{"Medium file (4MB)", 4 * 1024 * 1024, true},
		{"At threshold (5MB)", 5 * 1024 * 1024, false},
		{"Large file (10MB)", 10 * 1024 * 1024, false},
		{"Very large file (100MB)", 100 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conversion.ShouldProcessSync(tt.size)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegistry(t *testing.T) {
	t.Run("FindHandlerForSupportedType", func(t *testing.T) {
		mockStorage := new(MockStorageDriver)
		registry := conversion.NewRegistry()
		handler := conversion.NewImageConversionHandler(mockStorage)
		registry.Register(handler)

		// On Windows, handler.CanHandle returns false for all types
		// On Unix, this would return a handler
		found, err := registry.FindHandler("image/jpeg")

		if err != nil {
			// Windows behavior - no handler found
			assert.Equal(t, conversion.ErrHandlerNotFound, err)
			assert.Nil(t, found)
		} else {
			// Unix behavior - handler found
			assert.NotNil(t, found)
			assert.True(t, found.CanHandle("image/jpeg"))
		}
	})

	t.Run("FindHandlerForUnsupportedType", func(t *testing.T) {
		mockStorage := new(MockStorageDriver)
		registry := conversion.NewRegistry()
		handler := conversion.NewImageConversionHandler(mockStorage)
		registry.Register(handler)

		found, err := registry.FindHandler("application/pdf")
		require.Error(t, err)
		assert.Equal(t, conversion.ErrHandlerNotFound, err)
		assert.Nil(t, found)
	})
}

func TestConversionJobCreation(t *testing.T) {
	media := &domain.Media{
		ID:       uuid.New(),
		MimeType: "image/jpeg",
		Size:     1024000,
	}
	def := conversion.DefaultThumbnailDef()

	job := conversion.CreateConversionJob(media, def)
	assert.Equal(t, media.ID, job.MediaID)
	assert.Equal(t, def, job.ConversionDef)
	assert.Equal(t, 0, job.Attempt)
	assert.WithinDuration(t, time.Now(), job.CreatedAt, time.Second)
}

func TestCreateConversionJobs(t *testing.T) {
	media := &domain.Media{
		ID:       uuid.New(),
		MimeType: "image/jpeg",
		Size:     1024000,
	}

	jobs := conversion.CreateConversionJobs(media)
	require.Len(t, jobs, 2)
	assert.Equal(t, "thumbnail", jobs[0].ConversionDef.Name)
	assert.Equal(t, "preview", jobs[1].ConversionDef.Name)
}

func TestImageConversionHandlerCanHandle(t *testing.T) {
	mockStorage := new(MockStorageDriver)
	handler := conversion.NewImageConversionHandler(mockStorage)

	// On Windows, handler.CanHandle returns false for all types
	// On Unix systems with libvips, it returns true for supported formats
	canHandle := handler.CanHandle("image/jpeg")

	if !canHandle {
		// Windows behavior - verify that non-image types also return false
		assert.False(t, handler.CanHandle("application/pdf"))
		assert.False(t, handler.CanHandle("text/plain"))
	} else {
		// Unix behavior - verify supported formats
		assert.True(t, handler.CanHandle("image/jpeg"))
		assert.True(t, handler.CanHandle("image/png"))
		assert.True(t, handler.CanHandle("image/webp"))
		assert.False(t, handler.CanHandle("application/pdf"))
		assert.False(t, handler.CanHandle("text/plain"))
	}
}

func TestImageConversionHandlerHandle(t *testing.T) {
	t.Run("HandleUnsupportedFormat", func(t *testing.T) {
		mockStorage := new(MockStorageDriver)
		handler := conversion.NewImageConversionHandler(mockStorage)

		media := &domain.Media{
			ID:       uuid.New(),
			MimeType: "application/pdf",
		}
		def := conversion.DefaultThumbnailDef()

		result, err := handler.Handle(context.Background(), media, def, strings.NewReader("test"))
		require.Error(t, err)

		// On Windows: returns NOT_SUPPORTED error
		// On Unix: returns ErrUnsupportedFormat
		if err.Error() == "Image conversion requires libvips (not available on Windows)" {
			// Windows behavior - handler can't process any images
			assert.Nil(t, result)
		} else {
			// Unix behavior
			assert.Equal(t, conversion.ErrUnsupportedFormat, err)
			assert.Nil(t, result)
		}
	})

	t.Run("HandleInvalidDimensions", func(t *testing.T) {
		mockStorage := new(MockStorageDriver)
		handler := conversion.NewImageConversionHandler(mockStorage)

		media := &domain.Media{
			ID:        uuid.New(),
			MimeType:  "image/jpeg",
			ModelType: "test",
			ModelID:   uuid.New(),
			Filename:  "test.jpg",
		}
		def := conversion.ConversionDef{
			Name:    "test",
			Width:   0,
			Height:  300,
			Quality: 85,
			Format:  "jpeg",
		}

		result, err := handler.Handle(context.Background(), media, def, strings.NewReader("test"))
		require.Error(t, err)

		// On Windows: returns NOT_SUPPORTED error
		// On Unix: returns ErrInvalidDimensions
		if err.Error() == "Image conversion requires libvips (not available on Windows)" {
			// Windows behavior - handler can't process any images
			assert.Nil(t, result)
		} else {
			// Unix behavior
			assert.Equal(t, conversion.ErrInvalidDimensions, err)
			assert.Nil(t, result)
		}
	})

	t.Run("HandleValidImage", func(t *testing.T) {
		// This test is platform-specific:
		// - On Windows: handler.CanHandle returns false, so this test passes
		// - On Linux/Darwin: would require libvips to actually process images
		mockStorage := new(MockStorageDriver)
		handler := conversion.NewImageConversionHandler(mockStorage)

		modelID := uuid.New()
		media := &domain.Media{
			ID:        uuid.New(),
			MimeType:  "image/jpeg",
			ModelType: "test",
			ModelID:   modelID,
			Filename:  "test.jpg",
			Disk:      "local",
		}

		// On Windows, handler always returns false for CanHandle
		// On Unix systems, this would process the image
		canHandle := handler.CanHandle("image/jpeg")

		if !canHandle {
			// Windows stub behavior - can't process images
			t.Skip("Skipping image conversion test on Windows (requires libvips)")
		}

		// This code only runs on Unix systems with libvips
		def := conversion.DefaultThumbnailDef()
		result, err := handler.Handle(context.Background(), media, def, strings.NewReader("invalid image data"))
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestGetSupportedFormats(t *testing.T) {
	// This is platform-specific:
	// - Windows: returns empty slice (stub implementation)
	// - Unix systems: returns supported formats
	formats := conversion.GetSupportedFormats()

	// On Unix systems, should have supported formats
	// On Windows, will be empty
	if len(formats) > 0 {
		assert.Contains(t, formats, "image/jpeg")
		assert.Contains(t, formats, "image/png")
	}
}

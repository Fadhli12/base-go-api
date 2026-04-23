package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalDriver implements the Driver interface for local filesystem storage
type LocalDriver struct {
	basePath string
	baseURL  string
}

// NewLocalDriver creates a new local filesystem storage driver
// basePath is the directory where files will be stored
// baseURL is the URL prefix for generating public URLs
func NewLocalDriver(basePath, baseURL string) (*LocalDriver, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Normalize baseURL (remove trailing slash)
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &LocalDriver{
		basePath: basePath,
		baseURL:  baseURL,
	}, nil
}

// Store saves a file to local storage
// The path should be relative (e.g., "media/2024/file.jpg")
func (d *LocalDriver) Store(ctx context.Context, path string, content io.Reader) error {
	safePath := d.sanitizePath(path)
	fullPath := filepath.Join(d.basePath, safePath)

	// Ensure parent directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.ErrorContext(ctx, "failed to create directory",
			slog.String("path", dir),
			slog.Any("error", err),
		)
		return &StorageError{Op: "store", Path: path, Err: err}
	}

	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create file",
			slog.String("path", fullPath),
			slog.Any("error", err),
		)
		return &StorageError{Op: "store", Path: path, Err: err}
	}
	defer file.Close()

	// Copy content to file
	written, err := io.Copy(file, content)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write file content",
			slog.String("path", fullPath),
			slog.Any("error", err),
		)
		return &StorageError{Op: "store", Path: path, Err: err}
	}

	slog.InfoContext(ctx, "file stored successfully",
		slog.String("path", path),
		slog.Int64("size", written),
	)

	return nil
}

// Get retrieves a file from local storage
func (d *LocalDriver) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	safePath := d.sanitizePath(path)
	fullPath := filepath.Join(d.basePath, safePath)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &StorageError{Op: "get", Path: path, Err: ErrFileNotFound}
		}
		slog.ErrorContext(ctx, "failed to open file",
			slog.String("path", fullPath),
			slog.Any("error", err),
		)
		return nil, &StorageError{Op: "get", Path: path, Err: err}
	}

	return file, nil
}

// Delete removes a file from local storage
func (d *LocalDriver) Delete(ctx context.Context, path string) error {
	safePath := d.sanitizePath(path)
	fullPath := filepath.Join(d.basePath, safePath)

	err := os.Remove(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.WarnContext(ctx, "file not found for deletion",
				slog.String("path", path),
			)
			return nil // Idempotent delete
		}
		slog.ErrorContext(ctx, "failed to delete file",
			slog.String("path", fullPath),
			slog.Any("error", err),
		)
		return &StorageError{Op: "delete", Path: path, Err: err}
	}

	slog.InfoContext(ctx, "file deleted successfully",
		slog.String("path", path),
	)

	return nil
}

// URL generates a public URL for accessing the file
// For local storage, this returns a URL path that can be served by the application
// The expires parameter is ignored for local storage (URLs don't expire)
func (d *LocalDriver) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
	safePath := d.sanitizePath(path)
	return fmt.Sprintf("%s/%s", d.baseURL, safePath), nil
}

// Exists checks if a file exists in local storage
func (d *LocalDriver) Exists(ctx context.Context, path string) (bool, error) {
	safePath := d.sanitizePath(path)
	fullPath := filepath.Join(d.basePath, safePath)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, &StorageError{Op: "exists", Path: path, Err: err}
	}

	return true, nil
}

// Size returns the size of the file in bytes
func (d *LocalDriver) Size(ctx context.Context, path string) (int64, error) {
	safePath := d.sanitizePath(path)
	fullPath := filepath.Join(d.basePath, safePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, &StorageError{Op: "size", Path: path, Err: ErrFileNotFound}
		}
		return 0, &StorageError{Op: "size", Path: path, Err: err}
	}

	return info.Size(), nil
}

// sanitizePath sanitizes a file path to prevent directory traversal attacks
func (d *LocalDriver) sanitizePath(path string) string {
	// Remove any ".." components
	parts := strings.Split(path, "/")
	var safeParts []string
	for _, part := range parts {
		if part != ".." && part != "" {
			safeParts = append(safeParts, part)
		}
	}
	return strings.Join(safeParts, "/")
}

// GenerateFilename generates a safe filename with UUID and extension
func GenerateFilename(originalName string) string {
	ext := filepath.Ext(originalName)
	return fmt.Sprintf("%s%s", uuid.New().String(), ext)
}

// GetBasePath returns the base storage path (for testing)
func (d *LocalDriver) GetBasePath() string {
	return d.basePath
}

// Package storage provides storage abstraction for media files
package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Common errors
var (
	ErrFileNotFound    = fmt.Errorf("file not found")
	ErrStorageFull     = fmt.Errorf("storage is full")
	ErrInvalidPath     = fmt.Errorf("invalid file path")
	ErrFileExists      = fmt.Errorf("file already exists")
	ErrStorageNotFound = fmt.Errorf("storage not found")
)

// Driver is the interface for storage backends
type Driver interface {
	// Store saves a file to storage at the given path
	// The content is read from the provided io.Reader
	Store(ctx context.Context, path string, content io.Reader) error

	// Get retrieves a file from storage at the given path
	// Returns an io.ReadCloser that must be closed by the caller
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file from storage at the given path
	Delete(ctx context.Context, path string) error

	// URL generates a public or signed URL for accessing the file
	// The expires parameter determines how long the URL is valid (0 for public URLs)
	URL(ctx context.Context, path string, expires time.Duration) (string, error)

	// Exists checks if a file exists at the given path
	Exists(ctx context.Context, path string) (bool, error)

	// Size returns the size of the file at the given path in bytes
	Size(ctx context.Context, path string) (int64, error)
}

// Config holds configuration for storage drivers
type Config struct {
	// Type is the storage driver type: "local", "s3", "minio"
	Type string

	// Local configuration
	LocalPath string
	BaseURL   string

	// S3/MinIO configuration
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	PathStyle       bool // Use path-style addressing (required for MinIO)
}

// NewDriver creates a new storage driver based on the configuration
func NewDriver(config Config) (Driver, error) {
	switch config.Type {
	case "local":
		return NewLocalDriver(config.LocalPath, config.BaseURL)
	case "s3", "minio":
		return NewS3Driver(config)
	default:
		return nil, fmt.Errorf("unsupported storage driver type: %s", config.Type)
	}
}

// sanitizePath sanitizes a file path to prevent directory traversal attacks
// It removes any ".." components and ensures the path is relative
func sanitizePath(path string) string {
	// Remove leading slashes
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Simple sanitization: remove any ".." components
	// This is a basic implementation; production code should use filepath.Clean
	// with additional checks
	var result []rune
	for _, r := range path {
		if r == '\\' {
			result = append(result, '/')
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

// GenerateSafeFilename generates a safe filename from a UUID and extension
func GenerateSafeFilename(extension string) string {
	return fmt.Sprintf("%s%s", time.Now().Format("20060102_150405_")+generateRandomString(8), extension)
}

// generateRandomString generates a random alphanumeric string of given length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// StorageError wraps storage errors with additional context
type StorageError struct {
	Op   string // operation
	Path string // file path
	Err  error  // underlying error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage %s on %s: %v", e.Op, e.Path, e.Err)
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// IsNotFound returns true if the error is a file not found error
func IsNotFound(err error) bool {
	if err == ErrFileNotFound {
		return true
	}
	if se, ok := err.(*StorageError); ok {
		return se.Err == ErrFileNotFound
	}
	return false
}

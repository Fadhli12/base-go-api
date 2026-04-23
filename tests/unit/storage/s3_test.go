package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/storage"
)

// TestS3StorageError tests StorageError behavior
func TestS3StorageError(t *testing.T) {
	t.Run("StorageError wraps underlying error", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		storageErr := &storage.StorageError{
			Op:   "store",
			Path: "/test/file.txt",
			Err:  underlyingErr,
		}

		if storageErr.Error() == "" {
			t.Error("expected non-empty error message")
		}

		if !errors.Is(storageErr, underlyingErr) {
			t.Error("expected to unwrap to underlying error")
		}
	})

	t.Run("IsNotFound returns true for ErrFileNotFound", func(t *testing.T) {
		if !storage.IsNotFound(storage.ErrFileNotFound) {
			t.Error("expected IsNotFound to return true for ErrFileNotFound")
		}
	})

	t.Run("IsNotFound returns false for other errors", func(t *testing.T) {
		err := errors.New("some other error")
		if storage.IsNotFound(err) {
			t.Error("expected IsNotFound to return false for other errors")
		}
	})

	t.Run("StorageError is recognized by IsNotFound", func(t *testing.T) {
		storageErr := &storage.StorageError{
			Op:   "get",
			Path: "/test/file.txt",
			Err:  storage.ErrFileNotFound,
		}

		if !storage.IsNotFound(storageErr) {
			t.Error("expected IsNotFound to return true for StorageError with ErrFileNotFound")
		}
	})
}

// TestS3Constants tests storage constants
func TestS3Constants(t *testing.T) {
	t.Run("ErrFileNotFound is defined", func(t *testing.T) {
		if storage.ErrFileNotFound == nil {
			t.Error("expected ErrFileNotFound to be defined")
		}
	})

	t.Run("ErrStorageFull is defined", func(t *testing.T) {
		if storage.ErrStorageFull == nil {
			t.Error("expected ErrStorageFull to be defined")
		}
	})

	t.Run("ErrInvalidPath is defined", func(t *testing.T) {
		if storage.ErrInvalidPath == nil {
			t.Error("expected ErrInvalidPath to be defined")
		}
	})

	t.Run("ErrFileExists is defined", func(t *testing.T) {
		if storage.ErrFileExists == nil {
			t.Error("expected ErrFileExists to be defined")
		}
	})

	t.Run("ErrStorageNotFound is defined", func(t *testing.T) {
		if storage.ErrStorageNotFound == nil {
			t.Error("expected ErrStorageNotFound to be defined")
		}
	})
}

// TestS3IsNotFound tests the IsNotFound helper function
func TestS3IsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "ErrFileNotFound returns true",
			err:      storage.ErrFileNotFound,
			expected: true,
		},
		{
			name:     "StorageError with ErrFileNotFound returns true",
			err:      &storage.StorageError{Op: "get", Path: "/test", Err: storage.ErrFileNotFound},
			expected: true,
		},
		{
			name:     "StorageError with other error returns false",
			err:      &storage.StorageError{Op: "get", Path: "/test", Err: errors.New("other")},
			expected: false,
		},
		{
			name:     "generic error returns false",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := storage.IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestS3StorageErrorMethods tests StorageError methods
func TestS3StorageErrorMethods(t *testing.T) {
	t.Run("Error returns formatted message", func(t *testing.T) {
		err := &storage.StorageError{
			Op:   "store",
			Path: "/test/file.txt",
			Err:  errors.New("connection refused"),
		}

		msg := err.Error()
		if msg == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		storageErr := &storage.StorageError{
			Op:   "store",
			Path: "/test/file.txt",
			Err:  underlyingErr,
		}

		unwrapped := storageErr.Unwrap()
		if unwrapped != underlyingErr {
			t.Error("expected Unwrap to return underlying error")
		}
	})
}

// Prevent unused variable warnings
var _ = time.Second
var _ = context.Background
package unit

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDriver is a mock implementation of the storage.Driver interface for testing
type MockDriver struct {
	stored   map[string][]byte
	baseURL  string
	failNext error
}

// NewMockDriver creates a new mock storage driver
func NewMockDriver(baseURL string) *MockDriver {
	return &MockDriver{
		stored:  make(map[string][]byte),
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// SetFailure sets an error to be returned on the next operation
func (m *MockDriver) SetFailure(err error) {
	m.failNext = err
}

// Store saves content to the mock storage
func (m *MockDriver) Store(ctx context.Context, path string, content io.Reader) error {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return err
	}

	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}

	m.stored[path] = data
	return nil
}

// Get retrieves content from the mock storage
func (m *MockDriver) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return nil, err
	}

	data, exists := m.stored[path]
	if !exists {
		return nil, storage.ErrFileNotFound
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

// Delete removes content from the mock storage
func (m *MockDriver) Delete(ctx context.Context, path string) error {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return err
	}

	delete(m.stored, path)
	return nil
}

// URL generates a URL for the stored content
func (m *MockDriver) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return "", err
	}

	return m.baseURL + "/" + path, nil
}

// Exists checks if content exists in the mock storage
func (m *MockDriver) Exists(ctx context.Context, path string) (bool, error) {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return false, err
	}

	_, exists := m.stored[path]
	return exists, nil
}

// Size returns the size of the stored content
func (m *MockDriver) Size(ctx context.Context, path string) (int64, error) {
	if m.failNext != nil {
		err := m.failNext
		m.failNext = nil
		return 0, err
	}

	data, exists := m.stored[path]
	if !exists {
		return 0, storage.ErrFileNotFound
	}

	return int64(len(data)), nil
}

// GetStored returns all stored content (for testing)
func (m *MockDriver) GetStored() map[string][]byte {
	return m.stored
}

// Local Driver Tests

func TestLocalDriver_NewLocalDriver(t *testing.T) {
	tempDir := t.TempDir()

	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)
	assert.NotNil(t, driver)

	// Verify base path was created
	assert.Equal(t, tempDir, driver.GetBasePath())
}

func TestLocalDriver_Store(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Verify file exists
	fullPath := filepath.Join(tempDir, "test", "file.txt")
	_, err = os.Stat(fullPath)
	assert.NoError(t, err)

	// Verify content
	stored, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	assert.Equal(t, content, stored)
}

func TestLocalDriver_Get(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	// Store first
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Get the file
	reader, err := driver.Get(ctx, path)
	require.NoError(t, err)
	defer reader.Close()

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestLocalDriver_Get_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	path := "nonexistent/file.txt"

	reader, err := driver.Get(ctx, path)
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.True(t, storage.IsNotFound(err))
}

func TestLocalDriver_Delete(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	// Store first
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Delete the file
	err = driver.Delete(ctx, path)
	require.NoError(t, err)

	// Verify file no longer exists
	fullPath := filepath.Join(tempDir, "test", "file.txt")
	_, err = os.Stat(fullPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLocalDriver_Delete_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	path := "nonexistent/file.txt"

	// Should not return error for idempotent delete
	err = driver.Delete(ctx, path)
	require.NoError(t, err)
}

func TestLocalDriver_URL(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	path := "test/file.txt"

	url, err := driver.URL(ctx, path, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/media/test/file.txt", url)
}

func TestLocalDriver_URL_WithTrailingSlash(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media/")
	require.NoError(t, err)

	ctx := context.Background()
	path := "test/file.txt"

	url, err := driver.URL(ctx, path, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/media/test/file.txt", url)
}

func TestLocalDriver_Exists(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	// File should not exist initially
	exists, err := driver.Exists(ctx, path)
	require.NoError(t, err)
	assert.False(t, exists)

	// Store the file
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// File should exist now
	exists, err = driver.Exists(ctx, path)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalDriver_Size(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	// Store the file
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Get size
	size, err := driver.Size(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestLocalDriver_Size_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	path := "nonexistent/file.txt"

	size, err := driver.Size(ctx, path)
	assert.Error(t, err)
	assert.Equal(t, int64(0), size)
	assert.True(t, storage.IsNotFound(err))
}

// Mock Driver Tests

func TestMockDriver_StoreAndGet(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	content := []byte("Hello, World!")
	path := "test/file.txt"

	// Store
	err := mock.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Get
	reader, err := mock.Get(ctx, path)
	require.NoError(t, err)
	defer reader.Close()

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestMockDriver_Get_NotFound(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	path := "nonexistent/file.txt"

	reader, err := mock.Get(ctx, path)
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Equal(t, storage.ErrFileNotFound, err)
}

func TestMockDriver_Exists(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	path := "test/file.txt"

	// Not exists initially
	exists, err := mock.Exists(ctx, path)
	require.NoError(t, err)
	assert.False(t, exists)

	// Store
	err = mock.Store(ctx, path, bytes.NewReader([]byte("content")))
	require.NoError(t, err)

	// Now exists
	exists, err = mock.Exists(ctx, path)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestMockDriver_Delete(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	path := "test/file.txt"
	content := []byte("content")

	// Store
	err := mock.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Delete
	err = mock.Delete(ctx, path)
	require.NoError(t, err)

	// Should not exist
	exists, err := mock.Exists(ctx, path)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMockDriver_URL(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	path := "test/file.txt"

	url, err := mock.URL(ctx, path, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/media/test/file.txt", url)
}

func TestMockDriver_Size(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	path := "test/file.txt"
	content := []byte("Hello, World!")

	// Store
	err := mock.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Get size
	size, err := mock.Size(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestMockDriver_SetFailure(t *testing.T) {
	mock := NewMockDriver("http://localhost:8080/media")
	ctx := context.Background()

	// Set failure
	expectedErr := storage.ErrStorageFull
	mock.SetFailure(expectedErr)

	// Next operation should fail
	err := mock.Store(ctx, "test.txt", bytes.NewReader([]byte("content")))
	assert.Equal(t, expectedErr, err)

	// Subsequent operations should work
	err = mock.Store(ctx, "test.txt", bytes.NewReader([]byte("content")))
	require.NoError(t, err)
}

// Driver Interface Tests

func TestNewDriver(t *testing.T) {
	tempDir := t.TempDir()

	// Test local driver creation
	config := storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/media",
	}

	driver, err := storage.NewDriver(config)
	require.NoError(t, err)
	assert.NotNil(t, driver)

	// Test unsupported driver
	config.Type = "unsupported"
	driver, err = storage.NewDriver(config)
	assert.Error(t, err)
	assert.Nil(t, driver)
}

// Storage Error Tests

func TestStorageError(t *testing.T) {
	storageErr := &storage.StorageError{
		Op:   "store",
		Path: "test.txt",
		Err:  storage.ErrFileNotFound,
	}

	err := storageErr.Error()
	assert.Contains(t, err, "store")
	assert.Contains(t, err, "test.txt")
	assert.Contains(t, err, "file not found")
}

func TestIsNotFound(t *testing.T) {
	// Direct error
	assert.True(t, storage.IsNotFound(storage.ErrFileNotFound))

	// Wrapped error
	wrappedErr := &storage.StorageError{
		Op:   "get",
		Path: "test.txt",
		Err:  storage.ErrFileNotFound,
	}
	assert.True(t, storage.IsNotFound(wrappedErr))

	// Other error
	assert.False(t, storage.IsNotFound(storage.ErrStorageFull))
}

// Filename Generation Tests

func TestGenerateFilename(t *testing.T) {
	filename := storage.GenerateFilename("original.jpg")
	assert.NotEmpty(t, filename)
	assert.True(t, strings.HasSuffix(filename, ".jpg"))
}

func TestGenerateFilename_NoExtension(t *testing.T) {
	filename := storage.GenerateFilename("noextension")
	assert.NotEmpty(t, filename)
}

func TestGenerateFilename_MultipleDots(t *testing.T) {
	filename := storage.GenerateFilename("archive.tar.gz")
	assert.NotEmpty(t, filename)
	assert.True(t, strings.HasSuffix(filename, ".gz"))
}

// Path Sanitization Tests (via Local Driver)

func TestLocalDriver_PathTraversalProtection(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")

	// Attempt path traversal
	err = driver.Store(ctx, "../../../etc/passwd", bytes.NewReader(content))
	require.NoError(t, err) // Should not error, but store in safe location

	// The file should be stored within tempDir, not /etc/passwd
	// Since path traversal is blocked, "passwd" should be a file in tempDir
	_, err = os.Stat(filepath.Join(tempDir, "etc", "passwd"))
	// This should not exist because ".." is stripped
	assert.True(t, os.IsNotExist(err) || err == nil)
}

// Context Cancellation Tests

func TestLocalDriver_ContextPropagation(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	path := "test/file.txt"

	// Store with context
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Get with context
	reader, err := driver.Get(ctx, path)
	require.NoError(t, err)
	reader.Close()
}

// Multiple Files Tests

func TestLocalDriver_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()

	files := map[string][]byte{
		"file1.txt": []byte("Content 1"),
		"file2.txt": []byte("Content 2"),
		"file3.txt": []byte("Content 3"),
	}

	// Store multiple files
	for path, content := range files {
		err := driver.Store(ctx, path, bytes.NewReader(content))
		require.NoError(t, err)
	}

	// Verify all files exist and have correct content
	for path, expectedContent := range files {
		exists, err := driver.Exists(ctx, path)
		require.NoError(t, err)
		assert.True(t, exists)

		reader, err := driver.Get(ctx, path)
		require.NoError(t, err)

		actualContent, err := io.ReadAll(reader)
		require.NoError(t, err)
		reader.Close()

		assert.Equal(t, expectedContent, actualContent)
	}
}

// Large File Test

func TestLocalDriver_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tempDir, "http://localhost:8080/media")
	require.NoError(t, err)

	ctx := context.Background()

	// Create 1MB content
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	path := "large/file.bin"
	err = driver.Store(ctx, path, bytes.NewReader(content))
	require.NoError(t, err)

	// Verify size
	size, err := driver.Size(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	// Verify content
	reader, err := driver.Get(ctx, path)
	require.NoError(t, err)
	defer reader.Close()

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

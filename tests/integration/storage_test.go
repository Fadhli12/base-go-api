//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/go-api-base/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageUploadDownload(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err, "local driver should initialize")

	ctx := context.Background()
	content := []byte("hello, world!")

	err = driver.Store(ctx, "test/upload.txt", bytes.NewReader(content))
	require.NoError(t, err, "Store should succeed")

	reader, err := driver.Get(ctx, "test/upload.txt")
	require.NoError(t, err, "Get should succeed")
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, content, buf.Bytes(), "downloaded content should match uploaded")
}

func TestLocalStorageDelete(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("to be deleted")

	err = driver.Store(ctx, "test/delete-me.txt", bytes.NewReader(content))
	require.NoError(t, err)

	err = driver.Delete(ctx, "test/delete-me.txt")
	require.NoError(t, err, "Delete should succeed")

	exists, err := driver.Exists(ctx, "test/delete-me.txt")
	require.NoError(t, err)
	assert.False(t, exists, "deleted file should not exist")
}

func TestLocalStorageExists(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	exists, err := driver.Exists(ctx, "nonexistent.txt")
	require.NoError(t, err)
	assert.False(t, exists, "nonexistent file should not exist")

	err = driver.Store(ctx, "test/exists-check.txt", bytes.NewReader([]byte("data")))
	require.NoError(t, err)

	exists, err = driver.Exists(ctx, "test/exists-check.txt")
	require.NoError(t, err)
	assert.True(t, exists, "stored file should exist")
}

func TestLocalStorageCreateDirStructure(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	err = driver.Store(ctx, "deep/nested/dir/structure/file.txt", bytes.NewReader([]byte("nested")))
	require.NoError(t, err, "Store should create nested directories")

	fullPath := filepath.Join(tmpDir, "deep", "nested", "dir", "structure", "file.txt")
	_, err = os.Stat(fullPath)
	require.NoError(t, err, "file should exist on disk at nested path")
}

func TestLocalStorageURL(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	url, err := driver.URL(ctx, "images/photo.jpg", 0)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/storage/images/photo.jpg", url)
}

func TestLocalStorageSize(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	content := []byte("1234567890")
	err = driver.Store(ctx, "test/size-check.txt", bytes.NewReader(content))
	require.NoError(t, err)

	size, err := driver.Size(ctx, "test/size-check.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size, "file size should match content length")
}

func TestLocalStorageGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	_, err = driver.Get(ctx, "nonexistent-file.txt")
	require.Error(t, err, "Get on nonexistent file should return error")
	assert.True(t, storage.IsNotFound(err), "error should be ErrFileNotFound")
}

func TestLocalStorageSizeNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	_, err = driver.Size(ctx, "nonexistent-file.txt")
	require.Error(t, err)
	assert.True(t, storage.IsNotFound(err), "error should be ErrFileNotFound")
}

func TestLocalStorageDeleteIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	err = driver.Delete(ctx, "nonexistent-file.txt")
	require.NoError(t, err, "deleting nonexistent file should be idempotent (no error)")
}

func TestLocalStorageConcurrentUploads(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			content := []byte(strings.Repeat("a", idx+1))
			path := filepath.Join("concurrent", "file", string(rune('0'+idx))+".txt")
			done <- driver.Store(ctx, path, bytes.NewReader(content))
		}(i)
	}

	for i := 0; i < 10; i++ {
		err := <-done
		require.NoError(t, err, "concurrent upload should succeed")
	}

	exists, err := driver.Exists(ctx, filepath.Join("concurrent", "file", "0.txt"))
	require.NoError(t, err)
	assert.True(t, exists, "at least one uploaded file should exist")
}

func TestNewStorageDriverFactory(t *testing.T) {
	t.Run("local driver", func(t *testing.T) {
		tmpDir := t.TempDir()
		driver, err := storage.NewDriver(storage.Config{
			Type:     "local",
			LocalPath: tmpDir,
			BaseURL:  "http://localhost:8080/storage",
		})
		require.NoError(t, err)
		require.NotNil(t, driver)
	})

	t.Run("invalid driver", func(t *testing.T) {
		_, err := storage.NewDriver(storage.Config{
			Type: "ftp",
		})
		require.Error(t, err, "unsupported driver should return error")
		assert.Contains(t, err.Error(), "unsupported storage driver type")
	})
}

func TestLocalStorageOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	err = driver.Store(ctx, "test/overwrite.txt", bytes.NewReader([]byte("original")))
	require.NoError(t, err)

	err = driver.Store(ctx, "test/overwrite.txt", bytes.NewReader([]byte("updated")))
	require.NoError(t, err)

	reader, err := driver.Get(ctx, "test/overwrite.txt")
	require.NoError(t, err)
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, []byte("updated"), buf.Bytes(), "overwritten file should have new content")
}

func TestStorageErrorWrapping(t *testing.T) {
	tmpDir := t.TempDir()
	driver, err := storage.NewLocalDriver(tmpDir, "http://localhost:8080/storage")
	require.NoError(t, err)

	ctx := context.Background()

	_, err = driver.Get(ctx, "nonexistent.txt")
	require.Error(t, err)

	storageErr, ok := err.(*storage.StorageError)
	if assert.True(t, ok, "error should be *StorageError") {
		assert.Equal(t, "get", storageErr.Op)
		assert.Equal(t, "nonexistent.txt", storageErr.Path)
	}
}
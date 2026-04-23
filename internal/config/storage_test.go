package config

import (
	"os"
	"testing"
)

func TestDefaultStorageConfig(t *testing.T) {
	cfg := DefaultStorageConfig()

	if cfg.Driver != "local" {
		t.Errorf("expected Driver 'local', got '%s'", cfg.Driver)
	}
	if cfg.LocalPath != "./storage/uploads" {
		t.Errorf("expected LocalPath './storage/uploads', got '%s'", cfg.LocalPath)
	}
	if cfg.BaseURL != "http://localhost:8080/storage" {
		t.Errorf("expected BaseURL 'http://localhost:8080/storage', got '%s'", cfg.BaseURL)
	}
}

func TestS3ConfigParsing(t *testing.T) {
	os.Setenv("STORAGE_DRIVER", "s3")
	os.Setenv("STORAGE_S3_BUCKET", "my-bucket")
	os.Setenv("STORAGE_S3_ACCESS_KEY", "test-key")
	os.Setenv("STORAGE_S3_SECRET_KEY", "test-secret")
	defer os.Unsetenv("STORAGE_DRIVER")
	defer os.Unsetenv("STORAGE_S3_BUCKET")
	defer os.Unsetenv("STORAGE_S3_ACCESS_KEY")
	defer os.Unsetenv("STORAGE_S3_SECRET_KEY")

	// Test that Validate passes for S3 with required fields
	cfg := StorageConfig{
		Driver:      "s3",
		S3Bucket:    "my-bucket",
		S3AccessKey: "test-key",
		S3SecretKey: "test-secret",
		S3Region:    "us-east-1",
	}
	err := validateStorage(&Config{Storage: cfg})
	if err != nil {
		t.Errorf("expected no error for valid S3 config, got: %v", err)
	}
}

func TestInvalidStorageDriver(t *testing.T) {
	cfg := StorageConfig{Driver: "invalid"}
	err := validateStorage(&Config{Storage: cfg})
	if err == nil {
		t.Error("expected error for invalid driver")
	}
}

func TestMissingS3Bucket(t *testing.T) {
	cfg := StorageConfig{
		Driver:      "s3",
		S3Bucket:    "", // missing
		S3AccessKey: "test-key",
		S3SecretKey: "test-secret",
	}
	err := validateStorage(&Config{Storage: cfg})
	if err == nil {
		t.Error("expected error for missing S3 bucket")
	}
}
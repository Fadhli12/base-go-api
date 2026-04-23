// Package config provides application configuration management.
package config

import (
	"fmt"
	"strings"
)

// Validate checks the entire configuration for validity.
// Returns an aggregated error if any validation fails.
func Validate(cfg *Config) error {
	var errors []string

	// Validate storage
	if err := validateStorage(cfg); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate image
	if err := validateImage(cfg); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate swagger
	if err := validateSwagger(cfg); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate cache
	if err := validateCache(cfg); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// validateStorage validates storage configuration.
func validateStorage(cfg *Config) error {
	validDrivers := map[string]bool{"local": true, "s3": true, "minio": true}
	if !validDrivers[cfg.Storage.Driver] {
		return fmt.Errorf("invalid STORAGE_DRIVER: %s (valid: local, s3, minio)", cfg.Storage.Driver)
	}

	if cfg.Storage.Driver == "s3" || cfg.Storage.Driver == "minio" {
		if cfg.Storage.S3Bucket == "" {
			return fmt.Errorf("STORAGE_S3_BUCKET is required when STORAGE_DRIVER=%s", cfg.Storage.Driver)
		}
		if cfg.Storage.S3AccessKey == "" {
			return fmt.Errorf("STORAGE_S3_ACCESS_KEY is required when STORAGE_DRIVER=%s", cfg.Storage.Driver)
		}
		if cfg.Storage.S3SecretKey == "" {
			return fmt.Errorf("STORAGE_S3_SECRET_KEY is required when STORAGE_DRIVER=%s", cfg.Storage.Driver)
		}
		if cfg.Storage.Driver == "minio" && cfg.Storage.S3Endpoint == "" {
			return fmt.Errorf("STORAGE_S3_ENDPOINT is required for MinIO driver")
		}
	}
	return nil
}

// validateImage validates image processing configuration.
func validateImage(cfg *Config) error {
	if !cfg.Image.CompressionEnabled {
		return nil
	}
	if cfg.Image.ThumbnailQuality < 1 || cfg.Image.ThumbnailQuality > 100 {
		return fmt.Errorf("IMAGE_COMPRESSION_THUMBNAIL_QUALITY must be 1-100, got %d", cfg.Image.ThumbnailQuality)
	}
	if cfg.Image.PreviewQuality < 1 || cfg.Image.PreviewQuality > 100 {
		return fmt.Errorf("IMAGE_COMPRESSION_PREVIEW_QUALITY must be 1-100, got %d", cfg.Image.PreviewQuality)
	}
	return nil
}

// validateSwagger validates Swagger configuration.
func validateSwagger(cfg *Config) error {
	if !cfg.Swagger.Enabled {
		return nil
	}
	if cfg.Swagger.Path == "" {
		return fmt.Errorf("SWAGGER_PATH cannot be empty when swagger is enabled")
	}
	if !strings.HasPrefix(cfg.Swagger.Path, "/") {
		return fmt.Errorf("SWAGGER_PATH must start with '/': %s", cfg.Swagger.Path)
	}
	return nil
}

// validateCache validates cache configuration.
func validateCache(cfg *Config) error {
	validDrivers := map[string]bool{"redis": true, "memory": true, "none": true}
	if !validDrivers[cfg.Cache.Driver] {
		return fmt.Errorf("invalid CACHE_DRIVER: %s (valid: redis, memory, none)", cfg.Cache.Driver)
	}
	return nil
}
// Package config provides application configuration management.
package config

// StorageConfig holds media storage configuration.
// Supports local filesystem and S3/MinIO object storage backends.
type StorageConfig struct {
	Driver       string `mapstructure:"driver"`        // local, s3, minio
	LocalPath    string `mapstructure:"local_path"`    // ./storage/uploads
	BaseURL      string `mapstructure:"base_url"`      // http://localhost:8080/storage
	SigningKey   string `mapstructure:"signing_key"`   // secret for signed URLs (default: 32-char random)
	// S3/MinIO fields
	S3Endpoint  string `mapstructure:"s3_endpoint"`   // empty for AWS S3, set for MinIO
	S3Region    string `mapstructure:"s3_region"`     // us-east-1
	S3Bucket    string `mapstructure:"s3_bucket"`     // required for s3/minio
	S3AccessKey string `mapstructure:"s3_access_key"` // required
	S3SecretKey string `mapstructure:"s3_secret_key"` // required
	S3UseSSL    bool   `mapstructure:"s3_use_ssl"`    // true
	S3PathStyle bool   `mapstructure:"s3_path_style"`  // false for AWS, true for MinIO
}

// DefaultStorageConfig returns a StorageConfig with sensible defaults.
func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		Driver:    "local",
		LocalPath: "./storage/uploads",
		BaseURL:   "http://localhost:8080/storage",
		S3Region:  "us-east-1",
		S3UseSSL:  true,
	}
}
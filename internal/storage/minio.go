//go:build s3

// Package storage provides MinIO storage implementation
// This file is only compiled when the 's3' build tag is provided
// Example: go build -tags=s3 ./...

package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MinioDriver wraps S3Driver for MinIO compatibility
// MinIO is S3-compatible, so we reuse the S3 driver with custom endpoint configuration
type MinioDriver struct {
	*S3Driver
	endpoint string
	useSSL   bool
}

// NewMinioDriver creates a new MinIO storage driver
// MinIO uses the same AWS SDK but with a custom endpoint
func NewMinioDriver(cfg Config) (*MinioDriver, error) {
	ctx := context.Background()

	// Create custom resolver for MinIO endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           cfg.Endpoint,
			SigningRegion: cfg.Region,
			Source:        aws.EndpointSourceCustom,
		}, nil
	})

	// Load AWS config with custom endpoint and credentials
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load MinIO config: %w", err)
	}

	// Create S3 client with path-style addressing (required for MinIO)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle // Required for MinIO
	})

	// Verify bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		// Try to create bucket if it doesn't exist
		_, createErr := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(cfg.Bucket),
		})
		if createErr != nil {
			return nil, fmt.Errorf("failed to access or create MinIO bucket %s: %w", cfg.Bucket, err)
		}
	}

	// Create underlying S3 driver
	s3Driver := &S3Driver{
		client: client,
		bucket: cfg.Bucket,
		region: cfg.Region,
	}

	return &MinioDriver{
		S3Driver: s3Driver,
		endpoint: cfg.Endpoint,
		useSSL:   cfg.UseSSL,
	}, nil
}

// GetEndpoint returns the MinIO endpoint URL
func (d *MinioDriver) GetEndpoint() string {
	return d.endpoint
}

// IsSecure returns whether SSL is enabled
func (d *MinioDriver) IsSecure() bool {
	return d.useSSL
}

// String returns a string representation of the MinIO driver
func (d *MinioDriver) String() string {
	protocol := "http"
	if d.useSSL {
		protocol = "https"
	}
	return fmt.Sprintf("MinIO(%s://%s/%s)", protocol, d.endpoint, d.GetBucket())
}

// MakeBucket creates a new bucket (MinIO-specific)
func (d *MinioDriver) MakeBucket(ctx context.Context, bucket string) error {
	_, err := d.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

// ListBuckets returns all buckets (MinIO-specific)
func (d *MinioDriver) ListBuckets(ctx context.Context) ([]string, error) {
	result, err := d.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	var buckets []string
	for _, b := range result.Buckets {
		if b.Name != nil {
			buckets = append(buckets, *b.Name)
		}
	}
	return buckets, nil
}

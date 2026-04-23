// Package storage provides S3 storage implementation

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Driver implements the Driver interface for AWS S3 storage
type S3Driver struct {
	client *s3.Client
	bucket string
	region string
}

// NewS3Driver creates a new S3 storage driver
func NewS3Driver(cfg Config) (*S3Driver, error) {
	ctx := context.Background()

	var opts []func(*config.LoadOptions) error

	// Set region
	opts = append(opts, config.WithRegion(cfg.Region))

	// Set credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			),
		))
	}

	// Custom endpoint for MinIO or custom S3-compatible services
	if cfg.Endpoint != "" {
		opts = append(opts, config.WithEndpointResolver(
			aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           cfg.Endpoint,
					SigningRegion: cfg.Region,
				}, nil
			}),
		))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with options
	clientOpts := []func(*s3.Options){}
	if cfg.PathStyle {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)

	return &S3Driver{
		client: client,
		bucket: cfg.Bucket,
		region: cfg.Region,
	}, nil
}

// Store uploads a file to S3
func (d *S3Driver) Store(ctx context.Context, path string, content io.Reader) error {
	safePath := sanitizePath(path)

	// Upload file to S3
	_, err := d.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
		Body:   content,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to upload file to S3",
			slog.String("bucket", d.bucket),
			slog.String("key", safePath),
			slog.Any("error", err),
		)
		return &StorageError{Op: "store", Path: path, Err: err}
	}

	slog.InfoContext(ctx, "file stored successfully in S3",
		slog.String("bucket", d.bucket),
		slog.String("key", safePath),
	)

	return nil
}

// Get retrieves a file from S3
func (d *S3Driver) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	safePath := sanitizePath(path)

	result, err := d.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, &StorageError{Op: "get", Path: path, Err: ErrFileNotFound}
		}
		slog.ErrorContext(ctx, "failed to get file from S3",
			slog.String("bucket", d.bucket),
			slog.String("key", safePath),
			slog.Any("error", err),
		)
		return nil, &StorageError{Op: "get", Path: path, Err: err}
	}

	return result.Body, nil
}

// Delete removes a file from S3
func (d *S3Driver) Delete(ctx context.Context, path string) error {
	safePath := sanitizePath(path)

	_, err := d.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete file from S3",
			slog.String("bucket", d.bucket),
			slog.String("key", safePath),
			slog.Any("error", err),
		)
		return &StorageError{Op: "delete", Path: path, Err: err}
	}

	slog.InfoContext(ctx, "file deleted successfully from S3",
		slog.String("bucket", d.bucket),
		slog.String("key", safePath),
	)

	return nil
}

// URL generates a presigned URL for accessing the file
// For S3, this generates a presigned URL with the specified expiration
func (d *S3Driver) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
	safePath := sanitizePath(path)

	// Create presigned URL client
	presignClient := s3.NewPresignClient(d.client)

	// Generate presigned URL
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate presigned URL",
			slog.String("bucket", d.bucket),
			slog.String("key", safePath),
			slog.Any("error", err),
		)
		return "", &StorageError{Op: "url", Path: path, Err: err}
	}

	return presignResult.URL, nil
}

// Exists checks if a file exists in S3
func (d *S3Driver) Exists(ctx context.Context, path string) (bool, error) {
	safePath := sanitizePath(path)

	_, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
	})
	if err != nil {
		if isS3NotFound(err) {
			return false, nil
		}
		return false, &StorageError{Op: "exists", Path: path, Err: err}
	}

	return true, nil
}

// Size returns the size of the file in S3
func (d *S3Driver) Size(ctx context.Context, path string) (int64, error) {
	safePath := sanitizePath(path)

	result, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(safePath),
	})
	if err != nil {
		if isS3NotFound(err) {
			return 0, &StorageError{Op: "size", Path: path, Err: ErrFileNotFound}
		}
		return 0, &StorageError{Op: "size", Path: path, Err: err}
	}

	if result.ContentLength == nil {
		return 0, &StorageError{Op: "size", Path: path, Err: fmt.Errorf("content length is nil")}
	}

	return *result.ContentLength, nil
}

// isS3NotFound checks if an S3 error is a "not found" error
func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFoundErr *types.NotFound
	if errors.As(err, &notFoundErr) {
		return true
	}
	var noSuchKeyErr *types.NoSuchKey
	if errors.As(err, &noSuchKeyErr) {
		return true
	}
	// Check error code for S3 compatible services (MinIO, LocalStack)
	var apiErr *types.NoSuchBucket
	if errors.As(err, &apiErr) {
		return false // bucket doesn't exist, not key
	}
	return false
}

// GetBucket returns the S3 bucket name (for testing)
func (d *S3Driver) GetBucket() string {
	return d.bucket
}

// GetRegion returns the AWS region (for testing)
func (d *S3Driver) GetRegion() string {
	return d.region
}

// init registers the S3 driver factory
func init() {
	// This would be called during driver registration
}

// CreateS3Client creates an S3 client with custom credentials
func CreateS3Client(region, accessKey, secretKey string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg), nil
}

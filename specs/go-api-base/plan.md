# Implementation Plan: Dynamic Configuration System

**Branch**: `001-dynamic-config` | **Date**: 2026-04-23 | **Spec**: Feature request for environment-driven configuration

**Summary**: Add dynamic configuration capabilities to allow runtime configuration of storage backend (local/S3/MinIO), image compression quality settings, and Swagger documentation toggle via environment variables.

---

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Echo v4, Viper, GORM, swaggo/echo-swagger, go-aws-sdk (optional for S3)  
**Storage**: PostgreSQL (config persistence), Redis (cache), filesystem/object-storage  
**Testing**: Go testing, testcontainers-go for integration  
**Target Platform**: Linux server, Docker containers  
**Project Type**: REST API web-service  
**Performance Goals**: <200ms p95 latency  
**Constraints**: Zero-downtime config changes where possible, backward-compatible defaults  
**Scale/Scope**: Single API instance, potential multi-instance deployment

### Configuration Reload Scope Clarification

**NOT IMPLEMENTED: Hot Reload at Runtime**

This feature does NOT include runtime configuration reload (SIGHUP, admin API, or file watcher). Configuration is loaded once at startup.

**Rationale**:
- Storage driver switch at runtime risks data corruption (files in mid-transfer)
- Cache driver switch requires careful connection draining
- Swagger toggle is low-risk but not justified on its own
- Complexity not warranted for current scale

**Current Behavior**:
```
Startup → Load Config → Initialize Components → Serve → Shutdown
          ↑ Config loads ONCE here
```

**Future Enhancement Path** (out of scope):
- SIGHUP handler for signal-based reload
- Admin API endpoint `/admin/reload-config`
- File watcher for `.env` changes
- Graceful driver draining protocol

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Gate

| Principle | Status | Notes |
|-----------|--------|-------|
| Production-Ready Foundation | ✅ PASS | All config uses validated defaults |
| RBAC Mandatory | ✅ N/A | No permission changes required |
| Soft Deletes | ✅ N/A | No data model changes |
| Stateless JWT | ✅ N/A | No auth changes |
| SQL Migrations | ✅ N/A | No schema changes |
| Permission Cache Consistency | ✅ N/A | No RBAC changes |
| Audit Logging | ✅ N/A | Config changes are internal |

### Quality Requirements

- [ ] Unit tests: ≥80% coverage on new config parsing logic
- [ ] Integration tests: Verify storage driver switching
- [ ] Linting: `golangci-lint` passes
- [ ] Documentation: Update .env.example with new variables

---

## Project Structure

### Documentation (this feature)

```text
specs/go-api-base/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (config structs)
├── quickstart.md        # Phase 1 output
└── contracts/           # Phase 1 output (env var contracts)
    └── env-vars.md
```

### Source Code (repository root)

```text
internal/
├── config/
│   ├── config.go           # Extended config structs
│   ├── storage.go          # Storage-specific config
│   ├── image.go            # Image compression config
│   └── swagger.go          # Swagger toggle config
├── cache/                   # NEW package
│   ├── cache.go            # Driver interface and factory
│   ├── redis.go            # Redis cache implementation
│   ├── memory.go           # In-memory cache implementation
│   └── noop.go             # No-op cache (disabled)
├── storage/
│   ├── storage.go          # Existing Driver interface (extend)
│   ├── local.go            # Existing
│   ├── s3.go               # New S3 driver implementation
│   └── minio.go            # Existing (extend)
├── conversion/
│   └── config.go           # Configurable compression constants
└── http/
    └── server.go           # Swagger conditional registration

docs/
└── swagger/                # Auto-generated (no changes)

tests/
├── unit/
│   └── config_test.go      # Config parsing tests
└── integration/
    └── storage_test.go     # Storage driver tests
```

**Structure Decision**: Extend existing `internal/config/` and `internal/storage/` modules. No new top-level packages required.

---

## Phase 0: Research & Unknowns Resolution

### Research Topics

| Topic | Status | Findings |
|-------|--------|----------|
| Viper env var binding | ✅ Done | Project uses `v.AutomaticEnv()` with `strings.NewReplacer(".", "_")` |
| S3 SDK for Go | 🔍 Pending | AWS SDK v2 recommended for S3 driver |
| swaggo conditional serve | 🔍 Pending | Route registration can be conditional |

### Current Architecture Analysis

**Config Loading Pattern** (from `internal/config/config.go`):
```go
// Uses singleton pattern
func Load() (*Config, error) {
    once.Do(func() {
        instance, loadErr = loadConfig()
    })
    return instance, loadErr
}

// Viper setup
v.SetEnvPrefix("")
v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
v.AutomaticEnv()
```

**Storage Pattern** (from `internal/storage/storage.go`):
```go
type Driver interface {
    Store(ctx context.Context, path string, content io.Reader) error
    Get(ctx context.Context, path string) (io.ReadCloser, error)
    Delete(ctx context.Context, path string) error
    URL(ctx context.Context, path string, expires time.Duration) (string, error)
    Exists(ctx context.Context, path string) (bool, error)
    Size(ctx context.Context, path string) (int64, error)
}

func NewDriver(config Config) (Driver, error) {
    switch config.Type {
    case "local":
        return NewLocalDriver(config.LocalPath, config.BaseURL)
    case "s3", "minio":
        return nil, fmt.Errorf("S3/MinIO drivers require AWS SDK")
    }
}
```

**Image Conversion** (from `internal/conversion/handler.go`):
```go
const (
    ThumbnailWidth  = 300
    ThumbnailHeight = 300
    ThumbnailQuality = 85  // HARDCODED - needs env config
    
    PreviewWidth  = 800
    PreviewHeight = 600
    PreviewQuality = 90    // HARDCODED - needs env config
)
```

**Swagger Registration** (from `internal/http/server.go:302-303`):
```go
// Always registered, no toggle
swagger := s.echo.Group("/swagger")
swagger.GET("/*", echoSwagger.WrapHandler)
```

---

## Phase 1: Design & Contracts

### 1.1 Environment Variable Contracts

New environment variables to add:

```bash
# =============================================================================
# Storage Configuration (Extended)
# =============================================================================
STORAGE_DRIVER=local                    # local | s3 | minio
STORAGE_LOCAL_PATH=./storage/uploads    # Local driver path
STORAGE_BASE_URL=http://localhost:8080/storage

# S3/MinIO Configuration (when STORAGE_DRIVER=s3|minio)
STORAGE_S3_ENDPOINT=                    # Optional for S3, required for MinIO
STORAGE_S3_REGION=us-east-1             # AWS region
STORAGE_S3_BUCKET=my-bucket             # Bucket name
STORAGE_S3_ACCESS_KEY=                  # AWS_ACCESS_KEY_ID or MinIO access key
STORAGE_S3_SECRET_KEY=                  # AWS_SECRET_ACCESS_KEY or MinIO secret key
STORAGE_S3_USE_SSL=true                 # Use HTTPS
STORAGE_S3_PATH_STYLE=false             # Use path-style (required for MinIO)

# =============================================================================
# Image Compression Configuration
# =============================================================================
IMAGE_COMPRESSION_ENABLED=true          # Enable image compression
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=85  # JPEG quality 1-100
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=300   # Thumbnail width
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=300  # Thumbnail height
IMAGE_COMPRESSION_PREVIEW_QUALITY=90    # Preview quality 1-100
IMAGE_COMPRESSION_PREVIEW_WIDTH=800     # Preview width
IMAGE_COMPRESSION_PREVIEW_HEIGHT=600    # Preview height

# =============================================================================
# Swagger Configuration
# =============================================================================
SWAGGER_ENABLED=true                    # Enable/disable Swagger UI
SWAGGER_PATH=/swagger                   # Swagger UI path prefix

# =============================================================================
# Cache Configuration
# =============================================================================
CACHE_DRIVER=redis                      # redis | memory | none
CACHE_DEFAULT_TTL=300                   # Default cache TTL in seconds (5 min)
CACHE_PERMISSION_TTL=300                # Permission cache TTL in seconds
CACHE_RATE_LIMIT_TTL=60                 # Rate limit window in seconds
CACHE_MAX_KEY_SIZE=1024                 # Max key size in bytes
CACHE_MAX_VALUE_SIZE=1048576            # Max value size in bytes (1MB)
```

### 1.2 Config Struct Extensions

**internal/config/config.go** - Extend existing:

```go
// StorageConfig extended for S3/MinIO support
type StorageConfig struct {
    Driver    string `mapstructure:"driver"`
    LocalPath string `mapstructure:"local_path"`
    BaseURL   string `mapstructure:"base_url"`
    
    // S3/MinIO configuration
    S3Endpoint    string `mapstructure:"s3_endpoint"`
    S3Region     string `mapstructure:"s3_region"`
    S3Bucket     string `mapstructure:"s3_bucket"`
    S3AccessKey  string `mapstructure:"s3_access_key"`
    S3SecretKey  string `mapstructure:"s3_secret_key"`
    S3UseSSL     bool   `mapstructure:"s3_use_ssl"`
    S3PathStyle  bool   `mapstructure:"s3_path_style"`
}

// ImageConfig - NEW struct for compression settings
type ImageConfig struct {
    CompressionEnabled bool `mapstructure:"compression_enabled"`
    
    // Thumbnail settings
    ThumbnailQuality int `mapstructure:"thumbnail_quality"`
    ThumbnailWidth   int `mapstructure:"thumbnail_width"`
    ThumbnailHeight  int `mapstructure:"thumbnail_height"`
    
    // Preview settings
    PreviewQuality int `mapstructure:"preview_quality"`
    PreviewWidth   int `mapstructure:"preview_width"`
    PreviewHeight  int `mapstructure:"preview_height"`
}

// SwaggerConfig - NEW struct for swagger toggle
type SwaggerConfig struct {
    Enabled bool   `mapstructure:"enabled"`
    Path    string `mapstructure:"path"`
}

// CacheConfig - NEW struct for cache driver settings
type CacheConfig struct {
    Driver           string `mapstructure:"driver"`            // redis | memory | none
    DefaultTTL       int    `mapstructure:"default_ttl"`       // seconds
    PermissionTTL    int    `mapstructure:"permission_ttl"`    // seconds
    RateLimitTTL     int    `mapstructure:"rate_limit_ttl"`    // seconds
    MaxKeySize       int    `mapstructure:"max_key_size"`     // bytes
    MaxValueSize     int    `mapstructure:"max_value_size"`   // bytes
}

// Config - extend top-level struct
type Config struct {
    Database DatabaseConfig `mapstructure:"database"`
    Redis    RedisConfig    `mapstructure:"redis"`
    JWT      JWTConfig      `mapstructure:"jwt"`
    Server   ServerConfig   `mapstructure:"server"`
    Log      LogConfig      `mapstructure:"log"`
    CORS     CORSConfig     `mapstructure:"cors"`
    Storage  StorageConfig  `mapstructure:"storage"`
    Image    ImageConfig    `mapstructure:"image"`    // NEW
    Swagger  SwaggerConfig  `mapstructure:"swagger"`  // NEW
    Cache    CacheConfig    `mapstructure:"cache"`    // NEW
}
```

### 1.3 Storage Driver Factory Extension

**DESIGN DECISION**: S3 and MinIO use a **unified driver** implementation. MinIO is S3-compatible, so the same AWS SDK v2 client works for both. The difference is configuration:
- **AWS S3**: Empty `S3Endpoint`, `S3PathStyle=false`
- **MinIO**: Set `S3Endpoint`, `S3PathStyle=true`

**internal/storage/storage.go** - Extend:

```go
func NewDriver(config Config) (Driver, error) {
    switch config.Type {
    case "local":
        return NewLocalDriver(config.LocalPath, config.BaseURL)
    case "s3", "minio":
        // Single driver handles both S3 and MinIO via config differences
        return NewS3Driver(config)
    default:
        return nil, fmt.Errorf("unsupported storage driver type: %s", config.Type)
    }
}
```

**internal/storage/s3.go** - NEW implementation:

```go
package storage

import (
    "context"
    "io"
    "time"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Driver implements storage.Driver for S3 and MinIO
type S3Driver struct {
    client *s3.Client
    bucket string
    presignClient *s3.PresignClient
    isMinIO bool
}

// NewS3Driver creates a new S3/MinIO storage driver
// For MinIO: set Endpoint and PathStyle=true
// For AWS S3: leave Endpoint empty, PathStyle=false
func NewS3Driver(cfg Config) (*S3Driver, error) {
    ctx := context.Background()
    
    // Build AWS config options
    var opts []func(*config.LoadOptions) error
    
    // Static credentials (supports both AWS and MinIO)
    opts = append(opts, config.WithCredentialsProvider(
        credentials.NewStaticCredentialsProvider(
            cfg.S3AccessKey,
            cfg.S3SecretKey,
            "",
        ),
    ))
    
    // Custom endpoint for MinIO
    if cfg.S3Endpoint != "" {
        opts = append(opts, config.WithEndpointResolver(
            aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
                return aws.Endpoint{
                    URL:           cfg.S3Endpoint,
                    SigningRegion: cfg.S3Region,
                }, nil
            }),
        ))
    }
    
    // Region
    opts = append(opts, config.WithRegion(cfg.S3Region))
    
    // Load config
    awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }
    
    // Create S3 client
    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        // Path-style addressing for MinIO
        if cfg.S3PathStyle {
            o.UsePathStyle = true
        }
        // Disable HTTPS if needed (local development)
        if !cfg.S3UseSSL {
            o.EndpointResolver = s3.EndpointResolverFromURL(
                strings.Replace(cfg.S3Endpoint, "https://", "http://", 1),
            )
        }
    })
    
    return &S3Driver{
        client:        client,
        bucket:        cfg.S3Bucket,
        presignClient: s3.NewPresignClient(client),
        isMinIO:       cfg.S3PathStyle,
    }, nil
}

// Store uploads a file to S3/MinIO
func (d *S3Driver) Store(ctx context.Context, path string, content io.Reader) error {
    _, err := d.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
        Body:   content,
    })
    return err
}

// Get downloads a file from S3/MinIO
func (d *S3Driver) Get(ctx context.Context, path string) (io.ReadCloser, error) {
    resp, err := d.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
    })
    if err != nil {
        var noSuchKey *types.NoSuchKey
        if errors.As(err, &noSuchKey) {
            return nil, ErrFileNotFound
        }
        return nil, err
    }
    return resp.Body, nil
}

// Delete removes a file from S3/MinIO
func (d *S33Driver) Delete(ctx context.Context, path string) error {
    _, err := d.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
    })
    return err
}

// URL generates a presigned URL for secure access
func (d *S3Driver) URL(ctx context.Context, path string, expires time.Duration) (string, error) {
    req, err := d.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
    }, s3.WithPresignExpires(expires))
    if err != nil {
        return "", err
    }
    return req.URL, nil
}

// Exists checks if a file exists
func (d *S3Driver) Exists(ctx context.Context, path string) (bool, error) {
    _, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
    })
    if err != nil {
        var notFound *types.NotFound
        if errors.As(err, &notFound) {
            return false, nil
        }
        return false, err
    }
    return true, nil
}

// Size returns the file size
func (d *S3Driver) Size(ctx context.Context, path string) (int64, error) {
    resp, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(d.bucket),
        Key:    aws.String(path),
    })
    if err != nil {
        return 0, err
    }
    return resp.ContentLength, nil
}
```

### 1.4 Conversion Config Injection

**internal/conversion/config.go** - NEW file:

```go
package conversion

// Config holds image conversion configuration
type Config struct {
    Enabled         bool
    ThumbnailWidth  int
    ThumbnailHeight int
    ThumbnailQuality int
    PreviewWidth    int
    PreviewHeight   int
    PreviewQuality  int
}

// DefaultConfig returns production-safe defaults
func DefaultConfig() Config {
    return Config{
        Enabled:         true,
        ThumbnailWidth:  300,
        ThumbnailHeight: 300,
        ThumbnailQuality: 85,
        PreviewWidth:    800,
        PreviewHeight:   600,
        PreviewQuality:  90,
    }
}

// DefaultThumbnailDef returns thumbnail conversion definition from config
func (c Config) ThumbnailDef() ConversionDef {
    return ConversionDef{
        Name:    "thumbnail",
        Width:   c.ThumbnailWidth,
        Height:  c.ThumbnailHeight,
        Quality: c.ThumbnailQuality,
        Format:  "jpeg",
    }
}

// DefaultPreviewDef returns preview conversion definition from config
func (c Config) PreviewDef() ConversionDef {
    return ConversionDef{
        Name:    "preview",
        Width:   c.PreviewWidth,
        Height:  c.PreviewHeight,
        Quality: c.PreviewQuality,
        Format:  "jpeg",
    }
}

// GetDefaultConversions returns default conversions using config values
func (c Config) GetDefaultConversions() []ConversionDef {
    return []ConversionDef{
        c.ThumbnailDef(),
        c.PreviewDef(),
    }
}
```

#### Call Site Migration (internal/conversion/)

The following files need modification to use the new configurable values:

| File | Line(s) | Current Usage | Migration |
|------|---------|---------------|-----------|
| `handler.go` | 136-144 | `DefaultThumbnailDef()` uses hardcoded constants | Accept `Config` parameter |
| `handler.go` | 147-155 | `DefaultPreviewDef()` uses hardcoded constants | Accept `Config` parameter |
| `handler.go` | 191-196 | `GetDefaultConversions()` calls static functions | Accept `Config` parameter |
| `handler.go` | 41-48 | `CreateConversionJobs()` calls `GetDefaultConversions()` | Inject config from caller |
| `worker.go` | TBD | Conversion worker uses `ConversionDef` | No change needed (uses def struct) |

**Migration Pattern**:
```go
// BEFORE (hardcoded)
func DefaultThumbnailDef() ConversionDef {
    return ConversionDef{
        Name:    "thumbnail",
        Width:   ThumbnailWidth,    // Hardcoded const
        Height:  ThumbnailHeight,   // Hardcoded const
        Quality: ThumbnailQuality,  // Hardcoded const
        Format:  "jpeg",
    }
}

// AFTER (configurable)
func DefaultThumbnailDef(cfg Config) ConversionDef {
    return ConversionDef{
        Name:    "thumbnail",
        Width:   cfg.ThumbnailWidth,
        Height:  cfg.ThumbnailHeight,
        Quality: cfg.ThumbnailQuality,
        Format:  "jpeg",
    }
}
```

**Wire-up in main.go**:
```go
// cmd/api/main.go
func runServer() error {
    cfg, _ := config.Load()
    
    // Create conversion config
    conversionCfg := conversion.Config{
        Enabled:          cfg.Image.CompressionEnabled,
        ThumbnailWidth:   cfg.Image.ThumbnailWidth,
        ThumbnailHeight:  cfg.Image.ThumbnailHeight,
        ThumbnailQuality: cfg.Image.ThumbnailQuality,
        PreviewWidth:     cfg.Image.PreviewWidth,
        PreviewHeight:    cfg.Image.PreviewHeight,
        PreviewQuality:   cfg.Image.PreviewQuality,
    }
    
    // Pass to handlers/services that need it
    // ...
}
```

### 1.5 Conditional Swagger Registration

**CLARIFICATION: Swagger Doc Generation vs Toggle**

There are TWO separate concerns regarding Swagger documentation:

1. **Doc Generation** (build-time) - NOT affected by this toggle
   - Swag CLI generates docs from annotations during `make swagger`
   - Output files are generated regardless of `SWAGGER_ENABLED` setting
   - Files live in `docs/swagger/` directory

2. **Route Exposure** (runtime) - IS controlled by this toggle
   - The `SWAGGER_ENABLED=true/false` env var controls route registration
   - When `false`: no `/swagger/*` endpoints are registered
   - This hides the UI and JSON endpoint from the running server

**Why This Distinction Matters**:
- Generated docs are versioned with code (optional)
- Route exposure can be controlled per-environment (e.g., disabled in production)
- No runtime overhead when disabled (routes not registered)

**Implementation**:

```go
// Makefile - doc generation (unaffected by toggle)
swagger:
	swag init -g cmd/api/main.go -o docs/swagger

// internal/http/server.go - route registration (conditional)
func (s *Server) RegisterRoutes() {
    // ...existing code...
    
    // Conditional Swagger registration (runtime control)
    if s.config.Swagger.Enabled {
        swagger := s.echo.Group(s.config.Swagger.Path)
        swagger.GET("/*", echoSwagger.WrapHandler)
    }
    
    // ...rest of routes...
}
```

**Deployment Implications**:

| Environment | SWAGGER_ENABLED | Swag Generated | Routes Registered |
|-------------|-----------------|----------------|-------------------|
| Development | `true` | Yes | Yes |
| Staging | `true` or `false` | Yes | Depends on config |
| Production | `false` | Yes (can skip) | No |

**internal/http/server.go** - Modify:

```go
func (s *Server) RegisterRoutes() {
    // ...existing code...
    
    // Conditional Swagger registration
    if s.config.Swagger.Enabled {
        swagger := s.echo.Group(s.config.Swagger.Path)
        swagger.GET("/*", echoSwagger.WrapHandler)
    }
    
    // ...rest of routes...
}
```

### 1.6 Cache Driver Interface

**internal/cache/cache.go** - NEW package:

```go
package cache

import "context"

// Driver interface for cache backends
type Driver interface {
    // Get retrieves a value from cache
    Get(ctx context.Context, key string) ([]byte, error)
    
    // Set stores a value in cache with TTL
    Set(ctx context.Context, key string, value []byte, ttlSeconds int) error
    
    // Delete removes a key from cache
    Delete(ctx context.Context, key string) error
    
    // Exists checks if a key exists
    Exists(ctx context.Context, key string) (bool, error)
    
    // Clear removes all keys matching pattern
    Clear(ctx context.Context, pattern string) error
    
    // Close releases resources
    Close() error
}

// Config holds cache configuration
type Config struct {
    Driver         string // redis | memory | none
    DefaultTTL     int    // seconds
    PermissionTTL  int    // seconds
    RateLimitTTL   int    // seconds
    MaxKeySize     int    // bytes
    MaxValueSize   int    // bytes
}

// NewDriver creates a cache driver based on configuration
func NewDriver(cfg Config, redisClient *redis.Client) (Driver, error) {
    switch cfg.Driver {
    case "redis":
        if redisClient == nil {
            return nil, errors.New("redis driver requires Redis client")
        }
        return NewRedisCache(redisClient, cfg), nil
    case "memory":
        return NewMemoryCache(cfg), nil
    case "none":
        return NewNoopCache(), nil
    default:
        return nil, fmt.Errorf("unsupported cache driver: %s", cfg.Driver)
    }
}

// RedisCache implements Driver using Redis
type RedisCache struct {
    client *redis.Client
    config Config
}

// MemoryCache implements Driver using in-memory sync.Map
type MemoryCache struct {
    data   sync.Map
    config Config
    mu     sync.RWMutex
}

// NoopCache implements Driver with no-op operations (caching disabled)
type NoopCache struct{}
```

**Usage in permission cache** (internal/permission/cache.go) - Modify:

```go
// NewCache creates a permission cache with configurable TTL
func NewCache(driver cache.Driver, cfg *config.CacheConfig) *Cache {
    return &Cache{
        driver:  driver,
        ttl:     time.Duration(cfg.PermissionTTL) * time.Second,
    }
}
```

**Usage in rate limiter** (internal/http/middleware/rate_limit.go) - Modify:

```go
// NewRateLimiter creates a rate limiter with configurable TTL
func NewRateLimiter(cacheDriver cache.Driver, cfg *config.CacheConfig) *RateLimiter {
    return &RateLimiter{
        cache:   cacheDriver,
        ttl:     time.Duration(cfg.RateLimitTTL) * time.Second,
        limit:   100, // configurable via separate env var
    }
}
```

### 1.7 Configuration Validation

**internal/config/validate.go** - NEW file:

```go
package config

import (
    "fmt"
    "strings"
)

// Validate checks all configuration values for correctness
// Returns detailed error if validation fails
func Validate(cfg *Config) error {
    var errors []string

    // Validate storage driver
    if err := validateStorage(cfg); err != nil {
        errors = append(errors, err.Error())
    }

    // Validate image settings
    if err := validateImage(cfg); err != nil {
        errors = append(errors, err.Error())
    }

    // Validate cache settings
    if err := validateCache(cfg); err != nil {
        errors = append(errors, err.Error())
    }

    // Validate swagger settings
    if err := validateSwagger(cfg); err != nil {
        errors = append(errors, err.Error())
    }

    if len(errors) > 0 {
        return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
    }

    return nil
}

func validateStorage(cfg *Config) error {
    validDrivers := map[string]bool{"local": true, "s3": true, "minio": true}
    if !validDrivers[cfg.Storage.Driver] {
        return fmt.Errorf("invalid STORAGE_DRIVER: %s (valid: local, s3, minio)", cfg.Storage.Driver)
    }

    // S3/MinIO require additional fields
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
        // MinIO-specific validation
        if cfg.Storage.Driver == "minio" && cfg.Storage.S3Endpoint == "" {
            return fmt.Errorf("STORAGE_S3_ENDPOINT is required for MinIO driver")
        }
        if cfg.Storage.Driver == "minio" && !cfg.Storage.S3PathStyle {
            // Warn but don't fail - some MinIO setups work without path style
            // Log warning in production
        }
    }

    return nil
}

func validateImage(cfg *Config) error {
    if !cfg.Image.CompressionEnabled {
        return nil // Skip validation if compression disabled
    }

    if cfg.Image.ThumbnailQuality < 1 || cfg.Image.ThumbnailQuality > 100 {
        return fmt.Errorf("IMAGE_COMPRESSION_THUMBNAIL_QUALITY must be 1-100, got %d", cfg.Image.ThumbnailQuality)
    }
    if cfg.Image.PreviewQuality < 1 || cfg.Image.PreviewQuality > 100 {
        return fmt.Errorf("IMAGE_COMPRESSION_PREVIEW_QUALITY must be 1-100, got %d", cfg.Image.PreviewQuality)
    }
    if cfg.Image.ThumbnailWidth < 1 || cfg.Image.ThumbnailWidth > 4096 {
        return fmt.Errorf("IMAGE_COMPRESSION_THUMBNAIL_WIDTH must be 1-4096, got %d", cfg.Image.ThumbnailWidth)
    }
    if cfg.Image.ThumbnailHeight < 1 || cfg.Image.ThumbnailHeight > 4096 {
        return fmt.Errorf("IMAGE_COMPRESSION_THUMBNAIL_HEIGHT must be 1-4096, got %d", cfg.Image.ThumbnailHeight)
    }
    if cfg.Image.PreviewWidth < 1 || cfg.Image.PreviewWidth > 4096 {
        return fmt.Errorf("IMAGE_COMPRESSION_PREVIEW_WIDTH must be 1-4096, got %d", cfg.Image.PreviewWidth)
    }
    if cfg.Image.PreviewHeight < 1 || cfg.Image.PreviewHeight > 4096 {
        return fmt.Errorf("IMAGE_COMPRESSION_PREVIEW_HEIGHT must be 1-4096, got %d", cfg.Image.PreviewHeight)
    }

    return nil
}

func validateCache(cfg *Config) error {
    validDrivers := map[string]bool{"redis": true, "memory": true, "none": true}
    if !validDrivers[cfg.Cache.Driver] {
        return fmt.Errorf("invalid CACHE_DRIVER: %s (valid: redis, memory, none)", cfg.Cache.Driver)
    }

    // Redis driver requires Redis client (validated at runtime in NewDriver)
    
    if cfg.Cache.DefaultTTL < 0 {
        return fmt.Errorf("CACHE_DEFAULT_TTL cannot be negative: %d", cfg.Cache.DefaultTTL)
    }
    if cfg.Cache.PermissionTTL < 0 {
        return fmt.Errorf("CACHE_PERMISSION_TTL cannot be negative: %d", cfg.Cache.PermissionTTL)
    }
    if cfg.Cache.RateLimitTTL < 0 {
        return fmt.Errorf("CACHE_RATE_LIMIT_TTL cannot be negative: %d", cfg.Cache.RateLimitTTL)
    }

    return nil
}

func validateSwagger(cfg *Config) error {
    if cfg.Swagger.Path == "" {
        return fmt.Errorf("SWAGGER_PATH cannot be empty when swagger is enabled")
    }
    if !strings.HasPrefix(cfg.Swagger.Path, "/") {
        return fmt.Errorf("SWAGGER_PATH must start with '/': %s", cfg.Swagger.Path)
    }
    return nil
}
```

**Integration in loadConfig()**:

```go
// internal/config/config.go - modify loadConfig()
func loadConfig() (*Config, error) {
    v := viper.New()
    
    // ... existing setup code ...
    
    cfg := &Config{}
    
    // Parse all config sections
    if err := parseAllConfigs(v, cfg); err != nil {
        return nil, err
    }
    
    // VALIDATE CONFIGURATION (NEW)
    if err := Validate(cfg); err != nil {
        return nil, fmt.Errorf("configuration validation failed: %w", err)
    }
    
    return cfg, nil
}
```

**Startup Behavior on Invalid Config**:

| Scenario | Behavior | Log Message |
|----------|----------|-------------|
| Invalid `STORAGE_DRIVER` | Fail to start | `configuration validation failed: invalid STORAGE_DRIVER: xyz` |
| Missing S3 credentials | Fail to start | `STORAGE_S3_BUCKET is required when STORAGE_DRIVER=s3` |
| Invalid quality (150) | Fail to start | `IMAGE_COMPRESSION_THUMBNAIL_QUALITY must be 1-100` |
| Invalid dimensions | Fail to start | `IMAGE_COMPRESSION_THUMBNAIL_WIDTH must be 1-4096` |
| Invalid `CACHE_DRIVER` | Fail to start | `invalid CACHE_DRIVER: xyz` |

---

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None required | All changes follow existing patterns | N/A |

---

## Implementation Phases

### Phase 2.1: Config Struct Extensions

**Changes**:
- Extend `StorageConfig` with S3 fields
- Add `ImageConfig` struct
- Add `SwaggerConfig` struct
- Add `CacheConfig` struct
- Update `Config` top-level struct
- Add parser functions and defaults

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| Default storage config | No env vars set | `driver=local, local_path=./uploads` | Unit test `TestDefaultStorageConfig` |
| S3 config parsing | `STORAGE_DRIVER=s3, STORAGE_S3_BUCKET=mybucket` | Config.S3Bucket="mybucket" | Unit test `TestS3ConfigParsing` |
| Image config parsing | `IMAGE_COMPRESSION_ENABLED=false` | Config.Image.CompressionEnabled=false | Unit test `TestImageConfigParsing` |
| Invalid storage driver | `STORAGE_DRIVER=invalid` | Validation error | Unit test `TestInvalidStorageDriver` |
| Missing S3 bucket | `STORAGE_DRIVER=s3` (no bucket) | Validation error | Unit test `TestMissingS3Bucket` |
| Cache config parsing | `CACHE_DRIVER=memory` | Config.Cache.Driver="memory" | Unit test `TestCacheConfigParsing` |

### Phase 2.2: S3/MinIO Storage Driver

**Changes**:
- Implement `NewS3Driver()` using AWS SDK v2
- Configure for MinIO compatibility via endpoint/path-style
- Add presigned URL generation
- Update factory function to handle "s3" and "minio"

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| AWS S3 upload | `STORAGE_DRIVER=s3, valid credentials` | File uploaded to S3 | Integration test with LocalStack |
| MinIO upload | `STORAGE_DRIVER=minio, S3_ENDPOINT=http://minio:9000, S3_PATH_STYLE=true` | File uploaded to MinIO | Integration test with MinIO container |
| Presigned URL generation | Any S3/MinIO driver | URL with signature valid for TTL | Unit test `TestPresignedURL` |
| Invalid S3 credentials | Wrong access key | `storage.ErrConnectionFailed` | Unit test `TestS3InvalidCredentials` |
| LocalStack connectivity | `S3_ENDPOINT=http://localstack:4566` | Driver connects successfully | Integration test |
| File not found | Get non-existent key | `storage.ErrFileNotFound` | Unit test `TestS3FileNotFound` |

### Phase 2.3: Image Compression Configuration

**Changes**:
- Create `internal/conversion/config.go`
- Replace hardcoded constants with config values
- Inject config into conversion handlers
- Update `GetDefaultConversions()` to use config

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| Default compression values | No env vars | Quality: 85/90, Size: 300x300/800x600 | Unit test `TestDefaultConversionConfig` |
| Custom thumbnail quality | `IMAGE_COMPRESSION_THUMBNAIL_QUALITY=75` | Thumbnail quality=75 | Unit test `TestCustomThumbnailQuality` |
| Compression disabled | `IMAGE_COMPRESSION_ENABLED=false` | No conversion jobs created | Unit test `TestCompressionDisabled` |
| Invalid quality (out of range) | `IMAGE_COMPRESSION_THUMBNAIL_QUALITY=150` | Validation error | Unit test `TestInvalidQuality` |
| Invalid dimensions | `IMAGE_COMPRESSION_PREVIEW_WIDTH=5000` | Validation error | Unit test `TestInvalidDimensions` |
| Config injection | Upload image with custom config | Output matches config dimensions | Integration test `TestImageUploadWithConfig` |

### Phase 2.4: Swagger Toggle

**Changes**:
- Add swagger config parsing
- Conditional swagger route registration
- Update .env.example

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| Swagger enabled (default) | `SWAGGER_ENABLED=true` or unset | Routes registered at `/swagger/*` | Integration test `TestSwaggerEnabled` |
| Swagger disabled | `SWAGGER_ENABLED=false` | `/swagger/*` returns 404 | Integration test `TestSwaggerDisabled` |
| Custom swagger path | `SWAGGER_PATH=/api-docs` | Routes at `/api-docs/*` | Integration test `TestCustomSwaggerPath` |
| Invalid swagger path | `SWAGGER_PATH=api-docs` (no leading /) | Validation error | Unit test `TestInvalidSwaggerPath` |
| Swagger JSON accessible | Swagger enabled | `GET /swagger/doc.json` returns spec | Integration test |
| Swagger UI accessible | Swagger enabled | `GET /swagger/index.html` returns UI | Integration test |

### Phase 2.5: Cache Driver Configuration

**Changes**:
- Create `internal/cache/` package with driver interface
- Implement `RedisCache` (existing Redis client wrapper)
- Implement `MemoryCache` (in-memory sync.Map based)
- Implement `NoopCache` (disabled caching)
- Update permission cache and rate limiter to use driver interface
- Make cache TTL configurable via env vars

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| Redis cache driver | `CACHE_DRIVER=redis` | Uses Redis for caching | Integration test with Redis container |
| Memory cache driver | `CACHE_DRIVER=memory` | Uses sync.Map for caching | Unit test `TestMemoryCache` |
| Noop cache driver | `CACHE_DRIVER=none` | All operations are no-ops | Unit test `TestNoopCache` |
| Invalid cache driver | `CACHE_DRIVER=invalid` | Validation error | Unit test `TestInvalidCacheDriver` |
| Permission cache TTL | `CACHE_PERMISSION_TTL=600` | Cache expires after 600s | Unit test `TestPermissionCacheTTL` |
| Rate limiter with memory cache | `CACHE_DRIVER=memory` | Rate limiting works without Redis | Integration test |
| Cache invalidation | Permission revoked | Cache invalidated | Integration test `TestPermissionCacheInvalidation` |

### Phase 2.6: Testing & Documentation

**Changes**:
- Unit tests for config parsing
- Integration tests for storage drivers
- Integration tests for cache drivers
- Update README.md and .env.example

**QA Scenarios**:
| Test Case | Input | Expected | Validation |
|-----------|-------|----------|-----------|
| Full config integration test | All env vars set | All components use configured values | Integration test `TestFullConfigIntegration` |
| Backward compatibility | No new env vars | System behaves as before changes | Integration test `TestBackwardCompatibility` |
| Config hot reload | `make serve` + changed .env | Requires restart (no hot reload) | Manual test |
| Documentation accuracy | Follow .env.example | System starts successfully | Manual verification |
| Missing .env file | No .env, no env vars | Uses all defaults | Unit test `TestConfigDefaults` |
| README examples | Follow README instructions | Works as documented | Manual verification |

---

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/config v1.26.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.47.0
    github.com/aws/aws-sdk-go-v2/credentials v1.16.0
)
```

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| S3 credentials exposure | Medium | High | Use env vars, never log secrets |
| Config parsing failure | Low | Medium | Comprehensive defaults + validation |
| Breaking existing storage | Low | High | Maintain backward-compatible defaults |
| Swagger disable breaks docs | Low | Low | Default enabled, clear .env docs |

---

## Success Criteria

1. **Storage**: Can switch between local/S3/MinIO via environment variables
2. **Compression**: Image quality/size configurable without code changes
3. **Swagger**: Can completely disable Swagger in production
4. **Cache**: Can switch between redis/memory/none via environment variables
5. **Backward Compatible**: All existing configs work without changes
6. **Tests**: ≥80% coverage on new code
7. **Documentation**: .env.example updated with all new variables
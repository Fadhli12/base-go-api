# Media Library Configuration Reference

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Production-Ready

---

## Table of Contents

1. [Quick Reference](#quick-reference)
2. [Core Configuration](#core-configuration)
3. [Storage Configuration](#storage-configuration)
4. [Security Configuration](#security-configuration)
5. [Upload Limits](#upload-limits)
6. [Conversion Configuration](#conversion-configuration)
7. [Environment Examples](#environment-examples)
8. [Configuration Validation](#configuration-validation)

---

## Quick Reference

### Required Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `MEDIA_STORAGE_DRIVER` | Storage backend | `local`, `s3`, `minio` |
| `MEDIA_STORAGE_PATH` | Storage path/bucket | `./storage` or `my-bucket` |
| `MEDIA_BASE_URL` | URL for generated links | `https://cdn.example.com` |
| `MEDIA_SIGNING_SECRET` | HMAC secret for URLs | `min-32-char-secret` |

### Optional Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `MEDIA_MAX_FILE_SIZE` | `104857600` (100MB) | Maximum upload size |
| `MEDIA_SMALL_FILE_THRESHOLD` | `5242880` (5MB) | Sync/async threshold |
| `MEDIA_CONVERSION_WORKERS` | `4` | Number of async workers |

---

## Core Configuration

### MEDIA_STORAGE_DRIVER

**Required**: Yes  
**Default**: `local`  
**Description**: Determines which storage backend to use.

**Options**:
| Value | Description | Use Case |
|-------|-------------|----------|
| `local` | Local filesystem | Development, small deployments |
| `s3` | AWS S3 | Production, scalable storage |
| `minio` | MinIO compatible | Self-hosted S3-compatible |

**Example**:
```bash
MEDIA_STORAGE_DRIVER=s3
```

### MEDIA_STORAGE_PATH

**Required**: Yes  
**Default**: `./storage`  
**Description**: Path for local storage or bucket name for S3/MinIO.

**Local**:
```bash
MEDIA_STORAGE_PATH=/var/lib/media
```

**S3/MinIO**:
```bash
MEDIA_STORAGE_PATH=my-media-bucket
```

### MEDIA_BASE_URL

**Required**: Yes  
**Description**: Base URL used for generating public URLs to media files.

**Local Development**:
```bash
MEDIA_BASE_URL=http://localhost:8080
```

**Production with CDN**:
```bash
MEDIA_BASE_URL=https://cdn.example.com
```

**Production with S3 direct**:
```bash
MEDIA_BASE_URL=https://my-bucket.s3.amazonaws.com
```

---

## Storage Configuration

### Local Storage

**Required Variables**:
```bash
MEDIA_STORAGE_DRIVER=local
MEDIA_STORAGE_PATH=/app/storage
MEDIA_BASE_URL=http://localhost:8080
```

**Optional**:
```bash
# Permissions for created directories
MEDIA_DIR_PERMISSIONS=0755
MEDIA_FILE_PERMISSIONS=0644
```

**Directory Structure**:
```
/storage
├── news/
│   └── {model_id}/
│       ├── {uuid}.jpg          # Original
│       └── conversions/
│           └── {uuid}_thumb.jpg
├── user/
│   └── {model_id}/
└── invoice/
    └── {model_id}/
```

### S3 Storage

**Required Variables**:
```bash
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_PATH=my-bucket-name
MEDIA_BASE_URL=https://my-bucket.s3.amazonaws.com

AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Optional Variables**:

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_ENDPOINT` | (AWS default) | Custom endpoint for S3-compatible |
| `S3_USE_SSL` | `true` | Use HTTPS for connections |
| `S3_PATH_STYLE` | `false` | Use path-style addressing |
| `S3_ACL` | `private` | Default ACL for uploads |

**S3-Compatible (MinIO, DigitalOcean Spaces)**:
```bash
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_PATH=my-bucket
MEDIA_BASE_URL=https://spaces.nyc3.digitaloceanspaces.com/my-bucket

AWS_REGION=nyc3
AWS_ACCESS_KEY_ID=your-spaces-key
AWS_SECRET_ACCESS_KEY=your-spaces-secret
S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
S3_USE_SSL=true
S3_PATH_STYLE=false
```

**MinIO Configuration**:
```bash
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_PATH=my-bucket
MEDIA_BASE_URL=http://minio:9000/my-bucket

AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin
S3_ENDPOINT=http://minio:9000
S3_USE_SSL=false
S3_PATH_STYLE=true
```

### MinIO Direct Driver

**Alternative Configuration** (using MinIO SDK):
```bash
MEDIA_STORAGE_DRIVER=minio
MEDIA_STORAGE_PATH=my-bucket
MEDIA_BASE_URL=http://localhost:9000/my-bucket

MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_USE_SSL=false
```

---

## Security Configuration

### MEDIA_SIGNING_SECRET

**Required**: Yes  
**Description**: Secret key for HMAC-SHA256 signing of download URLs.

**Requirements**:
- Minimum 32 characters
- Keep secure (treat like JWT_SECRET)
- Rotate periodically in production

**Generation**:
```bash
# Linux/macOS
openssl rand -base64 32

# PowerShell
$bytes = New-Object byte[] 32
(New-Object Security.Cryptography.RNGCryptoServiceProvider).GetBytes($bytes)
[Convert]::ToBase64String($bytes)
```

**Example**:
```bash
MEDIA_SIGNING_SECRET=your-32-char-signing-secret-here
```

### CORS Configuration

When using direct S3/MinIO access, configure CORS:

**S3 CORS Policy**:
```json
{
  "CORSRules": [
    {
      "AllowedOrigins": ["https://app.example.com"],
      "AllowedMethods": ["GET", "HEAD"],
      "AllowedHeaders": ["*"],
      "MaxAgeSeconds": 3600
    }
  ]
}
```

**Application CORS** (via `CORS_ALLOWED_ORIGINS`):
```bash
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

---

## Upload Limits

### MEDIA_MAX_FILE_SIZE

**Required**: No  
**Default**: `104857600` (100MB)  
**Description**: Maximum file upload size in bytes.

**Examples**:
```bash
# 10MB
MEDIA_MAX_FILE_SIZE=10485760

# 50MB
MEDIA_MAX_FILE_SIZE=52428800

# 100MB (default)
MEDIA_MAX_FILE_SIZE=104857600

# 500MB (not recommended)
MEDIA_MAX_FILE_SIZE=524288000
```

**HTTP Server Configuration**:

For large uploads, ensure your HTTP server is configured:

**Nginx**:
```nginx
client_max_body_size 100m;
```

**Kubernetes Ingress**:
```yaml
annotations:
  nginx.ingress.kubernetes.io/proxy-body-size: "100m"
```

### MEDIA_SMALL_FILE_THRESHOLD

**Required**: No  
**Default**: `5242880` (5MB)  
**Description**: File size threshold for sync vs async conversion processing.

**How it works**:
- Files < threshold: Conversions processed synchronously (blocks response)
- Files >= threshold: Conversions queued for async processing

**Examples**:
```bash
# All files under 2MB process synchronously
MEDIA_SMALL_FILE_THRESHOLD=2097152

# Files under 10MB process synchronously
MEDIA_SMALL_FILE_THRESHOLD=10485760
```

---

## Conversion Configuration

### MEDIA_CONVERSION_WORKERS

**Required**: No  
**Default**: `4`  
**Description**: Number of concurrent workers for async conversion processing.

**Examples**:
```bash
# Small deployment
MEDIA_CONVERSION_WORKERS=2

# Medium deployment (default)
MEDIA_CONVERSION_WORKERS=4

# Large deployment
MEDIA_CONVERSION_WORKERS=8
```

**Scaling Considerations**:
- More workers = faster processing but higher CPU/memory
- Each worker processes one conversion at a time
- Recommendation: 2-4 workers per CPU core

### Conversion Quality Settings

**Thumbnail**:
| Setting | Default | Description |
|---------|---------|-------------|
| Width | 300px | Target width |
| Height | 300px | Target height |
| Quality | 85 | JPEG quality (0-100) |
| Format | jpeg | Output format |

**Preview**:
| Setting | Default | Description |
|---------|---------|-------------|
| Width | 800px | Target width |
| Height | 600px | Target height |
| Quality | 90 | JPEG quality (0-100) |
| Format | jpeg | Output format |

**Configuration**: Currently hardcoded, future releases will support custom definitions.

### Supported Formats

**Input Formats**:
- JPEG/JPG
- PNG
- WebP
- GIF
- TIFF

**Output Formats**:
- JPEG (primary)
- PNG (future)
- WebP (future)

---

## Environment Examples

### Development (Local Storage)

```bash
# File: .env

# Core settings
MEDIA_STORAGE_DRIVER=local
MEDIA_STORAGE_PATH=./storage
MEDIA_BASE_URL=http://localhost:8080
MEDIA_SIGNING_SECRET=dev-signing-secret-32-chars-long

# Limits
MEDIA_MAX_FILE_SIZE=52428800  # 50MB for development
MEDIA_SMALL_FILE_THRESHOLD=5242880  # 5MB

# Workers
MEDIA_CONVERSION_WORKERS=2
```

### Production (AWS S3)

```bash
# File: .env.production

# Core settings
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_PATH=company-media-bucket
MEDIA_BASE_URL=https://cdn.example.com
MEDIA_SIGNING_SECRET=prod-signing-secret-32-chars-long

# AWS credentials
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...

# S3 settings
S3_USE_SSL=true
S3_PATH_STYLE=false
S3_ACL=private

# Limits
MEDIA_MAX_FILE_SIZE=104857600  # 100MB
MEDIA_SMALL_FILE_THRESHOLD=5242880  # 5MB

# Workers
MEDIA_CONVERSION_WORKERS=4

# Security
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

### Production (MinIO)

```bash
# File: .env.minio

# Core settings
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_PATH=my-bucket
MEDIA_BASE_URL=https://minio.example.com/my-bucket
MEDIA_SIGNING_SECRET=prod-signing-secret-32-chars-long

# MinIO credentials
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin

# MinIO settings
S3_ENDPOINT=https://minio.example.com
S3_USE_SSL=true
S3_PATH_STYLE=true

# Limits
MEDIA_MAX_FILE_SIZE=104857600
MEDIA_SMALL_FILE_THRESHOLD=5242880

# Workers
MEDIA_CONVERSION_WORKERS=4
```

### Docker Compose

```yaml
# docker-compose.yml
services:
  api:
    environment:
      # Media configuration
      MEDIA_STORAGE_DRIVER: local
      MEDIA_STORAGE_PATH: /app/storage
      MEDIA_BASE_URL: http://localhost:8080
      MEDIA_SIGNING_SECRET: ${MEDIA_SIGNING_SECRET}
      MEDIA_MAX_FILE_SIZE: "104857600"
      MEDIA_CONVERSION_WORKERS: "4"
      
      # Other required vars
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/go_api_base?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      JWT_SECRET: ${JWT_SECRET}
    volumes:
      - media-storage:/app/storage

volumes:
  media-storage:
```

### Kubernetes ConfigMap

```yaml
# k8s/media-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: media-config
  namespace: go-api
data:
  MEDIA_STORAGE_DRIVER: "s3"
  MEDIA_STORAGE_PATH: "company-media-bucket"
  MEDIA_BASE_URL: "https://cdn.example.com"
  MEDIA_MAX_FILE_SIZE: "104857600"
  MEDIA_SMALL_FILE_THRESHOLD: "5242880"
  MEDIA_CONVERSION_WORKERS: "4"
  S3_USE_SSL: "true"
  S3_PATH_STYLE: "false"
  AWS_REGION: "us-east-1"
---
apiVersion: v1
kind: Secret
metadata:
  name: media-secrets
  namespace: go-api
type: Opaque
stringData:
  MEDIA_SIGNING_SECRET: "your-32-char-signing-secret-here"
  AWS_ACCESS_KEY_ID: "your-aws-access-key"
  AWS_SECRET_ACCESS_KEY: "your-aws-secret-key"
```

---

## Configuration Validation

### Startup Validation

The application validates configuration on startup:

**Required Checks**:
- `MEDIA_STORAGE_DRIVER` is valid (`local`, `s3`, `minio`)
- `MEDIA_STORAGE_PATH` is set
- `MEDIA_BASE_URL` is a valid URL
- `MEDIA_SIGNING_SECRET` is at least 32 characters

**Optional Checks**:
- `MEDIA_MAX_FILE_SIZE` is positive number
- `MEDIA_SMALL_FILE_THRESHOLD` < `MEDIA_MAX_FILE_SIZE`
- S3 credentials present if driver is `s3`

### Validation Errors

If configuration is invalid, the application will:
1. Log detailed error messages
2. Exit with non-zero status code
3. Prevent startup until fixed

**Example Error Output**:
```
2026/04/22 18:30:00 ERROR Configuration validation failed:
  - MEDIA_SIGNING_SECRET: must be at least 32 characters
  - MEDIA_STORAGE_DRIVER: unsupported driver "ftp", must be one of: local, s3, minio
```

### Health Check Endpoint

Configuration health is reflected in `/readyz`:

```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "storage": "ok"
  }
}
```

If storage driver fails to initialize:
```json
{
  "status": "not_ready",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "storage": "error: failed to connect to S3: InvalidAccessKeyId"
  }
}
```

---

## Configuration Best Practices

### Security

1. **Never commit secrets** to version control
2. **Use different signing secrets** per environment
3. **Rotate secrets** periodically (quarterly recommended)
4. **Use environment-specific** credentials (dev/staging/prod)

### Performance

1. **Set appropriate thresholds**:
   - Low traffic: `MEDIA_CONVERSION_WORKERS=2`
   - High traffic: `MEDIA_CONVERSION_WORKERS=4-8`

2. **Size limits**:
   - Match `client_max_body_size` in nginx/ingress
   - Set realistic limits based on use case

3. **CDN integration**:
   - Use `MEDIA_BASE_URL` pointing to CDN
   - Configure `Cache-Control` headers

### Reliability

1. **S3/MinIO**: Enable versioning for disaster recovery
2. **Local storage**: Set up backup for `/storage` directory
3. **Monitoring**: Alert on storage capacity and conversion queue depth

---

## Related Documentation

- [Architecture](./ARCHITECTURE.md) - System design and components
- [Deployment](./DEPLOYMENT.md) - Production deployment guide
- [API Reference](./API.md) - Complete endpoint documentation
- [Security](./SECURITY.md) - Security features and hardening
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions

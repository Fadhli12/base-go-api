# Environment Variables Contract

**Feature**: Dynamic Configuration System  
**Version**: 1.0  
**Date**: 2026-04-23

---

## Overview

This document defines the complete set of environment variables for the dynamic configuration system. All variables follow the existing project convention (UPPER_SNAKE_CASE with section prefixes).

---

## Storage Configuration

### Core Storage Settings

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `STORAGE_DRIVER` | string | `local` | Storage backend: `local`, `s3`, or `minio` |
| `STORAGE_LOCAL_PATH` | string | `./storage/uploads` | Local filesystem storage path |
| `STORAGE_BASE_URL` | string | `http://localhost:8080/storage` | Public URL prefix for storage access |

### S3/MinIO-Specific Settings

Required when `STORAGE_DRIVER=s3` or `STORAGE_DRIVER=minio`:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `STORAGE_S3_ENDPOINT` | string | (empty) | S3 endpoint (leave empty for AWS S3, set for MinIO) |
| `STORAGE_S3_REGION` | string | `us-east-1` | AWS region |
| `STORAGE_S3_BUCKET` | string | (required) | Bucket name |
| `STORAGE_S3_ACCESS_KEY` | string | (required) | AWS access key or MinIO access key |
| `STORAGE_S3_SECRET_KEY` | string | (required) | AWS secret key or MinIO secret key |
| `STORAGE_S3_USE_SSL` | bool | `true` | Use HTTPS for connections |
| `STORAGE_S3_PATH_STYLE` | bool | `false` | Use path-style addressing (required for MinIO) |

### Example: Local Storage (Default)

```bash
STORAGE_DRIVER=local
STORAGE_LOCAL_PATH=./storage/uploads
STORAGE_BASE_URL=http://localhost:8080/storage
```

### Example: AWS S3

```bash
STORAGE_DRIVER=s3
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=my-app-bucket
STORAGE_S3_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
STORAGE_S3_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
STORAGE_S3_USE_SSL=true
STORAGE_S3_PATH_STYLE=false
```

### Example: MinIO (Self-Hosted S3)

```bash
STORAGE_DRIVER=minio
STORAGE_S3_ENDPOINT=https://minio.example.com:9000
STORAGE_S3_REGION=local
STORAGE_S3_BUCKET=my-bucket
STORAGE_S3_ACCESS_KEY=minioadmin
STORAGE_S3_SECRET_KEY=minioadmin
STORAGE_S3_USE_SSL=true
STORAGE_S3_PATH_STYLE=true
```

---

## Image Compression Configuration

### Compression Settings

| Variable | Type | Default | Range | Description |
|----------|------|---------|-------|-------------|
| `IMAGE_COMPRESSION_ENABLED` | bool | `true` | - | Enable/disable image compression |
| `IMAGE_COMPRESSION_THUMBNAIL_QUALITY` | int | `85` | 1-100 | JPEG quality for thumbnails |
| `IMAGE_COMPRESSION_THUMBNAIL_WIDTH` | int | `300` | 1-4096 | Thumbnail target width (px) |
| `IMAGE_COMPRESSION_THUMBNAIL_HEIGHT` | int | `300` | 1-4096 | Thumbnail target height (px) |
| `IMAGE_COMPRESSION_PREVIEW_QUALITY` | int | `90` | 1-100 | JPEG quality for previews |
| `IMAGE_COMPRESSION_PREVIEW_WIDTH` | int | `800` | 1-4096 | Preview target width (px) |
| `IMAGE_COMPRESSION_PREVIEW_HEIGHT` | int | `600` | 1-4096 | Preview target height (px) |

### Example: High Quality (Production)

```bash
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=90
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=400
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=400
IMAGE_COMPRESSION_PREVIEW_QUALITY=95
IMAGE_COMPRESSION_PREVIEW_WIDTH=1200
IMAGE_COMPRESSION_PREVIEW_HEIGHT=900
```

### Example: Low Bandwidth (Mobile-Optimized)

```bash
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=70
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=200
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=200
IMAGE_COMPRESSION_PREVIEW_QUALITY=75
IMAGE_COMPRESSION_PREVIEW_WIDTH=600
IMAGE_COMPRESSION_PREVIEW_HEIGHT=450
```

### Example: Disabled (Original Files)

```bash
IMAGE_COMPRESSION_ENABLED=false
```

---

## Cache Configuration

### Core Cache Settings

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CACHE_DRIVER` | string | `redis` | Cache backend: `redis`, `memory`, or `none` |
| `CACHE_DEFAULT_TTL` | int | `300` | Default cache TTL in seconds (5 min) |
| `CACHE_PERMISSION_TTL` | int | `300` | Permission cache TTL in seconds |
| `CACHE_RATE_LIMIT_TTL` | int | `60` | Rate limit window in seconds |
| `CACHE_MAX_KEY_SIZE` | int | `1024` | Maximum cache key size (bytes) |
| `CACHE_MAX_VALUE_SIZE` | int | `1048576` | Maximum cache value size (bytes, 1MB) |

### Example: Production with Redis

```bash
CACHE_DRIVER=redis
CACHE_DEFAULT_TTL=300
CACHE_PERMISSION_TTL=300
CACHE_RATE_LIMIT_TTL=60
```

### Example: Development with In-Memory Cache

```bash
CACHE_DRIVER=memory
CACHE_DEFAULT_TTL=60
CACHE_PERMISSION_TTL=60
CACHE_RATE_LIMIT_TTL=60
```

### Example: Cache Disabled

```bash
CACHE_DRIVER=none
```

---

## Swagger Configuration

### Swagger Settings

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SWAGGER_ENABLED` | bool | `true` | Enable/disable Swagger UI |
| `SWAGGER_PATH` | string | `/swagger` | Swagger UI path prefix |

### Example: Enabled (Development)

```bash
SWAGGER_ENABLED=true
SWAGGER_PATH=/swagger
```

### Example: Disabled (Production)

```bash
SWAGGER_ENABLED=false
```

---

## Validation Rules

### Storage Driver Validation

| Driver | Required Variables |
|--------|-------------------|
| `local` | None (uses defaults) |
| `s3` | `STORAGE_S3_BUCKET`, `STORAGE_S3_ACCESS_KEY`, `STORAGE_S3_SECRET_KEY` |
| `minio` | `STORAGE_S3_ENDPOINT`, `STORAGE_S3_BUCKET`, `STORAGE_S3_ACCESS_KEY`, `STORAGE_S3_SECRET_KEY`, `STORAGE_S3_PATH_STYLE=true` |

### Image Quality Validation

- Quality must be between 1-100 (JPEG quality scale)
- Dimensions must be between 1-4096 pixels
- Zero or negative values will fail validation

### Cache Driver Validation

| Driver | Requirements |
|--------|-------------|
| `redis` | Redis client must be available |
| `memory` | None (always available) |
| `none` | None (no caching) |

---

## Backward Compatibility

All new environment variables have defaults that match existing hardcoded behavior:

| Variable | Default | Previous Behavior |
|----------|---------|-------------------|
| `STORAGE_DRIVER` | `local` | Hardcoded local storage |
| `CACHE_DRIVER` | `redis` | Hardcoded Redis |
| `IMAGE_COMPRESSION_THUMBNAIL_QUALITY` | `85` | Hardcoded const 85 |
| `IMAGE_COMPRESSION_PREVIEW_QUALITY` | `90` | Hardcoded const 90 |
| `IMAGE_COMPRESSION_THUMBNAIL_WIDTH` | `300` | Hardcoded const 300 |
| `IMAGE_COMPRESSION_THUMBNAIL_HEIGHT` | `300` | Hardcoded const 300 |
| `IMAGE_COMPRESSION_PREVIEW_WIDTH` | `800` | Hardcoded const 800 |
| `IMAGE_COMPRESSION_PREVIEW_HEIGHT` | `600` | Hardcoded const 600 |
| `SWAGGER_ENABLED` | `true` | Always enabled |

Existing `.env` files will continue to work without modification.

---

## .env.example Template

Add the following to `.env.example`:

```bash
# =============================================================================
# Storage Configuration
# =============================================================================
STORAGE_DRIVER=local
STORAGE_LOCAL_PATH=./storage/uploads
STORAGE_BASE_URL=http://localhost:8080/storage
# STORAGE_S3_ENDPOINT=
# STORAGE_S3_REGION=us-east-1
# STORAGE_S3_BUCKET=
# STORAGE_S3_ACCESS_KEY=
# STORAGE_S3_SECRET_KEY=
# STORAGE_S3_USE_SSL=true
# STORAGE_S3_PATH_STYLE=false

# =============================================================================
# Image Compression Configuration
# =============================================================================
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=85
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=300
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=300
IMAGE_COMPRESSION_PREVIEW_QUALITY=90
IMAGE_COMPRESSION_PREVIEW_WIDTH=800
IMAGE_COMPRESSION_PREVIEW_HEIGHT=600

# =============================================================================
# Cache Configuration
# =============================================================================
CACHE_DRIVER=redis
CACHE_DEFAULT_TTL=300
CACHE_PERMISSION_TTL=300
CACHE_RATE_LIMIT_TTL=60
CACHE_MAX_KEY_SIZE=1024
CACHE_MAX_VALUE_SIZE=1048576

# =============================================================================
# Swagger Configuration
# =============================================================================
SWAGGER_ENABLED=true
SWAGGER_PATH=/swagger
```

---

## Secrets Management

### Sensitive Environment Variables

The following variables contain secrets and require special handling:

| Variable | Secret Type | Risk Level | Storage Recommendation |
|----------|-------------|------------|------------------------|
| `STORAGE_S3_ACCESS_KEY` | AWS Access Key | **HIGH** | Kubernetes Secrets / AWS Secrets Manager / Vault |
| `STORAGE_S3_SECRET_KEY` | AWS Secret Key | **HIGH** | Kubernetes Secrets / AWS Secrets Manager / Vault |
| `JWT_SECRET` | Signing Secret | **HIGH** | Kubernetes Secrets / Vault (rotation recommended) |
| `DATABASE_URL` (if contains password) | Connection String | **MEDIUM** | Kubernetes Secrets / Vault |
| `REDIS_URL` (if contains password) | Connection String | **MEDIUM** | Kubernetes Secrets / Vault |

### Security Requirements

1. **Never commit secrets to git** - Add to `.gitignore`
2. **Never log secrets** - All secrets are excluded from structured logging
3. **Rotate periodically** - Especially `JWT_SECRET` (recommend: every 90 days)
4. **Use different values per environment** - Dev, staging, production must have unique secrets
5. **Prefer secret managers** - Use Kubernetes Secrets, AWS Secrets Manager, HashiCorp Vault

### Environment-Specific Guidance

**Local Development**:
```bash
# .env (gitignored)
STORAGE_S3_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
STORAGE_S3_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
JWT_SECRET=your-local-dev-secret-min-32-characters-long!
```

**Production**:
```yaml
# Kubernetes Secret
apiVersion: v1
kind: Secret
metadata:
  name: go-api-secrets
type: Opaque
data:
  STORAGE_S3_ACCESS_KEY: <base64-encoded>
  STORAGE_S3_SECRET_KEY: <base64-encoded>
  JWT_SECRET: <base64-encoded>
```

### Rotating Secrets

When rotating secrets:
1. Generate new secret value
2. Update secret manager (Kubernetes Secret, Vault, etc.)
3. Restart pods to load new secrets
4. Verify application starts successfully
5. Monitor for authentication errors
6. Revoke old secret after grace period

---

## Change Log

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-04-23 | Initial contract for storage, image, cache, swagger configs |
| 1.1 | 2026-04-23 | Added secrets management guidance |
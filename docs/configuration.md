# Configuration Guide

This document provides detailed configuration instructions for the Go API Base dynamic configuration system.

---

## Overview

The API supports multiple configurable subsystems through environment variables:

- **Storage**: Local filesystem, AWS S3, or MinIO
- **Image Compression**: Configurable quality and dimensions
- **Cache**: Redis, in-memory, or disabled
- **Swagger**: API documentation toggle

All configurations use environment variables with sensible defaults that match previous hardcoded behavior.

---

## Storage Configuration

### Local Storage (Default)

The default configuration uses local filesystem storage:

```bash
STORAGE_DRIVER=local
STORAGE_LOCAL_PATH=./storage/uploads
STORAGE_BASE_URL=http://localhost:8080/storage
```

**Use Case**: Development, single-server deployments

### AWS S3

For production cloud storage:

```bash
STORAGE_DRIVER=s3
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=my-app-bucket
STORAGE_S3_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
STORAGE_S3_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
STORAGE_S3_USE_SSL=true
STORAGE_S3_PATH_STYLE=false
```

**Use Case**: Production, multi-region deployments, CDN integration

### MinIO (Self-Hosted S3)

For self-hosted object storage:

```bash
STORAGE_DRIVER=minio
STORAGE_S3_ENDPOINT=https://minio.example.com:9000
STORAGE_S3_REGION=local
STORAGE_S3_BUCKET=my-bucket
STORAGE_S3_ACCESS_KEY=minioadmin
STORAGE_S3_SECRET_KEY=minioadmin
STORAGE_S3_USE_SSL=true
STORAGE_S3_PATH_STYLE=true  # Required for MinIO
```

**Use Case**: Self-hosted, air-gapped environments, data sovereignty requirements

### Storage Variables Reference

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `STORAGE_DRIVER` | string | `local` | Backend: `local`, `s3`, `minio` |
| `STORAGE_LOCAL_PATH` | string | `./storage/uploads` | Local storage path |
| `STORAGE_BASE_URL` | string | `http://localhost:8080/storage` | Public URL prefix |
| `STORAGE_S3_ENDPOINT` | string | (empty) | S3 endpoint (empty for AWS) |
| `STORAGE_S3_REGION` | string | `us-east-1` | AWS region |
| `STORAGE_S3_BUCKET` | string | (required) | Bucket name |
| `STORAGE_S3_ACCESS_KEY` | string | (required) | Access key |
| `STORAGE_S3_SECRET_KEY` | string | (required) | Secret key |
| `STORAGE_S3_USE_SSL` | bool | `true` | Use HTTPS |
| `STORAGE_S3_PATH_STYLE` | bool | `false` | Path-style addressing |

---

## Image Compression Configuration

### Quality Presets

**High Quality (Production)**:
```bash
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=90
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=400
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=400
IMAGE_COMPRESSION_PREVIEW_QUALITY=95
IMAGE_COMPRESSION_PREVIEW_WIDTH=1200
IMAGE_COMPRESSION_PREVIEW_HEIGHT=900
```

**Balanced (Default)**:
```bash
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=85
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=300
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=300
IMAGE_COMPRESSION_PREVIEW_QUALITY=90
IMAGE_COMPRESSION_PREVIEW_WIDTH=800
IMAGE_COMPRESSION_PREVIEW_HEIGHT=600
```

**Low Bandwidth (Mobile-Optimized)**:
```bash
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=70
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=200
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=200
IMAGE_COMPRESSION_PREVIEW_QUALITY=75
IMAGE_COMPRESSION_PREVIEW_WIDTH=600
IMAGE_COMPRESSION_PREVIEW_HEIGHT=450
```

**Disabled (Original Files)**:
```bash
IMAGE_COMPRESSION_ENABLED=false
```

### Image Variables Reference

| Variable | Type | Default | Range | Description |
|----------|------|---------|-------|-------------|
| `IMAGE_COMPRESSION_ENABLED` | bool | `true` | - | Enable/disable compression |
| `IMAGE_COMPRESSION_THUMBNAIL_QUALITY` | int | `85` | 1-100 | JPEG quality for thumbnails |
| `IMAGE_COMPRESSION_THUMBNAIL_WIDTH` | int | `300` | 1-4096 | Thumbnail width (px) |
| `IMAGE_COMPRESSION_THUMBNAIL_HEIGHT` | int | `300` | 1-4096 | Thumbnail height (px) |
| `IMAGE_COMPRESSION_PREVIEW_QUALITY` | int | `90` | 1-100 | JPEG quality for previews |
| `IMAGE_COMPRESSION_PREVIEW_WIDTH` | int | `800` | 1-4096 | Preview width (px) |
| `IMAGE_COMPRESSION_PREVIEW_HEIGHT` | int | `600` | 1-4096 | Preview height (px) |

---

## Cache Configuration

### Redis (Default, Production)

```bash
CACHE_DRIVER=redis
CACHE_DEFAULT_TTL=300
CACHE_PERMISSION_TTL=300
CACHE_RATE_LIMIT_TTL=60
CACHE_MAX_KEY_SIZE=1024
CACHE_MAX_VALUE_SIZE=1048576
```

**Use Case**: Production, multi-instance deployments

### In-Memory Cache

```bash
CACHE_DRIVER=memory
CACHE_DEFAULT_TTL=60
CACHE_PERMISSION_TTL=60
CACHE_RATE_LIMIT_TTL=60
```

**Use Case**: Development, single-instance deployments

### Cache Disabled

```bash
CACHE_DRIVER=none
```

**Use Case**: Testing, troubleshooting

### Cache Variables Reference

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CACHE_DRIVER` | string | `redis` | Backend: `redis`, `memory`, `none` |
| `CACHE_DEFAULT_TTL` | int | `300` | Default TTL in seconds (5 min) |
| `CACHE_PERMISSION_TTL` | int | `300` | Permission cache TTL in seconds |
| `CACHE_RATE_LIMIT_TTL` | int | `60` | Rate limit window in seconds |
| `CACHE_MAX_KEY_SIZE` | int | `1024` | Maximum cache key size (bytes) |
| `CACHE_MAX_VALUE_SIZE` | int | `1048576` | Maximum cache value size (bytes, 1MB) |

---

## Swagger Configuration

### Enabled (Default, Development)

```bash
SWAGGER_ENABLED=true
SWAGGER_PATH=/swagger
```

The Swagger UI will be available at `http://localhost:8080/swagger/index.html`.

### Disabled (Production)

```bash
SWAGGER_ENABLED=false
```

**Recommended**: Disable Swagger in production to avoid exposing API internals.

### Swagger Variables Reference

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SWAGGER_ENABLED` | bool | `true` | Enable/disable Swagger UI |
| `SWAGGER_PATH` | string | `/swagger` | Swagger UI path prefix |

---

## Secrets Management

### High-Security Variables

The following variables require secure handling:

| Variable | Risk Level | Storage Recommendation |
|----------|------------|------------------------|
| `STORAGE_S3_ACCESS_KEY` | **HIGH** | Kubernetes Secrets / AWS Secrets Manager / Vault |
| `STORAGE_S3_SECRET_KEY` | **HIGH** | Kubernetes Secrets / AWS Secrets Manager / Vault |
| `JWT_SECRET` | **HIGH** | Kubernetes Secrets / Vault (rotation recommended) |
| `DATABASE_URL` | **MEDIUM** | Kubernetes Secrets / Vault |
| `REDIS_URL` | **MEDIUM** | Kubernetes Secrets / Vault |

### Security Requirements

1. **Never commit secrets to git** - Add to `.gitignore`
2. **Never log secrets** - All secrets are excluded from structured logging
3. **Rotate periodically** - Especially `JWT_SECRET` (every 90 days recommended)
4. **Use different values per environment** - Development, staging, production must have unique secrets
5. **Prefer secret managers** - Use Kubernetes Secrets, AWS Secrets Manager, or HashiCorp Vault

### Local Development

```bash
# .env (gitignored)
cp .env.example .env
# Edit with local development values
```

### Production (Kubernetes)

```yaml
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

---

## Backward Compatibility

All new environment variables have defaults that match existing hardcoded behavior:

| Variable | Default | Previous Behavior |
|----------|---------|-------------------|
| `STORAGE_DRIVER` | `local` | Hardcoded local storage |
| `CACHE_DRIVER` | `redis` | Hardcoded Redis |
| `IMAGE_COMPRESSION_*` | Fixed values | Hardcoded constants |
| `SWAGGER_ENABLED` | `true` | Always enabled |

Existing `.env` files will continue to work without modification.

---

## Validation Rules

### Storage Driver Validation

| Driver | Required Variables |
|--------|-------------------|
| `local` | None (uses defaults) |
| `s3` | `STORAGE_S3_BUCKET`, `STORAGE_S3_ACCESS_KEY`, `STORAGE_S3_SECRET_KEY` |
| `minio` | `STORAGE_S3_ENDPOINT`, `STORAGE_S3_BUCKET`, `STORAGE_S3_ACCESS_KEY`, `STORAGE_S3_SECRET_KEY`, `STORAGE_S3_PATH_STYLE=true` |

### Image Quality Validation

- Quality: 1-100 (JPEG quality scale)
- Dimensions: 1-4096 pixels
- Zero or negative values will fail validation

### Cache Driver Validation

| Driver | Requirements |
|--------|-------------|
| `redis` | Redis client must be available |
| `memory` | None (always available) |
| `none` | None (no caching) |

---

## Environment Files

### Example .env for Development

```bash
# Storage (local)
STORAGE_DRIVER=local
STORAGE_LOCAL_PATH=./storage/uploads
STORAGE_BASE_URL=http://localhost:8080/storage

# Image Compression (balanced)
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=85
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=300
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=300

# Cache (in-memory)
CACHE_DRIVER=memory
CACHE_DEFAULT_TTL=60

# Swagger (enabled)
SWAGGER_ENABLED=true
SWAGGER_PATH=/swagger
```

### Example .env for Production

```bash
# Storage (S3)
STORAGE_DRIVER=s3
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=my-app-bucket
# STORAGE_S3_ACCESS_KEY and STORAGE_S3_SECRET_KEY from Kubernetes Secrets

# Image Compression (high quality)
IMAGE_COMPRESSION_ENABLED=true
IMAGE_COMPRESSION_THUMBNAIL_QUALITY=90
IMAGE_COMPRESSION_THUMBNAIL_WIDTH=400
IMAGE_COMPRESSION_THUMBNAIL_HEIGHT=400

# Cache (Redis)
CACHE_DRIVER=redis
CACHE_DEFAULT_TTL=300

# Swagger (disabled)
SWAGGER_ENABLED=false
```

---

## Change Log

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-04-23 | Initial configuration documentation |
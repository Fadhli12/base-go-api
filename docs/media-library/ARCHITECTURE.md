# Media Library Architecture

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Production-Ready

---

## Overview

The Media Library is a production-ready polymorphic file management system that enables any domain model (News, User, Invoice, etc.) to associate and manage related assets (images, documents, files). Inspired by Laravel's Spatie Media Library, it provides storage abstraction, automatic image conversions, permission-aware access, and comprehensive audit logging.

---

## System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HTTP Layer (Echo)                              │
│  ┌───────────────────────────────────────────────────────────────────────┐   │
│  │  POST /models/:type/:id/media    GET /media/:id/download             │   │
│  │  GET  /models/:type/:id/media    DELETE /media/:id                    │   │
│  │  GET  /media/:id                 GET /media/:id/url                 │   │
│  └───────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────┬───────────────────────────────────────┘
                                        │
                              ┌─────────▼─────────┐
                              │   MediaHandler    │
                              │  (internal/http)  │
                              └─────────┬─────────┘
                                        │
┌───────────────────────────────────────┼───────────────────────────────────────┐
│                                       │                                       │
│                          ┌────────────▼────────────┐                         │
│                          │      MediaService       │                         │
│                          │    (internal/service)   │                         │
│                          └────────────┬──────────────┘                         │
│                                       │                                       │
│           ┌───────────────────────────┼───────────────────────────┐           │
│           │                           │                           │           │
│    ┌──────▼──────┐           ┌───────▼────────┐         ┌──────▼──────┐   │
│    │   Storage   │           │  Repository    │         │ Conversion  │   │
│    │   Driver    │           │   (GORM)       │         │   Worker    │   │
│    └──────┬──────┘           └───────┬────────┘         └──────┬──────┘   │
│           │                          │                      │           │
└───────────┼──────────────────────────┼──────────────────────┼───────────┘
            │                          │                      │
    ┌───────▼───────┐          ┌───────▼────────┐    ┌────────▼────────┐
    │  Local/S3/    │          │   PostgreSQL   │    │  Redis Queue    │
    │    MinIO      │          │   (media)    │    │  (async jobs)   │
    └───────────────┘          └───────────────┘    └─────────────────┘
```

---

## Component Descriptions

### 1. HTTP Layer (`internal/http/handler/media.go`)

**Responsibilities**:
- Request validation and parsing
- JWT authentication extraction
- Permission checking (RBAC)
- File streaming (downloads)
- Audit logging triggers

**Key Handlers**:
| Handler | Endpoint | Purpose |
|---------|----------|---------|
| `Upload` | `POST /models/:type/:id/media` | Handle multipart uploads |
| `List` | `GET /models/:type/:id/media` | List media with pagination |
| `GetByID` | `GET /media/:id` | Get media details |
| `Download` | `GET /media/:id/download` | Stream file (supports signed URLs) |
| `Delete` | `DELETE /media/:id` | Soft delete media |
| `GetSignedURL` | `GET /media/:id/url` | Generate signed download URLs |
| `AdminList` | `GET /admin/media` | Admin: list all media |
| `AdminCleanup` | `POST /admin/media/cleanup` | Admin: trigger cleanup |
| `AdminStats` | `GET /admin/media/stats` | Admin: storage statistics |

### 2. Service Layer (`internal/service/media.go`)

**Responsibilities**:
- Business logic orchestration
- File validation (MIME type, magic bytes, size)
- Filename sanitization
- Atomic operations (DB + storage consistency)
- Signed URL generation

**Key Interfaces**:
```go
type MediaService interface {
    Upload(ctx context.Context, userID uuid.UUID, req UploadRequest) (*domain.Media, error)
    List(ctx context.Context, modelType string, modelID uuid.UUID, filter ListFilter) ([]*domain.Media, int64, error)
    Get(ctx context.Context, mediaID uuid.UUID) (*domain.Media, error)
    Delete(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, isAdmin bool) error
    UpdateMetadata(ctx context.Context, userID uuid.UUID, mediaID uuid.UUID, props map[string]interface{}, isAdmin bool) (*domain.Media, error)
    GetSignedURL(ctx context.Context, mediaID uuid.UUID, conversionName string, expiry time.Duration) (string, time.Time, error)
}
```

### 3. Repository Layer (`internal/repository/media.go`)

**Responsibilities**:
- GORM data access operations
- Soft delete handling
- Association preloading (conversions)
- Orphan tracking

**Key Methods**:
- `Create/FindByID/Update/SoftDelete` - Standard CRUD
- `FindByModelTypeAndID` - Polymorphic queries
- `MarkOrphaned/FindOrphaned` - Cleanup support
- `CreateConversion/FindConversionsByMediaID` - Conversion management
- `CreateDownload` - Download audit logging

### 4. Storage Layer (`internal/storage/`)

**Abstraction**: Pluggable storage backends via `storage.Driver` interface.

**Supported Drivers**:
| Driver | Type | Use Case |
|--------|------|----------|
| `local` | Local filesystem | Development, small deployments |
| `s3` | AWS S3 | Production, scalable storage |
| `minio` | MinIO compatible | Self-hosted S3-compatible |

**Interface**:
```go
type Driver interface {
    Store(ctx context.Context, path string, content io.Reader) error
    Get(ctx context.Context, path string) (io.ReadCloser, error)
    Delete(ctx context.Context, path string) error
    URL(ctx context.Context, path string, expires time.Duration) (string, error)
    Exists(ctx context.Context, path string) (bool, error)
    Size(ctx context.Context, path string) (int64, error)
}
```

**Path Structure**:
```
{model_type}/{model_id}/{uuid}.{ext}
Example: news/550e8400-e29b-41d4-a716-446655440000/660e8400-e29b-41d4-a716-446655440001.jpg
```

### 5. Conversion Pipeline (`internal/conversion/`)

**Architecture**: Hybrid sync/async approach

**Sync Processing** (files < 5MB):
- Thumbnail generation during upload
- Blocks upload response
- Fast feedback for small files

**Async Processing** (files >= 5MB):
- Queued in Redis
- Background worker processes
- Non-blocking upload experience

**Default Conversions**:
| Name | Dimensions | Quality | Format |
|------|------------|---------|--------|
| `thumbnail` | 300x300px | 85 | JPEG |
| `preview` | 800x600px | 90 | JPEG |

**Conversion Flow**:
```
Upload Request
     │
     ├─► [File Size < 5MB] ──► Sync Processing ──► Immediate Response
     │
     └─► [File Size >= 5MB] ──► Queue in Redis ──► Worker Processes ──► Response
```

---

## Data Flow

### Upload Flow

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Client  │────▶│ HTTP Handler │────▶│ MediaService │────▶│   Validate   │
└──────────┘     └──────────────┘     └──────────────┘     └──────┬───────┘
                                                                  │
    ┌──────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Validation Steps:                                                  │
│  1. File size check (max 100MB)                                      │
│  2. Extension check (blocked: .exe, .php, .sh, etc.)                │
│  3. Magic bytes detection (MIME type from content)                  │
│  4. MIME type allowlist (images, documents, archives)               │
│  5. Path traversal prevention (sanitize filename)                   │
└─────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Store File   │────▶│ Create DB    │────▶│ Trigger Conv │
│ (Storage)    │     │ Record       │     │ (if image)   │
└──────────────┘     └──────────────┘     └──────────────┘
    │                                             │
    │ On DB Error                                 ├─► [Sync] ──► Done
    │                                             │
    └─► Rollback (delete file)                   └─► [Async] ──► Redis Queue
```

### Download Flow

```
┌──────────┐     ┌─────────────────────────────────────────────────────────┐
│  Client  │────▶│ GET /media/:id/download                                 │
└──────────┘     │                                                         │
                 │  Auth Check:                                            │
                 │  1. JWT token? ──► Check permission                     │
                 │  2. Signed URL? ──► Validate HMAC + expiry               │
                 │  3. Neither? ──► 401 Unauthorized                        │
                 └─────────────────────────────────────────────────────────┘
                                          │
                                          ▼
                 ┌─────────────────────────────────────────────────────────┐
                 │  Permission Check:                                        │
                 │  - Owner can always download                             │
                 │  - Check `media:download` permission                      │
                 │  - Validate signed URL signature                        │
                 └─────────────────────────────────────────────────────────┘
                                          │
                                          ▼
                 ┌─────────────────────────────────────────────────────────┐
                 │  Fetch from Storage Driver                                │
                 │  - Get file reader                                        │
                 │  - Set Content-Type, Content-Disposition headers          │
                 │  - Stream to response                                     │
                 │  - Log download audit                                     │
                 └─────────────────────────────────────────────────────────┘
```

### Delete Flow

```
┌──────────┐     ┌─────────────────────────────────────────────────────────┐
│  Client  │────▶│ DELETE /media/:id                                       │
└──────────┘     │                                                         │
                 │  Soft Delete Process:                                   │
                 │  1. Verify ownership or admin permission                │
                 │  2. Mark orphaned (for cleanup tracking)                │
                 │  3. Set deleted_at timestamp (GORM soft delete)         │
                 │  4. Log delete audit                                      │
                 └─────────────────────────────────────────────────────────┘
                                          │
                                          ▼
                 ┌─────────────────────────────────────────────────────────┐
                 │  Actual file cleanup happens later:                     │
                 │  - Orphaned files tracked in DB (orphaned_at)           │
                 │  - Background job purges after 30 days                  │
                 │  - Can be restored within 30-day window                 │
                 └─────────────────────────────────────────────────────────┘
```

---

## Polymorphic Relationship Design

### Schema

The `media` table uses a polymorphic association pattern:

```sql
CREATE TABLE media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type VARCHAR(255) NOT NULL,  -- 'news', 'user', 'invoice'
    model_id UUID NOT NULL,             -- UUID of the parent model
    collection_name VARCHAR(255),       -- 'images', 'documents', 'avatars'
    -- ... other fields
);

CREATE INDEX idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;
```

### Benefits

1. **Flexibility**: No schema changes needed to add new model types
2. **Simplicity**: Single table for all media
3. **Query Performance**: Composite index on `(model_type, model_id)`
4. **Soft Delete Cascade**: When parent model is soft-deleted, media can be marked orphaned

### Example Queries

```go
// Find all media for a news article
var media []Media
db.Where("model_type = ? AND model_id = ?", "news", newsID).Find(&media)

// Find only images in the "featured" collection
db.Where("model_type = ? AND model_id = ? AND collection_name = ? AND mime_type LIKE ?",
    "news", newsID, "featured", "image/%").Find(&media)
```

---

## Storage Driver Abstraction

### Configuration

Storage driver is selected via environment variable:

```bash
MEDIA_STORAGE_DRIVER=local    # or: s3, minio
MEDIA_STORAGE_PATH=./storage  # Local path or S3 bucket
```

### Driver Selection

```go
func NewDriver(config Config) (Driver, error) {
    switch config.Type {
    case "local":
        return NewLocalDriver(config.LocalPath, config.BaseURL)
    case "s3", "minio":
        return NewS3Driver(config)  // Requires AWS SDK
    default:
        return nil, fmt.Errorf("unsupported storage driver: %s", config.Type)
    }
}
```

### Security Features

- **Path Traversal Prevention**: All paths sanitized via `sanitizePath()`
- **UUID Filenames**: Original filenames stored in DB, files stored with UUID names
- **No Direct Access**: Files served through handler (permission checks enforced)
- **Signed URLs**: Time-limited access URLs with HMAC-SHA256 signatures

---

## Conversion Pipeline (Sync/Async Hybrid)

### Sync Processing

Small files (< 5MB) are processed synchronously:

```go
func (s *mediaService) Upload(...) {
    // ... store file, create DB record
    
    if fileSize < SmallFileThreshold {
        // Generate thumbnail immediately
        thumbnail, _ := s.generateThumbnail(media)
        media.Conversions = append(media.Conversions, thumbnail)
    }
    
    return media, nil
}
```

### Async Processing

Large files (>= 5MB) queued for background processing:

```go
// After upload completes
if s.conversionWorker != nil && ShouldProcessSync(fileSize) == false {
    go func() {
        s.conversionWorker.ProcessConversions(context.Background(), media)
    }()
}
```

**Worker Responsibilities**:
1. Listen to Redis queue for conversion jobs
2. Acquire distributed lock (prevent duplicate processing)
3. Generate conversions using image library
4. Store converted files
5. Update DB with conversion metadata

### Conversion Definitions

```go
type ConversionDef struct {
    Name    string   // "thumbnail", "preview"
    Width   int      // Target width
    Height  int      // Target height
    Quality int      // JPEG quality (0-100)
    Format  string   // Output format
}
```

---

## Database Schema

### Tables

1. **`media`** - Main media registry
2. **`media_conversions`** - Generated file variants
3. **`media_downloads`** - Download audit log

### Key Indexes

```sql
-- Polymorphic queries
CREATE INDEX idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;

-- Collection filtering
CREATE INDEX idx_media_collection ON media(collection_name) WHERE deleted_at IS NULL;

-- MIME type filtering
CREATE INDEX idx_media_mime_type ON media(mime_type) WHERE deleted_at IS NULL;

-- Orphaned cleanup
CREATE INDEX idx_media_orphaned ON media(orphaned_at) WHERE orphaned_at IS NOT NULL;

-- Unique constraint per disk
CREATE UNIQUE INDEX idx_media_filename_disk ON media(disk, filename) WHERE deleted_at IS NULL;
```

---

## Performance Considerations

### Optimizations

1. **Eager Loading**: `Preload("Conversions")` prevents N+1 queries
2. **Soft Delete Scoping**: All queries exclude `deleted_at IS NOT NULL`
3. **Connection Pooling**: GORM connection pool for DB efficiency
4. **File Streaming**: Downloads use `io.Copy` for memory efficiency
5. **Async Conversions**: Large file processing offloaded to workers

### Scalability

- **Horizontal Scaling**: Stateless API servers, shared Redis/S3
- **CDN Integration**: Signed URLs compatible with CloudFront/Cloudflare
- **Database Sharding**: `model_type` column supports partition keys
- **Cleanup Jobs**: Orphaned file cleanup prevents storage bloat

---

## Integration Points

### Casbin RBAC

Permission enforcement uses Casbin enforcer:

```go
// Check permission
allowed, _ := enforcer.Enforce(userID, "default", "media", "upload")

// Or model-specific permission
allowed, _ := enforcer.Enforce(userID, "default", "news", "upload")
```

### Audit Logging

All operations logged to `audit_logs` table:

```go
// Async audit logging
h.auditSvc.LogAction(ctx, userID, "upload", "media", mediaID, before, after, ip, userAgent)
```

### Redis

Used for:
- Conversion job queue
- Distributed locks (prevent duplicate conversions)
- Rate limiting (via existing middleware)

---

## Related Documentation

- [API Reference](./API.md) - Complete endpoint documentation
- [Configuration](./CONFIG.md) - Environment variables and setup
- [Deployment](./DEPLOYMENT.md) - Production deployment guide
- [Security](./SECURITY.md) - Security features and hardening
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions

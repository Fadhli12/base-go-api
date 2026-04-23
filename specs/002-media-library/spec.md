# Feature Specification: Media Library System (Go)

**Feature**: `002-media-library`  
**Date**: 2026-04-22  
**Status**: Planning  
**Inspired by**: Spatie Laravel Media Library

---

## Executive Summary

A production-ready **polymorphic media library** for Go that allows **any domain model** (News, User, Invoice, etc.) to associate and manage related assets (images, documents, files). Similar to Laravel's Spatie Media Library, this system provides:

- **Polymorphic relationships**: Single `media` table linked to multiple entity types
- **File storage abstraction**: S3, local filesystem, or custom drivers
- **Conversions & transformations**: Auto-generate thumbnails, resize images, extract metadata
- **Permission-aware access**: RBAC controls who can upload/download/delete media
- **Audit trail**: Track all media operations (upload, delete, permission changes)
- **Query optimization**: Eager-load related media with minimal overhead
- **Atomic operations**: Transactional file uploads with database rollback on failure

---

## Requirements

### Functional Requirements

#### FR1: Polymorphic Media Association
- Any domain model can have 1:N media associations (e.g., News → many Images, Documents)
- Media linked via model type + model ID + collection name
- Example collections: `images`, `documents`, `avatars`, `featured-image`
- Support soft-deleted models (don't orphan media; cascade soft-delete)

#### FR2: File Management
- Upload files with metadata (size, MIME type, dimensions for images)
- Store files in pluggable storage drivers (S3, local, custom)
- Generate unique identifiers (UUID) for media items
- Support file variants/conversions (e.g., thumbnail, preview, original)

#### FR3: Media Conversions
- Define conversion templates (e.g., thumbnail from image)
- Auto-generate thumbnails on upload
- Lazy-load conversions (on-demand generation)
- Cache conversion metadata in DB

#### FR4: Permission Control
- RBAC: `media:upload`, `media:download`, `media:delete`, `media:view`
- Owner-based access (uploader can manage own media)
- Model-based scoping (user can upload to own profile, admin to any)
- Audit all permission-denied attempts

#### FR5: Query Interface
- Eager-load media with models (no N+1)
- Filter by collection name, MIME type, date range
- Search by filename/description
- Paginated listing

#### FR6: Audit Logging
- Log all uploads (file, user, model, timestamp, IP)
- Log all downloads (media, user, timestamp, IP)
- Log all deletions (media, user, timestamp, reason)
- Track conversion generation

### Non-Functional Requirements

#### NFR1: Storage
- Local filesystem for development
- S3-compatible for production (MinIO, AWS S3)
- Abstract driver interface for custom backends
- Atomic upload + DB commit

#### NFR2: Performance
- Lazy-load conversions (don't block upload)
- Background job queue for conversion generation (or async HTTP)
- Efficient metadata queries (indexed MIME type, created_at)
- CDN-friendly URLs (signed, versioned, cacheable)

#### NFR3: Security
- Validate file type (magic bytes, MIME type allowlist)
- Prevent directory traversal (safe filenames)
- Scan for malware (optional integration)
- Signed/expiring download URLs (prevent hotlinking)
- Permission checks on every file operation

#### NFR4: Scalability
- Multi-instance consistency (distributed locks for conversion)
- Cleanup orphaned files (periodic job)
- Archive old media (lifecycle policies)
- Support sharding by model type

---

## Data Model (Draft)

### Core Tables

**`media`** - Polymorphic media registry
```
id                    UUID PK
model_type            VARCHAR(255) -- "news", "user", "invoice"
model_id              UUID         -- Foreign key to specific model
collection_name       VARCHAR(255) -- "images", "documents", "avatars"
filename              VARCHAR(255)
original_filename     VARCHAR(255)
mime_type             VARCHAR(100)
size                  BIGINT
disk                  VARCHAR(50)  -- "local", "s3"
path                  VARCHAR(2000) -- Relative path on disk
metadata              JSONB        -- width, height, duration, etc.
custom_properties     JSONB        -- User-defined metadata
uploaded_by_id        UUID         -- Foreign key to users
created_at            TIMESTAMP
updated_at            TIMESTAMP
deleted_at            TIMESTAMP    -- Soft delete
```

**`media_conversions`** - Generated file variants
```
id                    UUID PK
media_id              UUID FK
name                  VARCHAR(255) -- "thumbnail", "preview"
size                  BIGINT
path                  VARCHAR(2000)
metadata              JSONB        -- Conversion-specific data
created_at            TIMESTAMP
```

**`media_downloads`** (Audit) - Download access log
```
id                    UUID PK
media_id              UUID FK
downloaded_by_id      UUID FK
downloaded_at         TIMESTAMP
ip_address            VARCHAR(45)
user_agent            VARCHAR(2000)
```

---

## API Contracts

### Upload Media
```
POST /api/v1/models/:model_type/:model_id/media
Headers: Authorization, Content-Type: multipart/form-data
Body:
  file: (binary)
  collection: "images"           # Optional, default "default"
  custom_properties: {...}       # JSON metadata

Response 201:
{
  "data": {
    "id": "uuid",
    "filename": "...",
    "mime_type": "image/jpeg",
    "size": 1024,
    "collection": "images",
    "conversions": [
      {
        "name": "thumbnail",
        "size": 512,
        "url": "/media/uuid/conversions/thumbnail"
      }
    ],
    "url": "/media/uuid/download",
    "created_at": "2026-04-22T...",
    "metadata": {...}
  }
}
```

### List Media
```
GET /api/v1/models/:model_type/:model_id/media?collection=images
Response 200:
{
  "data": [
    { "id": "uuid", "filename": "...", ... }
  ],
  "meta": { "total": 10, "limit": 20, "offset": 0 }
}
```

### Download Media
```
GET /media/:media_id/download
Query: ?conversion=thumbnail (optional)
Response: File stream with proper headers
```

### Delete Media
```
DELETE /api/v1/media/:media_id
Response 204: No Content
```

### Get Media URL (with Signature)
```
GET /api/v1/media/:media_id/url
Query: ?conversion=thumbnail&expires_in=3600
Response:
{
  "data": {
    "url": "https://cdn.example.com/media/uuid?sig=...",
    "expires_at": "2026-04-22T19:22:00Z"
  }
}
```

---

## Architecture Layers

### Storage Driver Interface
```go
type StorageDriver interface {
  Store(ctx context.Context, path string, content io.Reader) error
  Get(ctx context.Context, path string) (io.ReadCloser, error)
  Delete(ctx context.Context, path string) error
  URL(ctx context.Context, path string, expires time.Duration) (string, error)
  Exists(ctx context.Context, path string) (bool, error)
}
```

### Conversion Handler Interface
```go
type ConversionHandler interface {
  Handle(ctx context.Context, media *Media, definition ConversionDef) (*MediaConversion, error)
  // e.g., ImageConversionHandler, PDFConversionHandler
}
```

### Permission Scope
```go
type MediaScope interface {
  CanUpload(ctx context.Context, userID uuid.UUID, modelType string, modelID uuid.UUID) (bool, error)
  CanDownload(ctx context.Context, userID uuid.UUID, media *Media) (bool, error)
  CanDelete(ctx context.Context, userID uuid.UUID, media *Media) (bool, error)
}
```

---

## Key Decisions & Trade-offs

| Decision | Rationale | Alternative Rejected |
|----------|-----------|---------------------|
| Polymorphic via `model_type` + `model_id` | Flexible, avoids foreign key constraints; easier to support new models | Separate tables per model (bloat), inheritance (complex) |
| Soft deletes on media | Preserve audit trail; can restore deleted media | Hard deletes (audit loss) |
| JSONB metadata | Flexible extensibility; no schema migrations for new fields | Fixed columns (rigid), separate table (normalization burden) |
| Async conversions | Non-blocking uploads; don't tie up request handler | Sync conversions (slow uploads) |
| Signed URLs | Prevent hotlinking; control access expiry | Public URLs (CDN issues) |
| Collection names (not IDs) | Human-readable, easier debugging; no FK to schema | Collection IDs (requires lookup table) |

---

## Implementation Phases

| Phase | Deliverable | Effort |
|-------|-------------|--------|
| 0 | Research: File upload best practices, S3 integration, Go imaging libraries | 2d |
| 1 | Design: Data model, API contracts, storage interfaces, conversion pipelines | 3d |
| 2 | Core repository + storage abstraction | 3d |
| 3 | Upload handler + file validation | 2d |
| 4 | Download handler + permission checks | 2d |
| 5 | Image conversion (thumbnail) | 2d |
| 6 | Query optimization (eager-load, scopes) | 2d |
| 7 | Audit logging + API routes | 2d |
| 8 | Admin endpoints (cleanup, stats) | 2d |
| 9 | Integration tests + testcontainers | 3d |
| 10 | Documentation, benchmarks, production checklist | 2d |

**Estimated Total**: 25 days

---

## Technical Questions (NEEDS CLARIFICATION)

1. **Image library**: Use `github.com/davidbyttow/govips/v2` (fast, libvips) or `image/jpeg`, `image/png` (stdlib, slower)?
2. **Async conversions**: Background job queue (Redis queue, NATS) or fire-and-forget with exponential backoff?
3. **Multi-part upload**: Support S3 multipart for large files (>100MB)?
4. **Virus scanning**: ClamAV integration, or skip for v1?
5. **CDN integration**: CloudFront presigned URLs, or generic signed URLs?
6. **Cleanup policy**: Orphaned media after model deletion—automatic job, manual admin command, or both?
7. **Storage quotas**: Per-user/model storage limits? Track usage in DB?
8. **Versioning**: Support media versioning (keep history of overwritten files)?

---

## Success Criteria

- ✅ Any model can attach media without schema changes
- ✅ All file operations audited (upload, download, delete)
- ✅ Permission-aware (RBAC enforced on all operations)
- ✅ 80% unit + integration test coverage
- ✅ Supports S3 + local filesystem
- ✅ Auto-generate thumbnails on image upload
- ✅ Handles 10k concurrent uploads without data loss
- ✅ No orphaned files after model deletion
- ✅ Production-ready error handling + observability

---

## References

- [Spatie Media Library](https://spatie.be/docs/laravel-medialibrary/v10/introduction)
- [AWS S3 Multipart Upload](https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html)
- [Go imaging libraries](https://github.com/davidbyttow/govips)
- [OWASP File Upload Security](https://owasp.org/www-community/vulnerabilities/Unrestricted_File_Upload)

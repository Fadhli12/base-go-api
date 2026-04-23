# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial project setup with Go 1.22+, Echo v4, and GORM
- JWT-based authentication with access and refresh tokens
- Role-Based Access Control (RBAC) using Casbin
- Permission management system with resource/action model
- Audit logging for compliance and security
- Invoice management module as example domain feature
- Rate limiting middleware
- Health check endpoints (/healthz, /readyz)

## [1.1.0] - 2026-04-22

### Added

#### Media Library System (Phase 1-10)

A comprehensive polymorphic media library enabling any domain model to associate and manage related assets (images, documents, files).

**Core Features:**
- **Polymorphic Media Association**: Single `media` table supporting multiple model types (News, User, Invoice, etc.)
- **File Storage Abstraction**: Pluggable storage drivers (Local, S3, MinIO)
- **Automatic Image Conversions**: Thumbnail (300x300) and Preview (800x600) generation
- **Signed URL Support**: Time-limited secure download URLs with HMAC-SHA256
- **RBAC Permission Control**: Fine-grained permissions (upload, download, delete, view, manage)
- **Comprehensive Audit Logging**: Track all upload, download, and delete operations
- **Soft Delete Recovery**: 30-day recovery window for accidentally deleted files
- **Async Conversion Pipeline**: Hybrid sync/async processing for optimal performance
- **Orphaned File Cleanup**: Automatic cleanup job for tracking and removing orphaned files

**API Endpoints:**
- `POST /api/v1/models/{model_type}/{model_id}/media` - Upload media
- `GET /api/v1/models/{model_type}/{model_id}/media` - List media for model
- `GET /api/v1/media/{media_id}` - Get media details
- `GET /api/v1/media/{media_id}/download` - Download media (supports conversions)
- `GET /api/v1/media/{media_id}/url` - Generate signed URL
- `PATCH /api/v1/media/{media_id}` - Update metadata
- `DELETE /api/v1/media/{media_id}` - Soft delete media
- `GET /api/v1/admin/media` - Admin: list all media
- `POST /api/v1/admin/media/cleanup` - Admin: cleanup orphaned media
- `GET /api/v1/admin/media/stats` - Admin: storage statistics

**Security Features:**
- Magic bytes detection for MIME type validation
- Blocked extensions list (prevents script uploads)
- Path traversal prevention
- Filename sanitization with UUID-based storage
- HMAC-SHA256 signed URLs
- Permission-based access control
- Immutable audit logs

**Performance:**
- Upload: <500ms for files <10MB
- Download: <100ms for cached files
- Sync conversions for small files (<5MB)
- Async Redis queue for large files (≥5MB)
- Concurrent conversion workers

**Database Schema:**
- `media` table: Main media registry with polymorphic relationships
- `media_conversions` table: Generated file variants
- `media_downloads` table: Download audit tracking

**Documentation:**
- `docs/media-library/ARCHITECTURE.md` - System architecture and design
- `docs/media-library/DEPLOYMENT.md` - Deployment and operations guide
- `docs/media-library/API.md` - Complete API reference
- `docs/media-library/CONFIG.md` - Configuration reference
- `docs/media-library/SECURITY.md` - Security guide
- `docs/media-library/TROUBLESHOOTING.md` - Troubleshooting guide

**Migrations:**
- `000004_media_tables.up.sql` - Creates media tables and indexes

**File Size Limits:**
- Maximum upload: 100MB per file
- Small file threshold: 5MB (sync/async boundary)

**Supported Formats:**
- Images: JPEG, PNG, WebP, GIF, SVG
- Documents: PDF, Word, TXT, CSV
- Archives: ZIP, RAR, 7Z

### Technical Details

**Dependencies Added:**
- `github.com/davidbyttow/govips/v2` - Image processing (libvips wrapper)
- `github.com/aws/aws-sdk-go-v2` - S3/MinIO integration

**Architecture:**
- Domain-driven design with polymorphic associations
- Repository pattern for data access
- Service layer for business logic
- Storage driver abstraction for pluggable backends
- Conversion handler interface for extensible image processing

**Testing:**
- Unit tests for service and repository layers
- Integration tests with testcontainers
- Test coverage: 80%+ for media package

### Changed

- Updated API documentation with media endpoints
- Enhanced response envelope with pagination metadata
- Added conversion metadata to media responses

### Fixed

- N/A (new feature)

---

## [1.0.0] - 2026-04-01

### Added

- Initial release with core Go API Base functionality
- JWT authentication system
- RBAC with Casbin
- Audit logging
- Health checks
- Docker support

---

## Release Notes Template

```
## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Removed features

### Fixed
- Bug fixes

### Security
- Security improvements
```

---

## Versioning Notes

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

## Migration Guide

### From 1.0.0 to 1.1.0

1. Run database migrations:
   ```bash
   make migrate
   ```

2. Seed media permissions:
   ```bash
   make seed
   ```

3. Configure storage environment variables:
   ```bash
   MEDIA_STORAGE_DRIVER=local
   MEDIA_STORAGE_PATH=./storage
   MEDIA_SIGNING_SECRET=your-32-char-secret
   ```

4. Create storage directory:
   ```bash
   mkdir -p ./storage
   chmod 755 ./storage
   ```

5. Restart application

## Future Roadmap

### [1.2.0] - Planned
- Video transcoding support
- Multi-part upload for large files (>100MB)
- CDN integration helpers
- Media versioning
- Storage quotas per user/organization

### [1.3.0] - Planned
- ClamAV malware scanning integration
- EXIF data preservation
- Advanced image filters
- Webhook notifications

### [2.0.0] - Planned
- Breaking API changes (if any)
- Migration to new major dependencies

---

For detailed release information, see:
- GitHub Releases: https://github.com/example/go-api-base/releases
- Migration Guides: https://github.com/example/go-api-base/blob/main/docs/migrations/

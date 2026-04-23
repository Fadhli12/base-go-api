# Phase 0 Research: Media Library for Go

**Status**: Research Complete  
**Date**: 2026-04-22  
**Branch**: `002-media-library`

---

## NEEDS CLARIFICATION Resolution

### 1. Image Processing Library

**Question**: Use `github.com/davidbyttow/govips/v2` (fast, libvips) or `image/jpeg`, `image/png` (stdlib, slower)?

**Decision**: **Use `davidbyttow/govips/v2` with stdlib fallback**

**Rationale**:
- govips is 10-100x faster for bulk conversions (libvips C binding)
- Supports extensive transformations (crop, rotate, watermark, quality adjustment)
- Memory-efficient (streaming, no full image load)
- Production-proven in high-volume image services

**Alternatives Considered**:
- Pure stdlib (`image/jpeg`, `image/png`): slower, limited features, not suitable for thumbnails at scale
- ImageMagick bindings: slower, larger memory footprint, security concerns
- **Selected**: govips with fallback to stdlib for non-image files

**Implementation**: 
- Provide `ConversionHandler` interface
- Implement `ImageConversionHandler` using govips
- Document fallback behavior for binary files

---

### 2. Asynchronous Conversions

**Question**: Background job queue (Redis queue, NATS) or fire-and-forget with exponential backoff?

**Decision**: **Hybrid: fire-and-forget for small files (<5MB), Redis queue for large files (>5MB)**

**Rationale**:
- Small file conversions complete in <100ms; don't need queuing overhead
- Large files may timeout on HTTP request; need async processing
- Redis queue already available in project stack (for permission caching)
- Exponential backoff prevents hammering resources; distributed lock prevents duplicate work

**Alternatives Considered**:
- Pure fire-and-forget: Risk of lost conversions on service restart
- Pure Redis queue: Overhead for tiny operations
- NATS integration: Extra dependency, project uses Redis

**Implementation**:
- Upload handler spawns goroutine for conversions <5MB
- Conversions >5MB enqueued to Redis
- Worker process (or same instance) pulls jobs with exponential backoff
- Use distributed lock (Redis SET NX) to prevent duplicate conversions

---

### 3. Multi-part Upload Support

**Question**: Support S3 multipart for large files (>100MB)?

**Decision**: **Yes, but v1 MVP supports single-part only. Plan v2 for multipart**

**Rationale**:
- S3 multipart avoids timeouts on large uploads
- Single-part sufficient for MVP (most media <50MB)
- Adds complexity to v1 (separate resumable upload API)
- Can implement in v2 iteration

**Alternatives Considered**:
- No large file support: Limits use cases (video, archives)
- Always multipart: Overhead for small files

**Implementation (v1)**:
- Max upload size: 100MB per request
- Document resumable upload strategy for later

**Implementation (v2)**:
- Multipart API: `POST /upload/init`, `PUT /upload/:id/part/:num`, `POST /upload/:id/complete`
- Client-side SDK for automatic chunking

---

### 4. Virus Scanning

**Question**: ClamAV integration, or skip for v1?

**Decision**: **Skip v1. Plan optional integration point for v2**

**Rationale**:
- Adds external dependency (ClamAV daemon)
- Increases deployment complexity
- File validation (magic bytes, MIME type) covers most cases
- Can add as optional middleware later

**Alternatives Considered**:
- Mandatory ClamAV: Complex deployment, slower uploads
- HTTP scanner integration (VirusTotal): Rate-limited, adds latency

**Implementation (v1)**:
- Document security scanning interface
- Provide no-op scanner by default
- Allow custom scanner implementation (interface)

**Implementation (v2)**:
- Optional ClamAV scanner
- Optional third-party API scanner

---

### 5. CDN Integration

**Question**: CloudFront presigned URLs, or generic signed URLs?

**Decision**: **Generic signed URLs (HMAC-SHA256) for CDN-agnostic solution**

**Rationale**:
- Works with any CDN (CloudFront, Cloudflare, custom)
- No AWS dependency (supports S3-compatible, local, custom)
- HMAC signature validates URL integrity; expires after TTL
- CDN can cache (signature constant for same TTL)

**Alternatives Considered**:
- CloudFront presigned: Vendor lock-in, requires AWS SDK
- Unsigned public URLs: No access control, hotlinking issues

**Implementation**:
- Generate signature: `HMAC-SHA256(secret, media_id + expiry)`
- URL: `/media/:media_id/download?expires=<unix_ts>&sig=<hex>`
- Validate on download; reject if expired or signature invalid

---

### 6. Cleanup Policy for Orphaned Media

**Question**: Automatic job, manual admin command, or both?

**Decision**: **Both: automatic TTL cleanup + manual admin command**

**Rationale**:
- Automatic cleanup prevents infinite storage growth
- Manual command allows sysadmins to force cleanup on-demand
- TTL-based: After model deletion, media marked with `orphaned_at`, cleaned after 30 days (configurable)
- Manual: `go run ./cmd/api media:cleanup --force` for immediate cleanup

**Alternatives Considered**:
- Automatic only: No manual control, slow cleanup (30-day lag)
- Manual only: Requires human intervention, easy to forget

**Implementation**:
- Add `orphaned_at` TIMESTAMP to media table (nullable)
- Background job (once daily): Find media with `orphaned_at < now - 30 days`, delete files + DB records
- CLI command: `media:cleanup [--force] [--dry-run]`

---

### 7. Storage Quotas

**Question**: Per-user/model storage limits? Track usage in DB?

**Decision**: **Optional quotas (disabled by default, configurable per role)**

**Rationale**:
- Not all deployments need quotas (internal tools, unlimited plans)
- RBAC allows per-role limits (free user: 100MB, pro: 5GB, admin: unlimited)
- Track usage in DB (cached, updated on upload/delete)
- Enforce on upload (reject if would exceed quota)

**Alternatives Considered**:
- No quotas: Unlimited growth, expensive
- Hard-coded quotas: Inflexible
- Filesystem-based limits: Unreliable across instances

**Implementation**:
- Add `storage_quota_bytes` to roles (optional, null = unlimited)
- Add `storage_used_bytes` to media_owners table (user/model usage cache)
- Update cache on upload/delete
- Check before upload: `if storage_used + file_size > quota: reject`
- Document quota enforcement, cache invalidation

---

### 8. Media Versioning

**Question**: Support media versioning (keep history of overwritten files)?

**Decision**: **No versioning in v1. Plan as v2 feature**

**Rationale**:
- Adds complexity (version chain, cleanup logic)
- Most use cases don't need versioning (images, documents)
- Can implement later as non-breaking feature
- Alternative: Users upload new file, old file soft-deleted

**Alternatives Considered**:
- Always version: Storage overhead, complexity
- Optional versioning: Feature flag adds complexity

**Implementation (v1)**:
- No versioning; overwrite replaces media
- Document soft-delete behavior (recoverable for 30 days)

**Implementation (v2)**:
- Version chain: media.version_id → previous media
- Cleanup: Configurable retention (keep latest N versions, or versions < X days old)

---

## Technology Stack Decisions

### Image Processing

**Selected**: `github.com/davidbyttow/govips/v2`  
**Version**: v2.13+  
**Fallback**: stdlib `image/jpeg`, `image/png`

**Setup**:
```bash
# Install libvips on host
brew install vips              # macOS
apt-get install libvips-dev    # Debian/Ubuntu
yum install vips-devel         # RHEL/CentOS
```

**Go imports**:
```go
import "github.com/davidbyttow/govips/v2/vips"
```

---

### Storage Drivers

**Local Storage** (development):
- Path: `./storage/media/` (configurable via `MEDIA_STORAGE_PATH`)
- Safe filename generation: UUID + original extension
- No permission issues (single-user development)

**S3 Storage** (production):
- SDK: `github.com/aws/aws-sdk-go-v2`
- Bucket: `${PROJECT_NAME}-media-${ENV}` (from config)
- Path structure: `{model_type}/{model_id}/{uuid}/{filename}`
- Signed URLs: 1-hour default expiry (configurable)
- Server-side encryption: AES-256 (optional)

**MinIO** (S3-compatible, self-hosted):
- Endpoint: `minio.example.com:9000`
- Uses same AWS SDK with custom endpoint config
- Bucket naming: Same as S3

---

### Async Conversions

**Small file (<5MB)**: Synchronous goroutine
- Blocks upload request (acceptable <100ms overhead)
- No queue complexity
- Simple error handling

**Large file (>5MB)**: Redis queue
- Library: `go-redis/redis/v8` (already in project)
- Queue structure: JSON-encoded job to `media:conversions:queue` list
- Worker: Separate goroutine pool or process
- Retry: Exponential backoff (1s, 2s, 4s, 8s, max 5 retries)
- Lock: `media:conversions:lock:{media_id}` (Redis SET NX EX 3600)

---

### Audit Logging

**Existing Project Pattern**: JSONB audit_logs table  
**Alignment**: Use same audit_logs table for media operations

**Log structure**:
```json
{
  "actor_id": "uuid",
  "resource": "media",
  "resource_id": "uuid",
  "action": "UPLOAD|DOWNLOAD|DELETE|CONVERT",
  "timestamp": "2026-04-22T18:30:00Z",
  "ip_address": "192.168.1.1",
  "user_agent": "curl/7.68.0",
  "status": "success|failed",
  "error": "reason if failed",
  "metadata": {
    "file_size": 1024,
    "mime_type": "image/jpeg",
    "model_type": "news",
    "model_id": "uuid"
  }
}
```

---

### Permission Integration

**Existing Pattern**: `enforcer.Enforce(userID, domain, resource, action)`  
**Media Domain**: `"media"`  
**Resources**: `"media"`, `"{model_type}"` (e.g., `"news"`)  
**Actions**: `"upload"`, `"download"`, `"delete"`, `"view"`, `"publish"`

**Permission scope logic**:
- `media:upload` → Create media on any model (admin)
- `news:upload` → Create media on News model only (scoped)
- `media:delete` → Delete any media (admin)
- `media:download` → Download any media (admin)
- Owner exemption: Users can always download own media (enforcer.Enforce exemption)

---

## Best Practices Summary

| Practice | Why | Implementation |
|----------|-----|-----------------|
| **Atomic uploads** | Prevent orphaned files on DB failure | Transaction: store file, then commit media record |
| **Safe filenames** | Prevent directory traversal attacks | UUID + sanitized extension only |
| **Magic byte validation** | Prevent disguised malware | Check file header before MIME type |
| **Signed URLs** | Prevent hotlinking | HMAC-SHA256 signature with expiry |
| **Lazy conversions** | Non-blocking uploads | Async queue for heavy operations |
| **Distributed locks** | Prevent duplicate conversions | Redis SET NX EX |
| **Soft deletes** | Preserve audit trail | DeletedAt on media table |
| **Indexed queries** | Efficient listing | Index model_type, collection_name, created_at |

---

## Next Steps

1. ✅ Phase 0: Research complete
2. → Phase 1: Data model + contracts (use research.md decisions)
3. → Phase 2-10: Implementation per plan.md phases

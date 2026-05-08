# HTTP API Contracts: File Versioning v1

**Version**: 1.0 | **Date**: 2026-05-08 | **Status**: Phase 1 Design

---

## Overview

All version endpoints follow existing API conventions: JWT authentication, standardized envelope responses, audit middleware on mutations. Version endpoints extend the existing media routes.

**Base URL**: `/api/v1/media/{media_id}/versions`

**Authentication**: JWT Bearer token required for all endpoints.

**Permissions**: New `media_version` resource permissions:
- `media_version:upload` — upload new versions
- `media_version:view` — view version history
- `media_version:download` — download specific versions
- `media_version:restore` — restore previous versions
- `media_version:delete` — admin-only, delete specific versions

Existing `media:upload`, `media:view`, `media:download`, `media:manage` permissions apply to the base media operations. Version-specific operations use `media_version:*` permissions.

**Organization Context**: Version operations inherit the organization context from the parent `Media` entity via the `X-Organization-ID` header. The existing organization middleware extracts this header.

**Response Format**: All responses use the standard `Envelope` wrapper:
```json
{
  "data": { ... },
  "meta": {
    "request_id": "abc-123",
    "timestamp": "2026-05-08T10:00:00Z"
  }
}
```

---

## Endpoints

### 1. POST /media/{media_id}/versions

**Purpose**: Upload a new version of an existing media file. Creates a `MediaVersion` record with incremented version number, computes SHA-256 checksum, and advances the parent media's `current_version`.

**Authentication**: Required. Permission: `media_version:upload`

**Request**: `multipart/form-data`

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `file` | file | yes | Max 100MB, allowed MIME types, extension not blocked |
| `collection` | string | no | Max 255 chars (must match parent media's collection) |

**Response** (201 Created):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "current_version": 2,
    "version": {
      "id": "v1e2r3s4-i5o6-n789-0abc-def123456789",
      "version": 2,
      "filename": "a1b2c3d4.jpg",
      "original_filename": "updated-report.pdf",
      "mime_type": "application/pdf",
      "size": 2048576,
      "checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "uploaded_by_id": "u1s2e3r4-i5d6-7890-abcd-ef1234567890",
      "is_current": true,
      "created_at": "2026-05-08T10:30:00Z"
    }
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 400: Invalid file, MIME type mismatch with parent media, file extension blocked, size exceeds limit
- 404: Media record not found
- 409: Checksum matches current version (duplicate content)
- 409: Optimistic locking conflict (concurrent upload — retry with updated base version)
- 413: File exceeds max size
- 403: User lacks `media_version:upload`

**Behavior Notes**:
- If the media record has no `MediaVersion` rows (created before versioning), the system retroactively creates a v1 `MediaVersion` row from the existing media record, then creates the new upload as v2.
- Checksum is compared against the current version's checksum only, not all historical versions.
- Optimistic locking: the request implicitly references `current_version`. If another upload occurred between your read and write, the system returns 409 with the new `current_version` so you can retry.

---

### 2. GET /media/{media_id}/versions

**Purpose**: List the version history of a media record. Returns all versions in descending order with metadata.

**Authentication**: Required. Permission: `media_version:view`

**Query Params**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |

**Response** (200 OK):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "current_version": 3,
    "versions": [
      {
        "id": "v3e4r5s6-i7o8-n901-2abc-def123456789",
        "version": 3,
        "filename": "c3d4e5f6.jpg",
        "original_filename": "report-v3.pdf",
        "mime_type": "application/pdf",
        "size": 3145728,
        "checksum": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
        "uploaded_by_id": "u1s2e3r4-i5d6-7890-abcd-ef1234567890",
        "is_current": true,
        "created_at": "2026-05-08T12:00:00Z"
      },
      {
        "id": "v2e3r4s5-i6o7-n890-1abc-def123456789",
        "version": 2,
        "filename": "b2c3d4e5.jpg",
        "original_filename": "report-v2.pdf",
        "mime_type": "application/pdf",
        "size": 2621440,
        "checksum": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1",
        "uploaded_by_id": "u1s2e3r4-i5d6-7890-abcd-ef1234567890",
        "is_current": false,
        "created_at": "2026-05-08T11:00:00Z"
      },
      {
        "id": "v1e2r3s4-i5o6-n789-0abc-def123456789",
        "version": 1,
        "filename": "a1b2c3d4.jpg",
        "original_filename": "report-v1.pdf",
        "mime_type": "application/pdf",
        "size": 2097152,
        "checksum": "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
        "uploaded_by_id": "u1s2e3r4-i5d6-7890-abcd-ef1234567890",
        "is_current": false,
        "created_at": "2026-05-08T10:00:00Z"
      }
    ],
    "total": 3,
    "limit": 20,
    "offset": 0
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Note**: Soft-deleted versions are excluded from listing. Use `Unscoped()` query for admin audit views.

**Errors**:
- 404: Media record not found (or soft-deleted and user is not admin)
- 403: User lacks `media_version:view`

---

### 3. GET /media/{media_id}/versions/{version}

**Purpose**: Get details of a specific version by its version number.

**Authentication**: Required. Permission: `media_version:view`

**Path Params**:

| Param | Type | Description |
|-------|------|-------------|
| `media_id` | UUID | Parent media record ID |
| `version` | int | Version number (1-based) |

**Response** (200 OK):
```json
{
  "data": {
    "id": "v1e2r3s4-i5o6-n789-0abc-def123456789",
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "version": 1,
    "filename": "a1b2c3d4.jpg",
    "original_filename": "report-v1.pdf",
    "mime_type": "application/pdf",
    "size": 2097152,
    "file_path": "uploads/a1b2c3d4-e5f6-7890-abcd-ef1234567890/v1/a1b2c3d4.jpg",
    "checksum": "c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
    "uploaded_by_id": "u1s2e3r4-i5d6-7890-abcd-ef1234567890",
    "is_current": false,
    "created_at": "2026-05-08T10:00:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: Media record not found
- 404: Version number does not exist for this media
- 410: Version is soft-deleted
- 403: User lacks `media_version:view`

---

### 4. GET /media/{media_id}/versions/{version}/download

**Purpose**: Download the file content of a specific version directly. Streams the file from the storage backend.

**Authentication**: Required (or valid signed URL). Permission: `media_version:download`

**Query Params** (optional signed URL auth):

| Param | Type | Description |
|-------|------|-------------|
| `sig` | string | Signature for signed URL access |
| `expires` | int | Expiry timestamp for signed URL |

**Response** (200 OK):
- `Content-Type`: version's MIME type
- `Content-Disposition`: `attachment; filename="{original_filename}"`
- `Content-Length`: version's file size
- Body: binary file stream

**Errors**:
- 404: Media not found
- 404: Version not found or file missing from storage
- 410: Version is soft-deleted
- 401: Not authenticated and no valid signed URL
- 403: User lacks `media_version:download`

**Audit**: Each download creates an audit log entry with `action=version_download`, `resource=media_version`, `resourceID=<version UUID>`, with version number and media ID in metadata. Also creates a `MediaDownload` record for download tracking.

---

### 5. GET /media/{media_id}/versions/{version}/url

**Purpose**: Generate a signed URL for downloading a specific version without authentication. Useful for sharing specific version links.

**Authentication**: Required. Permission: `media_version:download`

**Query Params**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `expires_in` | int | 3600 | URL expiry in seconds (60-86400) |

**Response** (200 OK):
```json
{
  "data": {
    "url": "https://api.example.com/storage/...?sig=...&expires=...",
    "expires_at": "2026-05-08T11:30:00Z",
    "expires_in": 3600
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 404: Media not found
- 404: Version not found
- 410: Version is soft-deleted
- 403: User lacks `media_version:download`

---

### 6. POST /media/{media_id}/versions/{version}/restore

**Purpose**: Restore a specific previous version as the current version. Updates the parent media's `current_version` pointer. Does NOT create a new version or copy files — it simply changes the pointer.

**Authentication**: Required. Permission: `media_version:restore`

**Path Params**:

| Param | Type | Description |
|-------|------|-------------|
| `media_id` | UUID | Parent media record ID |
| `version` | int | Version number to restore as current |

**Response** (200 OK):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "previous_version": 3,
    "restored_version": 1,
    "current_version": 1,
    "message": "Version 1 restored as current version"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 400: Version is already the current version (nothing to restore)
- 404: Media not found
- 404: Version not found
- 410: Version is soft-deleted (cannot restore deleted versions)
- 403: User lacks `media_version:restore`

**Audit**: Creates audit log with `action=version_restore`, `resource=media_version`, `resourceID=<version UUID>`. Before state: `{"current_version": 3}`. After state: `{"current_version": 1}`.

---

### 7. DELETE /media/{media_id}/versions/{version}

**Purpose**: Admin-only. Soft-deletes a specific version. The file is removed from storage, the database record is marked with `deleted_at`. The current version cannot be deleted.

**Authentication**: Required. Permission: `media_version:delete`

**Path Params**:

| Param | Type | Description |
|-------|------|-------------|
| `media_id` | UUID | Parent media record ID |
| `version` | int | Version number to delete |

**Response** (200 OK):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "version": 2,
    "message": "Version 2 deleted. File removed from storage, metadata retained for audit."
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Errors**:
- 400: Cannot delete the current version; restore another version first
- 404: Media not found
- 404: Version not found
- 409: Version is already deleted
- 403: User lacks `media_version:delete` (admin only)

**Audit**: Creates audit log with `action=version_delete`, `resource=media_version`, `resourceID=<version UUID>`. Before state: version metadata snapshot.

---

## Route Registration Summary

All version endpoints are mounted under the existing `/api/v1/media/{media_id}` prefix, building on the existing `MediaHandler.RegisterRoutes()` pattern:

```
POST   /api/v1/media/{media_id}/versions           → UploadVersion
GET    /api/v1/media/{media_id}/versions           → ListVersions
GET    /api/v1/media/{media_id}/versions/{version}           → GetVersion
GET    /api/v1/media/{media_id}/versions/{version}/download  → DownloadVersion
GET    /api/v1/media/{media_id}/versions/{version}/url       → GetVersionSignedURL
POST   /api/v1/media/{media_id}/versions/{version}/restore  → RestoreVersion
DELETE /api/v1/media/{media_id}/versions/{version}           → DeleteVersion
```

All endpoints use JWT middleware. Upload, restore, and delete also use the audit middleware for mutation logging.

---

## Integration with Existing Media Endpoints

The existing `GET /api/v1/media/{media_id}` endpoint is extended to include version info:

**Updated GET /media/{media_id} Response**:
```json
{
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "model_type": "news",
    "collection_name": "default",
    "filename": "report-v3.pdf",
    "mime_type": "application/pdf",
    "size": 3145728,
    "current_version": 3,
    "version_count": 3,
    "url": "https://...",
    "conversions": [...],
    "custom_properties": {},
    "created_at": "2026-05-08T10:00:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

The existing `GET /api/v1/media/{media_id}/download` continues to serve the current version's file. Use the version-specific download endpoint for historical versions.

---

**Version**: 1.0 | **Created**: 2026-05-08 | **Status**: Phase 1 Design

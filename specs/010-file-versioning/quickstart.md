# Quickstart: File Versioning

**Feature**: 010-file-versioning | **Last Updated**: 2026-05-08

---

## Prerequisites

File versioning requires the base API running with the versioning migration applied. Existing media upload endpoints must be functional. See main [README](../../README.md) for base setup.

---

## Step 1: Run Versioning Migration

```bash
make migrate
# Creates: media_versions table
# Adds: current_version column to media table
```

---

## Step 2: Sync Permissions

```bash
go run ./cmd/api permission:sync
# Adds: media_version:upload, media_version:view,
#       media_version:download, media_version:restore,
#       media_version:delete
```

---

## Step 3: Upload an Initial File (via existing media endpoint)

```bash
curl -X POST http://localhost:8080/api/v1/models/news/{model_id}/media \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -F "file=@document-v1.pdf"
```

**Response** (201):
```json
{
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "model_type": "news",
    "filename": "document-v1.pdf",
    "mime_type": "application/pdf",
    "size": 2097152,
    "current_version": 1,
    "version_count": 1,
    "created_at": "2026-05-08T10:00:00Z"
  }
}
```

---

## Step 4: Upload a New Version

```bash
MEDIA_ID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"

curl -X POST "http://localhost:8080/api/v1/media/$MEDIA_ID/versions" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -F "file=@document-v2.pdf"
```

**Response** (201):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "current_version": 2,
    "version": {
      "id": "v1e2r3s4-...",
      "version": 2,
      "filename": "b2c3d4e5.pdf",
      "original_filename": "document-v2.pdf",
      "mime_type": "application/pdf",
      "size": 2621440,
      "checksum": "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c",
      "is_current": true,
      "created_at": "2026-05-08T11:00:00Z"
    }
  }
}
```

**Important**: If you upload a file with identical content to the current version, you get a 409 Conflict:
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "File content is identical to the current version. No new version created."
  }
}
```

---

## Step 5: View Version History

```bash
curl -X GET "http://localhost:8080/api/v1/media/$MEDIA_ID/versions" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200): Returns all versions in descending order with checksums, uploader info, and timestamps.

---

## Step 6: Download a Specific Version

```bash
# Download version 1
curl -X GET "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/1/download" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -o document-v1-downloaded.pdf
```

---

## Step 7: Get a Signed URL for a Version

```bash
curl -X GET "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/1/url?expires_in=3600" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "url": "https://api.example.com/storage/...",
    "expires_at": "2026-05-08T12:30:00Z",
    "expires_in": 3600
  }
}
```

---

## Step 8: Restore a Previous Version

```bash
# If version 3 is current and you want to go back to version 1:
curl -X POST "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/1/restore" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "previous_version": 3,
    "restored_version": 1,
    "current_version": 1,
    "message": "Version 1 restored as current version"
  }
}
```

**Note**: Restoring does NOT create a new version. It just updates the `current_version` pointer on the media record. The original version 1 file stays where it is.

---

## Step 9: Delete a Version (Admin Only)

```bash
# Delete version 2 (must not be the current version)
curl -X DELETE "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/2" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response** (200):
```json
{
  "data": {
    "media_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "version": 2,
    "message": "Version 2 deleted. File removed from storage, metadata retained for audit."
  }
}
```

**You cannot delete the current version:**
```json
{
  "error": {
    "code": "BAD_REQUEST",
    "message": "Cannot delete the current version. Restore another version first or delete the media record."
  }
}
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_DRIVER` | `local` | Storage backend: `local`, `s3`, or `minio` |
| `STORAGE_LOCAL_PATH` | `./storage/uploads` | Local storage base path |
| `MEDIA_MAX_FILE_SIZE` | `104857600` | Max file size (100MB) |

Versioning inherits all existing media configuration. No additional env vars are required for the versioning feature itself.

---

## Storage Layout

```
{STORAGE_LOCAL_PATH}/
└── {media_id}/
    ├── v1/
    │   └── {uuid_filename}.{ext}   ← First upload
    ├── v2/
    │   └── {uuid_filename}.{ext}   ← Second upload
    └── v3/
        └── {uuid_filename}.{ext}   ← Third upload (current)
```

Each version lives in its own subdirectory — no file overwrites.

---

## Backward Compatibility

Media records created before the versioning migration:
- `current_version = 1` (default)
- No `MediaVersion` rows exist initially
- First version upload retroactively creates v1 from existing media data, then uploads v2
- All existing media endpoints continue to work unchanged

---

## Common Workflows

### Rollback a Bad Upload

```bash
# 1. Check what versions exist
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/versions" \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 2. Download the previous good version to verify
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/2/download" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -o /tmp/verify.pdf

# 3. Restore the good version
curl -X POST "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/2/restore" \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 4. Optionally, delete the bad version (admin only)
curl -X DELETE "http://localhost:8080/api/v1/media/$MEDIA_ID/versions/3" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

### Check for Duplicate Uploads Before Uploading

```bash
# The system handles this automatically — if you upload a file with the same
# SHA-256 checksum as the current version, you get a 409 Conflict.
# No need to pre-check; the server validates on upload.
```

---

**Questions?** See [contracts/api-v1.md](./contracts/api-v1.md) for full endpoint specs and [data-model.md](./data-model.md) for entity details.

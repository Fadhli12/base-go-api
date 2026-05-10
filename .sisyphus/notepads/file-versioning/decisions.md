# File Versioning Implementation - Decisions

## 2026-05-08 Architecture Decisions

### D1: VersionService Location
- **Decision**: Create `internal/service/media_version.go` as a separate service
- **Reason**: Keeps media service unchanged; version logic is complex enough to warrant its own file

### D2: MediaVersionRepository Interface
- **Decision**: New interface `MediaVersionRepository` in `internal/repository/media_version.go`
- **Reason**: Follows existing pattern (separate interface per entity)

### D3: Version Numbering
- **Decision**: Integer version numbers starting at 1, stored in `media_versions.version`
- **Reason**: Matches spec; monotonically increasing, unique per media

### D4: Storage Path Format
- **Decision**: `{media_id}/v{version_number}/{uuid_filename}.{ext}`
- **Reason**: No overwrites possible; version-specific paths; consistent with spec

### D5: Retroactive v1 Creation
- **Decision**: When uploading first version to media with no MediaVersion rows, retroactively create v1 from existing media fields
- **Reason**: Backward compatibility — existing media gets version history

### D6: Checksum Deduplication
- **Decision**: SHA-256 checksum via `crypto/sha256` stdlib; compare only against current version; reject with 409 if match
- **Reason**: Prevents unnecessary storage; spec requirement

### D7: Optimistic Locking
- **Decision**: Use `media.current_version` as optimistic lock; increment in transaction; 409 Conflict on concurrent upload
- **Reason**: Prevents race conditions on version numbering

### D8: Restore = Pointer Update
- **Decision**: Restore updates `media.current_version` only; does NOT create new version or copy files
- **Reason**: Matches spec; efficient; preserves audit trail

### D9: Delete = Soft Delete + File Removal
- **Decision**: Soft delete sets `deleted_at` on MediaVersion row; physical file removed from storage; current version cannot be deleted
- **Reason**: Audit trail preserved; storage freed; data integrity maintained
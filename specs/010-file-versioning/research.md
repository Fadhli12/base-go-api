# Research: File Versioning

**Feature**: 010-file-versioning | **Date**: 2026-05-08

---

## Research Tasks & Decisions

### R1: Version Storage Layout

**Decision**: Versioned files stored at `{base_path}/{media_id}/v{version_number}/{filename}`. This pattern is selected because:
- Guarantees zero file overwrites across versions — each version gets its own directory
- Human-readable paths for debugging and manual recovery
- Compatible with all three storage drivers (local, S3, MinIO)
- The existing unversioned storage layout (`{model_type}/{model_id}/{uuid_filename}`) continues for backward compatibility — existing media records created before versioning keep their original path; a retroactive `MediaVersion` row (version=1) is created when their first new version is uploaded

**Alternatives rejected**:
- `{base_path}/{media_id}/{version_number}_{filename}` — risk of filename collisions with long names + version prefix
- `{base_path}/{media_id}/{uuid}` per version — loses human readability, harder to debug
- Pure UUID storage with lookup table — breaks existing storage driver expectations, complicates migrations

### R2: Checksum Strategy for Duplicate Detection

**Decision**: SHA-256 over the entire file content, computed server-side during upload. Compared against the current version's checksum to detect identical uploads (409 Conflict). Stdlib only — `crypto/sha256`. Stored as hex-encoded string in `checksum` column.

**Alternatives rejected**:
- MD5 — cryptographically broken, collisions theoretically possible
- SHA-1 — deprecated for security contexts
- Client-side checksum only — not trustable
- Partial checksum (first N bytes) — false negatives on same-header-different-body files

### R3: Optimistic Locking for Concurrent Uploads

**Decision**: Use the `current_version` field on the `Media` entity as an optimistic lock. When uploading a new version:
1. Read `media.current_version` (call it `expected_version`)
2. Upload file to storage
3. In a database transaction: INSERT `MediaVersion` with `version = expected_version + 1`, then UPDATE `media SET current_version = expected_version + 1 WHERE id = ? AND current_version = expected_version`
4. If 0 rows affected → conflict, rollback transaction + cleanup stored file

**Alternatives rejected**:
- Database-level advisory lock — Postgres-specific, complicates multi-driver
- Redis distributed lock — adds dependency, eventual consistency concerns
- Timestamp-based — not reliable for near-simultaneous uploads
- Row-level `SELECT ... FOR UPDATE` — blocks reads, worse performance

### R4: Backward Compatibility with Existing Media Records

**Decision**: Existing media records without `MediaVersion` rows are treated as version 1 implicitly. When a new version is uploaded for such media:
1. Retroactively create a `MediaVersion` row for the existing file (version=1, using existing file path, size, MIME type)
2. Create the new version as version=2
3. Set `media.current_version = 2`

The `Media` table gains a `current_version` column with `DEFAULT 1`. Existing records get `current_version=1` on migration.

**Alternatives rejected**:
- Require migration of all existing media — huge operational burden, risk of data loss
- Separate `legacy_media` logic — code bifurcation, maintenance cost
- Ignore old media — breaks existing integrations

### R5: MIME Type Consistency Validation

**Decision**: New versions of an existing media record MUST match the original's MIME type. This prevents confusing situations where a PDF gets replaced with a PNG under the same media ID. Server detects MIME type from magic bytes (first 512 bytes), same as existing `detectMimeType()` function, and compares against media's stored `mime_type`.

**Alternatives rejected**:
- Allow any MIME type — user confusion, breaks downstream consumers expecting specific format
- Restrict to same extension only — extensions are unreliable MIME indicators
- Allow change with explicit flag — adds API complexity for rare use case

### R6: Version Restore Semantics

**Decision**: Restoring a version updates the `current_version` pointer on the parent `Media` entity only — does NOT create a new `MediaVersion` row, does NOT copy files. The version number that was restored from becomes the new current version. Audit trail captures the restore action with `before_version` and `after_version`.

**Alternatives rejected**:
- Create a new version on restore (e.g., v4 with v1's content) — inflates version count, creates confusion about "which v1 is real"
- Copy file to new location on restore — wastes storage space
- Renumber versions — breaks audit trails referencing version numbers

### R7: Version Delete Semantics

**Decision**: Soft-delete the `MediaVersion` row (`deleted_at` set). The physical file is removed from storage synchronously during the delete operation. The metadata record persists for audit trail. The current version cannot be deleted — users must either promote another version first or delete the entire media record.

**Alternatives rejected**:
- Hard-delete the row — loses audit trail, violates constitution Principle III
- Keep the file on disk — wastes storage, doesn't match user expectation of "delete"
- Async file deletion — adds complexity, leaves orphan window

### R8: Database Schema Design

**Decision**: New table `media_versions` with FK to `media` on `CASCADE` (when media is deleted, all version records are deleted too — media still has soft delete, but if someone hard-deletes the media row, version rows go with it). The `Media` table gets one new column: `current_version INT DEFAULT 1 NOT NULL`. Migration number: `000019`.

**Indexes**: `media_id` (FK lookups, version history), `(media_id, version)` unique (prevent duplicate version numbers), `checksum` (duplicate detection for future features), `uploaded_by_id` (audit queries).

### R9: Permission Model

**Decision**: New permissions under the `media_version` resource:
- `media_version:upload` — upload a new version of an existing media file
- `media_version:view` — view version history
- `media_version:download` — download a specific historic version
- `media_version:restore` — restore a previous version as current
- `media_version:delete` — admin-only, delete a specific version

Existing `media:upload`, `media:view`, `media:download` permissions remain. Version-specific operations require version-specific permissions. Route-level `RequirePermission` middleware + handler-level ownership/scope checks.

Added to permissions manifest and synced via `permission:sync`.

### R10: Audit Logging Pattern

**Decision**: Follow existing 9-param `LogAction` pattern (`actorID, action, resource, resourceID, before, after, ipAddress, userAgent`). Resource for version operations is `media_version`. Audit actions:
- `version_upload` (resource: `media_version`, resourceID: version UUID)
- `version_download` (resource: `media_version`, resourceID: version UUID)
- `version_restore` (resource: `media_version`, resourceID: version UUID)
- `version_delete` (resource: `media_version`, resourceID: version UUID)

All audit logging is async (goroutine via `auditSvc.LogAction`) to not block response.

### R11: Organization Scoping

**Decision**: Version operations inherit organization context from the parent `Media` entity. The `X-Organization-ID` header is extracted by existing middleware. The handler checks org membership and Casbin domain-based permissions (`enforcer.Enforce(userID, orgID, "media_version", action)`) at the route level. Media belonging to an organization has its versions implicitly scoped to that organization.

**Alternatives rejected**:
- Add `organization_id` to `media_versions` directly — redundant, denormalizes, risks inconsistency

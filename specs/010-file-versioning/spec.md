# Feature Specification: File Versioning

**Feature Branch**: `010-file-versioning`
**Created**: 2026-05-08
**Status**: Draft
**Input**: User description: "File versioning feature — version history for uploaded files with diff tracking, extending the existing Media entity"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Upload New Version of a File (Priority: P1)

A user uploads a file that already exists (same media ID or same model_type/model_id/collection/filename). Instead of replacing the file, the system creates a new version of the existing media record, preserving the previous version's file on disk and database record. The current version pointer advances, and the user can later retrieve any previous version.

**Why this priority**: File versioning is useless without the core ability to upload new versions. This is the foundational capability that all other user stories depend on.

**Independent Test**: Upload a file, then upload a new version of the same file. Verify the media record's `current_version` increments, both file versions are accessible, and the newest version is returned by default.

**Acceptance Scenarios**:

1. **Given** a media record exists with ID `abc-123`, **When** the owner uploads a new version of that file, **Then** a new `MediaVersion` record is created with `version = 2`, the media record's `current_version` updates to `2`, and the original file (version 1) remains accessible.

2. **Given** a media record with `current_version = 3`, **When** a user retrieves the media by ID, **Then** the response includes the current version's file path and metadata, along with the version count.

3. **Given** a media record does not exist, **When** the first upload occurs, **Then** a `MediaVersion` record is created with `version = 1` and the media's `current_version` is set to `1`.

4. **Given** a media record with checksum matching the previous version, **When** a user uploads a "new version" with identical content, **Then** the system rejects the upload with a 409 Conflict error indicating the file content is unchanged.

---

### User Story 2 - View Version History (Priority: P1)

A user wants to see the complete version history of a media file — who uploaded each version, when, and what changed. This enables audit trails and rollback decisions.

**Why this priority**: Without visibility into version history, users cannot assess what changed or make informed decisions about rollback. This is essential alongside the upload capability.

**Independent Test**: Upload 3 versions of a file, then call the version history endpoint. Verify all 3 versions are returned in descending order with correct metadata (version number, uploader, timestamp, size, checksum).

**Acceptance Scenarios**:

1. **Given** a media record with 3 versions, **When** the user requests the version history, **Then** the response contains an ordered list (descending by version number) showing each version's ID, version number, filename, size, checksum, uploaded_by, and created_at.

2. **Given** a media record with no versions other than the original, **When** the user requests version history, **Then** the response contains a single entry for version 1.

3. **Given** a media record that has been soft-deleted, **When** a non-admin user requests the version history, **Then** the system returns 404 Not Found.

---

### User Story 3 - Download Specific Version (Priority: P2)

A user needs to download a specific version of a file (not just the current one). This is critical for rollback scenarios where the current version has issues and the user needs a previous version.

**Why this priority**: Download of specific versions extends the version history viewing into actionable recovery, but depends on upload (P1) and viewing history (P1) being in place.

**Independent Test**: Upload 2 versions, then request a download of version 1. Verify the correct file content is served and a download audit record is created.

**Acceptance Scenarios**:

1. **Given** a media record with version 1 and version 2, **When** the user requests to download version 1, **Then** the original file content is served with the correct filename.

2. **Given** a media record with version 1 at path `/uploads/v1/doc.pdf`, **When** a version 2 is uploaded at path `/uploads/v2/doc.pdf`, **Then** both paths remain valid and serve their respective file versions.

3. **Given** a version number that does not exist for a media record, **When** the user requests that version, **Then** the system returns 404 Not Found.

4. **Given** a user downloads a specific version, **When** the download completes, **Then** an audit log entry is recorded with resource `media_version`, action `download`, and the version number in the metadata.

---

### User Story 4 - Restore a Previous Version (Priority: P2)

A user wants to promote a previous version as the current version without creating a new upload. This is for quick rollback scenarios where the previous content is already on disk and just needs to be made current.

**Why this priority**: Restore is a common operation after discovering a bad upload, but it is an extension of the download-specific-version capability rather than foundational.

**Independent Test**: Upload 3 versions, then restore version 1 as current. Verify `current_version` points to version 1's data, and a new version history entry (or update) records the restoration with the restoring user.

**Acceptance Scenarios**:

1. **Given** a media record with `current_version = 3`, **When** the owner restores version 1, **Then** the media record updates its `current_version` to 1 (or creates a new version 4 that references version 1's content), and the original file data for version 1 remains preserved.

2. **Given** a user without `media:update` permission, **When** they attempt to restore a version, **Then** the system returns 403 Forbidden.

3. **Given** a media record with `current_version = 1`, **When** the owner attempts to restore version 1 (the current version), **Then** the system returns 400 Bad Request with a message indicating the version is already current.

---

### User Story 5 - Delete a Specific Version (Priority: P3)

An admin wants to delete a specific version's file from storage while keeping the version record in the database for audit purposes. The file is removed from disk, but the version metadata record is retained with a `deleted_at` marker.

**Why this priority**: Version deletion is a cleanup/management operation that is less frequently needed than core versioning features. It's important for storage management but not critical for initial deployment.

**Independent Test**: Upload 3 versions, then soft-delete version 2's file. Verify the version metadata remains with a `deleted_at` timestamp, the file is removed from disk (or marked as deleted), and versions 1 and 3 remain accessible.

**Acceptance Scenarios**:

1. **Given** a media record with 3 versions, **When** an admin soft-deletes version 2, **Then** version 2's record remains with `deleted_at` set, the file is removed from storage, and the version is excluded from normal version listing.

2. **Given** a media record with only the current version (version 1), **When** a user attempts to delete version 1, **Then** the system returns 400 Bad Request — the current version cannot be deleted; the entire media record must be deleted instead.

3. **Given** a non-admin user, **When** they attempt to delete a version, **Then** the system returns 403 Forbidden.

---

### Edge Cases

- What happens when storage upload fails mid-version-creation? → The database transaction rolls back; no orphan version record is created.
- What happens when the same user uploads two new versions concurrently? → Optimistic locking on `current_version` prevents race conditions; the second upload must reference the correct base version.
- What happens when a file exceeds the maximum allowed size during version upload? → Return 413 Payload Too Large with a clear error message; no version record created.
- What happens when checksum computation fails? → Return 500 Internal Server Error; do not create a partial version record.
- What happens when version 3 is restored and then a new upload occurs? → The new upload becomes version 4 (version numbers always increment; restoration creates a pointer to existing content, not a renumbering).
- What happens when a media record is soft-deleted? → All version records remain for audit; they are excluded from normal queries via GORM soft-delete scope.
- What happens when downloading a version whose file was removed from disk? → Return 404 Not Found with a clear message that the version's file is no longer available.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST create a `MediaVersion` record for every file upload, starting at version 1 for new media and incrementing for each subsequent version upload.
- **FR-002**: System MUST compute and store a SHA-256 checksum for each uploaded file version to detect duplicate uploads.
- **FR-003**: System MUST reject a new version upload if the file content checksum matches the current version's checksum (return 409 Conflict).
- **FR-004**: System MUST maintain a `current_version` field on the `Media` entity that tracks which version is the active/current one.
- **FR-005**: System MUST preserve all previous version files and records when a new version is uploaded, ensuring they remain independently accessible.
- **FR-006**: Users MUST be able to view version history for a media record, including version number, filename, size, checksum, uploaded_by, and created_at for each version.
- **FR-007**: Users MUST be able to download a specific version of a file by specifying the version number.
- **FR-008**: Users with `media:update` permission MUST be able to restore a previous version as the current version.
- **FR-009**: System MUST use optimistic locking on `current_version` to prevent race conditions during concurrent version uploads.
- **FR-010**: System MUST audit-log all version operations (upload, download, restore, delete) with actor, action, resource, and before/after state.
- **FR-011**: System MUST soft-delete version records when a specific version is deleted (not hard-delete), preserving the audit trail.
- **FR-012**: System MUST prevent deletion of the current (active) version; users must either promote another version first or delete the entire media record.
- **FR-013**: Version file paths MUST include the version number to prevent overwriting (e.g., `media/{uuid}/v{N}/{filename}`).
- **FR-014**: System MUST validate that the uploaded file's MIME type matches the original media's MIME type when creating a new version of an existing media record.
- **FR-015**: System MUST support organization scoping for version operations via the `X-Organization-ID` header and Casbin domain-based permissions.

### Key Entities

- **MediaVersion**: Represents a single version of a media file. Contains the version number, file path, size, SHA-256 checksum, the user who uploaded it, and timestamps. Belongs to a `Media` entity.
- **Media (extended)**: The existing media entity gains a `current_version` integer field tracking the active version number, and a `Versions` has-many relationship to `MediaVersion`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can upload a new version of an existing file and retrieve both the current and previous versions within 2 seconds.
- **SC-002**: Version history listing returns results in under 1 second for media records with up to 100 versions.
- **SC-003**: Download of any specific file version completes within the same timeframe as a regular media download (no measurable degradation).
- **SC-004**: Duplicate content uploads are correctly rejected with a 409 Conflict response in 100% of cases.
- **SC-005**: Concurrent version uploads to the same media record are handled correctly without data loss (optimistic locking prevents race conditions).
- **SC-006**: All version operations (upload, download, restore, delete) produce audit log entries with correct actor, action, and resource metadata.

## Assumptions

- **Storage layout**: Versioned files are stored at `{base_path}/{media_id}/v{version_number}/{filename}` to guarantee no file overwrites across versions. The existing non-versioned storage layout continues to work for media records created before versioning was introduced (version = 1, path unchanged).
- **Backward compatibility**: Existing media records without any `MediaVersion` rows will be treated as version 1 (the current upload). When a new version is uploaded for such media, a `MediaVersion` row is retroactively created for version 1.
- **MIME type consistency**: New versions of a media record must match the original MIME type to prevent confusing file type changes (e.g., replacing a PDF with a PNG under the same media ID).
- **Version numbering**: Version numbers are monotonically increasing integers starting at 1. Restoration of a previous version does NOT create a new version number — it simply updates the `current_version` pointer on the media record.
- **Soft delete approach**: Versions follow the project's soft-delete convention with `deleted_at`. A soft-deleted version's file is removed from storage, but the metadata record is retained for audit.
- **Organization scoping**: Version operations inherit the organization scoping from the parent media record, using Casbin domain-based permissions.
- **No binary diff**: This feature provides version tracking (who uploaded what, when), not binary diff between versions. Checksum comparison detects identical content but does not produce deltas.
- **Maximum version limit**: No hard limit on the number of versions per media record. Practical limits are governed by storage capacity.
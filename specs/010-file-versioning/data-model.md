# Data Model: File Versioning

**Feature**: 010-file-versioning | **Date**: 2026-05-08 | **Status**: Phase 1 Design

---

## Entity Relationship Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│ File Versioning System                                            │
├──────────────────────────────────────────────────────────────────┤
│                                                                    │
│  users ──1:N──► media ──1:N──► media_versions                    │
│    │               │                   │                           │
│    │          (soft delete)       (soft delete)                    │
│    │               │                   │                           │
│    │          disk: VARCHAR         version: INT                  │
│    │          path: VARCHAR         file_path: VARCHAR            │
│    │          current_version: INT  checksum: CHAR(64)            │
│    │          mime_type: VARCHAR    size: BIGINT                  │
│    │          size: BIGINT          mime_type: VARCHAR            │
│    │                                uploaded_by_id: UUID          │
│    │                                checksum: CHAR(64)            │
│                                                                    │
│  Each MediaVersion stores its own file_path + checksum.           │
│  The Media.current_version pointer determines which version       │
│  is served by default GET/download requests.                     │
│                                                                    │
│  Storage Layout: {base_path}/{media_id}/v{version}/{filename}     │
│                                                                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## Entities

### 1. media (extended)

**Purpose**: Existing media entity gains one new column `current_version` to track the active version. All existing columns remain unchanged.

**New column added via migration 000019**:
```sql
ALTER TABLE media ADD COLUMN current_version INTEGER NOT NULL DEFAULT 1;

-- Backfill: existing media implicitly have version 1
-- No data migration needed — the DEFAULT 1 handles it
```

| New Field | Type | Constraints | Notes |
|-----------|------|-------------|-------|
| `current_version` | INTEGER | NOT NULL, DEFAULT 1 | Points to the active `MediaVersion.version` for this media |

**Updated Go Domain Entity** (only new field shown):
```go
type Media struct {
    // ... existing fields unchanged ...
    CurrentVersion int                   `gorm:"default:1;not null" json:"current_version"`
    Versions       []*MediaVersion       `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE" json:"versions,omitempty"`
}
```

**Updated `MediaResponse`** (adds version info):
```go
type MediaResponse struct {
    // ... existing fields unchanged ...
    CurrentVersion int                      `json:"current_version"`
    VersionCount   int                      `json:"version_count"`
    Versions       []*MediaVersionResponse  `json:"versions,omitempty"`
}
```

**Updated `ToResponse()`**:
```go
func (m *Media) ToResponse() *MediaResponse {
    conversions := make([]*MediaConversionResponse, len(m.Conversions))
    for i, c := range m.Conversions {
        conversions[i] = c.ToResponse()
    }

    versions := make([]*MediaVersionResponse, len(m.Versions))
    for i, v := range m.Versions {
        versions[i] = v.ToResponse()
    }

    return &MediaResponse{
        // ... existing fields ...
        CurrentVersion: m.CurrentVersion,
        VersionCount:   len(m.Versions),
        Versions:       versions,
    }
}
```

---

### 2. media_versions (new table)

**Purpose**: Immutable record of each uploaded file version. Each row represents a discrete version with its own file path, checksum, and metadata. Soft-deletable for version cleanup while preserving audit trail.

```sql
-- Migration: 000019_create_media_versions.up.sql
CREATE TABLE IF NOT EXISTS media_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id        UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    filename        VARCHAR(255) NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    mime_type       VARCHAR(100) NOT NULL,
    size            BIGINT NOT NULL,
    file_path       VARCHAR(2000) NOT NULL,
    checksum        CHAR(64) NOT NULL,
    uploaded_by_id  UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at      TIMESTAMPTZ DEFAULT NULL,

    -- Constraints
    CONSTRAINT uq_media_versions_version UNIQUE (media_id, version),
    CONSTRAINT chk_media_versions_size CHECK (size > 0),
    CONSTRAINT chk_media_versions_version_positive CHECK (version > 0),
    CONSTRAINT chk_media_versions_mime_type CHECK (mime_type ~ '^[a-z]+/[a-zA-Z0-9+\-\.]+$')
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_media_versions_media_id ON media_versions(media_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_uploaded_by ON media_versions(uploaded_by_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_checksum ON media_versions(media_id, checksum) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_versions_created_at ON media_versions(created_at DESC) WHERE deleted_at IS NULL;

-- Auto-update trigger
CREATE TRIGGER update_media_versions_updated_at BEFORE UPDATE ON media_versions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Rollback**:
```sql
-- Migration: 000019_create_media_versions.down.sql
DROP TRIGGER IF EXISTS update_media_versions_updated_at ON media_versions;
DROP TABLE IF EXISTS media_versions;
ALTER TABLE media DROP COLUMN IF EXISTS current_version;
```

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| `id` | UUID | PK, `gen_random_uuid()` | Unique version record identifier |
| `media_id` | UUID | FK media, NOT NULL, CASCADE | Parent media record |
| `version` | INTEGER | NOT NULL, > 0 | Monotonically increasing version number |
| `filename` | VARCHAR(255) | NOT NULL | Storage filename (UUID+ext, same convention as media) |
| `original_filename` | VARCHAR(500) | NOT NULL | User's original filename |
| `mime_type` | VARCHAR(100) | NOT NULL | Detected MIME type |
| `size` | BIGINT | NOT NULL, > 0 | File size in bytes |
| `file_path` | VARCHAR(2000) | NOT NULL | Storage path: `{base_path}/{media_id}/v{version}/{filename}` |
| `checksum` | CHAR(64) | NOT NULL | SHA-256 hex digest (lowercase) |
| `uploaded_by_id` | UUID | FK users, NOT NULL, RESTRICT | User who uploaded this version |
| `created_at` | TIMESTAMPTZ | NOT NULL | Version creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update (rarely changes) |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete marker |

**Unique Constraint**: `(media_id, version)` — prevents duplicate version numbers for the same media record.

---

## Go Domain Entities

### MediaVersion Entity

```go
package domain

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

// MediaVersion represents a single version of a media file
type MediaVersion struct {
    ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    MediaID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"media_id"`
    Media            *Media         `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE" json:"-"`
    Version          int            `gorm:"not null" json:"version"`
    Filename         string         `gorm:"size:255;not null" json:"filename"`
    OriginalFilename string         `gorm:"size:500;not null" json:"original_filename"`
    MimeType         string         `gorm:"size:100;not null" json:"mime_type"`
    Size             int64          `gorm:"not null" json:"size"`
    FilePath         string         `gorm:"size:2000;not null" json:"file_path"`
    Checksum         string         `gorm:"size:64;not null" json:"checksum"`
    UploadedByID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"uploaded_by_id"`
    UploadedBy       *User          `gorm:"foreignKey:UploadedByID;constraint:OnDelete:RESTRICT" json:"uploaded_by,omitempty"`
    CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MediaVersion) TableName() string {
    return "media_versions"
}
```

### Business Methods

```go
// IsCurrent checks if this version is the current/active version
func (mv *MediaVersion) IsCurrent(mediaCurrentVersion int) bool {
    return mv.Version == mediaCurrentVersion
}

// IsDeleted checks if this version has been soft-deleted
func (mv *MediaVersion) IsDeleted() bool {
    return mv.DeletedAt.Valid
}
```

### Response DTOs

```go
// MediaVersionResponse represents a media version in API responses
type MediaVersionResponse struct {
    ID               uuid.UUID `json:"id"`
    Version          int       `json:"version"`
    Filename         string    `json:"filename"`
    OriginalFilename string    `json:"original_filename"`
    MimeType         string    `json:"mime_type"`
    Size             int64     `json:"size"`
    Checksum         string    `json:"checksum"`
    UploadedByID     uuid.UUID `json:"uploaded_by_id"`
    IsCurrent        bool      `json:"is_current"`
    CreatedAt        time.Time `json:"created_at"`
}

// ToResponse converts a MediaVersion to MediaVersionResponse
func (mv *MediaVersion) ToResponse(currentVersion int) *MediaVersionResponse {
    return &MediaVersionResponse{
        ID:               mv.ID,
        Version:          mv.Version,
        Filename:         mv.Filename,
        OriginalFilename: mv.OriginalFilename,
        MimeType:         mv.MimeType,
        Size:             mv.Size,
        Checksum:         mv.Checksum,
        UploadedByID:     mv.UploadedByID,
        IsCurrent:        mv.Version == currentVersion,
        CreatedAt:        mv.CreatedAt,
    }
}
```

### Version History List Response

```go
// VersionHistoryResponse represents the version history listing
type VersionHistoryResponse struct {
    MediaID         uuid.UUID               `json:"media_id"`
    CurrentVersion  int                      `json:"current_version"`
    Versions        []*MediaVersionResponse  `json:"versions"`
    Total           int                      `json:"total"`
}
```

---

## Version Lifecycle State Machine

```
                ┌──────────┐
                │  Upload  │ (creates version N+1)
                └────┬─────┘
                     │
                ┌────▼─────┐
                │  Active  │ (current_version points here)
                └────┬─────┘
                     │
          ┌──────────┼──────────┐
          │          │          │
     ┌────▼───┐ ┌───▼────┐ ┌───▼──────┐
     │Restore │ │ Upload │ │  Delete  │
     │ (stale)│ │new ver │ │(soft-del)│
     └────────┘ └────────┘ └──────────┘
```

**Transitions**:

| From | To | Action | Condition |
|------|----|--------|-----------|
| N/A | Active (v1) | First upload | Media record created |
| Active (vN) | Active (vN+1) | New version uploaded | Checksum != current checksum |
| Active (vN) | Active (vN) | Rejected | Checksum == current checksum (409) |
| Stale (vM) | Active (vM) | Restore | current_version updated to M |
| Any (non-current) | Soft-deleted | Delete version | File removed, deleted_at set |
| Active (vN) | N/A (rejected) | Delete attempt | 400: cannot delete current version |

---

## Storage Path Format

```
Versioned:   {base_path}/{media_id}/v{version_number}/{uuid_filename}.{ext}
             Example: uploads/abc-123/v1/a1b2c3d4.jpg
                      uploads/abc-123/v2/e5f6g7h8.jpg

Unversioned: {model_type}/{model_id}/{uuid_filename}.{ext}
             Example: news/def-456/x1y2z3w4.pdf
                      (media created before versioning — treated as v1)
```

**Path generation in code**:
```go
func generateVersionedPath(mediaID uuid.UUID, version int, filename string) string {
    return fmt.Sprintf("%s/v%d/%s", mediaID.String(), version, filename)
}
```

---

## Relationships Summary

```
users (1) ──< (M) media (1) ──< (M) media_versions
                                     │
                               uploaded_by_id → users (1)
```

- `Media` has many `MediaVersion` records (via `MediaID` FK, CASCADE on delete)
- `MediaVersion` references `users` via `uploaded_by_id` (RESTRICT on delete — can't delete a user who uploaded versions)

---

## Backward Compatibility Strategy

| Scenario | Behavior |
|----------|----------|
| Media created BEFORE migration 000019 | `current_version = 1` (default), no `MediaVersion` row exists |
| First version upload on pre-existing media | Retroactive `MediaVersion v1` created (from existing media fields), then v2 uploaded |
| Media created AFTER migration 000019 | All uploads create `MediaVersion` rows starting at v1 |
| GET /media/{id} on pre-existing media | Returns `current_version: 1`, `version_count: 0` (no MediaVersion rows yet) |
| Upload on media with `MediaVersion` rows | Version N+1 created, checksum compared against vN |

---

**Version**: 1.0 | **Created**: 2026-05-08 | **Status**: Phase 1 Design

# Data Model: Media Library System

**Phase**: 1 - Design  
**Date**: 2026-04-22  
**Status**: Complete

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     Polymorphic Media Model                     │
└─────────────────────────────────────────────────────────────────┘

                            ┌──────────────┐
                            │    users     │
                            └──────┬───────┘
                                   │
                        uploaded_by_id (FK)
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
    ┌───▼────────┐         ┌──────▼──────┐         ┌─────────▼──────┐
    │   media    │         │ media_      │         │ media_         │
    │            │         │ conversions │         │ downloads      │
    └────┬───────┘         └─────────────┘         └────────────────┘
         │                       │                       │
         │ (1:N)                 │ media_id (FK)         │ media_id (FK)
         │                       │                       │
    ┌────┴──────────────────────────────────────────────────┐
    │                                                        │
    │    Polymorphic Links                                  │
    │    ├─ news.id (via model_type='news', model_id)      │\n    │    ├─ users.id (via model_type='user', model_id)     │\n    │    └─ invoices.id (via model_type='invoice', model_id)│\n    │                                                        │\n    └────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ Audit Trail Integration                 │\n│ ┌───────────────────────────────────┐ │\n│ │ audit_logs (existing table)       │ │\n│ ├───────────────────────────────────┤ │\n│ │ • resource='media'                │ │\n│ │ • action='UPLOAD|DOWNLOAD|DELETE' │ │\n│ │ • resource_id=media.id            │ │\n│ └───────────────────────────────────┘ │\n└─────────────────────────────────────────┘
```

---

## Core Tables

### `media` - Main Media Registry

**Purpose**: Polymorphic media storage for any entity

| Column | Type | Constraints | Index | Purpose |
|--------|------|-------------|-------|---------|
| `id` | UUID | PK, default gen_random_uuid() | ✓ | Unique media identifier |
| `model_type` | VARCHAR(255) | NOT NULL | ✓ | Entity type ('news', 'user', 'invoice') |
| `model_id` | UUID | NOT NULL | ✓ | Foreign key to specific entity |
| `collection_name` | VARCHAR(255) | NOT NULL, default 'default' | ✓ | Grouping (e.g., 'images', 'documents') |
| `disk` | VARCHAR(50) | NOT NULL, default 'local' | ✓ | Storage backend ('local', 's3') |
| `filename` | VARCHAR(255) | NOT NULL | ✓ | UUID-based filename on disk |
| `original_filename` | VARCHAR(500) | NOT NULL | | Original uploaded filename |
| `mime_type` | VARCHAR(100) | NOT NULL | ✓ | MIME type (image/jpeg, application/pdf) |
| `size` | BIGINT | NOT NULL | ✓ | File size in bytes |
| `path` | VARCHAR(2000) | NOT NULL | | Relative path on storage backend |
| `metadata` | JSONB | | | Image: {width, height, format, colorspace} |
| `custom_properties` | JSONB | | | User-defined metadata |
| `uploaded_by_id` | UUID | NOT NULL, FK(users) | ✓ | Actor who uploaded |
| `created_at` | TIMESTAMP | NOT NULL, default CURRENT_TIMESTAMP | ✓ | Upload timestamp |
| `updated_at` | TIMESTAMP | NOT NULL, default CURRENT_TIMESTAMP | | Last modification |
| `deleted_at` | TIMESTAMP | | ✓ | Soft delete marker |
| `orphaned_at` | TIMESTAMP | | ✓ | Marked for cleanup (after model deletion) |

**Indexes**:
```sql
CREATE INDEX idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_collection ON media(collection_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_created_at ON media(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_uploaded_by ON media(uploaded_by_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_mime_type ON media(mime_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_media_orphaned ON media(orphaned_at) WHERE orphaned_at IS NOT NULL;
CREATE UNIQUE INDEX idx_media_filename ON media(disk, filename) WHERE deleted_at IS NULL;
```

**Constraints**:
```sql
ALTER TABLE media ADD CONSTRAINT check_media_size CHECK (size > 0);
ALTER TABLE media ADD CONSTRAINT check_media_mime_type CHECK (mime_type ~ '^[a-z]+/[a-z0-9\+\-\.]+$');
```

---

### `media_conversions` - Generated File Variants

**Purpose**: Store metadata for processed/generated versions (thumbnails, previews)

| Column | Type | Constraints | Index | Purpose |
|--------|------|-------------|-------|---------|
| `id` | UUID | PK, default gen_random_uuid() | ✓ | Unique conversion ID |
| `media_id` | UUID | NOT NULL, FK(media) | ✓ | Parent media |
| `name` | VARCHAR(255) | NOT NULL | ✓ | Conversion name ('thumbnail', 'preview') |
| `disk` | VARCHAR(50) | NOT NULL, default 'local' | | Storage backend |
| `path` | VARCHAR(2000) | NOT NULL | | Relative path on disk |
| `size` | BIGINT | NOT NULL | | Conversion file size |
| `metadata` | JSONB | | | Conversion-specific: {width, height, quality} |
| `created_at` | TIMESTAMP | NOT NULL, default CURRENT_TIMESTAMP | ✓ | Generation timestamp |

**Indexes**:
```sql
CREATE UNIQUE INDEX idx_media_conversions_name ON media_conversions(media_id, name);
CREATE INDEX idx_media_conversions_created ON media_conversions(created_at DESC);
```

**Cascade**: ON DELETE CASCADE (when media deleted, conversions deleted)

---

### `media_downloads` - Access Audit Log (Optional)

**Purpose**: Track who downloaded what media (for analytics/security)

| Column | Type | Constraints | Index | Purpose |
|--------|------|-------------|-------|---------|
| `id` | UUID | PK, default gen_random_uuid() | ✓ | Log entry ID |
| `media_id` | UUID | NOT NULL, FK(media) | ✓ | Downloaded media |
| `downloaded_by_id` | UUID | FK(users) | ✓ | Downloader (nullable for anonymous) |
| `downloaded_at` | TIMESTAMP | NOT NULL, default CURRENT_TIMESTAMP | ✓ | Download timestamp |
| `ip_address` | VARCHAR(45) | | ✓ | Client IP (v4/v6) |
| `user_agent` | VARCHAR(2000) | | | Browser/client identifier |

**Indexes**:
```sql
CREATE INDEX idx_downloads_media ON media_downloads(media_id, downloaded_at DESC);
CREATE INDEX idx_downloads_user ON media_downloads(downloaded_by_id, downloaded_at DESC);
```

**Note**: This table grows fast; consider retention policy (archive/delete >90 days)

---

## Domain Entities (Go Structs)

### Media Entity

```go
type Media struct {
    ID                uuid.UUID           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    ModelType         string              `gorm:"type:varchar(255);not null;index:idx_media_model"`
    ModelID           uuid.UUID           `gorm:"type:uuid;not null;index:idx_media_model"`
    CollectionName    string              `gorm:"type:varchar(255);default:'default';not null;index"`
    Disk              string              `gorm:"type:varchar(50);default:'local';not null"`
    Filename          string              `gorm:"type:varchar(255);not null;uniqueIndex:idx_media_filename"`
    OriginalFilename  string              `gorm:"type:varchar(500);not null"`
    MimeType          string              `gorm:"type:varchar(100);not null;index"`
    Size              int64               `gorm:"not null"`
    Path              string              `gorm:"type:varchar(2000);not null"`
    Metadata          datatypes.JSONMap   `gorm:"type:jsonb"`
    CustomProperties  datatypes.JSONMap   `gorm:"type:jsonb"`
    UploadedByID      uuid.UUID           `gorm:"type:uuid;not null;index"`
    UploadedBy        *User               `gorm:"foreignKey:UploadedByID;constraint:OnDelete:RESTRICT"`
    Conversions       []*MediaConversion  `gorm:"foreignKey:MediaID;constraint:OnDelete:CASCADE"`
    CreatedAt         time.Time           `gorm:"autoCreateTime"`
    UpdatedAt         time.Time           `gorm:"autoUpdateTime"`
    DeletedAt         gorm.DeletedAt      `gorm:"index"`
    OrphanedAt        *time.Time          `gorm:"index"`
}

func (Media) TableName() string { return "media" }

func (m *Media) ToResponse() *MediaResponse {
    conversions := make([]*MediaConversionResponse, len(m.Conversions))
    for i, c := range m.Conversions {
        conversions[i] = c.ToResponse()
    }
    return &MediaResponse{
        ID:               m.ID,
        ModelType:        m.ModelType,
        CollectionName:   m.CollectionName,
        Filename:         m.OriginalFilename,
        MimeType:         m.MimeType,
        Size:             m.Size,
        Conversions:      conversions,
        CustomProperties: m.CustomProperties,
        CreatedAt:        m.CreatedAt,
    }
}
```

### MediaConversion Entity

```go
type MediaConversion struct {
    ID        uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    MediaID   uuid.UUID         `gorm:"type:uuid;not null"`
    Name      string            `gorm:"type:varchar(255);not null;uniqueIndex:idx_media_conv_name,composite:idx_media_conv_name"`
    Disk      string            `gorm:"type:varchar(50);default:'local'"`
    Path      string            `gorm:"type:varchar(2000);not null"`
    Size      int64             `gorm:"not null"`
    Metadata  datatypes.JSONMap `gorm:"type:jsonb"`
    CreatedAt time.Time         `gorm:"autoCreateTime;index"`
}

func (MediaConversion) TableName() string { return "media_conversions" }
```

### Response Models

```go
type MediaResponse struct {
    ID               uuid.UUID                  `json:"id"`
    ModelType        string                     `json:"model_type"`
    CollectionName   string                     `json:"collection_name"`
    Filename         string                     `json:"filename"`
    MimeType         string                     `json:"mime_type"`
    Size             int64                      `json:"size"`
    Conversions      []*MediaConversionResponse `json:"conversions,omitempty"`
    URL              string                     `json:"url"`
    CustomProperties map[string]interface{}     `json:"custom_properties,omitempty"`
    CreatedAt        time.Time                  `json:"created_at"`
}

type MediaConversionResponse struct {
    Name     string                 `json:"name"`
    Size     int64                  `json:"size"`
    URL      string                 `json:"url"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

---

## Polymorphic Relationship Rules

### Association Pattern

```go
// Attach media to any model
news := &News{ID: newsID, ...}
media := &Media{
    ModelType: "news",
    ModelID: newsID,
    CollectionName: "images",
    ...
}

// Query media for a specific model
var mediaItems []*Media
db.Where("model_type = ? AND model_id = ?", "news", newsID).
   Find(&mediaItems)

// Eager-load media with model (GORM hooks)
type News struct {
    ID uuid.UUID
    Media []*Media `gorm:"polymorphic:Model;polymorphicValue:news"`
}
```

### Supported Model Types

| Model Type | Entity | Example |
|------------|--------|---------|
| `news` | News | `internal/domain/news.go` |
| `user` | User | Profile pictures, documents |
| `invoice` | Invoice | Receipts, attachments |
| `comment` | Comment | Embedded images |
| *(extensible)* | Custom | Any future entity |

---

## State Transitions & Constraints

### Media Lifecycle

```
┌─────────────┐
│   UPLOADED  │ (initial state)
└──────┬──────┘
       │
       ├─► ACCESSED (first download)
       │
       ├─► CONVERTED (thumbnail generated)
       │
       └─► DELETED (soft delete, orphaned_at set)
           └─► PURGED (orphaned_at < now - 30 days, hard delete)
```

### Validation Rules

| Rule | Enforcement | Error |
|------|-------------|-------|
| **File size > 0** | CHECK constraint | `check_media_size` |
| **MIME type format** | CHECK constraint + handler validation | `INVALID_MIME_TYPE` |
| **Safe filename** | Application logic | `UNSAFE_FILENAME` |
| **Model exists** | App-level (no FK to prevent new models) | `MODEL_NOT_FOUND` |
| **Permission check** | Service layer enforcer.Enforce() | `PERMISSION_DENIED` |
| **Storage quota** | Service layer pre-upload check | `QUOTA_EXCEEDED` |

---

## Metadata Examples

### Image Metadata (Stored in media.metadata)

```json
{
  "width": 1920,
  "height": 1080,
  "format": "jpeg",
  "colorspace": "sRGB",
  "dpi": 72,
  "exif": {
    "camera": "Canon EOS 5D",
    "timestamp": "2026-04-22T14:30:00Z"
  }
}
```

### Conversion Metadata (media_conversions.metadata)

```json
{
  "width": 200,
  "height": 200,
  "quality": 85,
  "filter": "lanczos"
}
```

### Custom Properties (media.custom_properties - User-Defined)

```json
{
  "alt_text": "Product photo",
  "credits": "John Doe",
  "license": "CC-BY-4.0",
  "tags": ["product", "featured"]
}
```

---

## Performance Considerations

### Indexes

- **Composite** `(model_type, model_id)`: Fast polymorphic queries
- **Collection queries**: `collection_name` for list views
- **Date range**: `created_at DESC` for pagination
- **Disk/filename**: UNIQUE constraint prevents duplicate uploads
- **Orphaned cleanup**: `orphaned_at` for daily vacuum job

### Soft Delete Scoping

All queries should exclude soft-deleted media:

```go
// GORM scope (automatic if using scopes)
db.Where("deleted_at IS NULL")

// Or use scoping function
func (m *Media) Scope() *gorm.DB {
    return db.Where("deleted_at IS NULL")
}
```

### Query Examples

```go
// List media for a news article with conversions
var media []*Media
db.Where("model_type = ? AND model_id = ?", "news", newsID).
   Preload("Conversions").
   Order("created_at DESC").
   Limit(20).
   Offset(0).
   Find(&media)

// Find image media only
db.Where("model_type = ? AND collection_name = ? AND mime_type LIKE ?", 
         "news", "images", "image/%").
   Find(&media)

// Cleanup orphaned media
db.Where("orphaned_at IS NOT NULL AND orphaned_at < ?", 
         time.Now().AddDate(0, 0, -30)).
   FindAndDelete(&Media{})
```

---

## Next Steps

1. ✅ Data model defined
2. → Create migrations (000004_media_tables.up.sql)
3. → Implement repository interfaces
4. → Define API contracts (see contracts/api-v1.md)

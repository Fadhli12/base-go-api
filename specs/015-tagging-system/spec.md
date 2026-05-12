# Feature Specification: Tagging System

**Feature ID:** 015-tagging-system
**Priority:** P2
**Complexity:** Low
**Estimated Effort:** 1 day
**Dependencies:** None
**Branch:** 015-tagging-system

---

## 1. Overview

Flexible polymorphic tagging system for entities (news, invoices, media, etc.) with tag CRUD, autocomplete, and soft delete. Tags are global labels that can be attached to any taggable entity via a join table, with slug-based URL identifiers and usage count tracking.

## 2. Goals

- CRUD for tags with slug auto-generation from name
- Polymorphic entity-tag associations: tags attach to any taggable entity (`entity_type` + `entity_id` + `tag_id`)
- Tag autocomplete endpoint for type-ahead search
- Usage count tracking (increment on attach, decrement on detach)
- RBAC enforcement: `tag:view`, `tag:create`, `tag:update`, `tag:delete`, `tag:manage`
- Audit logging for create/update/delete operations (Constitution Principle VII)
- Organization-scoped tag management
- Soft delete for tags with partial unique index (`WHERE deleted_at IS NULL`)
- SQL migrations only — no AutoMigrate (Constitution Principle V)
- Dedicated slug column with unique constraint for clean URLs

## 3. Entities

### 3.1 Tag

```go
type Tag struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name      string         `gorm:"size:100;not null;uniqueIndex:idx_tag_name_where_deleted_at_null,where:deleted_at IS NULL"`
    Slug      string         `gorm:"size:100;not null;uniqueIndex:idx_tag_slug_where_deleted_at_null,where:deleted_at IS NULL"`
    Color     string         `gorm:"size:7"`  // hex color code, e.g. "#FF5733"
    UsageCount int           `gorm:"default:0;not null"`
    CreatedAt time.Time      `gorm:"autoCreateTime"`
    UpdatedAt time.Time      `gorm:"autoUpdateTime"`
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 EntityTag (Join Table)

```go
type EntityTag struct {
    ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    EntityType string    `gorm:"size:50;not null"`  // "news", "invoice", "media"
    EntityID   uuid.UUID `gorm:"type:uuid;not null"`
    TagID      uuid.UUID `gorm:"type:uuid;not null"`
    CreatedBy  uuid.UUID `gorm:"type:uuid;not null"`
    CreatedAt  time.Time `gorm:"autoCreateTime"`
    // Composite unique: (entity_type, entity_id, tag_id) — one tag per entity
}
```

### 3.3 DTOs

```go
// Request DTOs
type CreateTagRequest struct {
    Name  string `json:"name" validate:"required,min=1,max=100"`
    Color string `json:"color" validate:"omitempty,len=7"`  // "#FF5733"
}

type UpdateTagRequest struct {
    Name  string `json:"name" validate:"required,min=1,max=100"`
    Color string `json:"color" validate:"omitempty,len=7"`
}

type AttachTagRequest struct {
    TagIDs     []uuid.UUID `json:"tag_ids" validate:"required,min=1,dive,required"`
    EntityType string      `json:"entity_type" validate:"required,oneof=news invoice media"`
    EntityID   string      `json:"entity_id" validate:"required,uuid"`
}

type DetachTagRequest struct {
    TagIDs     []uuid.UUID `json:"tag_ids" validate:"required,min=1,dive,required"`
    EntityType string      `json:"entity_type" validate:"required,oneof=news invoice media"`
    EntityID   string      `json:"entity_id" validate:"required,uuid"`
}

// Response DTOs
type TagResponse struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    Slug      string    `json:"slug"`
    Color     string    `json:"color,omitempty"`
    UsageCount int      `json:"usage_count"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type TagListResponse struct {
    Tags  []json.RawMessage `json:"tags"`
    Total int64             `json:"total"`
}

type EntityTagsResponse struct {
    EntityType string             `json:"entity_type"`
    EntityID   uuid.UUID          `json:"entity_id"`
    Tags       []TagResponse      `json:"tags"`
}

// Validation helper
var taggableTypes = map[string]bool{
    "news":    true,
    "invoice": true,
    "media":   true,
}

func IsValidTaggableType(t string) bool {
    return taggableTypes[t]
}
```

## 4. API Endpoints

### 4.1 Tag CRUD (admin-managed global tags)

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| POST | `/api/v1/tags` | Create tag | `tag:create` |
| GET | `/api/v1/tags` | List tags (paginated, sortable) | `tag:view` |
| GET | `/api/v1/tags/:id` | Get tag by ID | `tag:view` |
| GET | `/api/v1/tags/slug/:slug` | Get tag by slug | `tag:view` |
| PUT | `/api/v1/tags/:id` | Update tag | `tag:update` |
| DELETE | `/api/v1/tags/:id` | Soft-delete tag | `tag:delete` |
| GET | `/api/v1/tags/autocomplete` | Autocomplete tags by prefix | `tag:view` |

### 4.2 Entity-Tag Associations (attach/detach tags to entities)

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| POST | `/api/v1/:type/:id/tags` | Attach tags to entity | `tag:manage` |
| DELETE | `/api/v1/:type/:id/tags` | Detach tags from entity | `tag:manage` |
| GET | `/api/v1/:type/:id/tags` | List tags for entity | `tag:view` |

### 4.3 Autocomplete Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | "" | Search prefix for tag name/slug |
| `limit` | int | 20 | Max results (1-100) |

## 5. Business Logic

### 5.1 Tag Creation

- Auto-generate slug from name: lowercase, spaces→hyphens, remove special chars
- Validate color format: must match `^#[0-9A-Fa-f]{6}$` if provided
- Check slug uniqueness within non-deleted tags (partial unique index)
- Set `UsageCount = 0` on creation

### 5.2 Tag Update

- If name changes, regenerate slug (check uniqueness against non-deleted tags)
- Allow color update independently of name
- Do NOT reset usage count on update

### 5.3 Tag Soft Delete

- Set `deleted_at` timestamp
- Do NOT cascade-delete EntityTag associations (tags are soft-deleted, not permanently removed)
- Detached tags' EntityTag rows remain for audit trail; soft-deleted tags are excluded from query results via GORM scope
- Usage count is NOT reset on soft delete

### 5.4 Entity-Tag Attach

- Validate `entity_type` against `taggableTypes` map (news, invoice, media)
- Validate `entity_id` exists (optional: soft-check, don't fail if entity doesn't exist — polymorphic)
- Prevent duplicate: composite unique constraint `(entity_type, entity_id, tag_id)`
- Increment `tag.usage_count` on successful attach
- Ignore already-attached tags (idempotent)

### 5.5 Entity-Tag Detach

- Validate `entity_type` against `taggableTypes` map
- Delete the EntityTag row
- Decrement `tag.usage_count` on successful detach
- Ignore already-detached associations (idempotent)

### 5.6 Autocomplete

- Search tags by name or slug prefix (case-insensitive, `ILIKE query%`)
- Filter out soft-deleted tags
- Return results sorted alphabetically by name
- Limit results to configurable max (default 20, max 100)

### 5.7 List Tags

- Paginated results (limit/offset)
- Sortable by: name, usage_count, created_at
- Default sort: name ASC
- Filter out soft-deleted tags

## 6. Permissions

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `tag:view` | tag | view | View tags and tag associations |
| `tag:create` | tag | create | Create new tags |
| `tag:update` | tag | update | Update tag name/color |
| `tag:delete` | tag | delete | Soft-delete tags |
| `tag:manage` | tag | manage | Attach/detach tags to entities |

Permissions seeded in `cmd/api/main.go` alongside existing permissions.

## 7. Audit Logging

All mutations are audit-logged (Constitution Principle VII):

| Action | Audit Event | Details |
|--------|-------------|---------|
| Create tag | `tag.created` | Tag ID, name, slug, color |
| Update tag | `tag.updated` | Tag ID, before/after changes |
| Delete tag | `tag.deleted` | Tag ID |
| Attach tag | `tag.attached` | Tag ID, entity type, entity ID |
| Detach tag | `tag.detached` | Tag ID, entity type, entity ID |

## 8. Database Schema

```sql
-- +goose Up
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    color VARCHAR(7),  -- hex color, e.g. '#FF5733'
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Partial unique indexes (soft-delete aware)
CREATE UNIQUE INDEX idx_tags_name_active ON tags (name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_tags_slug_active ON tags (slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_tags_deleted_at ON tags (deleted_at);
CREATE INDEX idx_tags_usage_count ON tags (usage_count DESC) WHERE deleted_at IS NULL;

-- Auto-update trigger
CREATE TRIGGER update_tags_updated_at
    BEFORE UPDATE ON tags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE entity_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One tag per entity
    CONSTRAINT idx_entity_tag_unique UNIQUE (entity_type, entity_id, tag_id),
    CONSTRAINT fk_entity_tag_tag FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Indexes for entity-tag lookups
CREATE INDEX idx_entity_tags_entity ON entity_tags (entity_type, entity_id);
CREATE INDEX idx_entity_tags_tag ON entity_tags (tag_id);
CREATE INDEX idx_entity_tags_created_by ON entity_tags (created_by);
```

## 9. File Structure

| File | Purpose |
|------|---------|
| `internal/domain/tag.go` | Tag + EntityTag entities, DTOs, validation helpers |
| `internal/repository/tag.go` | TagRepository + EntityTagRepository interfaces + GORM impl |
| `internal/service/tag.go` | TagService: CRUD, attach/detach, autocomplete, RBAC, audit |
| `internal/http/handler/tag.go` | TagHandler: HTTP endpoints |
| `internal/http/request/tag.go` | CreateTagRequest, UpdateTagRequest, AttachTagRequest, DetachTagRequest |
| `internal/http/response/tag.go` | TagResponse, TagListResponse, EntityTagsResponse |
| `migrations/000023_create_tags.up.sql` | Tags + entity_tags tables |
| `migrations/000023_create_tags.down.sql` | Drop tables |
| `tests/unit/tag_service_test.go` | Unit tests |

## 10. DI Wiring

In `internal/http/server.go`:
```go
// Initialize tag repositories
tagRepo := repository.NewTagRepository(s.db)
entityTagRepo := repository.NewEntityTagRepository(s.db)

// Initialize tag service
tagService := service.NewTagService(tagRepo, entityTagRepo, s.enforcer, s.auditSvc, slog.Default())

// Initialize tag handler
tagHandler := handler.NewTagHandler(tagService, s.auditSvc, s.enforcer)

// Register tag routes
s.RegisterTagRoutes(v1, tagHandler)
```

In `cmd/api/main.go` (permission seeding):
```go
{Name: "tag:view", Description: "View tags and tag associations", Resource: "tag", Action: "view"},
{Name: "tag:create", Description: "Create new tags", Resource: "tag", Action: "create"},
{Name: "tag:update", Description: "Update tag name and color", Resource: "tag", Action: "update"},
{Name: "tag:delete", Description: "Soft-delete tags", Resource: "tag", Action: "delete"},
{Name: "tag:manage", Description: "Attach and detach tags to entities", Resource: "tag", Action: "manage"},
```

## 11. Route Registration

```go
func (s *Server) RegisterTagRoutes(api *echo.Group, tagHandler *handler.TagHandler) {
    tags := api.Group("/tags")
    tags.Use(middleware.JWT(middleware.JWTConfig{
        SigningKey: s.jwtSecret,
    }))
    tags.Use(middleware.ExtractOrganizationID())

    // Tag CRUD (authenticated)
    tags.POST("", tagHandler.Create)
    tags.GET("", tagHandler.List)
    tags.GET("/autocomplete", tagHandler.Autocomplete)
    tags.GET("/slug/:slug", tagHandler.GetBySlug)
    tags.GET("/:id", tagHandler.GetByID)
    tags.PUT("/:id", tagHandler.Update)
    tags.DELETE("/:id", tagHandler.Delete)

    // Entity-tag associations (requires manage permission)
    entityTags := api.Group("")
    entityTags.Use(middleware.JWT(middleware.JWTConfig{
        SigningKey: s.jwtSecret,
    }))
    entityTags.Use(middleware.ExtractOrganizationID())

    entityTags.POST("/:type/:id/tags", tagHandler.AttachTags)
    entityTags.DELETE("/:type/:id/tags", tagHandler.DetachTags)
    entityTags.GET("/:type/:id/tags", tagHandler.ListEntityTags)
}
```

## 12. Edge Cases

- **Duplicate slug on soft-deleted tag**: Partial unique index handles this — `WHERE deleted_at IS NULL` allows reusing slugs of soft-deleted tags
- **Attach already-attached tag**: Idempotent — silently ignore, don't increment usage count
- **Detach already-detached tag**: Idempotent — silently ignore, don't decrement usage count
- **Attach tag to invalid entity type**: Return 422 with `VALIDATION_ERROR` and list of valid types
- **Usage count goes negative**: Guard with `MAX(usage_count - 1, 0)` in decrement query
- **Autocomplete with empty query**: Return most popular tags (sorted by `usage_count DESC`)
- **Tag deletion with existing associations**: Soft delete the tag; EntityTag rows cascade-delete via FK (or remain orphaned, excluded from queries by GORM scope)
- **Slug generation conflicts**: Append numeric suffix (e.g., `my-tag-2`, `my-tag-3`) until unique
- **Color validation**: Must match `^#[0-9A-Fa-f]{6}$` or be empty/null

## 13. Conformance Checklist (Constitution)

- [x] **Principle V**: SQL migrations only, no AutoMigrate
- [x] **Principle VII**: All mutations audit-logged (create, update, delete, attach, detach)
- [x] **Principle III**: Soft delete with `deleted_at`, partial unique indexes
- [x] **Principle II**: RBAC enforcement via Casbin (5 permissions)
- [x] **Principle I**: Context propagation in all repo/service methods
- [x] **UUID primary keys**: All entities use `gen_random_uuid()`
- [x] **Handler-level permission checks**: Following existing pattern (not service-level Enforce)

## 14. Out of Scope

- Organization-scoped tags (future iteration — current implementation is global)
- Tag hierarchies / parent-child tags
- Tag merging / aliasing
- Tag-based search filtering (separate from existing search feature)
- Batch attach/detach across multiple entities at once
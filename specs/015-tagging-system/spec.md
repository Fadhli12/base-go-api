# Feature Specification: Tagging System

**Feature ID:** 015-tagging-system
**Priority:** P2
**Complexity:** Low-Medium (organization scoping adds moderate complexity)
**Estimated Effort:** 1.5 days
**Dependencies:** None
**Branch:** 015-tagging-system

---

## Clarification Decisions (2026-05-12)

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Tag scoping model | **Organization-scoped** | Tags belong to organizations. Each org manages its own taxonomy. Follows existing org-scoped pattern (comments, webhooks). |
| 2 | Soft-delete cascade behavior | **Soft-delete tag, keep EntityTags** | ON DELETE CASCADE only fires on hard delete. Soft-deleted tags remain queryable for audit. EntityTag rows persist with GORM scope filtering them out of normal queries. |
| 3 | Bulk attach/detach | **Bulk operations** | Accept arrays of tag IDs. More efficient for UIs that multi-tag entities. Matches spec DTO design. |
| 4 | Name reuse after soft-delete | **Allow reuse** | Partial unique index (`WHERE deleted_at IS NULL`) allows new tags with same name. Old record preserved for audit. |
| 5 | Entity tag listing detail | **Full tag details** | Return complete `TagResponse` objects when listing tags for an entity. Avoids N+1 lookups. |

---

## 1. Overview

Organization-scoped polymorphic tagging system for entities (news, invoices, media, etc.) with tag CRUD, autocomplete, bulk attach/detach, and soft delete. Tags belong to organizations and can be attached to any taggable entity via a join table, with slug-based URL identifiers and usage count tracking.

## 2. Goals

- CRUD for tags with slug auto-generation from name
- **Organization-scoped**: Tags belong to organizations; each org manages its own tag taxonomy
- Polymorphic entity-tag associations: tags attach to any taggable entity (`entity_type` + `entity_id` + `tag_id`)
- Tag autocomplete endpoint for type-ahead search (within org scope)
- Usage count tracking (increment on attach, decrement on detach)
- Bulk attach/detach: accept arrays of tag IDs for efficient multi-tagging
- RBAC enforcement: `tag:view`, `tag:create`, `tag:update`, `tag:delete`, `tag:manage`
- Audit logging for create/update/delete/attach/detach operations (Constitution Principle VII)
- Soft delete for tags with partial unique index (`WHERE deleted_at IS NULL`)
- Name reuse allowed: soft-deleted tag names can be reused by new tags in the same org
- Entity tag listing returns full tag details (not just references)
- SQL migrations only — no AutoMigrate (Constitution Principle V)
- Dedicated slug column with org-scoped unique constraint for clean URLs

## 3. Entities

### 3.1 Tag

```go
type Tag struct {
    ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
    Name           string         `gorm:"size:100;not null"`
    Slug           string         `gorm:"size:100;not null"`
    Color          string         `gorm:"size:7"`  // hex color code, e.g. "#FF5733"
    UsageCount     int            `gorm:"default:0;not null"`
    CreatedAt      time.Time      `gorm:"autoCreateTime"`
    UpdatedAt      time.Time      `gorm:"autoUpdateTime"`
    DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (Tag) TableName() string { return "tags" }

func (t *Tag) ToResponse() TagResponse {
    return TagResponse{
        ID:             t.ID,
        OrganizationID: t.OrganizationID,
        Name:           t.Name,
        Slug:           t.Slug,
        Color:          t.Color,
        UsageCount:     t.UsageCount,
        CreatedAt:      t.CreatedAt,
        UpdatedAt:      t.UpdatedAt,
    }
}

// GenerateSlug creates a URL-safe slug from the tag name.
// Lowercase, spaces→hyphens, remove special chars.
func GenerateSlug(name string) string {
    slug := strings.ToLower(strings.TrimSpace(name))
    slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
    slug = strings.Trim(slug, "-")
    return slug
}

// IsValidTaggableType checks whether a given type is registered as taggable.
var taggableTypes = map[string]bool{
    "news":    true,
    "invoice": true,
    "media":   true,
}

func IsValidTaggableType(t string) bool {
    return taggableTypes[t]
}
```

### 3.2 EntityTag (Join Table)

```go
type EntityTag struct {
    ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    EntityType     string    `gorm:"size:50;not null"`  // "news", "invoice", "media"
    EntityID       uuid.UUID `gorm:"type:uuid;not null"`
    TagID          uuid.UUID `gorm:"type:uuid;not null"`
    OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
    CreatedBy      uuid.UUID `gorm:"type:uuid;not null"`
    CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (EntityTag) TableName() string { return "entity_tags" }
```

### 3.3 DTOs

```go
// --- Request DTOs ---

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

func (r *AttachTagRequest) Validate() error {
    if err := validate.Struct(r); err != nil {
        return err
    }
    if _, err := uuid.Parse(r.EntityID); err != nil {
        return apperrors.NewAppError("VALIDATION_ERROR", "entity_id must be a valid UUID", 422)
    }
    if !domain.IsValidTaggableType(r.EntityType) {
        return apperrors.NewAppError("VALIDATION_ERROR", "invalid entity_type: must be one of news, invoice, media", 422)
    }
    return nil
}

func (r *DetachTagRequest) Validate() error {
    if err := validate.Struct(r); err != nil {
        return err
    }
    if _, err := uuid.Parse(r.EntityID); err != nil {
        return apperrors.NewAppError("VALIDATION_ERROR", "entity_id must be a valid UUID", 422)
    }
    if !domain.IsValidTaggableType(r.EntityType) {
        return apperrors.NewAppError("VALIDATION_ERROR", "invalid entity_type: must be one of news, invoice, media", 422)
    }
    return nil
}

// --- Response DTOs ---

type TagResponse struct {
    ID             uuid.UUID `json:"id"`
    OrganizationID uuid.UUID `json:"organization_id"`
    Name           string    `json:"name"`
    Slug           string    `json:"slug"`
    Color          string    `json:"color,omitempty"`
    UsageCount     int       `json:"usage_count"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type TagListResponse struct {
    Tags  []json.RawMessage `json:"tags"`
    Total int64             `json:"total"`
}

type EntityTagsResponse struct {
    EntityType string         `json:"entity_type"`
    EntityID   uuid.UUID      `json:"entity_id"`
    Tags       []TagResponse  `json:"tags"`
}

type AutocompleteResponse struct {
    Tags []TagResponse `json:"tags"`
}
```

## 4. API Endpoints

### 4.1 Tag CRUD (within organization context)

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| POST | `/api/v1/tags` | Create tag | `tag:create` |
| GET | `/api/v1/tags` | List tags (paginated, org-scoped) | `tag:view` |
| GET | `/api/v1/tags/:id` | Get tag by ID | `tag:view` |
| GET | `/api/v1/tags/slug/:slug` | Get tag by slug | `tag:view` |
| PUT | `/api/v1/tags/:id` | Update tag | `tag:update` |
| DELETE | `/api/v1/tags/:id` | Soft-delete tag | `tag:delete` |
| GET | `/api/v1/tags/autocomplete` | Autocomplete tags by prefix | `tag:view` |

### 4.2 Entity-Tag Associations (attach/detach tags to entities)

| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| POST | `/api/v1/:type/:id/tags` | Bulk attach tags to entity | `tag:manage` |
| DELETE | `/api/v1/:type/:id/tags` | Bulk detach tags from entity | `tag:manage` |
| GET | `/api/v1/:type/:id/tags` | List tags for entity (full details) | `tag:view` |

### 4.3 Query Parameters

**List Tags** (`GET /api/v1/tags`):
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 20 | Results per page (max 100) |
| `offset` | int | 0 | Pagination offset |
| `sort` | string | "name" | Sort field: name, usage_count, created_at |
| `order` | string | "asc" | Sort order: asc, desc |

**Autocomplete** (`GET /api/v1/tags/autocomplete`):
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | "" | Search prefix for tag name/slug |
| `limit` | int | 20 | Max results (1-100) |

## 5. Business Logic

### 5.1 Tag Creation

- Extract `organization_id` from request context (X-Organization-ID header via middleware)
- Auto-generate slug from name: lowercase, spaces→hyphens, remove special chars
- If slug conflicts within org (including soft-deleted tags with same slug), append numeric suffix (`my-tag-2`, `my-tag-3`)
- Validate color format: must match `^#[0-9A-Fa-f]{6}$` if provided, or empty/null
- Set `UsageCount = 0` on creation
- Audit log: `tag.created`

### 5.2 Tag Update

- If name changes, regenerate slug (check org-scoped uniqueness)
- Allow color update independently of name
- Do NOT reset usage count on update
- Only allow updates to tags within the same organization
- Audit log: `tag.updated`

### 5.3 Tag Soft Delete

- Set `deleted_at` timestamp
- Soft-deleted tags are excluded from normal queries via GORM scope
- Name/slug can be reused by a new tag in the same org (partial unique index allows this)
- Usage count is NOT reset on soft delete
- EntityTag rows persist for audit trail; soft-deleted tags are filtered from association queries
- Audit log: `tag.deleted`

### 5.4 Entity-Tag Bulk Attach

- Validate `entity_type` against `taggableTypes` map (news, invoice, media)
- All tag IDs must belong to the same organization
- Validate `entity_id` format (UUID)
- Prevent duplicate: composite unique constraint `(organization_id, entity_type, entity_id, tag_id)`
- For each tag in the request:
  - Skip already-attached tags (idempotent — no error, no usage count change)
  - For newly attached tags: create EntityTag row, increment `tag.usage_count`
- Audit log: `tag.attached` (with entity_type, entity_id, tag_ids)

### 5.5 Entity-Tag Bulk Detach

- Validate `entity_type` against `taggableTypes` map
- For each tag in the request:
  - Delete the EntityTag row if it exists
  - Decrement `tag.usage_count` (with `MAX(usage_count - 1, 0)` guard)
  - Skip already-detached associations (idempotent — no error)
- Audit log: `tag.detached` (with entity_type, entity_id, tag_ids)

### 5.6 Autocomplete

- Search tags by name or slug prefix (case-insensitive, `ILIKE query%`)
- Scoped to the requesting user's organization
- Filter out soft-deleted tags
- Return results sorted by name ASC
- If query is empty, return most popular tags (sorted by `usage_count DESC`)
- Limit results to configurable max (default 20, max 100)

### 5.7 List Tags

- Scoped to the requesting user's organization
- Paginated results (limit/offset)
- Sortable by: name, usage_count, created_at
- Default sort: name ASC
- Filter out soft-deleted tags

### 5.8 Get Tag by Slug

- Lookup by slug within organization context
- Exclude soft-deleted tags
- Returns 404 if not found or tag is soft-deleted

## 6. Permissions

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `tag:view` | tag | view | View tags and tag associations |
| `tag:create` | tag | create | Create new tags |
| `tag:update` | tag | update | Update tag name/color |
| `tag:delete` | tag | delete | Soft-delete tags |
| `tag:manage` | tag | manage | Attach/detach tags to entities |

Permissions seeded in `cmd/api/main.go` alongside existing permissions.

RBAC enforcement via Casbin: `enforcer.Enforce(userID, orgDomain, "tag", action)`

## 7. Audit Logging

All mutations are audit-logged (Constitution Principle VII), using the 9-parameter `LogAction` signature:

| Action | Audit Event | Details |
|--------|-------------|---------|
| Create tag | `tag.created` | Tag ID, name, slug, color, org ID |
| Update tag | `tag.updated` | Tag ID, before/after changes |
| Delete tag | `tag.deleted` | Tag ID |
| Attach tags | `tag.attached` | Entity type, entity ID, tag IDs |
| Detach tags | `tag.detached` | Entity type, entity ID, tag IDs |

## 8. Database Schema

```sql
-- +goose Up
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    color VARCHAR(7),  -- hex color, e.g. '#FF5733'
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Org-scoped partial unique indexes (soft-delete aware)
CREATE UNIQUE INDEX idx_tags_name_org_active ON tags (organization_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_tags_slug_org_active ON tags (organization_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_tags_organization_id ON tags (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tags_deleted_at ON tags (deleted_at);
CREATE INDEX idx_tags_usage_count ON tags (usage_count DESC) WHERE deleted_at IS NULL;

-- Foreign key to organizations
ALTER TABLE tags ADD CONSTRAINT fk_tags_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- Auto-update trigger
CREATE TRIGGER update_tags_updated_at
    BEFORE UPDATE ON tags
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE entity_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One tag per entity within an org
    CONSTRAINT idx_entity_tag_unique UNIQUE (organization_id, entity_type, entity_id, tag_id)
);

-- Indexes for entity-tag lookups
CREATE INDEX idx_entity_tags_entity ON entity_tags (entity_type, entity_id);
CREATE INDEX idx_entity_tags_tag ON entity_tags (tag_id);
CREATE INDEX idx_entity_tags_organization ON entity_tags (organization_id);
CREATE INDEX idx_entity_tags_created_by ON entity_tags (created_by);

-- Foreign key: tag must exist (cascade on hard delete)
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tag_tag FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE;
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tag_organization FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
ALTER TABLE entity_tags ADD CONSTRAINT fk_entity_tag_created_by FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
DROP TABLE IF EXISTS entity_tags;
DROP TABLE IF EXISTS tags;
```

## 9. File Structure

| File | Purpose |
|------|---------|
| `internal/domain/tag.go` | Tag + EntityTag entities, DTOs, validation helpers (TaggableTypes, GenerateSlug) |
| `internal/repository/tag.go` | TagRepository + EntityTagRepository interfaces + GORM impl |
| `internal/service/tag.go` | TagService: CRUD, attach/detach, autocomplete, RBAC, audit |
| `internal/http/handler/tag.go` | TagHandler: HTTP endpoints with Echo |
| `internal/http/request/tag.go` | CreateTagRequest, UpdateTagRequest, AttachTagRequest, DetachTagRequest |
| `internal/http/response/tag.go` | TagResponse, TagListResponse, EntityTagsResponse, AutocompleteResponse |
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

    // Tag CRUD (authenticated + org-scoped)
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
    entityTags.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
        AuditService: s.auditSvc,
    }))

    entityTags.POST("/:type/:id/tags", tagHandler.AttachTags)
    entityTags.DELETE("/:type/:id/tags", tagHandler.DetachTags)
    entityTags.GET("/:type/:id/tags", tagHandler.ListEntityTags)
}
```

**Note:** `/autocomplete` route MUST be registered before `/:id` to prevent Echo from matching "autocomplete" as an ID parameter.

## 12. Edge Cases

- **Duplicate slug on soft-deleted tag**: Partial unique index `WHERE deleted_at IS NULL` allows reusing slugs/names of soft-deleted tags in the same org.
- **Slug conflict within org**: Append numeric suffix (e.g., `my-tag-2`, `my-tag-3`) until unique within the org.
- **Attach already-attached tag**: Idempotent — silently skip, don't increment usage count.
- **Detach already-detached tag**: Idempotent — silently skip, don't decrement usage count.
- **Attach tag to invalid entity type**: Return 422 with `VALIDATION_ERROR` and list of valid types (news, invoice, media).
- **Usage count goes negative**: Guard with `MAX(usage_count - 1, 0)` in decrement query.
- **Autocomplete with empty query**: Return most popular tags within org (sorted by `usage_count DESC`).
- **Tag deletion with existing associations**: Soft delete the tag; EntityTag rows persist but tag is filtered from queries by GORM soft-delete scope. Hard delete cascades EntityTags.
- **Color validation**: Must match `^#[0-9A-Fa-f]{6}$` or be empty/null.
- **Organization context required**: All tag operations require X-Organization-ID header. Return 400 if missing.
- **Cross-org tag access**: Tags from one org are not visible to another org's queries.

## 13. Conformance Checklist (Constitution)

- [x] **Principle V**: SQL migrations only, no AutoMigrate
- [x] **Principle VII**: All mutations audit-logged (create, update, delete, attach, detach)
- [x] **Principle III**: Soft delete with `deleted_at`, partial unique indexes
- [x] **Principle II**: RBAC enforcement via Casbin (5 permissions)
- [x] **Principle I**: Context propagation in all repo/service methods
- [x] **Principle VIII**: Organization-scoped — all tags and EntityTags have `organization_id`
- [x] **UUID primary keys**: All entities use `gen_random_uuid()`
- [x] **Handler-level permission checks**: Following existing pattern (not service-level Enforce)
- [x] **Audit Log**: Uses 9-parameter `LogAction` signature matching existing pattern

## 14. Out of Scope

- Global tags shared across organizations (current: org-scoped only)
- Tag hierarchies / parent-child tags
- Tag merging / aliasing
- Tag-based search filtering (separate from existing search feature)
- Batch attach/detach across multiple entities at once (current: one entity, multiple tags)
- Tag groups or categories
- Tag color auto-suggestion
# Tasks: Tagging System

**Input**: Design documents from `/specs/015-tagging-system/`
**Prerequisites**: plan.md ✅, spec.md ✅ (Momus-reviewed)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

---

## Phase 1: Database & Domain (Blocking Prerequisites)

**Purpose**: Migration, domain entities, and validation helpers — all other tasks depend on these.

- [ ] T001 Create migration `migrations/000023_create_tags.up.sql` with:
  - `tags` table (id, organization_id, name, slug, color, usage_count, created_at, updated_at, deleted_at)
  - `entity_tags` table (id, entity_type, entity_id, tag_id, organization_id, created_by, created_at)
  - Partial unique indexes: `idx_tags_name_org_active`, `idx_tags_slug_org_active` (WHERE deleted_at IS NULL)
  - Standard indexes: `idx_tags_organization_id`, `idx_tags_deleted_at`, `idx_tags_usage_count`
  - Composite unique constraint on entity_tags: `(organization_id, entity_type, entity_id, tag_id)`
  - Foreign keys: tags→organizations, entity_tags→tags (ON DELETE CASCADE), entity_tags→organizations, entity_tags→users
  - `update_tags_updated_at` trigger
  - Down migration: `migrations/000023_create_tags.down.sql` (DROP TABLE IF EXISTS entity_tags; tags)

- [ ] T002 [P] Create `internal/domain/tag.go` with:
  - `Tag` struct (ID, OrganizationID, Name, Slug, Color, UsageCount, CreatedAt, UpdatedAt, DeletedAt)
  - `EntityTag` struct (ID, EntityType, EntityID, TagID, OrganizationID, CreatedBy, CreatedAt)
  - `TableName()` methods for both
  - `Tag.ToResponse()` method
  - `GenerateSlug(name string) string` function (lowercase, regex `[^a-z0-9]+` → `-`, trim hyphens, fallback to `tag-{uuid_prefix}`)
  - `IsValidTaggableType(t string) bool` map (news, invoice, media)
  - Response DTOs: `TagResponse`, `TagListResponse`, `EntityTagsResponse`, `AutocompleteResponse`
  - Bulk result DTOs: `BulkAttachResult`, `BulkDetachResult`, `BulkError`

- [ ] T003 [P] Create `internal/http/request/tag.go` with:
  - `CreateTagRequest` (Name, Color) with `Validate()` method
  - `UpdateTagRequest` (Name, Color) with `Validate()` method
  - `AttachTagRequest` (TagIDs []uuid.UUID, EntityType, EntityID string) with `Validate()` method (entity_type validation, entity_id UUID parse)
  - `DetachTagRequest` (TagIDs []uuid.UUID, EntityType, EntityID string) with `Validate()` method

- [ ] T004 [P] Create `internal/http/response/tag.go` with:
  - `TagResponse`, `TagListResponse`, `EntityTagsResponse`, `AutocompleteResponse` structs

---

## Phase 2: Repository Layer

**Purpose**: Data access interfaces and GORM implementations for Tag and EntityTag.

- [ ] T005 Create `internal/repository/tag.go` with:
  - `TagRepository` interface: Create, FindByID, FindBySlug, FindByOrg (paginated), Update, SoftDelete, IncrementUsageCount, DecrementUsageCount
  - `EntityTagRepository` interface: Create, FindByEntity, FindByTag, DeleteByEntityAndTag, FindByEntityAndTag (for idempotency check), DeleteByEntityAndTags (bulk)
  - GORM implementations for both interfaces
  - All methods use `r.db.WithContext(ctx)` pattern
  - All queries scoped by `organization_id`
  - `FindByOrg`: pagination with limit/offset + total count, sort by name/usage_count/created_at
  - `FindBySlug`: scoped by org and `deleted_at IS NULL`
  - `IncrementUsageCount`: `UPDATE tags SET usage_count = usage_count + 1 WHERE id = ?`
  - `DecrementUsageCount`: `UPDATE tags SET usage_count = usage_count - 1 WHERE id = ? AND usage_count > 0`

---

## Phase 3: Service Layer (Business Logic)

**Purpose**: TagService with RBAC, audit logging, and all business rules.

- [ ] T006 Create `internal/service/tag.go` with:
  - `TagService` struct (tagRepo, entityTagRepo, enforcer, auditSvc, logger)
  - `NewTagService()` constructor
  - `resolveTagOrgDomain(hasOrgID bool, orgID uuid.UUID) string` — matches comment system pattern
  - `Create(ctx, orgID, userID, req)` — validate name, generate slug (with conflict retry + fallback), RBAC check, save, audit `tag.created`
  - `GetByID(ctx, orgID, id)` — RBAC check `tag:view`, fetch, return
  - `GetBySlug(ctx, orgID, slug)` — RBAC check `tag:view`, fetch by slug within org
  - `List(ctx, orgID, limit, offset, sort, order)` — RBAC check `tag:view`, paginated list
  - `Update(ctx, orgID, userID, id, req)` — RBAC check `tag:update`, if name changed regenerate slug, audit `tag.updated`
  - `Delete(ctx, orgID, userID, id)` — RBAC check `tag:delete`, soft delete, audit `tag.deleted`
  - `Autocomplete(ctx, orgID, query, limit)` — RBAC check `tag:view`, ILIKE search by name/slug, empty query = most popular
  - `AttachTags(ctx, orgID, userID, entityType, entityID, tagIDs)` — RBAC check `tag:manage`, validate taggable type, reject soft-deleted tags, idempotent skip already-attached, increment usage counts, return `BulkAttachResult`, audit `tag.attached`
  - `DetachTags(ctx, orgID, userID, entityType, entityID, tagIDs)` — RBAC check `tag:manage`, idempotent skip already-detached, decrement usage counts, return `BulkDetachResult`, audit `tag.detached`
  - `ListEntityTags(ctx, orgID, entityType, entityID)` — RBAC check `tag:view`, JOIN tags WHERE deleted_at IS NULL, return full `TagResponse` list
  - Slug conflict resolution: loop with numeric suffix (my-tag-2, my-tag-3) up to 10 attempts, then fail
  - Empty slug fallback: `tag-{first 8 chars of UUID}`

---

## Phase 4: Handler & Routes

**Purpose**: Echo HTTP handlers, route registration, and DI wiring.

- [ ] T007 Create `internal/http/handler/tag.go` with:
  - `TagHandler` struct (tagService, auditSvc, enforcer)
  - `NewTagHandler()` constructor
  - `Create(c)` — parse request, extract orgID, call service
  - `List(c)` — parse pagination params (limit, offset, sort, order), call service
  - `GetByID(c)` — parse ID param, extract orgID, call service
  - `GetBySlug(c)` — parse slug param, extract orgID, call service
  - `Update(c)` — parse ID + request, extract orgID + userID, call service
  - `Delete(c)` — parse ID, extract orgID + userID, call service
  - `Autocomplete(c)` — parse query + limit params, extract orgID, call service
  - `AttachTags(c)` — parse :type, :id, request body, extract orgID + userID, call service
  - `DetachTags(c)` — parse :type, :id, request body, extract orgID + userID, call service
  - `ListEntityTags(c)` — parse :type, :id, extract orgID, call service
  - Use `ParsePagination` helper for List endpoint
  - Use `response.Success`, `response.ErrorWithContext` for response formatting

- [ ] T008 Update `internal/http/server.go`:
  - Add `RegisterTagRoutes(api *echo.Group, tagHandler *handler.TagHandler)` method
  - Wire tag routes: `/tags` group with JWT + ExtractOrganizationID middleware
  - **CRITICAL**: Register `/autocomplete` and `/slug/:slug` routes BEFORE `/:id` (Echo route precedence)
  - Register entity-tag association routes under `/:type/:id/tags`
  - Add Audit middleware on mutation routes (POST, PUT, DELETE, not GET)
  - Add DI wiring in `NewServer()`: tagRepo → entityTagRepo → tagService → tagHandler

- [ ] T009 Update `cmd/api/main.go`:
  - Add 5 tag permissions to the seed data: `tag:view`, `tag:create`, `tag:update`, `tag:delete`, `tag:manage`
  - Add tag permissions to admin role permissions list

---

## Phase 5: Testing

**Purpose**: Unit tests for service layer business logic.

- [ ] T010 Create `tests/unit/tag_service_test.go` with test cases for:
  - CreateTag: success, slug generation, slug conflict resolution (numeric suffix), empty slug fallback, color validation, duplicate name within org
  - GetByID: success, not found, soft-deleted returns 404
  - GetBySlug: success, not found
  - List: pagination, sorting (name, usage_count, created_at), org-scoped filtering
  - Update: name change regenerates slug, color update, slug conflict handling
  - Delete: soft delete sets deleted_at, name reusable after soft delete
  - Autocomplete: prefix search, empty query returns popular, org-scoped
  - AttachTags: success with bulk IDs, skip already-attached (idempotent), reject soft-deleted tags, invalid entity type, BulkAttachResult response
  - DetachTags: success with bulk IDs, skip already-detached (idempotent), decrement usage count with MAX(count-1, 0) guard, BulkDetachResult response
  - ListEntityTags: returns full tag details, excludes soft-deleted tags from results
  - RBAC: all methods check enforcer.Enforce with correct (userID, orgDomain, "tag", action)

---

## Phase 6: Integration & Verification

**Purpose**: Build verification, lint, and final validation.

- [ ] T011 Run `go build ./...` and fix any compilation errors
- [ ] T012 Run `golangci-lint run ./...` and fix any lint errors
- [ ] T013 Run `go test -v -race ./tests/unit/... -run Tag` and ensure all tests pass
- [ ] T014 Verify route registration order: `/tags/autocomplete` before `/tags/:id`

---

## Dependency Graph

```
T001 (migration) ─┐
T002 (domain)    ─┤
T003 (request)   ─┤── T005 (repository) ── T006 (service) ── T007 (handler) ── T008 (server.go) ── T009 (main.go) ── T010 (tests) ── T011-T014 (verify)
T004 (response)  ─┘
```

Tasks T002, T003, T004 can run in parallel (no dependencies between them).
All other tasks are sequential (each depends on the previous).

---

## Estimated Effort

| Task | Estimated Time |
|------|---------------|
| T001 Migration | 30 min |
| T002 Domain entities + DTOs | 45 min |
| T003 Request DTOs | 20 min |
| T004 Response DTOs | 15 min |
| T005 Repository layer | 45 min |
| T006 Service layer (core) | 1.5 hr |
| T007 Handler layer | 45 min |
| T008 Server.go DI + routes | 30 min |
| T009 Main.go permissions | 15 min |
| T010 Unit tests | 1 hr |
| T011-T014 Build & verify | 30 min |
| **Total** | **~5.5 hr (~1 day)** |
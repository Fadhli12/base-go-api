# Implementation Plan: Activity Feed / Timeline

**Branch**: `016-activity-feed` | **Date**: 2026-05-12 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/016-activity-feed/spec.md`

## Summary

Implement a chronological activity feed/timeline system that records domain events from the existing EventBus, provides org-scoped feed endpoints with per-user read tracking and resource following, and auto-archives activities older than 90 days via a background reaper goroutine. Follows existing codebase patterns (comment, feature_flag) for domain entities, repositories, services, handlers, and DI wiring.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Echo v4, GORM, Casbin, PostgreSQL, Redis
**Storage**: PostgreSQL (activities, activity_reads, activity_follows tables) + Redis (caching, not required for activity feed)
**Testing**: Go standard testing + testify + testcontainers-go for integration tests
**Target Platform**: Linux server (Docker)
**Project Type**: REST API (web service)
**Performance Goals**: <500ms p95 for first page of activity feed
**Constraints**: Must integrate with existing EventBus without modifying existing services; 90-day retention; per-user read tracking (not on Activity entity)
**Scale/Scope**: Multi-tenant (organization-scoped), up to 100 activities per page, soft-delete retention compliance

## Constitution Check

*GATE: Must pass before Phase 0 research.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready | ✅ PASS | Logging via `internal/logger`, graceful shutdown for reaper, error handling wrapped |
| II. RBAC Mandatory | ✅ PASS | `activity:view` and `activity:manage` permissions via Casbin |
| III. Soft Deletes | ✅ PASS | Activity uses `deleted_at` for soft delete (also used for 90-day archival) |
| IV. Stateless JWT | ✅ PASS | User ID from JWT context, org ID from X-Organization-ID middleware |
| V. Versioned Migrations | ✅ PASS | SQL migration files, no AutoMigrate |
| VI. Multi-Instance Consistency | ✅ N/A | Permission cache invalidation handled by existing enforcer |
| VII. Audit Logging | ✅ PASS | Mark as read and follow/unfollow mutations logged via AuditService |
| VIII. Org-Scoped Multi-Tenancy | ✅ PASS | Activities scoped by organization_id, read tracking per-user per-org |
| IX. Event-Driven Architecture | ✅ PASS | Subscribes to existing EventBus via SetEventBus setter pattern |
| X. Background Processing | ✅ PASS | Reaper goroutine for 90-day archival, not inline in handlers |

No violations. All principles satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/016-activity-feed/
├── plan.md              # This file
├── spec.md              # Feature specification (completed)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── api-v1.md        # REST API contracts
│   └── event-bus.md     # EventBus integration contract
└── checklists/
    └── requirements.md  # Spec quality checklist (completed)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── activity.go              # Activity, ActivityRead, ActivityFollow entities + DTOs
│   └── activity_events.go       # ActivityEventType constants (optional, or in activity.go)
├── repository/
│   ├── activity.go              # ActivityRepository + ActivityReadRepository + ActivityFollowRepository interfaces
│   └── activity_repo.go         # GORM implementations
├── service/
│   ├── activity.go              # ActivityService (CRUD, feed, filtering, mark read)
│   ├── activity_follow.go       # ActivityFollowService (follow/unfollow logic)
│   └── activity_reaper.go       # Background goroutine for 90-day archival
├── http/
│   ├── handler/
│   │   └── activity.go          # ActivityHandler (8 endpoints)
│   ├── request/
│   │   └── activity.go          # Request DTOs with validation
│   ├── response/
│   │   └── activity.go          # Response DTOs
│   └── middleware/
│       └── (existing, no new middleware needed)
├── config/
│   └── activity.go              # ActivityConfig (retention days, default page size)

migrations/
├── 000023_create_activities.up.sql      # Create tables + indexes
└── 000023_create_activities.down.sql   # Drop tables

tests/
├── unit/
│   ├── activity_service_test.go
│   └── activity_follow_service_test.go
└── integration/
    └── activity_handler_test.go

cmd/api/main.go               # Add permissions seeding, wire services
```

**Structure Decision**: Follows existing repository pattern (interface + GORM impl in separate files), service pattern (concrete struct with constructors), handler pattern (service dependency + enforcer), and route registration pattern (`RegisterActivityRoutes` on `Server`).

## Metis Pre-Plan Analysis — Critical Directives

> **Must follow these directives to avoid rework.** Based on deep analysis of the codebase and spec.

### CRITICAL: Use `archived_at` NOT `DeletedAt` for 90-Day Archival

The spec says "soft-delete via `deleted_at`" but this conflicts with how GORM handles soft deletes. If we use `DeletedAt`:
- GORM auto-filters ALL queries with `WHERE deleted_at IS NULL`
- Querying archived activities requires `Unscoped()` which bypasses ALL soft-delete filters
- The Notification system uses `ArchivedAt` — a separate, explicit mechanism

**Decision**: Use `archived_at` (nullable TIMESTAMPTZ) for 90-day reaper archival. Reserve `DeletedAt` only for explicit user/admin deletions (FR-008). This matches the Notification pattern and avoids GORM confusion.

### CRITICAL: Batch `is_read` Computation — No LEFT JOIN in Main Query

Computing `is_read` for a list of activities via LEFT JOIN on `activity_reads` degrades catastrophically at scale. The correct pattern:
1. Query activities with pagination (no JOIN)
2. Extract activity IDs
3. Batch-fetch read status: `SELECT activity_id, read_at FROM activity_reads WHERE user_id = ? AND activity_id IN (?)`
4. Merge `is_read` in Go code

### CRITICAL: Buffered Channel for EventBus Subscription

Do NOT process EventBus events synchronously in the handler. The existing `callHandlers()` runs each handler sequentially, and a slow handler blocks all subscribers. Follow the WebhookService pattern:
```go
func (s *ActivityService) SubscribeToEventBus(bus *domain.EventBus) {
    ch := make(chan domain.WebhookEvent, 256)
    bus.Subscribe(func(event domain.WebhookEvent) { ch <- event })
    go s.processEvents(ch)
}
```

### ActionType Mapping Registry

Define explicit mapping from WebhookEvent types to Activity ActionType and ResourceType:

| EventBus Type | ActionType | ResourceType | Description Template |
|--------------|-----------|-------------|-------------------|
| user.created | created | user | "{actor_name} created a user account" |
| user.deleted | deleted | user | "{actor_name} deleted user {resource_id}" |
| invoice.created | created | invoice | "{actor_name} created invoice {metadata.invoice_number}" |
| invoice.paid | paid | invoice | "{actor_name} marked invoice {metadata.invoice_number} as paid" |
| news.published | published | news | "{actor_name} published news article {metadata.title}" |
| news.deleted | deleted | news | "{actor_name} deleted news article {resource_id}" |
| comment.created | created | comment | "{actor_name} commented on {metadata.commentable_type}" |

**Description generation**: Use template strings with actor name resolution. Fallback to `"{actor_name} {action_type} {resource_type}"` for unmapped types.

### Missing Endpoints (Confirmed in Spec)

The spec already includes FR-010 (count-unread), FR-011 (mark single as read), FR-012 (mark all as read). These are confirmed requirements.

## Complexity Tracking

No constitution violations to justify.

---

## Phase 0: Research

### Research Tasks

#### R-001: EventBus subscription pattern

**Decision**: Use `SetEventBus()` setter injection pattern (same as webhook, user, invoice, news services)
**Rationale**: Avoids constructor signature changes across features; established pattern in codebase
**Alternatives considered**: Constructor injection (rejected — requires changing all service constructors), global singleton (rejected — harder to test)

#### R-002: Per-user read tracking approach

**Decision**: Separate `activity_reads` table with composite unique index on (user_id, activity_id)
**Rationale**: Activities are shared across org members — a single `read_at` on Activity cannot serve multi-user tracking. ActivityReads table enables per-user read status, unread counts via COUNT with LEFT JOIN, and mark-all-as-read via batch INSERT.
**Alternatives considered**: `read_at` column on Activity (rejected — doesn't work for multi-user), JSONB array of user IDs on Activity (rejected — poor query performance at scale)

#### R-003: Activity follow/bookmark approach

**Decision**: Separate `activity_follows` table with composite unique index on (user_id, resource_type, resource_id)
**Rationale**: Enables `?following=true` filter via JOIN. Clean relational model, easy to query, follows existing pattern for association tables.
**Alternatives considered**: JSONB array on user record (rejected — poor query performance), separate read/unread per follow (rejected — over-engineering)

#### R-004: 90-day archival approach

**Decision**: Background reaper goroutine (matches webhook stuck-delivery reaper pattern)
**Rationale**: Simple, no dependency on job queue infrastructure, low risk (soft-delete is reversible). Runs every 60s, soft-deletes activities older than 90 days.
**Alternatives considered**: Job queue (feature 009) (rejected — over-engineering for a simple DELETE WHERE query), cron job (rejected — Go services don't have external cron)

#### R-005: Metadata schema

**Decision**: Semi-structured JSONB with required `description` field
**Rationale**: Ensures every activity has a human-readable summary for display, while allowing event publishers to add context-specific fields (invoice_number, old_status, new_status, etc.)
**Alternatives considered**: Fully flexible JSONB (rejected — no guaranteed display field), strictly typed per event (rejected — too rigid, requires schema migration for new event types)

#### R-006: Event type to ActionType mapping

**Decision**: Map EventBus event types to activity ActionTypes via a configuration map in the ActivityService
**Rationale**: EventBus uses string types like "user.created", "invoice.paid". Activity ActionType uses shorter strings like "created", "paid". A mapping function converts between them, keeping both systems decoupled.
**Alternatives considered**: Use EventBus types directly as ActionTypes (rejected — couples internal schemas, verbose), registry pattern with separate mapper per event type (rejected — over-engineering for v1)

---

## Phase 1: Design & Contracts

### Data Model

See [data-model.md](data-model.md) for full entity definitions.

#### Entity: Activity

| Field | Type | Constraints | Description |
|-------|------|------------|-------------|
| id | UUID | PK, default gen_random_uuid() | Primary key |
| actor_id | UUID | NOT NULL, FK → users(id) | User who performed the action |
| action_type | VARCHAR(50) | NOT NULL | created, updated, deleted, published, etc. |
| resource_type | VARCHAR(50) | NOT NULL | invoice, news, user, comment, etc. |
| resource_id | VARCHAR(100) | NOT NULL | ID of the affected resource |
| organization_id | UUID | FK → organizations(id) | Org scope (NULL for global) |
| metadata | JSONB | | Semi-structured: must contain `description` key |
| archived_at | TIMESTAMPTZ | | Auto-archive timestamp (set by 90-day reaper). NULL = active. |
| deleted_at | TIMESTAMPTZ | | Soft delete timestamp (set by explicit admin DELETE). NULL = not deleted. |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes:**
- `idx_activities_organization_id` ON (organization_id) WHERE archived_at IS NULL
- `idx_activities_actor_id` ON (actor_id) WHERE archived_at IS NULL
- `idx_activities_resource` ON (resource_type, resource_id) WHERE archived_at IS NULL
- `idx_activities_action_type` ON (action_type) WHERE archived_at IS NULL
- `idx_activities_created_at` ON (created_at DESC) WHERE archived_at IS NULL
- `idx_activities_feed` ON (organization_id, created_at DESC) WHERE archived_at IS NULL AND deleted_at IS NULL
- `idx_activities_archived_at` ON (archived_at) WHERE archived_at IS NOT NULL
- `idx_activities_deleted_at` ON (deleted_at)

**Foreign Keys:**
- `fk_activities_actor` → users(id) ON DELETE CASCADE
- `fk_activities_organization` → organizations(id) ON DELETE CASCADE

**Trigger:**
- `update_activities_updated_at` BEFORE UPDATE → `update_updated_at_column()`

#### Entity: ActivityRead

| Field | Type | Constraints | Description |
|-------|------|------------|-------------|
| id | UUID | PK, default gen_random_uuid() | Primary key |
| user_id | UUID | NOT NULL, FK → users(id) | User who read the activity |
| activity_id | UUID | NOT NULL, FK → activities(id) | Activity that was read |
| read_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When it was read |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Record creation |

**Unique Constraint:**
- `idx_activity_reads_user_activity` UNIQUE ON (user_id, activity_id)

**Indexes:**
- `idx_activity_reads_user_id` ON (user_id)
- `idx_activity_reads_activity_id` ON (activity_id)

**Foreign Keys:**
- `fk_activity_reads_user` → users(id) ON DELETE CASCADE
- `fk_activity_reads_activity` → activities(id) ON DELETE CASCADE

#### Entity: ActivityFollow

| Field | Type | Constraints | Description |
|-------|------|------------|-------------|
| id | UUID | PK, default gen_random_uuid() | Primary key |
| user_id | UUID | NOT NULL, FK → users(id) | User who follows the resource |
| resource_type | VARCHAR(50) | NOT NULL | Type of resource being followed |
| resource_id | VARCHAR(100) | NOT NULL | ID of the resource being followed |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When the follow was created |

**Unique Constraint:**
- `idx_activity_follows_user_resource` UNIQUE ON (user_id, resource_type, resource_id)

**Indexes:**
- `idx_activity_follows_user_id` ON (user_id)
- `idx_activity_follows_resource` ON (resource_type, resource_id)

**Foreign Keys:**
- `fk_activity_follows_user` → users(id) ON DELETE CASCADE

### API Contracts

See [contracts/api-v1.md](contracts/api-v1.md) for full REST API contract.

### EventBus Integration Contract

See [contracts/event-bus.md](contracts/event-bus.md) for EventBus subscription contract.

### Quickstart

See [quickstart.md](quickstart.md) for usage guide.

---

## Implementation Phases

### Phase 2.1: Domain Entities & Migration (T001)

**Deliverables:**
- `internal/domain/activity.go` — Activity, ActivityRead, ActivityFollow entities + DTOs + business methods
- `internal/domain/activity_events.go` — ActivityEventType constants + event-to-activity mapping
- `migrations/000023_create_activities.up.sql` — Create tables, indexes, FKs, trigger
- `migrations/000023_create_activities.down.sql` — Drop tables

**Pattern reference:** `internal/domain/comment.go` for entity structure, `internal/domain/notification.go` for type constants, `migrations/000022_create_comments.up.sql` for migration structure

**Key decisions:**
- Activity.ReadAt moved to ActivityRead (per-user tracking)
- Activity.Metadata is JSONB with required `description` field
- Activity.ArchivedAt for 90-day auto-archival (reaper sets this, NOT `deleted_at` — avoids GORM confusion)
- Activity.DeletedAt for explicit admin deletions (GORM soft-delete, used by DELETE endpoint)
- ActionType constants mirror but don't exactly match EventBus event types
- Composite unique indexes on ActivityReads and ActivityFollows

### Phase 2.2: Repository Layer (T002)

**Deliverables:**
- `internal/repository/activity.go` — 3 repository interfaces
- `internal/repository/activity_repo.go` — GORM implementations

**Repositories:**
1. **ActivityRepository** — Create, FindByID, FindByOrganization (paginated, filtered, excludes archived+deleted), FindByResource (paginated, filtered), SoftDelete, ArchiveOlderThan (reaper uses this), CountUnread, MarkAllAsRead (bulk)
2. **ActivityReadRepository** — Upsert (create or update read_at), FindByUserAndActivity, DeleteByUserAndActivity, CountByUser, MarkAllReadByUser
3. **ActivityFollowRepository** — Create, Delete, FindByUser, FindByUserAndResource, ExistsByUserAndResource

**Pattern reference:** `internal/repository/comment.go` for interface structure, `internal/repository/feature_flag.go` for GORM patterns

### Phase 2.3: Service Layer (T003)

**Deliverables:**
- `internal/service/activity.go` — ActivityService (feed, filtering, read tracking, EventBus subscription)
- `internal/service/activity_follow.go` — ActivityFollowService (follow/unfollow/list)
- `internal/service/activity_reaper.go` — Background reaper goroutine for 90-day archival

**ActivityService methods:**
- `ListByOrganization(ctx, userID, hasOrgID, orgID, filters, limit, offset)` — Main feed endpoint
- `ListByResource(ctx, userID, hasOrgID, orgID, resourceType, resourceID, limit, offset)` — Resource history
- `MarkAsRead(ctx, userID, activityID)` — Create/update ActivityRead
- `MarkAllAsRead(ctx, userID, hasOrgID, orgID)` — Batch create ActivityReads
- `CountUnread(ctx, userID, hasOrgID, orgID)` — Get unread count
- `SubscribeToEventBus(eventBus)` — Register EventBus handler
- `handleEvent(event)` — Internal: convert WebhookEvent → Activity

**ActivityFollowService methods:**
- `Follow(ctx, userID, resourceType, resourceID)` — Create ActivityFollow
- `Unfollow(ctx, userID, followID)` — Delete ActivityFollow
- `ListFollows(ctx, userID)` — List user's follows
- `IsFollowing(ctx, userID, resourceType, resourceID)` — Check follow status

**ActivityReaper:**
- `Start(ctx)` — Launch goroutine, tick every 60s, set `archived_at` on activities older than 90 days where `archived_at IS NULL AND deleted_at IS NULL`
- `Stop()` — Graceful shutdown with context cancellation

**SetEventBus pattern:**
```go
func (s *ActivityService) SetEventBus(eventBus *domain.EventBus) {
    s.eventBus = eventBus
}
```

**Event-to-Activity mapping:**
| EventBus Type | Activity ActionType | ResourceType |
|---------------|-------------------|--------------|
| user.created | created | user |
| user.deleted | deleted | user |
| invoice.created | created | invoice |
| invoice.paid | paid | invoice |
| news.published | published | news |
| news.deleted | deleted | news |
| comment.created | created | comment |

**Pattern reference:** `internal/service/comment.go` for service structure, `internal/service/webhook.go` for EventBus subscription

### Phase 2.4: Handler & Routes (T004)

**Deliverables:**
- `internal/http/handler/activity.go` — ActivityHandler (8 endpoints)
- `internal/http/request/activity.go` — Request DTOs with validation
- `internal/http/response/activity.go` — Response DTOs
- `internal/http/server.go` — RegisterActivityRoutes method + DI wiring

**Endpoints:**
| Method | Path | Description | Permission |
|--------|------|-------------|------------|
| GET | /api/v1/activities | List org activities (paginated, filterable) | activity:view |
| GET | /api/v1/activities/count-unread | Get unread count | activity:view |
| PUT | /api/v1/activities/read-all | Mark all as read | activity:view |
| PUT | /api/v1/activities/:id/read | Mark single as read | activity:view |
| POST | /api/v1/activities/follow | Follow a resource | activity:view |
| DELETE | /api/v1/activities/follow/:id | Unfollow a resource | activity:view |
| GET | /api/v1/activities/follows | List user's follows | activity:view |
| DELETE | /api/v1/activities/:id | Soft-delete (admin) | activity:manage |

**Pattern reference:** `internal/http/handler/comment.go` for handler structure, `internal/http/handler/notification.go` for read tracking endpoints

### Phase 2.5: Configuration & DI Wiring (T005)

**Deliverables:**
- `internal/config/activity.go` — ActivityConfig
- `internal/http/server.go` — DI wiring in RegisterRoutes()
- `cmd/api/main.go` — Permission seeding + service wiring + EventBus subscription + reaper start/stop

**Config:**
```go
type ActivityConfig struct {
    RetentionDays    int // default: 90
    DefaultPageSize   int // default: 20
    MaxPageSize       int // default: 100
    ReaperInterval    time.Duration // default: 60s
}
```

**Permission seeding (cmd/api/main.go DefaultPermissions):**
```
activity:view    → resource: "activity", action: "view"
activity:manage  → resource: "activity", action: "manage"
```

**Permission seeding (cmd/api/main.go runSeed):**
Add to permissions slice: `activity:view`, `activity:manage`

**Startup wiring order:**
1. Create repositories
2. Create services (with enforcer, auditSvc, logger)
3. Create handler
4. Register routes
5. Wire EventBus subscription (`activityService.SetEventBus(eventBus)`)
6. Start reaper (`activityReaper.Start(ctx)`)

**Shutdown order:** server → eventBus → activityReaper → workers → enforcer → db → redis

### Phase 2.6: Unit Tests (T006)

**Deliverables:**
- `tests/unit/activity_service_test.go` — Activity service tests (mock repos, mock enforcer)
- `tests/unit/activity_follow_service_test.go` — Follow service tests

**Test coverage targets:**
- ActivityService: ListByOrganization with filters, MarkAsRead, MarkAllAsRead, CountUnread, EventBus event handling
- ActivityFollowService: Follow, Unfollow, ListFollows, IsFollowing
- Edge cases: empty feed, org scoping, soft-deleted activities excluded, concurrent mark-as-read

**Pattern reference:** `tests/unit/comment_service_test.go` for service test structure (54 test cases)

### Phase 2.7: Integration Tests (T007)

**Deliverables:**
- `tests/integration/activity_handler_test.go` — Full HTTP integration tests

**Test coverage targets:**
- Create activity via EventBus event → verify appears in feed
- List activities with filters (resource_type, action_type, unread, following)
- Mark as read / mark all as read
- Follow / unfollow resources
- Count unread
- Org scoping (cross-org visibility blocked)
- Pagination
- 90-day archival (reaper test)

**Pattern reference:** `tests/integration/notification_handler_test.go` for handler test structure

### Phase 2.8: Startup Integration & Documentation (T008)

**Deliverables:**
- Wire ActivityService.SetEventBus in `cmd/api/main.go`
- Start/stop ActivityReaper in `cmd/api/main.go`
- Update `AGENTS.md` with Activity Feed feature documentation
- Update `docs/FEATURE_STATUS.md`
- Verify `go build ./...` passes
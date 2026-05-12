# Activity Feed — Data Model

**Branch**: `016-activity-feed` | **Date**: 2026-05-12

## Entity Relationship Diagram

```
┌──────────────────┐     ┌──────────────────┐     ┌───────────────────┐
│    Activity       │     │  ActivityRead     │     │  ActivityFollow    │
├──────────────────┤     ├──────────────────┤     ├───────────────────┤
│ id (UUID, PK)     │     │ id (UUID, PK)     │     │ id (UUID, PK)      │
│ actor_id (FK)     │─┐   │ user_id (FK)       │─┐   │ user_id (FK)       │─┐
│ action_type       │ │   │ activity_id (FK)   │─┼─→ │ resource_type      │ │
│ resource_type     │ │   │ read_at            │ │   │ resource_id        │ │
│ resource_id       │ │   │ created_at         │ │   │ created_at         │ │
│ organization_id   │ │   └──────────────────┘  │   └───────────────────┘ │
│ metadata (JSONB)  │ │          │              │                         │
│ archived_at       │ │   UNIQUE(user_id,       │   UNIQUE(user_id,        │
│ deleted_at        │ │            activity_id)  │         resource_type,  │
│ created_at        │ │                          │         resource_id)     │
│ updated_at        │ │                          │                         │
└──────────────────┘ │                          │                         │
         │           │                          │                         │
         └───────────┼──────────────────────────┼─────────────────────────┘
                     │                          │
              users(id)                  users(id)
```

## Activity

Core entity representing a single activity event. Immutable once created (only `archived_at`, `deleted_at`, and `updated_at` can change).

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| actor_id | UUID | NOT NULL, FK → users(id) ON DELETE CASCADE | User who performed the action |
| action_type | VARCHAR(50) | NOT NULL | created, updated, deleted, published, paid, etc. |
| resource_type | VARCHAR(50) | NOT NULL | invoice, news, user, comment, etc. |
| resource_id | VARCHAR(100) | NOT NULL | UUID or identifier of the affected resource |
| organization_id | UUID | FK → organizations(id) ON DELETE CASCADE | Organization scope (NULL for global) |
| metadata | JSONB | | Semi-structured: MUST contain `description` key, MAY contain arbitrary context |
| archived_at | TIMESTAMPTZ | | Auto-archive timestamp (set by 90-day reaper). NULL = active. See Metis directive. |
| deleted_at | TIMESTAMPTZ | | Soft delete timestamp (set by explicit admin DELETE). NULL = not deleted. GORM managed. |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes (partial, WHERE archived_at IS NULL AND deleted_at IS NULL):**
- `idx_activities_organization_id` ON (organization_id)
- `idx_activities_actor_id` ON (actor_id)
- `idx_activities_resource` ON (resource_type, resource_id)
- `idx_activities_action_type` ON (action_type)
- `idx_activities_created_at` ON (created_at DESC) — primary sort index

**Performance index (for feed queries):**
- `idx_activities_feed` ON (organization_id, created_at DESC) WHERE archived_at IS NULL AND deleted_at IS NULL — composite for org-scoped paginated feed

**Archival index:**
- `idx_activities_archived_at` ON (archived_at) WHERE archived_at IS NOT NULL — for reaper queries

**Soft-delete index:**
- `idx_activities_deleted_at` ON (deleted_at) — GORM soft-delete default

**Triggers:**
- `update_activities_updated_at` BEFORE UPDATE → `update_updated_at_column()`

**Business methods:**
- `TableName() string` → "activities"
- `ToResponse(actorName string) ActivityResponse` — convert to DTO

## ActivityRead

Per-user read tracking. Records which activities each user has read. Enables unread count and `?unread=true` filtering.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| user_id | UUID | NOT NULL, FK → users(id) ON DELETE CASCADE | User who read the activity |
| activity_id | UUID | NOT NULL, FK → activities(id) ON DELETE CASCADE | Activity that was read |
| read_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When it was read |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Record creation |

**Unique Constraint:**
- `idx_activity_reads_user_activity` UNIQUE ON (user_id, activity_id)

**Indexes:**
- `idx_activity_reads_user_id` ON (user_id) — for user's read status queries
- `idx_activity_reads_activity_id` ON (activity_id) — for cleanup/reaper

## ActivityFollow

Per-user resource following. Enables "following" filter on the activity feed.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Primary key |
| user_id | UUID | NOT NULL, FK → users(id) ON DELETE CASCADE | User who follows the resource |
| resource_type | VARCHAR(50) | NOT NULL | Type of resource (invoice, news, etc.) |
| resource_id | VARCHAR(100) | NOT NULL | ID of the resource being followed |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When the follow was created |

**Unique Constraint:**
- `idx_activity_follows_user_resource` UNIQUE ON (user_id, resource_type, resource_id)

**Indexes:**
- `idx_activity_follows_user_id` ON (user_id) — for listing user's follows
- `idx_activity_follows_resource` ON (resource_type, resource_id) — for resource follow count

## ActivityEventType Constants

Maps EventBus event types to Activity ActionType and ResourceType:

| EventBus Type | ActionType | ResourceType |
|---------------|-----------|-------------|
| user.created | created | user |
| user.deleted | deleted | user |
| invoice.created | created | invoice |
| invoice.paid | paid | invoice |
| news.published | published | news |
| news.deleted | deleted | news |
| comment.created | created | comment |

## ActivityResponse DTO

| Field | Type | Description |
|-------|------|-------------|
| id | string (UUID) | Activity ID |
| actor_id | string (UUID) | User who performed the action |
| actor_name | string | Resolved user display name |
| action_type | string | Action performed |
| resource_type | string | Type of resource |
| resource_id | string | ID of affected resource |
| organization_id | string (UUID) \| null | Organization scope |
| metadata | object | Semi-structured: `description` + arbitrary context |
| is_read | boolean | Whether current user has read this activity |
| is_following | boolean | Whether current user follows this resource |
| created_at | string (RFC3339) | Creation timestamp |

## ActivityListResponse DTO

| Field | Type | Description |
|-------|------|-------------|
| activities | array | Array of ActivityResponse objects |
| total | int64 | Total count of matching activities |
| unread_count | int64 | Count of unread activities (included in list response) |
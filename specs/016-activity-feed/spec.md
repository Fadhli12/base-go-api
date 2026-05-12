# Feature Specification: Activity Feed / Timeline

**Feature Branch**: `016-activity-feed`  
**Created**: 2026-05-12  
**Status**: Draft  
**Input**: User description: "Activity Feed / Timeline - chronological activity stream for users showing actions across the system, integrated with existing EventBus and Notification system"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Personal Activity Feed (Priority: P1)

A logged-in user wants to see a chronological stream of activities relevant to them, including their own actions and actions by others in their organization. This is the core value proposition — visibility into "what's happening" across the platform.

**Why this priority**: Without a feed to display, no other feature makes sense. This is the minimum viable product.

**Independent Test**: Can be fully tested by creating activities via EventBus and verifying they appear in the user's feed endpoint, delivering immediate value for awareness and transparency.

**Acceptance Scenarios**:

1. **Given** a user has performed or been involved in activities, **When** they call `GET /api/v1/activities`, **Then** they receive a paginated, reverse-chronological list of activities relevant to them
2. **Given** an organization member has created an invoice, **When** another member of the same organization views their feed, **Then** they see that invoice creation activity
3. **Given** a user has no activities, **When** they call the feed endpoint, **Then** they receive an empty list (not an error)

---

### User Story 2 - View Resource Activity History (Priority: P2)

A user wants to see the history of all activities that have occurred on a specific resource (e.g., an invoice, a news article, a comment), providing a full audit trail of who did what and when.

**Why this priority**: Resource-scoped activity is valuable for accountability and context, but depends on the core feed infrastructure first.

**Independent Test**: Can be tested by creating activities for a specific resource type and ID, then verifying the resource-scoped endpoint returns only those activities.

**Acceptance Scenarios**:

1. **Given** multiple activities exist for invoice `abc-123`, **When** a user calls `GET /api/v1/activities?resource_type=invoice&resource_id=abc-123`, **Then** they receive only activities related to that invoice
2. **Given** activities exist across different resource types, **When** a user filters by `resource_type=news`, **Then** they receive only news-related activities

---

### User Story 3 - Mark Activities as Read and Manage Feed (Priority: P3)

A user wants to mark activities as read, filter by read/unread status, and manage their activity feed preferences so they can focus on new information.

**Why this priority**: Read status and filtering enhance usability but are not required for the core feed value.

**Independent Test**: Can be tested by marking activities as read via the endpoint and verifying the unread count decreases.

**Acceptance Scenarios**:

1. **Given** a user has 5 unread activities, **When** they call `PUT /api/v1/activities/read-all`, **Then** all activities are marked as read and the unread count becomes 0
2. **Given** a user has mixed read/unread activities, **When** they call `GET /api/v1/activities?unread=true`, **Then** only unread activities are returned
3. **Given** a user marks a single activity as read, **When** they call `PUT /api/v1/activities/:id/read`, **Then** that activity's `read_at` field is set to the current timestamp

---

### Edge Cases

- What happens when EventBus publishes an activity for a user who doesn't exist? → Activity is still recorded (actor-based, not target-based); feed queries filter by organization membership
- How does the system handle very high activity volume? → Pagination with cursor/offset and server-side limits per page (max 100)
- What happens if an activity references a soft-deleted resource? → Activity record is retained (immutable audit trail) and the resource_id still points to the original identifier
- What happens when an organization member is removed? → Their historical activities remain visible to the organization (activities are not deleted on membership change)
- What happens when the same activity type is emitted rapidly (burst)? → All activities are recorded independently; no deduplication at the activity level
- What happens to activities older than 90 days? → They are archived by setting `archived_at` (NOT `deleted_at`) via a background reaper goroutine; excluded from default feed queries (`WHERE archived_at IS NULL`) but accessible via `?since` parameter. Explicit admin deletions set `deleted_at` and are permanently excluded via GORM soft-delete.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record activities from existing domain events (user.created, invoice.created, invoice.paid, news.published, news.deleted, comment.created, etc.)
- **FR-002**: System MUST expose a paginated personal activity feed endpoint (`GET /api/v1/activities`) returning activities scoped to the authenticated user's organization. By default, the feed shows all organization activities; users can follow specific resources for a focused view
- **FR-003**: System MUST support filtering activities by `resource_type` (e.g., `invoice`, `news`, `comment`, `user`)
- **FR-004**: System MUST support filtering activities by `resource_id` to view a specific resource's history
- **FR-005**: System MUST support filtering activities by `action_type` (e.g., `created`, `updated`, `deleted`, `published`)
- **FR-006**: System MUST integrate with the existing EventBus to automatically create activities when domain events are published
- **FR-007**: System MUST record the actor (who performed the action), action type, resource type, resource ID, organization ID, and optional metadata (JSONB). Metadata MUST include a `description` field (human-readable summary) and MAY include arbitrary additional context fields (e.g., `invoice_number`, `old_status`, `new_status`)
- **FR-008**: System MUST use soft delete (`deleted_at`) for explicit user/admin deletions, and a separate `archived_at` field for 90-day auto-archival. This avoids GORM soft-delete confusion where `Unscoped()` would bypass ALL filters including real deletions
- **FR-009**: System MUST enforce RBAC permissions via Casbin for activity feed access (`activity:view` to read, `activity:manage` to manage)
- **FR-010**: System MUST provide an unread activity count endpoint (`GET /api/v1/activities/count-unread`), calculated from the ActivityReads table scoped to the current user
- **FR-011**: System MUST allow marking individual activities as read (`PUT /api/v1/activities/:id/read`) by creating/updating an ActivityRead record for the current user
- **FR-012**: System MUST allow marking all activities as read (`PUT /api/v1/activities/read-all`) by creating ActivityRead records for all visible unread activities
- **FR-013**: System MUST support filtering by read/unread status (`?unread=true` query parameter), determined via the ActivityReads table for the current user
- **FR-014**: System MUST enforce pagination with a default page size of 20 and maximum of 100 activities per page
- **FR-015**: System MUST automatically set `organization_id` on activities based on the actor's organization context at the time of the event
- **FR-016**: System MUST provide audit logging for activity mutations (mark as read, follow/unfollow) via the existing audit service
- **FR-017**: System MUST subscribe to EventBus events for activity creation at service startup, similar to webhook subscription patterns
- **FR-018**: System MUST allow users to follow specific resources (`POST /api/v1/activities/follow`) and unfollow them (`DELETE /api/v1/activities/follow/:id`), creating ActivityFollow records
- **FR-019**: System MUST support a "following" filter on the activity feed (`GET /api/v1/activities?following=true`) that returns only activities for resources the user has followed
- **FR-020**: System MUST auto-archive activities older than 90 days by setting `archived_at` (NOT `deleted_at`) via a background reaper goroutine; default feed queries exclude archived activities (`WHERE archived_at IS NULL`), but they remain accessible via the `?since` query parameter. Explicit admin deletions use `deleted_at` (GORM soft delete) and are permanently excluded from all queries

### Key Entities

- **Activity**: Core entity representing a single activity event. Fields: ID (UUID), ActorID (UUID), ActionType (string: created/updated/deleted/published etc.), ResourceType (string: invoice/news/user/comment), ResourceID (string), OrganizationID (UUID, nullable for global), Metadata (JSONB, semi-structured — MUST contain `description` field, MAY contain arbitrary context fields), ArchivedAt (nullable TIMESTAMPTZ — set by 90-day reaper, NULL = active), DeletedAt (nullable TIMESTAMPTZ — GORM soft delete for explicit admin deletions), CreatedAt, UpdatedAt. Immutable once created (no updates to action/resource fields). No `ReadAt` field — read status is per-user via ActivityReads.
- **ActivityRead**: Per-user read tracking entity. Fields: ID (UUID), UserID (UUID), ActivityID (UUID), ReadAt (timestamp), CreatedAt. Tracks which activities each user has read. Composite unique index on (user_id, activity_id).
- **ActivityFollow**: Per-user resource follow/bookmark entity. Fields: ID (UUID), UserID (UUID), ResourceType (string), ResourceID (string), CreatedAt. Allows users to follow specific resources for a focused feed view. Composite unique index on (user_id, resource_type, resource_id).
- **ActivityResponse**: DTO for API responses. Includes computed `is_read` boolean (from ActivityReads lookup for the current user), formatted timestamps, and resolved actor name/email from the User relation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view their activity feed with sub-second response time for the first page (under 500ms for standard page sizes)
- **SC-002**: Activity feed correctly shows all activities from services already emitting events (user, invoice, news), with no missing or duplicate entries
- **SC-003**: Users can filter their feed by resource type, resource ID, and action type and receive correctly scoped results
- **SC-004**: Read/unread tracking works accurately — marking activities as read persists across sessions and the unread count stays consistent
- **SC-005**: Activities are organization-scoped — users in one organization cannot see activities from another organization
- **SC-006**: The system handles existing EventBus event types without requiring changes to existing services (only the new ActivityService subscribes)

## Clarifications

### Session 2026-05-12

- Q: What does "personal feed" mean — should users see all org activities, only activities where they're directly involved, or a hybrid? → A: Hybrid — all org activities by default, with per-user follow/bookmark for specific resources. Read tracking uses a separate `activity_reads` per-user table since activities are shared across org members (a single `read_at` on the Activity entity cannot serve multi-user read tracking).
- Q: How long are activities retained? → A: Auto-archive after 90 days (archived_at field set, NOT deleted_at — avoids GORM soft-delete confusion), available via `?since` parameter for explicit retrieval. Explicit admin deletions use `deleted_at`. Matches existing Notification `ArchivedAt` pattern.
- Q: What metadata schema for Activities? → A: Semi-structured — require `description` field as human-readable summary, allow arbitrary additional fields (e.g., `invoice_number`, `old_status`, `new_status`).
- Q: How should 90-day archiving be implemented? → A: Background reaper goroutine (like webhook stuck-delivery reaper) — runs periodically, soft-deletes aged activities. No dependency on the job queue infrastructure.

## Assumptions

- The existing EventBus (`internal/domain/webhook_events.go`) infrastructure will be reused for activity event dispatch — activities subscribe to the same events that webhooks use
- Activity records are immutable once created — only `ReadAt` and `DeletedAt` can be modified
- The Notification system (P1, already implemented) is a separate concern — notifications are user-targeted alerts; activities are a chronological record of events. They share event sources but serve different purposes
- RBAC enforcement follows the same Casbin pattern used across all features: handler-level permission checks, service-level business logic
- Activity feed is read-only for end users — no create/update/delete endpoints for manual activity creation (only system-generated via EventBus)
- Marking activities as read is the only mutation users can perform (plus soft delete for admins with `activity:manage`)
- The existing audit log infrastructure is used for tracking activity mutations (mark as read operations)
- Organization scoping follows the same `X-Organization-ID` middleware pattern used by other features
- Activity types are defined as domain constants, not as a separate database table — matching the existing pattern for notification types
- Activity metadata is semi-structured: the `description` field is required (human-readable summary), and event publishers MAY add arbitrary context fields (e.g., `invoice_number`, `old_status`, `new_status`)
- No real-time push (WebSocket) in this iteration — activities are fetched on demand via REST. WebSocket is a separate P3 feature
- Activities are auto-archived after 90 days via `archived_at` field (NOT `deleted_at`), matching the Notification `ArchivedAt` pattern. The `deleted_at` field is reserved for explicit admin deletions only. A background reaper goroutine handles archival — runs periodically, no dependency on the job queue. Archived activities are excluded from default queries (`WHERE archived_at IS NULL`) but remain accessible via `?since` parameter. Explicitly deleted activities (via admin DELETE endpoint) use `deleted_at` and are excluded by GORM soft-delete.
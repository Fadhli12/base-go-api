# Feature Specification: Analytics Dashboard

**Feature Branch**: `018-analytics-dashboard`  
**Created**: 2026-05-12  
**Status**: Draft  
**Input**: User description: "Analytics Dashboard — usage analytics, metrics, and dashboard data aggregation for the Go API platform"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Organization Dashboard (Priority: P1)

An organization admin wants to see a high-level overview of activity within their organization — how many users are active, how many resources were created, and what actions are trending — so they can understand platform engagement at a glance.

**Why this priority**: Without a dashboard view, the analytics feature has no user-facing value. This is the minimum viable product — even if aggregation and ingestion are working, users need a way to see the data.

**Independent Test**: Can be fully tested by calling `GET /api/v1/analytics/dashboard` with organization context and verifying that returned metrics include counts for users, resources, and actions within a given time range. Delivers immediate value: org admins get visibility into their organization's activity.

**Acceptance Scenarios**:

1. **Given** an authenticated admin with `analytics:view` permission, **When** they request `GET /api/v1/analytics/dashboard` with `X-Organization-ID` header, **Then** the system returns aggregated metrics including total users, active users (daily/weekly/monthly), resource creation counts, and top action types for that organization.
2. **Given** an authenticated admin with `analytics:view` permission, **When** they request `GET /api/v1/analytics/dashboard` with a `period` query parameter (e.g., `daily`, `weekly`, `monthly`), **Then** the system returns metrics aggregated for the specified time period.
3. **Given** an authenticated user without `analytics:view` permission, **When** they request `GET /api/v1/analytics/dashboard`, **Then** the system returns 403 Forbidden.
4. **Given** an authenticated admin, **When** they request `GET /api/v1/analytics/dashboard` without `X-Organization-ID`, **Then** the system returns global (non-org-scoped) metrics.

---

### User Story 2 - Track Metric Events (Priority: P1)

The platform needs to capture raw metric events from existing system actions (user registrations, content creation, webhook deliveries, job completions) so that the dashboard has data to aggregate and display.

**Why this priority**: Without event ingestion, there is no data for the dashboard. This is foundational — the dashboard and time-series endpoints depend on events being captured.

**Independent Test**: Can be fully tested by having existing services (UserService, InvoiceService, NewsService, WebhookService) publish events via the EventBus, and verifying that `metric_events` rows are created with correct event_type, actor_id, resource_type, and metadata. The ingestion pipeline is the data backbone of the feature.

**Acceptance Scenarios**:

1. **Given** the analytics service is subscribed to the EventBus, **When** a `user.created` event is published, **Then** a `MetricEvent` record is created with `event_type="user.created"`, the actor's user ID, and timestamp with pre-aggregated date/hour fields.
2. **Given** the analytics service is subscribed to the EventBus, **When** an `invoice.paid` event is published with organization context, **Then** a `MetricEvent` record is created with the correct `organization_id`, enabling org-scoped aggregation.
3. **Given** a metric event is ingested, **When** the event includes metadata (e.g., response time, status code), **Then** the metadata is stored as JSONB and queryable in aggregation.
4. **Given** the EventBus publishes an event, **When** the AnalyticsService receives it, **Then** the event is persisted within 100ms of the original action completing.

---

### User Story 3 - Query Time-Series Metrics (Priority: P2)

A platform operator wants to query metric trends over time (e.g., user signups per day for the last 30 days, API calls per hour for the last week) so they can identify trends and anomalies.

**Why this priority**: Time-series queries enable deeper analysis beyond the dashboard summary. They are important but secondary to having data (P1) and a summary view (P1).

**Independent Test**: Can be fully tested by calling `GET /api/v1/analytics/metrics?type=user.created&period=daily&from=2026-04-01&to=2026-05-01` and verifying the response contains data points with timestamps and values for each day in the range.

**Acceptance Scenarios**:

1. **Given** metric events exist for the last 30 days, **When** an admin requests `GET /api/v1/analytics/metrics?type=user.created&period=daily&from=<30d-ago>&to=<today>`, **Then** the system returns a time-series of daily counts for user creation events.
2. **Given** metric events exist for multiple event types, **When** an admin requests `GET /api/v1/analytics/metrics?type=user.created,invoice.paid&period=weekly`, **Then** the system returns separate time-series for each event type.
3. **Given** an admin requests metrics with `period=hourly` for the last 24 hours, **When** data exists across 24 hours, **Then** the system returns hourly data points with pre-aggregated hour fields.
4. **Given** no metric events exist for the requested range, **When** an admin queries metrics, **Then** the system returns an empty time-series with zero values for each period, not a 404 error.

---

### User Story 4 - View Pre-Aggregated Dashboard Metrics (Priority: P2)

The platform needs a background worker that periodically pre-computes dashboard metrics (daily active users, resource counts, top actions) so that dashboard queries return instantly without scanning raw events every time.

**Why this priority**: Pre-aggregation is critical for performance at scale but the system can function without it by querying raw events directly for small datasets. This is P2 because without pre-aggregation, the dashboard still works — just slower for large datasets.

**Independent Test**: Can be fully tested by triggering the aggregation worker (or waiting for its scheduled interval), then verifying that `dashboard_metrics` rows are created/updated for daily, weekly, and monthly periods, and that the dashboard endpoint uses the pre-aggregated data.

**Acceptance Scenarios**:

1. **Given** the aggregation worker runs on schedule, **When** the daily aggregation cycle completes, **Then** `DashboardMetric` records exist for all active event types with `period_type=daily` and correct `period_start`/`period_end` values.
2. **Given** pre-aggregated metrics exist, **When** the dashboard endpoint is called with `period=daily`, **Then** it returns data from `dashboard_metrics` (O(1) lookup) rather than scanning `metric_events` (O(n) scan).
3. **Given** the aggregation worker has not yet run for the current day, **When** the dashboard endpoint is called, **Then** it falls back to computing from raw `metric_events` for the incomplete period and uses pre-aggregated data for completed periods.

---

### User Story 5 - Manage Dashboard Visibility (Priority: P3)

An org admin wants to control which metrics are visible on their organization's dashboard by toggling metric categories on or off, so they can focus on what matters to their team.

**Why this priority**: This is a quality-of-life feature that enhances the dashboard experience but is not required for core functionality. Default visibility is sufficient for initial release.

**Independent Test**: Can be fully tested by calling `PUT /api/v1/analytics/dashboard/preferences` with metric visibility toggles and verifying that subsequent dashboard requests honor those preferences.

**Acceptance Scenarios**:

1. **Given** an admin accesses their dashboard preferences, **When** they toggle "webhook_metrics" to hidden, **Then** the dashboard response no longer includes webhook delivery metrics.
2. **Given** an admin has never set preferences, **When** they view the dashboard, **Then** all metric categories are visible by default.

---

### Edge Cases

- What happens when no metric events exist for an organization? → Dashboard returns zero counts with correct structure (not 404).
- What happens when the aggregation worker fails mid-cycle? → Transaction-per-period: each period's metrics are committed individually. On restart, the worker resumes from the last uncommitted period. Already-computed periods remain valid.
- What happens when an organization has no activity for a given period? → Dashboard returns zeros for that period; time-series returns zero values for each interval.
- What happens when EventBus events arrive out of order (e.g., delayed delivery)? → Events are stored with their original timestamp; aggregation uses the event's timestamp, not ingestion time.
- What happens when two concurrent aggregation workers run? → Optimistic locking on `dashboard_metrics` prevents double-counting; last-write wins with `calculated_at` check.
- What happens when the `metric_events` table grows very large? → Time-based partitioning via `date` column index; retention reaper sets `archived_at` on events older than 90 days (configurable). Queries filter by `archived_at IS NULL`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST capture metric events from existing system actions via EventBus subscription, using an explicit `AnalyticsMapping` registry that maps specific event types to metric categories. Unmapped events are ignored. Initial mappings: user.created, user.deleted, invoice.created, invoice.paid, news.published, news.deleted, webhook.delivered, webhook.failed, comment.created, job.completed, job.failed.
- **FR-002**: System MUST store raw metric events with event type, actor ID, resource type, resource ID, organization ID (optional), JSONB metadata, timestamp, pre-aggregated date, and pre-aggregated hour fields. System MUST enforce idempotent ingestion using a uniqueness constraint on (event_type, resource_id, date, hour) to silently skip duplicate events. This uses the pre-aggregated fields for better index performance.
- **FR-003**: System MUST provide a dashboard endpoint (`GET /api/v1/analytics/dashboard`) that returns aggregated metrics for the requesting user's organization context or globally if no organization context is provided.
- **FR-004**: System MUST support time-series metric queries (`GET /api/v1/analytics/metrics`) filtered by event type, period (hourly, daily, weekly, monthly), date range, and organization scope.
- **FR-005**: System MUST pre-compute dashboard metrics via a background aggregation worker that runs on a configurable interval (default: 60 seconds) and stores results in a `dashboard_metrics` table. Each period's computation MUST be committed as an individual transaction so that partial failures do not roll back already-computed periods.
- **FR-006**: System MUST enforce RBAC permissions: `analytics:view` for read access (dashboard, metrics), `analytics:manage` for administrative actions (preferences, manual aggregation trigger for the current period only).
- **FR-007**: System MUST scope all analytics data by organization when `X-Organization-ID` header is provided, returning global data when no organization context is present.
- **FR-008**: System MUST handle the fallback from pre-aggregated data to raw event computation when aggregated data is not yet available for a requested period.
- **FR-009**: System MUST soft-delete metric events to preserve audit trail. The retention reaper uses `archived_at` (consistent with ActivityReaper pattern) rather than `deleted_at`; queries filter by `archived_at IS NULL`. The `deleted_at` field is reserved for manual admin deletions.
- **FR-010**: System MUST use organization-scoped multi-tenancy consistent with existing patterns (Casbin domain-based enforcement).
- **FR-011**: System MUST implement a retention reaper (following ActivityReaper pattern) that sets `archived_at` on metric events older than a configurable retention period (default: 90 days). Queries exclude archived events via `archived_at IS NULL` scope.
- **FR-012**: System MUST provide dashboard preferences endpoint (`PUT /api/v1/analytics/dashboard/preferences`) allowing admins to toggle metric category visibility per organization.
- **FR-013**: System MUST use versioned SQL migrations (no AutoMigrate) consistent with project conventions.
- **FR-014**: System MUST subscribe to the existing EventBus infrastructure for event ingestion, using the `SetEventBus()` setter pattern for post-construction injection.
- **FR-015**: System MUST log all analytics read operations via the structured logging system and all write operations via the audit logging system.
- **FR-016**: Dashboard endpoint MUST return the following metric categories: user activity (total users, active users), content metrics (news/articles created, published), engagement metrics (comments created, webhook delivery success rate), system metrics (job success/failure rate).

### Key Entities

- **MetricEvent**: Raw event record captured from system actions. Contains event_type, actor_id, resource_type, resource_id, organization_id (nullable for global), metadata (JSONB), timestamp, pre-aggregated date and hour fields. Unique constraint on (event_type, resource_id, date, hour) for idempotent ingestion. Uses `archived_at` for retention reaper archival (consistent with ActivityReaper pattern); `gorm.DeletedAt` for automatic GORM soft-delete scope on admin deletions. `UpdatedAt` intentionally omitted — events are never updated after creation.
- **DashboardMetric**: Pre-computed aggregated metric. Contains metric_type, period_type (daily/weekly/monthly), period_start, period_end, value, metadata (JSONB), calculated_at. Optimistic locking via calculated_at timestamp.
- **DashboardPreference**: Per-organization metric visibility settings. Contains organization_id, metric_categories (JSONB — which categories are visible), updated_by_user_id, timestamps.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Org admins can view a complete dashboard with activity metrics within 2 seconds for datasets up to 100,000 events per organization per month.
- **SC-002**: Metric event ingestion adds less than 5ms latency to existing service operations (EventBus publish is async and non-blocking).
- **SC-003**: Pre-aggregated dashboard metrics are computed within 60 seconds of the aggregation cycle for any period.
- **SC-004**: Time-series queries for 30-day periods return results within 500ms for organizations with up to 1 million events.
- **SC-005**: 80%+ unit test coverage on business logic (AnalyticsService, AggregationWorker, AnalyticsReaper).
- **SC-006**: Zero breaking changes to existing APIs — analytics is additive only.
- **SC-007**: Dashboard returns meaningful zero-state data for new organizations with no activity (not 404 or empty responses).

## Clarifications

### Session 2026-05-12

- Q: Should the analytics system deduplicate metric events if the EventBus publishes the same event twice? → A: Idempotent ingestion — use uniqueness constraint on (event_type, resource_id, date, hour) to silently skip duplicates. Uses pre-aggregated date/hour fields instead of raw timestamp for better index performance and alignment with the pre-aggregation strategy.
- Q: What should happen when the aggregation worker fails mid-cycle? → A: Transaction-per-period — each period's metrics are committed individually; on restart, the worker resumes from the last uncommitted period.
- Q: Should manual aggregation triggers support full historical re-aggregation or only the current period? → A: Current period only — manual trigger forces re-computation of the current (incomplete) period only for safety and performance.
- Q: Should the analytics system capture events from all EventBus types or use an explicit mapping? → A: Explicit mapping registry — define an `AnalyticsMapping` registry (like `ActivityMapping`) that maps specific event types to metric events; unmapped events are ignored.
- Q: How should the retention reaper handle expired metric events? → A: Soft-delete with `archived_at` (like ActivityReaper pattern); queries filter by `archived_at IS NULL`. Consistent with existing project conventions.

## Assumptions

- The existing EventBus infrastructure (from webhook and activity feed systems) is the primary source of analytics events — no separate event ingestion API is needed for v1.
- PostgreSQL is sufficient for analytics storage and aggregation queries at the expected scale (up to 1M events per org per month). No external time-series database (e.g., ClickHouse, TimescaleDB) is needed for v1.
- Pre-aggregation is optimized for the dashboard summary view; time-series queries for raw data fall back to `metric_events` table scans with indexed date/hour columns.
- The ActivityReaper pattern (background goroutine with configurable interval) is the template for both the aggregation worker and the retention reaper.
- Dashboard preferences are optional — the dashboard returns all categories by default when no preferences are set.
- Organization scoping follows the existing `X-Organization-ID` header pattern; global (non-org-scoped) metrics are available when no header is provided.
- Event data older than the retention period is archived via `archived_at` (consistent with ActivityReaper pattern) rather than hard-deleted, preserving the audit trail. `deleted_at` is reserved for manual admin deletions only.
- The `metric_events.date` and `metric_events.hour` columns provide sufficient index performance for time-range queries; no separate time-series table structure is needed.
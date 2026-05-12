# Tasks: Analytics Dashboard

**Input**: Design documents from `/specs/018-analytics-dashboard/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: Not explicitly requested in the spec, but 80%+ test coverage is a project constraint. Unit test tasks are included in the final phase.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Domain**: `internal/domain/`
- **Repository**: `internal/repository/`
- **Service**: `internal/service/`
- **Config**: `internal/config/`
- **Handler**: `internal/http/handler/`
- **Request DTOs**: `internal/http/request/`
- **Response DTOs**: `internal/http/response/`
- **Migrations**: `migrations/`
- **Unit tests**: `tests/unit/`
- **Main wiring**: `cmd/api/main.go`
- **Server routing**: `internal/http/server.go`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create database schema and domain entities that all user stories depend on

- [ ] T001 Create SQL migration `migrations/000025_create_analytics.up.sql` with all 3 tables (metric_events, dashboard_metrics, dashboard_preferences) including indexes, FKs, UNIQUE constraint on (event_type, resource_id, date, hour), partial indexes with `WHERE archived_at IS NULL`, and auto-update triggers
- [ ] T002 Create SQL migration `migrations/000025_create_analytics.down.sql` dropping all 3 tables in reverse order

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain entities, mapping registry, config, and repositories that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 [P] Create MetricEvent entity with DTOs and business methods in `internal/domain/metric_event.go` — includes MetricEvent struct (EventType, ActorID, ResourceType, ResourceID, OrganizationID, Metadata, EventTimestamp, Date, Hour, ArchivedAt, gorm.DeletedAt, CreatedAt), MetricEventResponse, MetricTimeSeriesPoint, ToResponse() method, TableName() returning "metric_events"
- [ ] T004 [P] Create DashboardMetric entity with DTOs in `internal/domain/dashboard_metric.go` — includes DashboardMetric struct (MetricType, PeriodType, PeriodStart, PeriodEnd, Value float64, Metadata JSONB, OrganizationID, CalculatedAt), DashboardMetricResponse, ToResponse() method, TableName() returning "dashboard_metrics"
- [ ] T005 [P] Create DashboardPreference entity with DTOs in `internal/domain/dashboard_preference.go` — includes DashboardPreference struct (OrganizationID uuid.UUID NOT NULL with uniqueIndex, MetricCategories JSONB, UpdatedByUserID, CreatedAt, UpdatedAt), DashboardPreferenceResponse, ToResponse() method, TableName() returning "dashboard_preferences"
- [ ] T006 [P] Create AnalyticsMapping registry in `internal/domain/analytics_events.go` — includes AnalyticsMapping struct with MetricCategory, ResourceType, ExtractResourceID func, ExtractActorID func; analyticsEventMapping map for all 11 event types (user.created, user.deleted, invoice.created, invoice.paid, news.published, news.deleted, webhook.delivered, webhook.failed, comment.created, job.completed, job.failed); GetAnalyticsMapping() function; metric category constants (MetricCategoryUserActivity, MetricCategoryContentMetrics, MetricCategoryEngagementMetrics, MetricCategorySystemMetrics); common extraction helpers (extractPayloadID, extractPayloadActorID)
- [ ] T007 [P] Create AnalyticsConfig in `internal/config/analytics.go` — includes AnalyticsConfig struct (RetentionDays int, AggregationInterval time.Duration, ReaperInterval time.Duration, DefaultPageSize int, MaxPageSize int) with DefaultAnalyticsConfig() and parseAnalyticsConfig() wired into Config struct in `internal/config/config.go`
- [ ] T008 Create MetricEventRepository interface and GORM implementation in `internal/repository/metric_event.go` — includes MetricEventFilters struct, MetricEventRepository interface (Create with ON CONFLICT DO NOTHING, FindByID, FindByOrganization, CountByTypeAndPeriod, CountDistinctActorsByPeriod, FindTimeSeries, ArchiveOlderThan, SoftDelete) and GORM implementation; ALL queries MUST include `WHERE archived_at IS NULL` scope; ArchiveOlderThan sets archived_at on records older than retention days where archived_at IS NULL AND deleted_at IS NULL
- [ ] T009 [P] Create DashboardMetricRepository interface and GORM implementation in `internal/repository/dashboard_metric.go` — includes DashboardMetricRepository interface (Upsert, FindByTypeAndPeriod, FindByTypesAndPeriod, DeleteByTypeAndPeriodBefore, FindMaxCalculatedAt) and GORM implementation; Upsert uses ON CONFLICT UPDATE for calculated_at watermark
- [ ] T010 [P] Create DashboardPreferenceRepository interface and GORM implementation in `internal/repository/dashboard_preference.go` — includes DashboardPreferenceRepository interface (FindByOrganization, Upsert) and GORM implementation; FindByOrganization returns nil (not error) when no preference row exists

**Checkpoint**: Foundation ready — all entities, mapping, config, and repositories created. User story implementation can now begin.

---

## Phase 3: User Story 1+2 — Track Metric Events & View Dashboard (Priority: P1) 🎯 MVP

**Goal**: Capture metric events from existing system actions via EventBus and provide a dashboard endpoint that returns aggregated metrics for the requesting user's organization context or globally.

**Independent Test**: Publish a `user.created` event via EventBus → verify MetricEvent row created with correct event_type, actor_id, resource_id, date, hour → call `GET /api/v1/analytics/dashboard` with JWT auth → verify response contains total_users, active_users, new_users counts for the organization.

### Implementation for User Story 1+2

- [ ] T011 Create AnalyticsService in `internal/service/analytics.go` — includes AnalyticsService struct (metricEventRepo, dashboardRepo, preferenceRepo, enforcer, audit, log, eventBus *domain.EventBus); NewAnalyticsService constructor; SubscribeToEventBus(eventBus) with buffered channel (256) and processEvents goroutine (follow ActivityService pattern); SetEventBus setter; handleEvent that uses AnalyticsMapping to extract fields and creates MetricEvent with ON CONFLICT DO NOTHING; GetDashboard(ctx, userID, hasOrgID, orgID, period) that checks pre-aggregated DashboardMetric first (completed periods) then falls back to raw MetricEvent queries (current incomplete period); GetMetrics(ctx, userID, hasOrgID, orgID, types, period, from, to) with zero-fill time-series generation; GetPreferences that returns default visibility when no row exists; UpdatePreferences that upserts preference row. NOTE: Permission enforcement is handler-level only — service does NOT call enforcer.Enforce()
- [ ] T012 Create AnalyticsHandler in `internal/http/handler/analytics.go` — includes 5 endpoints: GetDashboard (GET /dashboard?period=daily|weekly|monthly), GetMetrics (GET /metrics?type=...&period=...&from=...&to=...), GetPreferences (GET /dashboard/preferences), UpdatePreferences (PUT /dashboard/preferences), TriggerAggregation (POST /aggregate); RegisterRoutes with JWT middleware, ExtractOrganizationID, RequirePermission(enforcer, "analytics", "view") on route group; TriggerAggregation and UpdatePreferences check h.enforcer.Enforce(userID, orgDomain, "analytics", "manage") at handler level
- [ ] T013 [P] Create request DTOs in `internal/http/request/analytics.go` — includes DashboardQuery (period query param with validation: daily|weekly|monthly), MetricsQuery (type, period, from, to with required validation), UpdatePreferencesRequest (metric_categories map[string]bool with validation)
- [ ] T014 [P] Create response DTOs in `internal/http/response/analytics.go` — includes DashboardResponse (period + 4 category metric structs: UserActivityMetrics, ContentMetrics, EngagementMetrics, SystemMetrics), MetricsTimeSeriesResponse (type, period, from, to, data_points []DataPoint), DataPoint (timestamp, value), PreferencesResponse (organization_id, metric_categories)
- [ ] T015 Wire AnalyticsService, AggregationWorker (placeholder), AnalyticsReaper (placeholder), and AnalyticsHandler into `internal/http/server.go` — add fields, getters/setters; add RegisterAnalyticsRoutes method; parse AnalyticsConfig in `cmd/api/main.go`; add analytics:view and analytics:manage to DefaultPermissions; add admin role permissions; wire repositories, service, handler; subscribe analyticsService.SubscribeToEventBus(eventBus); start/stop analyticsReaper and aggregationWorker in startup/shutdown lifecycle; update shutdown order: server → eventBus → activityReaper → analyticsReaper → aggregationWorker → webhookWorker → enforcer → db → redis

**Checkpoint**: At this point, metric event ingestion works (EventBus → MetricEvent table) and the dashboard endpoint returns data. Pre-aggregation is not yet implemented — the dashboard falls back to raw queries for ALL periods.

---

## Phase 4: User Story 3 — Query Time-Series Metrics (Priority: P2)

**Goal**: Enable time-series metric queries filtered by event type, period (hourly/daily/weekly/monthly), date range, and organization scope.

**Independent Test**: Call `GET /api/v1/analytics/metrics?type=user.created&period=daily&from=2026-04-01&to=2026-05-01` → verify response contains data points for each day in the range, with zero values for days with no events.

### Implementation for User Story 3

- [ ] T016 Implement MetricsQuery time-series generation in AnalyticsService.GetMetrics() in `internal/service/analytics.go` — generate all interval boundaries for the requested range (hourly=hours, daily=days, weekly=weeks starting Monday, monthly=months starting 1st); query raw MetricEvents for the range filtered by event types, organization scope, and `archived_at IS NULL`; merge query results with interval boundaries, filling missing intervals with value=0; return MetricsTimeSeriesResponse with data_points sorted chronologically

**Checkpoint**: Time-series endpoint returns zero-filled data for any date range, filtered by event types and organization scope. Combined with Phase 3, both dashboard and time-series are functional.

---

## Phase 5: User Story 4 — Pre-Aggregated Dashboard Metrics (Priority: P2)

**Goal**: Background AggregationWorker that pre-computes dashboard metrics (daily active users, resource counts, top actions) on a 60-second interval so dashboard queries return instantly for completed periods.

**Independent Test**: Start AggregationWorker → wait for aggregation cycle → verify DashboardMetric rows exist with period_type=daily and correct calculated_at → call dashboard endpoint → verify it uses pre-aggregated data for completed periods (check response time improvement).

### Implementation for User Story 4

- [ ] T017 Create AggregationWorker in `internal/service/aggregation_worker.go` — includes AggregationWorker struct (metricEventRepo, dashboardRepo, config, log, ctx, cancel); NewAggregationWorker constructor; Start(ctx) launches goroutine with ticker at config.AggregationInterval; Stop() cancels context; run() method that: (1) for each period_type (daily, weekly, monthly), (2) finds max calculated_at per (metric_type, period_type) as watermark cursor, (3) determines unprocessed periods from watermark to now-minus-1-period, (4) for each unprocessed period iterates over distinct organization IDs (including NULL for global), (5) queries raw MetricEvents for that org/period with `archived_at IS NULL`, (6) computes aggregations (counts per event_type, distinct actor counts for DAU, success rates from metadata numerator/denominator), (7) upserts DashboardMetric rows with value=float64 and metadata JSONB for ratio metrics, (8) commits each period as individual transaction (transaction-per-period). The current incomplete period is NEVER pre-aggregated — it always falls back to raw queries.
- [ ] T018 Create AnalyticsReaper in `internal/service/analytics_reaper.go` — exact copy of ActivityReaper pattern with MetricEvent instead of Activity; includes AnalyticsReaper struct (repo, config, log, ctx, cancel); NewAnalyticsReaper constructor; Start(ctx) with ticker; Stop(); run() calls repo.ArchiveOlderThan(ctx, config.RetentionDays); logs count of archived events

**Checkpoint**: AggregationWorker pre-computes metrics every 60 seconds. AnalyticsReaper archives events older than 90 days. Dashboard endpoint now returns pre-aggregated data for completed periods and raw query data for the current period.

---

## Phase 6: User Story 5 — Dashboard Visibility Preferences (Priority: P3)

**Goal**: Allow org admins to toggle metric category visibility per organization via a preferences endpoint.

**Independent Test**: Call `PUT /api/v1/analytics/dashboard/preferences` with `{"metric_categories": {"webhook_metrics": false}}` → verify subsequent dashboard response excludes webhook_metrics → default (no preference row) returns all categories.

### Implementation for User Story 5

- [ ] T019 Implement GetPreferences and UpdatePreferences in AnalyticsService in `internal/service/analytics.go` — GetPreferences: query DashboardPreferenceRepository.FindByOrganization(orgID); if nil (no row exists), return default visibility (all categories visible); if row exists, return stored preferences. UpdatePreferences: validate metric_categories keys against known categories (user_activity, content_metrics, engagement_metrics, system_metrics); upsert preference row with UpdatedByUserID; return updated preference. Both methods require analytics:manage permission checked at handler level.

**Checkpoint**: Preferences endpoint works. Dashboard response respects preference toggles. All 5 user stories are functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Unit tests, AGENTS.md update, and final verification

- [ ] T020 [P] Write unit tests for AnalyticsService in `tests/unit/analytics_service_test.go` — covers: EventBus subscription and event ingestion, idempotent insertion (ON CONFLICT DO NOTHING for duplicate events), unmapped event types silently ignored, GetDashboard with org scope and global scope, GetDashboard with pre-aggregated data and raw fallback, GetDashboard zero-state (no events), GetMetrics with time-series zero-fill, GetMetrics for hourly/daily/weekly/monthly periods, GetPreferences default (no row), UpdatePreferences upsert, TriggerAggregation (current period only)
- [ ] T021 [P] Write unit tests for AggregationWorker in `tests/unit/aggregation_worker_test.go` — covers: Start/Stop lifecycle, ProcessCompletedPeriod creates DashboardMetric rows, SkipCurrentPeriod (incomplete period not pre-aggregated), ResumeAfterStop (picks up from calculated_at watermark), TransactionPerPeriod (partial failure doesn't roll back committed periods), RatioMetrics (success rates stored with numerator/denominator in metadata), OrganizationIteration (processes per-org metrics including global/NULL)
- [ ] T022 [P] Write unit tests for AnalyticsReaper in `tests/unit/analytics_reaper_test.go` — covers: Start/Stop lifecycle, ArchiveOlderThan sets archived_at on old events, Idempotent (already archived events not re-archived)
- [ ] T023 [P] Write unit tests for AnalyticsHandler in `tests/unit/analytics_handler_test.go` — covers: GetDashboard with valid auth returns 200, GetDashboard without analytics:view permission returns 403, GetDashboard without org header returns global metrics, GetMetrics with valid params returns 200, GetMetrics with invalid period returns 400, UpdatePreferences with analytics:manage returns 200, UpdatePermissions without analytics:manage returns 403, TriggerAggregation requires analytics:manage
- [ ] T024 Update AGENTS.md with Analytics Dashboard section — includes architecture diagram, key components table, code map, endpoints table, configuration environment variables, design decisions (idempotent ingestion, transaction-per-period, archived_at, AnalyticsMapping registry, handler-only permission enforcement)
- [ ] T025 Final verification: `go build ./...` and `go vet ./...` pass clean; all unit tests pass with `go test -v -race ./tests/unit/... -run TestAnalytics`; manually verify `make serve` starts without errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (migration must exist for entities)
- **US1+US2 (Phase 3)**: Depends on Phase 2 (entities, mapping, config, repositories)
- **US3 (Phase 4)**: Depends on Phase 3 (AnalyticsService.GetMetrics already partially implemented in T011)
- **US4 (Phase 5)**: Depends on Phase 2 (repositories) and Phase 3 (service wiring)
- **US5 (Phase 6)**: Depends on Phase 2 (DashboardPreferenceRepository) and Phase 3 (handler)
- **Polish (Phase 7)**: Depends on all implementation phases being complete

### User Story Dependencies

- **US1+US2 (P1)**: Can start after Phase 2 — No dependencies on other stories
- **US3 (P2)**: Minor dependency on US1+US2 (GetMetrics method extends the service created in T011)
- **US4 (P2)**: Can start after Phase 2 (repositories) — AggregationWorker is independent of handler/service
- **US5 (P3)**: Can start after Phase 2 (repositories) — DashboardPreference is independent

### Within Each User Story

- Models before services (T003-T006 before T011)
- Repositories before services (T008-T010 before T011)
- Services before handlers (T011 before T012)
- Core implementation before wiring (T011-T014 before T015)

### Parallel Opportunities

- T003, T004, T005, T006 can all run in parallel (different files, no dependencies)
- T008, T009, T010 can all run in parallel (different repositories)
- T013, T014 can run in parallel (request and response DTOs)
- T017, T018 can run in parallel (AggregationWorker and AnalyticsReaper are independent)
- T020, T021, T022, T023 can all run in parallel (different test files)

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Launch all domain entities in parallel:
Task: "Create MetricEvent entity in internal/domain/metric_event.go"
Task: "Create DashboardMetric entity in internal/domain/dashboard_metric.go"
Task: "Create DashboardPreference entity in internal/domain/dashboard_preference.go"
Task: "Create AnalyticsMapping registry in internal/domain/analytics_events.go"
Task: "Create AnalyticsConfig in internal/config/analytics.go"

# Then launch all repositories in parallel:
Task: "Create MetricEventRepository in internal/repository/metric_event.go"
Task: "Create DashboardMetricRepository in internal/repository/dashboard_metric.go"
Task: "Create DashboardPreferenceRepository in internal/repository/dashboard_preference.go"
```

## Parallel Example: Phase 7 (Tests)

```bash
# Launch all test files in parallel:
Task: "Write unit tests for AnalyticsService in tests/unit/analytics_service_test.go"
Task: "Write unit tests for AggregationWorker in tests/unit/aggregation_worker_test.go"
Task: "Write unit tests for AnalyticsReaper in tests/unit/analytics_reaper_test.go"
Task: "Write unit tests for AnalyticsHandler in tests/unit/analytics_handler_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1+2 Only)

1. Complete Phase 1: Setup (migration files)
2. Complete Phase 2: Foundational (entities, mapping, config, repositories)
3. Complete Phase 3: US1+US2 (service, handler, wiring)
4. **STOP and VALIDATE**: Test event ingestion and dashboard endpoint independently
5. Deploy/demo if ready — MVP delivers immediate value showing activity metrics

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1+US2 → Event ingestion + Dashboard endpoint (MVP!)
3. Add US3 → Time-series queries for trend analysis
4. Add US4 → Pre-aggregation worker for instant dashboard responses
5. Add US5 → Dashboard preferences for org admins
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1+US2 (service + handler)
   - Developer B: US4 (AggregationWorker + Reaper)
   - (US3 and US5 depend on service completion but can be parallelized after)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies — can be delegated in parallel
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- AnalyticsService does NOT call enforcer.Enforce() — permission enforcement is handler/middleware level only
- All MetricEvent repository queries MUST include `WHERE archived_at IS NULL` scope
- MetricEvent uses `gorm.DeletedAt` (not `*time.Time`) for automatic soft-delete scope
- MetricEvent intentionally omits `UpdatedAt` — events are never updated after creation
- UNIQUE constraint uses `(event_type, resource_id, date, hour)` not `(event_type, resource_id, timestamp)` — see plan design decision
- DashboardPreference does NOT auto-create rows — all categories visible by default when no preference row exists
- AggregationWorker skips the current incomplete period — always falls back to raw queries for today
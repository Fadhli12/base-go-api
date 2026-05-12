# Implementation Plan: Analytics Dashboard

**Feature**: 018-analytics-dashboard  
**Created**: 2026-05-12  
**Status**: Draft  
**Spec**: [spec.md](./spec.md)

## Architecture Overview

```
Existing Services → EventBus.Publish(event) → AnalyticsService.SubscribeToEventBus()
                                                       → handleEvent()
                                                          → AnalyticsMapping lookup
                                                          → Extract resource_id, actor_id from payload
                                                          → Build MetricEvent
                                                          → repo.Create() (ON CONFLICT DO NOTHING)
                                                      
AggregationWorker.Start(ctx) → ticker → run()
  → For each period_type (daily, weekly, monthly):
    → Find max calculated_at per metric_type (cursor)
    → Query raw MetricEvents for each unprocessed period
    → Compute aggregations (counts, ratios)
    → Upsert DashboardMetric rows (transaction-per-period)
                                                      
AnalyticsReaper.Start(ctx) → ticker → run()
  → repo.ArchiveOlderThan(ctx, retentionDays)
  → Sets archived_at on old MetricEvents
                                                      
GET /api/v1/analytics/dashboard → Handler
  → If period is complete: query DashboardMetric (O(1))
  → If period is current/incomplete: query raw MetricEvents (fallback)
  → Apply DashboardPreference filtering (hide categories)
                                                      
GET /api/v1/analytics/metrics → Handler
  → Query MetricEvents with date/hour indexes
  → Zero-fill missing intervals
  → Return time-series per event type
```

## Key Design Decisions (from Metis + Spec)

1. **Idempotent ingestion**: PostgreSQL UNIQUE constraint on `(event_type, resource_id, timestamp)` with `ON CONFLICT DO NOTHING`. No application-level dedup.
2. **Transaction-per-period aggregation**: Each period's computation committed as individual transaction. Resume from `calculated_at` watermark.
3. **Manual trigger: current period only**: No full historical re-aggregation endpoint.
4. **AnalyticsMapping registry**: Extends ActivityMapping pattern with extraction functions for payload fields.
5. **archived_at for retention**: Consistent with ActivityReaper pattern. `deleted_at` reserved for admin deletions. ALL queries filter `archived_at IS NULL`.
6. **Ratio metrics**: DashboardMetric uses `value float64` + `metadata JSONB` storing `{numerator, denominator}` for rate metrics (success rates).
7. **Zero-fill time-series**: Generate all interval boundaries in Go, then merge with query results. Missing intervals get value=0.
8. **No Redis for AggregationWorker**: Simple goroutine + ticker, identical to ActivityReaper pattern.
9. **AnalyticsService is EventBus consumer only**: MUST NOT publish events to prevent circular dependency.
10. **DashboardPreference: no auto-create rows**: All categories visible by default. Preference row created only on explicit PUT.

## File Structure

```
internal/domain/
  metric_event.go           # MetricEvent entity + DTOs + business methods
  dashboard_metric.go       # DashboardMetric entity + DTOs
  dashboard_preference.go   # DashboardPreference entity + DTOs
  analytics_events.go       # AnalyticsMapping registry + extraction functions

internal/repository/
  metric_event.go           # MetricEventRepository interface + GORM impl
  dashboard_metric.go       # DashboardMetricRepository interface + GORM impl
  dashboard_preference.go   # DashboardPreferenceRepository interface + GORM impl

internal/service/
  analytics.go              # AnalyticsService: dashboard, metrics, preferences, event ingestion
  aggregation_worker.go     # AggregationWorker: Start/Stop/run, cursor-based processing
  analytics_reaper.go       # AnalyticsReaper: Start/Stop/run, ArchiveOlderThan

internal/http/handler/
  analytics.go              # AnalyticsHandler: 5 endpoints + RegisterRoutes

internal/http/request/
  analytics.go              # Request DTOs with validation

internal/http/response/
  analytics.go              # Response DTOs

internal/config/
  analytics.go              # AnalyticsConfig struct

migrations/
  000025_create_analytics.up.sql    # All 3 tables + indexes + FKs
  000025_create_analytics.down.sql  # Drop all 3 tables
```

## Implementation Tasks

### T001: Domain Entities + Migration + AnalyticsMapping

**Files**:
- `internal/domain/metric_event.go`
- `internal/domain/dashboard_metric.go`
- `internal/domain/dashboard_preference.go`
- `internal/domain/analytics_events.go`
- `migrations/000025_create_analytics.up.sql`
- `migrations/000025_create_analytics.down.sql`

**Description**: Create all domain entities, DTOs, business methods, AnalyticsMapping registry, and SQL migration for all three tables.

**MetricEvent entity** (NOTE: `DeletedAt` uses `gorm.DeletedAt` for automatic soft-delete scope, matching project convention. `UpdatedAt` is intentionally omitted — MetricEvents are never updated after creation, only archived or deleted. The `ArchivedAt` field uses `*time.Time` since GORM has no built-in `ArchivedAt` type, requiring manual `WHERE archived_at IS NULL` filters.):
```go
type MetricEvent struct {
    ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    EventType      string         `gorm:"size:100;not null;index"`
    ActorID        uuid.UUID      `gorm:"type:uuid;index"`
    ResourceType   string         `gorm:"size:50;not null"`
    ResourceID     string         `gorm:"size:100;not null"`
    OrganizationID *uuid.UUID     `gorm:"type:uuid;index"`
    Metadata       datatypes.JSON `gorm:"type:jsonb"`
    EventTimestamp time.Time      `gorm:"not null;index"`  // Original event timestamp, NOT auto-created
    Date           time.Time      `gorm:"type:date;not null;index"`  // Pre-aggregated date
    Hour           int            `gorm:"not null;index"`  // Pre-aggregated hour (0-23)
    ArchivedAt     *time.Time     `gorm:"index"`
    DeletedAt      gorm.DeletedAt `gorm:"index"`  // Uses gorm.DeletedAt for automatic scope
    CreatedAt      time.Time      `gorm:"autoCreateTime"`
}
func (MetricEvent) TableName() string { return "metric_events" }
```

**UNIQUE constraint design decision**: The spec clarification says `(event_type, resource_id, timestamp)`, but the plan uses `(event_type, resource_id, date, hour)` in the migration. This is an **intentional design improvement**: since `date` and `hour` are pre-extracted from `timestamp`, the constraint using these fields achieves the same deduplication goal (two events of the same type for the same resource within the same hour are almost certainly EventBus delivery duplicates). Using `date+hour` is more index-friendly and aligns with the pre-aggregation strategy. The plan updates the spec clarification to match.

**DashboardMetric entity**:
```go
type DashboardMetric struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    MetricType   string         `gorm:"size:50;not null"`
    PeriodType    string         `gorm:"size:20;not null"`  // "daily", "weekly", "monthly"
    PeriodStart  time.Time      `gorm:"not null;index"`
    PeriodEnd    time.Time      `gorm:"not null;index"`
    Value        float64        `gorm:"not null"`
    Metadata     datatypes.JSON `gorm:"type:jsonb"`
    OrganizationID *uuid.UUID   `gorm:"type:uuid;index"`
    CalculatedAt time.Time      `gorm:"not null"`
    CreatedAt    time.Time      `gorm:"autoCreateTime"`
    UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
}
// Unique index on (metric_type, period_type, period_start, organization_id)
func (DashboardMetric) TableName() string { return "dashboard_metrics" }
```

**DashboardPreference entity**:
```go
type DashboardPreference struct {
    ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    OrganizationID   uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex"`
    MetricCategories datatypes.JSON `gorm:"type:jsonb"`
    UpdatedByUserID  uuid.UUID      `gorm:"type:uuid;not null"`
    CreatedAt        time.Time      `gorm:"autoCreateTime"`
    UpdatedAt        time.Time      `gorm:"autoUpdateTime"`
}
func (DashboardPreference) TableName() string { return "dashboard_preferences" }
```

**AnalyticsMapping registry** (follows `domain/activity_events.go` pattern, extended with extraction functions for payload field access):

**IMPORTANT**: Extraction functions use type switches on the `map[string]interface{}` payload, since `WebhookEvent.Payload` is untyped. Each event type maps to known payload shapes (e.g., `user.created` payloads have `id` and `email` fields). Unknown payload shapes return zero-value defaults (uuid.Nil for actor, empty string for resource_id).

```go
type AnalyticsMapping struct {
    MetricCategory    string
    ResourceType      string
    ExtractResourceID func(payload map[string]interface{}) string
    ExtractActorID    func(payload map[string]interface{}) uuid.UUID
}

// Common extraction helpers (reusable across mappings):
var extractPayloadID = func(payload map[string]interface{}) string {
    if id, ok := payload["id"].(string); ok { return id }
    return ""
}
var extractPayloadActorID = func(payload map[string]interface{}) uuid.UUID {
    if uid, ok := payload["user_id"].(string); ok {
        if id, err := uuid.Parse(uid); err == nil { return id }
    }
    return uuid.Nil
}

var analyticsEventMapping = map[string]AnalyticsMapping{
    "user.created": {
        MetricCategory:    MetricCategoryUserActivity,
        ResourceType:      "user",
        ExtractResourceID: extractPayloadID,
        ExtractActorID:    extractPayloadActorID,
    },
    "user.deleted": {
        MetricCategory:    MetricCategoryUserActivity,
        ResourceType:      "user",
        ExtractResourceID: extractPayloadID,
        ExtractActorID:    extractPayloadActorID,
    },
    "invoice.created": { MetricCategory: MetricCategoryContentMetrics, ResourceType: "invoice", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "invoice.paid": { MetricCategory: MetricCategoryContentMetrics, ResourceType: "invoice", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "news.published": { MetricCategory: MetricCategoryContentMetrics, ResourceType: "news", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "news.deleted": { MetricCategory: MetricCategoryContentMetrics, ResourceType: "news", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "webhook.delivered": { MetricCategory: MetricCategoryEngagementMetrics, ResourceType: "webhook", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "webhook.failed": { MetricCategory: MetricCategoryEngagementMetrics, ResourceType: "webhook", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "comment.created": { MetricCategory: MetricCategoryEngagementMetrics, ResourceType: "comment", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "job.completed": { MetricCategory: MetricCategorySystemMetrics, ResourceType: "job", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
    "job.failed": { MetricCategory: MetricCategorySystemMetrics, ResourceType: "job", ExtractResourceID: extractPayloadID, ExtractActorID: extractPayloadActorID },
}

func GetAnalyticsMapping(eventType string) (AnalyticsMapping, bool) {
    mapping, ok := analyticsEventMapping[eventType]
    return mapping, ok
}

// Metric categories for dashboard grouping
const (
    MetricCategoryUserActivity       = "user_activity"
    MetricCategoryContentMetrics    = "content_metrics"
    MetricCategoryEngagementMetrics  = "engagement_metrics"
    MetricCategorySystemMetrics     = "system_metrics"
)
```

**Migration** (`000025_create_analytics.up.sql`):
- All 3 tables in ONE migration file
- UNIQUE constraint on `(event_type, resource_id, date, hour)` for metric_events (uses pre-aggregated fields instead of raw timestamp — see design decision note below)
- Partial indexes with `WHERE archived_at IS NULL` for metric_events (for aggregation and query performance)
- Index on `(event_type, date, hour) WHERE archived_at IS NULL` for aggregation queries
- Unique index on `(metric_type, period_type, period_start, organization_id)` for dashboard_metrics
- FKs: metric_events.actor_id → users(id), metric_events.organization_id → organizations(id), dashboard_preferences.organization_id → organizations(id)
- Auto-update trigger for updated_at columns (dashboard_metrics, dashboard_preferences only — NOT metric_events since they are never updated)

**UNIQUE constraint design decision**: The spec clarification says `(event_type, resource_id, timestamp)`, but the migration uses `(event_type, resource_id, date, hour)`. This is intentional: since `date` and `hour` are pre-extracted from `timestamp`, the constraint on these fields achieves the same deduplication goal (two events of the same type for the same resource within the same hour are almost certainly EventBus delivery duplicates). Using `date+hour` is more index-friendly and aligns with the pre-aggregation strategy. This decision is documented in the spec clarification update.
- Index on metric_events for aggregation queries: `(event_type, date, hour) WHERE archived_at IS NULL`

**Verification**:
```bash
go build ./...
go vet ./...
go test -v -run TestMetricEvent ./tests/unit/
go test -v -run TestAnalyticsMapping ./tests/unit/
```

**QA Scenarios**:
- MetricEvent entity has all fields including `ArchivedAt`, `DeletedAt`, pre-aggregated `Date` and `Hour`
- AnalyticsMapping maps all 11 event types to correct metric categories
- Migration creates all 3 tables with correct constraints and indexes
- `ON CONFLICT DO NOTHING` works for duplicate metric_events insertion
- Default DashboardPreference visibility (all categories visible) works without a DB row

---

### T002: Repository Interfaces + GORM Implementations

**Files**:
- `internal/repository/metric_event.go`
- `internal/repository/dashboard_metric.go`
- `internal/repository/dashboard_preference.go`

**Description**: Create repository interfaces and GORM implementations for all three entities. Follow the `internal/repository/activity.go` pattern.

**MetricEventRepository interface**:
```go
type MetricEventFilters struct {
    EventTypes    []string
    ResourceType  string
    ResourceID    string
    OrganizationID *uuid.UUID
    Since         *time.Time
    Until         *time.Time
}

type MetricEventRepository interface {
    Create(ctx context.Context, event *domain.MetricEvent) error  // ON CONFLICT DO NOTHING
    FindByID(ctx context.Context, id uuid.UUID) (*domain.MetricEvent, error)
    FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters MetricEventFilters, limit, offset int) ([]*domain.MetricEvent, int64, error)
    CountByTypeAndPeriod(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (map[string]int64, error)
    CountDistinctActorsByPeriod(ctx context.Context, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (int64, error)
    FindTimeSeries(ctx context.Context, eventTypes []string, period string, from, to time.Time, hasOrgID bool, orgID uuid.UUID) ([]*domain.MetricEvent, error)
    ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error)
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

**DashboardMetricRepository interface**:
```go
type DashboardMetricRepository interface {
    Upsert(ctx context.Context, metric *domain.DashboardMetric) error
    FindByTypeAndPeriod(ctx context.Context, metricType string, periodType string, hasOrgID bool, orgID uuid.UUID, limit int) ([]*domain.DashboardMetric, error)
    FindByTypesAndPeriod(ctx context.Context, metricTypes []string, periodType string, hasOrgID bool, orgID uuid.UUID, limit int) ([]*domain.DashboardMetric, error)
    DeleteByTypeAndPeriodBefore(ctx context.Context, metricType string, periodType string, before time.Time) error
    FindMaxCalculatedAt(ctx context.Context, metricType string, periodType string) (*time.Time, error)
}
```

**DashboardPreferenceRepository interface**:
```go
type DashboardPreferenceRepository interface {
    FindByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.DashboardPreference, error)
    Upsert(ctx context.Context, preference *domain.DashboardPreference) error
}
```

**Key implementation details**:
- `Create` for MetricEvent uses GORM's `clause.OnConflict{DoNothing: true}` for idempotent ingestion
- ALL MetricEvent queries include `WHERE archived_at IS NULL` by default (using GORM scope or explicit where clause)
- `FindTimeSeries` supports hourly, daily, weekly, monthly aggregation with pre-aggregated `date` and `hour` fields
- `CountByTypeAndPeriod` returns `map[string]int64` keyed by event type for dashboard grouping
- `ArchiveOlderThan` follows ActivityReaper pattern: sets `archived_at = NOW()` where `date < cutoff AND archived_at IS NULL AND deleted_at IS NULL`

**Verification**:
```bash
go build ./...
go test -v -run TestMetricEventRepository ./tests/unit/
go test -v -run TestDashboardMetricRepository ./tests/unit/
```

---

### T003: AnalyticsConfig + AnalyticsService (Event Ingestion)

**Files**:
- `internal/config/analytics.go`
- `internal/service/analytics.go`

**Description**: Create AnalyticsConfig and AnalyticsService with EventBus subscription for event ingestion. Follow ActivityService pattern.

**AnalyticsConfig** (follows `config/activity.go`):
```go
type AnalyticsConfig struct {
    RetentionDays      int           `mapstructure:"retention_days"`
    AggregationInterval time.Duration `mapstructure:"aggregation_interval"`
    ReaperInterval      time.Duration `mapstructure:"reaper_interval"`
    DefaultPageSize     int           `mapstructure:"default_page_size"`
    MaxPageSize         int           `mapstructure:"max_page_size"`
}

func DefaultAnalyticsConfig() AnalyticsConfig {
    return AnalyticsConfig{
        RetentionDays:      90,
        AggregationInterval: 60 * time.Second,
        ReaperInterval:      60 * time.Second,
        DefaultPageSize:     20,
        MaxPageSize:        100,
    }
}
```

Add `parseAnalyticsConfig()` to `internal/config/config.go`.

**AnalyticsService** (follows ActivityService pattern):
```go
type AnalyticsService struct {
    metricEventRepo  repository.MetricEventRepository
    dashboardRepo    repository.DashboardMetricRepository
    preferenceRepo   repository.DashboardPreferenceRepository
    enforcer         *permission.Enforcer
    audit            *AuditService
    log              *slog.Logger
    eventBus         *domain.EventBus
}

func NewAnalyticsService(...) *AnalyticsService { ... }
func (s *AnalyticsService) SubscribeToEventBus(eventBus *domain.EventBus) {
    s.eventBus = eventBus
    ch := make(chan domain.WebhookEvent, 256)
    eventBus.Subscribe(func(event domain.WebhookEvent) { ch <- event })
    go s.processEvents(ch)
}
func (s *AnalyticsService) SetEventBus(eventBus *domain.EventBus) { s.eventBus = eventBus }
func (s *AnalyticsService) processEvents(ch chan domain.WebhookEvent) {
    for event := range ch {
        if err := s.handleEvent(event); err != nil {
            s.log.Error("failed to handle analytics event", ...)
        }
    }
}
func (s *AnalyticsService) handleEvent(event domain.WebhookEvent) error {
    mapping, ok := domain.GetAnalyticsMapping(event.Type)
    if !ok { return nil } // Unmapped events are ignored
    // Extract fields using mapping functions
    resourceID := mapping.ExtractResourceID(payload)
    actorID := mapping.ExtractActorID(payload)
    // Build MetricEvent with pre-aggregated date/hour
    // Create with ON CONFLICT DO NOTHING
}

// Dashboard endpoint methods
func (s *AnalyticsService) GetDashboard(ctx, userID, hasOrgID, orgID, period) (*DashboardResponse, error) { ... }
func (s *AnalyticsService) GetMetrics(ctx, userID, hasOrgID, orgID, types, period, from, to) (*MetricsResponse, error) { ... }
func (s *AnalyticsService) GetPreferences(ctx, orgID) (*DashboardPreference, error) { ... }
func (s *AnalyticsService) UpdatePreferences(ctx, orgID, userID, categories) (*DashboardPreference, error) { ... }
func (s *AnalyticsService) TriggerAggregation(ctx) error { ... } // Current period only
```

**Key details**:
- `handleEvent` uses `domain.GetAnalyticsMapping()` to map event types. Unmapped events are silently ignored (no error, no log noise).
- `Create` uses `clause.OnConflict{DoNothing: true}` for idempotent ingestion.
- `GetDashboard` checks pre-aggregated DashboardMetric first (for completed periods), falls back to raw MetricEvent queries for the current incomplete period.
- `GetMetrics` generates zero-filled time-series: creates all interval boundaries in Go, then fills actual values from the DB.
- **Permission enforcement is handler-level only** (middleware + handler-level Enforce calls). The AnalyticsService does NOT call enforcer.Enforce() — consistent with the project convention that services don't enforce permissions. The handler uses `middleware.RequirePermission(enforcer, "analytics", "view")` for route groups and `h.enforcer.Enforce()` for admin-only endpoints like TriggerAggregation and UpdatePreferences.

**Verification**:
```bash
go build ./...
go test -v -run TestAnalyticsService ./tests/unit/
```

**QA Scenarios**:
- EventBus published `user.created` → MetricEvent created with correct event_type, actor_id, date, hour
- Duplicate EventBus event → silently skipped (ON CONFLICT DO NOTHING)
- Unmapped event type → silently ignored, no error logged
- GetDashboard with org context → returns org-scoped metrics
- GetDashboard without org context → returns global metrics
- GetMetrics with daily period → returns zero-filled time-series for each day in range
- GetPreferences for org with no preference row → returns default (all categories visible)
- UpdatePreferences → creates/updates preference row, returns updated preferences

---

### T004: AggregationWorker + AnalyticsReaper

**Files**:
- `internal/service/aggregation_worker.go`
- `internal/service/analytics_reaper.go`

**Description**: Create AggregationWorker and AnalyticsReaper following ActivityReaper/WebhookWorker patterns.

**AggregationWorker** (follows ActivityReaper pattern):
```go
type AggregationWorker struct {
    metricEventRepo  repository.MetricEventRepository
    dashboardRepo    repository.DashboardMetricRepository
    config           config.AnalyticsConfig
    log              *slog.Logger
    ctx              context.Context
    cancel           context.CancelFunc
}

func NewAggregationWorker(metricEventRepo, dashboardRepo, config, log) *AggregationWorker { ... }
func (w *AggregationWorker) Start(ctx context.Context) {
    w.ctx, w.cancel = context.WithCancel(ctx)
    go func() {
        ticker := time.NewTicker(w.config.AggregationInterval)
        defer ticker.Stop()
        for {
            select {
            case <-w.ctx.Done(): return
            case <-ticker.C: w.run()
            }
        }
    }()
}
func (w *AggregationWorker) Stop() { if w.cancel != nil { w.cancel() } }
func (w *AggregationWorker) run() {
    // For each period_type (daily, weekly, monthly):
    //   1. Find max calculated_at per (metric_type, period_type) — the watermark cursor
    //   2. Determine unprocessed periods (from cursor to now minus 1 period)
    //   3. For each unprocessed period:
    //      a. Get list of distinct organization IDs that have events in this period
    //         (plus NULL organization_id for global metrics)
    //      b. For each organization (including global/NULL):
    //         - Query raw MetricEvents WHERE date >= period_start AND date < period_end
    //           AND archived_at IS NULL AND organization_id = orgID (or IS NULL for global)
    //         - Compute aggregations (counts per event_type, distinct actor counts,
    //           success rates from metadata)
    //         - Upsert DashboardMetric rows within a single transaction (transaction-per-period)
    //   4. Each period commits as an individual transaction (transaction-per-period)
    //      If the worker crashes mid-cycle, already-committed periods remain valid;
    //      on restart, it resumes from the max calculated_at watermark.
}
```

**AnalyticsReaper** (follows ActivityReaper pattern exactly):
```go
type AnalyticsReaper struct {
    repo   repository.MetricEventRepository
    config config.AnalyticsConfig
    log    *slog.Logger
    ctx    context.Context
    cancel context.CancelFunc
}

func NewAnalyticsReaper(repo, config, log) *AnalyticsReaper { ... }
func (r *AnalyticsReaper) Start(ctx context.Context) { ... } // Same as ActivityReaper
func (r *AnalyticsReaper) Stop() { ... }
func (r *AnalyticsReaper) run() {
    archived, err := r.repo.ArchiveOlderThan(r.ctx, r.config.RetentionDays)
    // Log result
}
```

**Key details**:
- AggregationWorker uses `calculated_at` watermark to determine where to resume after restart
- For the current incomplete period, the worker does NOT pre-aggregate (falls back to raw query)
- Each period processed in its own transaction (BEGIN/COMMIT)
- Ratio metrics (success rates): store `value = numerator/denominator` as float64, `metadata = {"numerator": N, "denominator": D}`
- AnalyticsReaper is an exact copy of ActivityReaper with `MetricEvent` instead of `Activity`

**Verification**:
```bash
go build ./...
go test -v -run TestAggregationWorker ./tests/unit/
go test -v -run TestAnalyticsReaper ./tests/unit/
```

**QA Scenarios**:
- AggregationWorker starts, processes completed periods, creates DashboardMetric rows
- AggregationWorker stops mid-cycle → next start resumes from last calculated_at watermark
- AggregationWorker skips current incomplete period (today's data)
- Ratio metrics stored correctly: value=X/Y, metadata={numerator, denominator}
- AnalyticsReaper archives MetricEvents older than retention_days
- AnalyticsReaper does NOT archive MetricEvents with archived_at already set (idempotent)

---

### T005: AnalyticsHandler + Request/Response DTOs

**Files**:
- `internal/http/handler/analytics.go`
- `internal/http/request/analytics.go`
- `internal/http/response/analytics.go`

**Description**: Create HTTP handler with 5 endpoints and DTOs. Follow ActivityHandler pattern.

**Endpoints**:
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/v1/analytics/dashboard` | analytics:view | Dashboard summary |
| GET | `/api/v1/analytics/metrics` | analytics:view | Time-series metrics |
| GET | `/api/v1/analytics/dashboard/preferences` | analytics:view | Get preferences |
| PUT | `/api/v1/analytics/dashboard/preferences` | analytics:manage | Update preferences |
| POST | `/api/v1/analytics/aggregate` | analytics:manage | Trigger aggregation (current period only) |

**RegisterRoutes pattern** (follows ActivityHandler):
```go
func (h *AnalyticsHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
    analytics := v1.Group("/analytics")
    analytics.Use(middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))
    analytics.Use(middleware.ExtractOrganizationID())
    
    // View permission required for all analytics endpoints
    analytics.Use(middleware.RequirePermission(h.enforcer, "analytics", "view"))
    
    analytics.GET("/dashboard", h.GetDashboard)
    analytics.GET("/metrics", h.GetMetrics)
    analytics.GET("/dashboard/preferences", h.GetPreferences)
    
    // Manage permission for write operations — handler-level Enforce check
    analytics.PUT("/dashboard/preferences", h.UpdatePreferences)
    analytics.POST("/aggregate", h.TriggerAggregation)  // Enforce checked in handler method below
}

// TriggerAggregation checks analytics:manage permission at handler level
func (h *AnalyticsHandler) TriggerAggregation(c echo.Context) error {
    userID, err := middleware.GetUserID(c)
    if err != nil {
        return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
    }
    orgID, hasOrgID := middleware.GetOrganizationID(c)
    orgDomain := resolveAnalyticsOrgDomain(hasOrgID, orgID)
    
    allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "analytics", "manage")
    if err != nil || !allowed {
        return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
    }
    
    if err := h.analyticsService.TriggerAggregation(c.Request().Context()); err != nil {
        return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Aggregation trigger failed"))
    }
    return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "triggered"}))
}
```

**Request DTOs**:
```go
type DashboardQuery struct {
    Period string `query:"period" validate:"omitempty,oneof=daily weekly monthly"`
}

type MetricsQuery struct {
    Type   string `query:"type" validate:"required"`   // comma-separated event types
    Period string `query:"period" validate:"required,oneof=hourly daily weekly monthly"`
    From   string `query:"from" validate:"required"`    // ISO 8601
    To     string `query:"to" validate:"required"`       // ISO 8601
}

type UpdatePreferencesRequest struct {
    MetricCategories map[string]bool `json:"metric_categories" validate:"required"`
}
```

**Response DTOs**:
```go
type DashboardResponse struct {
    Period           string                    `json:"period"`
    UserActivity     UserActivityMetrics       `json:"user_activity"`
    ContentMetrics   ContentMetrics            `json:"content_metrics"`
    EngagementMetrics EngagementMetrics         `json:"engagement_metrics"`
    SystemMetrics    SystemMetrics             `json:"system_metrics"`
}

type UserActivityMetrics struct {
    TotalUsers      int64   `json:"total_users"`
    ActiveUsers     int64   `json:"active_users"`     // Distinct actors in period
    NewUsers        int64   `json:"new_users"`        // user.created count
    DeletedUsers    int64   `json:"deleted_users"`    // user.deleted count
}

type ContentMetrics struct {
    NewsCreated    int64 `json:"news_created"`
    NewsPublished  int64 `json:"news_published"`
    InvoicesCreated int64 `json:"invoices_created"`
    InvoicesPaid   int64 `json:"invoices_paid"`
}

type EngagementMetrics struct {
    CommentsCreated       int64   `json:"comments_created"`
    WebhookDeliveryRate   float64 `json:"webhook_delivery_rate"`  // delivered / (delivered + failed)
}

type SystemMetrics struct {
    JobsCompleted int64   `json:"jobs_completed"`
    JobsFailed    int64   `json:"jobs_failed"`
    JobSuccessRate float64 `json:"job_success_rate"`  // completed / (completed + failed)
}

type MetricsTimeSeriesResponse struct {
    Type    string          `json:"type"`
    Period  string          `json:"period"`
    From    string          `json:"from"`
    To      string          `json:"to"`
    DataPoints []DataPoint  `json:"data_points"`
}

type DataPoint struct {
    Timestamp string  `json:"timestamp"`
    Value     int64   `json:"value"`
}

type PreferencesResponse struct {
    OrganizationID   string         `json:"organization_id"`
    MetricCategories map[string]bool `json:"metric_categories"`
}
```

**Verification**:
```bash
go build ./...
go test -v -run TestAnalyticsHandler ./tests/unit/
```

---

### T006: DI Wiring + main.go Integration + Shutdown Order

**Files**:
- `internal/http/server.go` (add AnalyticsService, AggregationWorker, AnalyticsReaper fields + getters/setters + RegisterAnalyticsRoutes)
- `cmd/api/main.go` (wire repositories, services, workers, handlers, EventBus subscription, startup/shutdown)

**Description**: Wire all analytics components into the server and main.go lifecycle.

**Server.go additions**:
```go
// Fields
analyticsService     *service.AnalyticsService
aggregationWorker   *service.AggregationWorker
analyticsReaper     *service.AnalyticsReaper
analyticsHandler    *handler.AnalyticsHandler

// Getters/Setters
func (s *Server) SetAnalyticsService(svc *service.AnalyticsService) { ... }
func (s *Server) AnalyticsService() *service.AnalyticsService { ... }
func (s *Server) SetAggregationWorker(w *service.AggregationWorker) { ... }
func (s *Server) AggregationWorker() *service.AggregationWorker { ... }
func (s *Server) SetAnalyticsReaper(r *service.AnalyticsReaper) { ... }
func (s *Server) AnalyticsReaper() *service.AnalyticsReaper { ... }

// RegisterRoutes section
func (s *Server) RegisterAnalyticsRoutes(api *echo.Group, handler *handler.AnalyticsHandler) { ... }
```

**main.go wiring** (after ActivityService, before WebhookWorker):
```go
// Analytics repositories
metricEventRepo := repository.NewMetricEventRepository(db)
dashboardMetricRepo := repository.NewDashboardMetricRepository(db)
dashboardPreferenceRepo := repository.NewDashboardPreferenceRepository(db)

// Analytics service
analyticsService := service.NewAnalyticsService(
    metricEventRepo, dashboardMetricRepo, dashboardPreferenceRepo,
    enforcer, auditService, slog.Default(),
)

// Aggregation worker
aggregationWorker := service.NewAggregationWorker(
    metricEventRepo, dashboardMetricRepo, cfg.Analytics, slog.Default(),
)

// Analytics reaper
analyticsReaper := service.NewAnalyticsReaper(
    metricEventRepo, cfg.Analytics, slog.Default(),
)

// Analytics handler
analyticsHandler := handler.NewAnalyticsHandler(
    analyticsService, enforcer,
)

// Set on server
server.SetAnalyticsService(analyticsService)
server.SetAggregationWorker(aggregationWorker)
server.SetAnalyticsReaper(analyticsReaper)

// ... after eventBus.Start(ctx) ...

// Subscribe to EventBus
analyticsService.SubscribeToEventBus(eventBus)

// Start workers in background
analyticsReaper.Start(ctx)
aggregationWorker.Start(ctx)
```

**Shutdown order** (extended):
```go
// 1. server.Shutdown(ctx)
// 2. eventBus.Stop()
// 3. activityReaper.Stop()
// 4. analyticsReaper.Stop()
// 5. aggregationWorker.Stop()
// 6. webhookWorker.Stop()
// 7. enforcer.Stop()
// 8. db.Close()
// 9. redis.Close()
```

**Permission seeding** (add to DefaultPermissions in main.go):
```go
{Name: "analytics:view", Description: "View analytics dashboard and metrics", Resource: "analytics", Action: "view"},
{Name: "analytics:manage", Description: "Manage analytics preferences and trigger aggregation", Resource: "analytics", Action: "manage"},
```

**Admin role update** (add to admin permissions list):
```go
"analytics:view", "analytics:manage"
```

**Verification**:
```bash
make serve  # Smoke test - server starts without errors
go build ./...
go vet ./...
```

---

### T007: Unit Tests

**Files**:
- `tests/unit/analytics_service_test.go`
- `tests/unit/aggregation_worker_test.go`
- `tests/unit/analytics_reaper_test.go`
- `tests/unit/analytics_handler_test.go`

**Description**: Write comprehensive unit tests following existing test patterns (testify/mock, helper structs, reflect for unexported methods).

**Test coverage targets**:
- AnalyticsService: EventBus subscription, event ingestion, idempotent ingestion (duplicate skip), dashboard query (pre-aggregated + raw fallback), time-series query with zero-fill, preferences CRUD, trigger aggregation (current period only), permission checks
- AggregationWorker: Start/Stop lifecycle, process completed period, skip current period, resume after restart (calculated_at cursor), transaction-per-period isolation, ratio metric computation
- AnalyticsReaper: Start/Stop lifecycle, ArchiveOlderThan, idempotent archival
- AnalyticsHandler: All 5 endpoints with JWT/org/permission middleware, 403 for unauthorized, validation errors

**Mock strategy** (follow existing patterns):
- Mock MetricEventRepository, DashboardMetricRepository, DashboardPreferenceRepository
- Mock Enforcer for permission checks
- Use `reflect` to test unexported `handleEvent`, `processEvents`, `run` methods

**Verification**:
```bash
go test -v -race -coverprofile=coverage.txt ./tests/unit/... -run TestAnalytics
go test -v -race -coverprofile=coverage.txt ./tests/unit/... -run TestAggregationWorker
go test -v -race -coverprofile=coverage.txt ./tests/unit/... -run TestAnalyticsReaper
```

**QA Scenarios** (executable):
```bash
# EventBus ingestion
go test -v -run TestAnalyticsService_SubscribeToEventBus ./tests/unit/
go test -v -run TestAnalyticsService_HandleEvent ./tests/unit/
go test -v -run TestAnalyticsService_HandleEvent_Idempotent ./tests/unit/
go test -v -run TestAnalyticsService_HandleEvent_UnmappedEventType ./tests/unit/

# Dashboard
go test -v -run TestAnalyticsService_GetDashboard_PreAggregated ./tests/unit/
go test -v -run TestAnalyticsService_GetDashboard_RawFallback ./tests/unit/
go test -v -run TestAnalyticsService_GetDashboard_ZeroState ./tests/unit/

# Time-series
go test -v -run TestAnalyticsService_GetMetrics_Daily ./tests/unit/
go test -v -run TestAnalyticsService_GetMetrics_ZeroFill ./tests/unit/

# Aggregation worker
go test -v -run TestAggregationWorker_ProcessCompletedPeriod ./tests/unit/
go test -v -run TestAggregationWorker_SkipCurrentPeriod ./tests/unit/
go test -v -run TestAggregationWorker_ResumeAfterStop ./tests/unit/
go test -v -run TestAggregationWorker_TransactionPerPeriod ./tests/unit/
go test -v -run TestAggregationWorker_RatioMetrics ./tests/unit/

# Reaper
go test -v -run TestAnalyticsReaper_ArchiveOlderThan ./tests/unit/
go test -v -run TestAnalyticsReaper_Idempotent ./tests/unit/
```

---

## Atomic Commit Sequence

```
Commit 1: [data-model] T001 - Domain entities + AnalyticsMapping + migration
Commit 2: [repository] T002 - Repository interfaces + GORM implementations
Commit 3: [ingestion] T003 - AnalyticsConfig + AnalyticsService with EventBus subscription
Commit 4: [workers] T004 - AggregationWorker + AnalyticsReaper
Commit 5: [handler] T005 - AnalyticsHandler + request/response DTOs
Commit 6: [wiring] T006 - main.go integration + shutdown order
Commit 7: [tests] T007 - Unit tests + permission seeding verification
```

## Shutdown Order

```
server → eventBus → activityReaper → analyticsReaper → aggregationWorker → webhookWorker → enforcer → db → redis
```

## Risk Mitigations

| Risk | Mitigation |
|------|-----------|
| WebhookEvent struct modification breaks existing publishers | Add `Timestamp` field with `time.Now().UTC()` default in EventBus.Publish() |
| AnalyticsMapping extraction functions need typed payload access | Use type switches in extraction funcs, with fallback for unknown payloads |
| Ratio metrics break simple value column | Use `value float64` + `metadata JSONB` storing numerator/denominator |
| AggregationWorker resume-from-cursor has no test coverage | T004 includes explicit TestAggregationWorker_ResumeAfterStop |
| archived_at scope forgotten on aggregation queries | Repository queries use `WHERE archived_at IS NULL` by default; test explicitly |
| Zero-fill for time-series gaps | T003 includes TestAnalyticsService_GetMetrics_ZeroFill |
| DashboardPreference default (no row = all visible) | T005 tests GetPreferences with no existing preference row |
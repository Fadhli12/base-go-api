package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// --- Response types ---

// DashboardResponse represents the full dashboard data for an organization.
type DashboardResponse struct {
	Period            string                `json:"period"`
	UserActivity     *UserActivityMetrics  `json:"user_activity"`
	ContentMetrics   *ContentMetrics       `json:"content_metrics"`
	EngagementMetrics *EngagementMetricsData `json:"engagement_metrics"`
	SystemMetrics    *SystemMetricsData    `json:"system_metrics"`
}

// UserActivityMetrics represents user activity category metrics.
type UserActivityMetrics struct {
	TotalUsers  int64 `json:"total_users"`
	NewUsers   int64 `json:"new_users"`
	ActiveUsers int64 `json:"active_users"`
}

// ContentMetrics represents content category metrics.
type ContentMetrics struct {
	InvoicesCreated int64 `json:"invoices_created"`
	InvoicesPaid   int64 `json:"invoices_paid"`
	NewsPublished  int64 `json:"news_published"`
	MediaUploaded  int64 `json:"media_uploaded"`
	MediaVersioned int64 `json:"media_versioned"`
}

// EngagementMetricsData represents engagement category metrics.
type EngagementMetricsData struct {
	CommentsCreated int64 `json:"comments_created"`
}

// SystemMetricsData represents system category metrics.
type SystemMetricsData struct {
	LoginSuccesses int64 `json:"login_successes"`
	LoginFailures  int64 `json:"login_failures"`
}

// MetricsTimeSeriesResponse represents time-series metric data.
type MetricsTimeSeriesResponse struct {
	EventType  string              `json:"event_type"`
	Period     string              `json:"period"`
	From       string              `json:"from"`
	To        string              `json:"to"`
	DataPoints []MetricsDataPoint  `json:"data_points"`
}

// MetricsDataPoint represents a single data point in a time series.
type MetricsDataPoint struct {
	Timestamp string  `json:"timestamp"`
	Value    float64 `json:"value"`
}

// --- Service ---

// AnalyticsService handles analytics dashboard business logic including
// event ingestion, dashboard metrics, time-series data, and preferences.
type AnalyticsService struct {
	metricEventRepo repository.MetricEventRepository
	dashboardRepo  repository.DashboardMetricRepository
	preferenceRepo  repository.DashboardPreferenceRepository
	enforcer       *permission.Enforcer
	audit         *AuditService
	log           *slog.Logger
	config        config.AnalyticsConfig
	eventBus      *domain.EventBus
	cancel        context.CancelFunc
}

// NewAnalyticsService creates a new AnalyticsService instance.
func NewAnalyticsService(
	metricEventRepo repository.MetricEventRepository,
	dashboardRepo repository.DashboardMetricRepository,
	preferenceRepo repository.DashboardPreferenceRepository,
	enforcer *permission.Enforcer,
	audit *AuditService,
	log *slog.Logger,
	config config.AnalyticsConfig,
) *AnalyticsService {
	return &AnalyticsService{
		metricEventRepo: metricEventRepo,
		dashboardRepo:  dashboardRepo,
		preferenceRepo:  preferenceRepo,
		enforcer:       enforcer,
		audit:         audit,
		log:           log,
		config:        config,
	}
}

// SetEventBus sets the event bus (post-construction injection).
func (s *AnalyticsService) SetEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
}

// SubscribeToEventBus registers the service as a handler on the given EventBus.
// Uses buffered channel pattern to decouple event publishing from processing.
func (s *AnalyticsService) SubscribeToEventBus(eventBus *domain.EventBus) {
	s.eventBus = eventBus
	ch := make(chan domain.WebhookEvent, 256)
	eventBus.Subscribe(func(event domain.WebhookEvent) {
		ch <- event
	})
	go s.processEvents(ch)
}

// processEvents consumes events from the buffered channel in a background goroutine.
func (s *AnalyticsService) processEvents(ch chan domain.WebhookEvent) {
	for event := range ch {
		s.handleEvent(context.Background(), event)
	}
}

// handleEvent processes a single event from the event bus.
// Uses AnalyticsMapping to extract fields; unmapped events are silently ignored.
func (s *AnalyticsService) handleEvent(ctx context.Context, event domain.WebhookEvent) {
	mapping, ok := domain.GetAnalyticsMapping(event.Type)
	if !ok {
		// Unmapped event type — silently ignore (not an error)
		return
	}

	// Convert payload to map[string]interface{} for mapping functions
	var payloadMap map[string]interface{}
	if m, ok := event.Payload.(map[string]interface{}); ok {
		payloadMap = m
	}

	// Extract fields using mapping functions
	resourceID := mapping.ExtractResourceID(payloadMap)
	actorID, _ := mapping.ExtractActorID(payloadMap)

	// Marshal payload to JSONB metadata
	metadataBytes, err := json.Marshal(event.Payload)
	if err != nil {
		s.log.Error("failed to marshal event payload",
			slog.String("event_type", event.Type),
			slog.String("error", err.Error()),
		)
		metadataBytes = nil
	}

	now := time.Now()
	metricEvent := &domain.MetricEvent{
		EventType:      event.Type,
		ActorID:        actorID,
		ResourceType:  mapping.ResourceType,
		ResourceID:    resourceID,
		OrganizationID: event.OrgID,
		Metadata:      datatypes.JSON(metadataBytes),
		EventTimestamp: now,
		Date:          now.Truncate(24 * time.Hour),
		Hour:         now.Hour(),
	}

	if err := s.metricEventRepo.Create(ctx, metricEvent); err != nil {
		s.log.Error("failed to create metric event",
			slog.String("event_type", event.Type),
			slog.String("error", err.Error()),
		)
		return
	}

	s.log.Info("metric event created",
		slog.String("event_type", event.Type),
		slog.String("resource_type", mapping.ResourceType),
		slog.String("resource_id", resourceID),
	)
}

// GetDashboard returns aggregated dashboard data for the given organization context.
// Permission enforcement is handler-level — this method does NOT call enforcer.Enforce().
func (s *AnalyticsService) GetDashboard(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	period string,
) (*DashboardResponse, error) {
	// Validate and default period
	if !domain.ValidatePeriodType(period) {
		if period == "" {
			period = domain.MetricPeriodDaily
		} else {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "period must be daily, weekly, or monthly", 400)
		}
	}

	// Compute period range
	now := time.Now()
	var periodStart time.Time
	switch period {
	case domain.MetricPeriodDaily:
		periodStart = now.AddDate(0, 0, -30) // last 30 days
	case domain.MetricPeriodWeekly:
		periodStart = now.AddDate(0, 0, -7*12) // last 12 weeks
	case domain.MetricPeriodMonthly:
		periodStart = now.AddDate(0, -12, 0) // last 12 months
	}

	// Fetch preferences to filter categories
	showCategories, err := s.getVisibleCategories(ctx, hasOrgID, orgID)
	if err != nil {
		s.log.Warn("failed to fetch preferences, showing all categories",
			slog.String("error", err.Error()),
		)
		showCategories = domain.DefaultMetricCategories()
	}

	// Compute each category
	var userActivity *UserActivityMetrics
	if showCategories[domain.MetricCategoryUserActivity] {
		userActivity, err = s.computeUserActivity(ctx, hasOrgID, orgID, periodStart, now)
		if err != nil {
			s.log.Error("failed to compute user activity metrics",
				slog.String("error", err.Error()),
			)
			userActivity = &UserActivityMetrics{} // zero-value fallback
		}
	}

	var contentMetrics *ContentMetrics
	if showCategories[domain.MetricCategoryContentMetrics] {
		contentMetrics, err = s.computeContentMetrics(ctx, hasOrgID, orgID, periodStart, now)
		if err != nil {
			s.log.Error("failed to compute content metrics",
				slog.String("error", err.Error()),
			)
			contentMetrics = &ContentMetrics{} // zero-value fallback
		}
	}

	var engagementMetrics *EngagementMetricsData
	if showCategories[domain.MetricCategoryEngagementMetrics] {
		engagementMetrics, err = s.computeEngagementMetrics(ctx, hasOrgID, orgID, periodStart, now)
		if err != nil {
			s.log.Error("failed to compute engagement metrics",
				slog.String("error", err.Error()),
			)
			engagementMetrics = &EngagementMetricsData{} // zero-value fallback
		}
	}

	var systemMetrics *SystemMetricsData
	if showCategories[domain.MetricCategorySystemMetrics] {
		systemMetrics, err = s.computeSystemMetrics(ctx, hasOrgID, orgID, periodStart, now)
		if err != nil {
			s.log.Error("failed to compute system metrics",
				slog.String("error", err.Error()),
			)
			systemMetrics = &SystemMetricsData{} // zero-value fallback
		}
	}

	return &DashboardResponse{
		Period:            period,
		UserActivity:     userActivity,
		ContentMetrics:   contentMetrics,
		EngagementMetrics: engagementMetrics,
		SystemMetrics:    systemMetrics,
	}, nil
}

// GetMetrics returns time-series metric data for the given filters.
// Generates zero-filled intervals for the requested date range.
func (s *AnalyticsService) GetMetrics(
	ctx context.Context,
	userID uuid.UUID,
	hasOrgID bool,
	orgID uuid.UUID,
	eventTypes []string,
	period string,
	from, to time.Time,
) (*MetricsTimeSeriesResponse, error) {
	// Validate period
	if !domain.ValidatePeriodType(period) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "period must be hourly, daily, weekly, or monthly", 400)
	}

	if len(eventTypes) == 0 {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "at least one event type is required", 400)
	}

	// Default date range: last 7 days
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -7)
	}
	if to.IsZero() {
		to = time.Now()
	}

	// Query raw metric events for time-series data
	points, err := s.metricEventRepo.FindTimeSeries(ctx, eventTypes, hasOrgID, orgID, from, to, period)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Generate all interval boundaries for zero-fill
	dataPoints := s.GenerateTimeSeriesDataPoints(points, from, to, period, eventTypes)

	return &MetricsTimeSeriesResponse{
		EventType:  eventTypes[0], // Primary event type for display
		Period:     period,
		From:       from.Format(time.RFC3339),
		To:         to.Format(time.RFC3339),
		DataPoints: dataPoints,
	}, nil
}

// GetPreferences returns the dashboard preferences for the given organization.
// Returns default visibility when no preference row exists.
func (s *AnalyticsService) GetPreferences(ctx context.Context, hasOrgID bool, orgID uuid.UUID) (map[string]bool, error) {
	if !hasOrgID {
		return domain.DefaultMetricCategories(), nil
	}

	pref, err := s.preferenceRepo.FindByOrganization(ctx, orgID)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	if pref == nil {
		return domain.DefaultMetricCategories(), nil
	}
	return pref.ToResponse().MetricCategories, nil
}

// UpdatePreferences upserts dashboard preferences for the given organization.
func (s *AnalyticsService) UpdatePreferences(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	categories map[string]bool,
) (*domain.DashboardPreferenceResponse, error) {
	// Validate category keys against known categories
	validCategories := domain.DefaultMetricCategories()
	filtered := make(map[string]bool, len(categories))
	for k, v := range categories {
		if _, ok := validCategories[k]; ok {
			filtered[k] = v
		}
	}

	categoriesJSON, err := json.Marshal(filtered)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	pref := &domain.DashboardPreference{
		OrganizationID:  orgID,
		MetricCategories: datatypes.JSON(categoriesJSON),
		UpdatedByUserID: userID,
	}

	if err := s.preferenceRepo.Upsert(ctx, pref); err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Re-fetch to get the complete record with timestamps
	saved, err := s.preferenceRepo.FindByOrganization(ctx, orgID)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	response := saved.ToResponse()
	return &response, nil
}

// --- Private helper methods ---

// getVisibleCategories returns the visibility map for metric categories.
// Falls back to defaults if no preference exists.
func (s *AnalyticsService) getVisibleCategories(ctx context.Context, hasOrgID bool, orgID uuid.UUID) (map[string]bool, error) {
	if !hasOrgID {
		return domain.DefaultMetricCategories(), nil
	}

	pref, err := s.preferenceRepo.FindByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if pref == nil {
		return domain.DefaultMetricCategories(), nil
	}
	return pref.ToResponse().MetricCategories, nil
}

// computeUserActivity computes user activity metrics from raw events.
func (s *AnalyticsService) computeUserActivity(
	ctx context.Context,
	hasOrgID bool,
	orgID uuid.UUID,
	from, to time.Time,
) (*UserActivityMetrics, error) {
	// Total users = count of user.created events
	totalCounts, err := s.metricEventRepo.CountByTypeAndPeriod(
		ctx, []string{domain.MetricEventTypeUserCreated}, hasOrgID, orgID, from, to,
	)
	if err != nil {
		return nil, err
	}

	totalUsers := totalCounts[domain.MetricEventTypeUserCreated]

	// New users = same as total for this period
	newUsers := totalUsers

	// Active users = distinct actor count
	activeUsers, err := s.metricEventRepo.CountDistinctActorsByPeriod(ctx, hasOrgID, orgID, from, to)
	if err != nil {
		return nil, err
	}

	return &UserActivityMetrics{
		TotalUsers:  totalUsers,
		NewUsers:    newUsers,
		ActiveUsers: activeUsers,
	}, nil
}

// computeContentMetrics computes content metrics from raw events.
func (s *AnalyticsService) computeContentMetrics(
	ctx context.Context,
	hasOrgID bool,
	orgID uuid.UUID,
	from, to time.Time,
) (*ContentMetrics, error) {
	eventTypes := []string{
		domain.MetricEventTypeInvoiceCreated,
		domain.MetricEventTypeInvoicePaid,
		domain.MetricEventTypeNewsPublished,
		domain.MetricEventTypeMediaUploaded,
		domain.MetricEventTypeFileVersioned,
	}

	counts, err := s.metricEventRepo.CountByTypeAndPeriod(ctx, eventTypes, hasOrgID, orgID, from, to)
	if err != nil {
		return nil, err
	}

	return &ContentMetrics{
		InvoicesCreated: counts[domain.MetricEventTypeInvoiceCreated],
		InvoicesPaid:   counts[domain.MetricEventTypeInvoicePaid],
		NewsPublished:  counts[domain.MetricEventTypeNewsPublished],
		MediaUploaded:  counts[domain.MetricEventTypeMediaUploaded],
		MediaVersioned: counts[domain.MetricEventTypeFileVersioned],
	}, nil
}

// computeEngagementMetrics computes engagement metrics from raw events.
func (s *AnalyticsService) computeEngagementMetrics(
	ctx context.Context,
	hasOrgID bool,
	orgID uuid.UUID,
	from, to time.Time,
) (*EngagementMetricsData, error) {
	counts, err := s.metricEventRepo.CountByTypeAndPeriod(
		ctx, []string{domain.MetricEventTypeCommentCreated}, hasOrgID, orgID, from, to,
	)
	if err != nil {
		return nil, err
	}

	return &EngagementMetricsData{
		CommentsCreated: counts[domain.MetricEventTypeCommentCreated],
	}, nil
}

// computeSystemMetrics computes system metrics from raw events.
func (s *AnalyticsService) computeSystemMetrics(
	ctx context.Context,
	hasOrgID bool,
	orgID uuid.UUID,
	from, to time.Time,
) (*SystemMetricsData, error) {
	eventTypes := []string{
		domain.MetricEventTypeLoginSuccess,
		domain.MetricEventTypeLoginFailed,
	}

	counts, err := s.metricEventRepo.CountByTypeAndPeriod(ctx, eventTypes, hasOrgID, orgID, from, to)
	if err != nil {
		return nil, err
	}

	return &SystemMetricsData{
		LoginSuccesses: counts[domain.MetricEventTypeLoginSuccess],
		LoginFailures:  counts[domain.MetricEventTypeLoginFailed],
	}, nil
}

// GenerateTimeSeriesDataPoints generates zero-filled time series data points.
// Exported for testing purposes.
func (s *AnalyticsService) GenerateTimeSeriesDataPoints(
	points []*domain.MetricTimeSeriesPoint,
	from, to time.Time,
	period string,
	eventTypes []string,
) []MetricsDataPoint {
	// Build a lookup map from existing points
	pointMap := make(map[string]float64)
	for _, p := range points {
		key := p.Date // date string like "2006-01-02" or "2006-01-02T15:04"
		if p.Hour != nil {
			key = p.Date // hour is embedded in the point
		}
		pointMap[key] = float64(p.Count)
	}

	// Generate all intervals for zero-fill
	var dataPoints []MetricsDataPoint
	current := from

	switch period {
	case "hourly":
		for current.Before(to) || current.Equal(to) {
			key := current.Format("2006-01-02T15:04")
			value := pointMap[key]
			dataPoints = append(dataPoints, MetricsDataPoint{
				Timestamp: current.Format(time.RFC3339),
				Value:    value,
			})
			current = current.Add(time.Hour)
		}
	case "daily":
		for current.Before(to) || current.Equal(to) {
			key := current.Format("2006-01-02")
			value := pointMap[key]
			dataPoints = append(dataPoints, MetricsDataPoint{
				Timestamp: current.Format(time.RFC3339),
				Value:    value,
			})
			current = current.AddDate(0, 0, 1)
		}
	case "weekly":
		// Align to Monday
		for current.Weekday() != time.Monday && current.Before(to) {
			current = current.AddDate(0, 0, 1)
		}
		for current.Before(to) || current.Equal(to) {
			key := current.Format("2006-01-02")
			value := pointMap[key]
			dataPoints = append(dataPoints, MetricsDataPoint{
				Timestamp: current.Format(time.RFC3339),
				Value:    value,
			})
			current = current.AddDate(0, 0, 7)
		}
	case "monthly":
		current = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
		for current.Before(to) || current.Equal(to) {
			key := current.Format("2006-01-02")
			value := pointMap[key]
			dataPoints = append(dataPoints, MetricsDataPoint{
				Timestamp: current.Format(time.RFC3339),
				Value:    value,
			})
			current = current.AddDate(0, 1, 0)
		}
	}

	if len(dataPoints) == 0 {
		dataPoints = []MetricsDataPoint{}
	}

	return dataPoints
}
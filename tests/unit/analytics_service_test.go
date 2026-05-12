package unit

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// ======================================================================
// Mock implementations (prefixed with ASvc to avoid collision with
// analytics_reaper_worker_test.go in the same package)
// ======================================================================

// ASvcMockMetricEventRepo implements repository.MetricEventRepository
type ASvcMockMetricEventRepo struct {
	mock.Mock
	mu     sync.Mutex
	events map[uuid.UUID]*domain.MetricEvent
}

func newASvcMockMetricEventRepo() *ASvcMockMetricEventRepo {
	return &ASvcMockMetricEventRepo{
		events: make(map[uuid.UUID]*domain.MetricEvent),
	}
}

// Compile-time interface check
var _ repository.MetricEventRepository = (*ASvcMockMetricEventRepo)(nil)

func (m *ASvcMockMetricEventRepo) Create(ctx context.Context, event *domain.MetricEvent) error {
	args := m.Called(ctx, event)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.events[event.ID] = event
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *ASvcMockMetricEventRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.MetricEvent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MetricEvent), args.Error(1)
}

func (m *ASvcMockMetricEventRepo) FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters repository.MetricEventFilters, limit, offset int) ([]*domain.MetricEvent, int64, error) {
	args := m.Called(ctx, hasOrgID, orgID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.MetricEvent), args.Get(1).(int64), args.Error(2)
}

func (m *ASvcMockMetricEventRepo) CountByTypeAndPeriod(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (map[string]int64, error) {
	args := m.Called(ctx, eventTypes, hasOrgID, orgID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *ASvcMockMetricEventRepo) CountDistinctActorsByPeriod(ctx context.Context, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (int64, error) {
	args := m.Called(ctx, hasOrgID, orgID, from, to)
	return args.Get(0).(int64), args.Error(1)
}

func (m *ASvcMockMetricEventRepo) FindTimeSeries(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time, periodType string) ([]*domain.MetricTimeSeriesPoint, error) {
	args := m.Called(ctx, eventTypes, hasOrgID, orgID, from, to, periodType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MetricTimeSeriesPoint), args.Error(1)
}

func (m *ASvcMockMetricEventRepo) ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	args := m.Called(ctx, retentionDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *ASvcMockMetricEventRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *ASvcMockMetricEventRepo) FindDistinctOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

// ASvcMockDashboardMetricRepo implements repository.DashboardMetricRepository
type ASvcMockDashboardMetricRepo struct {
	mock.Mock
}

func newASvcMockDashboardMetricRepo() *ASvcMockDashboardMetricRepo {
	return &ASvcMockDashboardMetricRepo{}
}

// Compile-time interface check
var _ repository.DashboardMetricRepository = (*ASvcMockDashboardMetricRepo)(nil)

func (m *ASvcMockDashboardMetricRepo) Upsert(ctx context.Context, metric *domain.DashboardMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *ASvcMockDashboardMetricRepo) FindByTypeAndPeriod(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (*domain.DashboardMetric, error) {
	args := m.Called(ctx, metricType, periodType, hasOrgID, orgID, periodStart, periodEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DashboardMetric), args.Error(1)
}

func (m *ASvcMockDashboardMetricRepo) FindByTypesAndPeriod(ctx context.Context, metricTypes []string, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*domain.DashboardMetric, error) {
	args := m.Called(ctx, metricTypes, periodType, hasOrgID, orgID, periodStart, periodEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DashboardMetric), args.Error(1)
}

func (m *ASvcMockDashboardMetricRepo) DeleteByTypeAndPeriodBefore(ctx context.Context, metricType, periodType string, before time.Time) error {
	args := m.Called(ctx, metricType, periodType, before)
	return args.Error(0)
}

func (m *ASvcMockDashboardMetricRepo) FindMaxCalculatedAt(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID) (*time.Time, error) {
	args := m.Called(ctx, metricType, periodType, hasOrgID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

// ASvcMockDashboardPrefRepo implements repository.DashboardPreferenceRepository
type ASvcMockDashboardPrefRepo struct {
	mock.Mock
	mu          sync.Mutex
	preferences map[uuid.UUID]*domain.DashboardPreference
}

func newASvcMockDashboardPrefRepo() *ASvcMockDashboardPrefRepo {
	return &ASvcMockDashboardPrefRepo{
		preferences: make(map[uuid.UUID]*domain.DashboardPreference),
	}
}

// Compile-time interface check
var _ repository.DashboardPreferenceRepository = (*ASvcMockDashboardPrefRepo)(nil)

func (m *ASvcMockDashboardPrefRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.DashboardPreference, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DashboardPreference), args.Error(1)
}

func (m *ASvcMockDashboardPrefRepo) Upsert(ctx context.Context, preference *domain.DashboardPreference) error {
	args := m.Called(ctx, preference)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.preferences[preference.OrganizationID] = preference
		m.mu.Unlock()
	}
	return args.Error(0)
}

// ASvcMockAuditLogRepo implements repository.AuditLogRepository
type ASvcMockAuditLogRepo struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func newASvcMockAuditLogRepo() *ASvcMockAuditLogRepo {
	return &ASvcMockAuditLogRepo{}
}

func (m *ASvcMockAuditLogRepo) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *ASvcMockAuditLogRepo) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *ASvcMockAuditLogRepo) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *ASvcMockAuditLogRepo) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *ASvcMockAuditLogRepo) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// ======================================================================
// Helpers
// ======================================================================

func newAnalyticsTestEnforcer(t *testing.T) *permission.Enforcer {
	t.Helper()
	enforcer, err := permission.NewTestEnforcer()
	require.NoError(t, err)
	return enforcer
}

func newAnalyticsTestService(
	metricEventRepo *ASvcMockMetricEventRepo,
	dashboardRepo *ASvcMockDashboardMetricRepo,
	preferenceRepo *ASvcMockDashboardPrefRepo,
	enforcer *permission.Enforcer,
	auditRepo *ASvcMockAuditLogRepo,
) *service.AnalyticsService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	cfg := config.DefaultAnalyticsConfig()
	return service.NewAnalyticsService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditSvc, log, cfg)
}

// newAnalyticsSvc creates a fresh service with fresh mocks for a test.
func newAnalyticsSvc() (*service.AnalyticsService, *ASvcMockMetricEventRepo, *ASvcMockDashboardMetricRepo, *ASvcMockDashboardPrefRepo, *permission.Enforcer, *ASvcMockAuditLogRepo) {
	metricEventRepo := newASvcMockMetricEventRepo()
	dashboardRepo := newASvcMockDashboardMetricRepo()
	preferenceRepo := newASvcMockDashboardPrefRepo()
	auditRepo := newASvcMockAuditLogRepo()
	return nil, metricEventRepo, dashboardRepo, preferenceRepo, nil, auditRepo
}

// ======================================================================
// TestAnalyticsService_GetDashboard
// ======================================================================

func TestAnalyticsService_GetDashboard(t *testing.T) {
	t.Run("returns dashboard with default categories when no org context", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()

		// When hasOrgID=false, all categories are visible by default
		// Mock all the CountByTypeAndPeriod calls for each category
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, mock.AnythingOfType("[]string"), false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64{"user.created": 10}, nil)
		metricEventRepo.On("CountDistinctActorsByPeriod",
			mock.Anything, false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(int64(5), nil)

		resp, err := svc.GetDashboard(context.Background(), userID, false, uuid.Nil, "daily")

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "daily", resp.Period)
		// All category groups should be populated (not nil) when defaults apply
		assert.NotNil(t, resp.UserActivity)
		assert.NotNil(t, resp.ContentMetrics)
		assert.NotNil(t, resp.EngagementMetrics)
		assert.NotNil(t, resp.SystemMetrics)
	})

	t.Run("returns dashboard with org preferences", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()

		// Create a preference that hides engagement_metrics and system_metrics
		categoriesJSON, _ := json.Marshal(map[string]bool{
			"user_activity":       true,
			"content_metrics":    true,
			"engagement_metrics": false,
			"system_metrics":     false,
		})
		pref := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(categoriesJSON),
			UpdatedByUserID:  userID,
		}

		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(pref, nil)

		// Mock CountByTypeAndPeriod for user_activity
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, []string{"user.created"}, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64{"user.created": 15}, nil)
		metricEventRepo.On("CountDistinctActorsByPeriod",
			mock.Anything, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(int64(8), nil)

		// Mock CountByTypeAndPeriod for content_metrics
		contentTypes := []string{
			domain.MetricEventTypeInvoiceCreated,
			domain.MetricEventTypeInvoicePaid,
			domain.MetricEventTypeNewsPublished,
			domain.MetricEventTypeMediaUploaded,
			domain.MetricEventTypeFileVersioned,
		}
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, contentTypes, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64{
			"invoice.created": 5,
			"invoice.paid":   3,
			"news.published":  2,
			"media.uploaded":  1,
			"media.versioned": 1,
		}, nil)

		resp, err := svc.GetDashboard(context.Background(), userID, true, orgID, "daily")

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "daily", resp.Period)
		// Visible categories have data
		assert.NotNil(t, resp.UserActivity)
		assert.NotNil(t, resp.ContentMetrics)
		// Hidden categories should be nil
		assert.Nil(t, resp.EngagementMetrics)
		assert.Nil(t, resp.SystemMetrics)
	})

	t.Run("falls back to defaults when preference fetch fails", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()

		// Preference fetch fails — should fall back to defaults
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(nil, apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		// Mock all four category computations (all visible since defaults)
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, mock.AnythingOfType("[]string"), true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64{}, nil)
		metricEventRepo.On("CountDistinctActorsByPeriod",
			mock.Anything, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(int64(0), nil)

		resp, err := svc.GetDashboard(context.Background(), userID, true, orgID, "daily")

		require.NoError(t, err)
		// Should still return a response (with defaults)
		assert.NotNil(t, resp)
		// All categories visible by default
		assert.NotNil(t, resp.UserActivity)
		assert.NotNil(t, resp.ContentMetrics)
		assert.NotNil(t, resp.EngagementMetrics)
		assert.NotNil(t, resp.SystemMetrics)
	})

	t.Run("returns validation error for invalid period", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()

		resp, err := svc.GetDashboard(context.Background(), userID, false, uuid.Nil, "invalid_period")

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 400, appErr.HTTPStatus)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})

	t.Run("defaults period to daily when empty string", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()

		// Mock all category computations
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, mock.AnythingOfType("[]string"), false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64{}, nil)
		metricEventRepo.On("CountDistinctActorsByPeriod",
			mock.Anything, false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(int64(0), nil)

		resp, err := svc.GetDashboard(context.Background(), userID, false, uuid.Nil, "")

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "daily", resp.Period)
	})

	t.Run("returns zero-value fallback when repo returns error", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()

		// Preference repo returns preference with all categories enabled
		categoriesJSON, _ := json.Marshal(domain.DefaultMetricCategories())
		pref := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(categoriesJSON),
		}
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(pref, nil)

		// CountByTypeAndPeriod fails for all categories
		metricEventRepo.On("CountByTypeAndPeriod",
			mock.Anything, mock.AnythingOfType("[]string"), true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(map[string]int64(nil), apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))
		metricEventRepo.On("CountDistinctActorsByPeriod",
			mock.Anything, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"),
		).Return(int64(0), apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		resp, err := svc.GetDashboard(context.Background(), userID, true, orgID, "daily")

		// Should NOT return error — dashboard returns zero-value fallbacks
		require.NoError(t, err)
		assert.NotNil(t, resp)
		// All categories should be populated but with zero values
		assert.NotNil(t, resp.UserActivity)
		assert.Equal(t, int64(0), resp.UserActivity.TotalUsers)
		assert.NotNil(t, resp.ContentMetrics)
		assert.Equal(t, int64(0), resp.ContentMetrics.InvoicesCreated)
	})
}

// ======================================================================
// TestAnalyticsService_GetMetrics
// ======================================================================

func TestAnalyticsService_GetMetrics(t *testing.T) {
	t.Run("returns time series data", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()
		from := time.Now().AddDate(0, 0, -7)
		to := time.Now()

		// Mock FindTimeSeries
		points := []*domain.MetricTimeSeriesPoint{
			{Date: from.Format("2006-01-02"), Count: 5, Value: 0},
		}
		metricEventRepo.On("FindTimeSeries",
			mock.Anything, []string{"user.created"}, true, orgID, from, to, "daily",
		).Return(points, nil)

		resp, err := svc.GetMetrics(context.Background(), userID, true, orgID, []string{"user.created"}, "daily", from, to)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "user.created", resp.EventType)
		assert.Equal(t, "daily", resp.Period)
		assert.NotEmpty(t, resp.DataPoints)
	})

	t.Run("returns validation error for invalid period", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		from := time.Now().AddDate(0, 0, -7)
		to := time.Now()

		resp, err := svc.GetMetrics(context.Background(), userID, true, uuid.New(), []string{"user.created"}, "invalid", from, to)

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 400, appErr.HTTPStatus)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})

	t.Run("returns validation error for empty event types", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		from := time.Now().AddDate(0, 0, -7)
		to := time.Now()

		resp, err := svc.GetMetrics(context.Background(), userID, true, uuid.New(), []string{}, "daily", from, to)

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 400, appErr.HTTPStatus)
		assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	})

	t.Run("defaults date range to last 7 days when empty", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()

		// Empty from/to — should default to last 7 days
		metricEventRepo.On("FindTimeSeries",
			mock.Anything, []string{"user.created"}, true, orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), "daily",
		).Return([]*domain.MetricTimeSeriesPoint{}, nil)

		resp, err := svc.GetMetrics(context.Background(), userID, true, orgID, []string{"user.created"}, "daily", time.Time{}, time.Time{})

		require.NoError(t, err)
		assert.NotNil(t, resp)
		// The response should have from/to populated (not zero)
		assert.NotEmpty(t, resp.From)
		assert.NotEmpty(t, resp.To)
	})

	t.Run("wraps repo error as internal error", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		userID := uuid.New()
		orgID := uuid.New()
		from := time.Now().AddDate(0, 0, -7)
		to := time.Now()

		metricEventRepo.On("FindTimeSeries",
			mock.Anything, []string{"user.created"}, true, orgID, from, to, "daily",
		).Return([]*domain.MetricTimeSeriesPoint(nil), apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		resp, err := svc.GetMetrics(context.Background(), userID, true, orgID, []string{"user.created"}, "daily", from, to)

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 500, appErr.HTTPStatus)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})
}

// ======================================================================
// TestAnalyticsService_GetPreferences
// ======================================================================

func TestAnalyticsService_GetPreferences(t *testing.T) {
	t.Run("returns default categories when no org context", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		categories, err := svc.GetPreferences(context.Background(), false, uuid.Nil)

		require.NoError(t, err)
		assert.Equal(t, domain.DefaultMetricCategories(), categories)
		// All categories should be enabled by default
		for k, v := range categories {
			assert.True(t, v, "category %s should be true", k)
		}
	})

	t.Run("returns stored preferences", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		userID := uuid.New()

		customCategories := map[string]bool{
			"user_activity":       true,
			"content_metrics":    true,
			"engagement_metrics": false,
			"system_metrics":     false,
		}
		categoriesJSON, _ := json.Marshal(customCategories)
		pref := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(categoriesJSON),
			UpdatedByUserID:  userID,
		}

		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(pref, nil)

		categories, err := svc.GetPreferences(context.Background(), true, orgID)

		require.NoError(t, err)
		assert.Equal(t, customCategories, categories)
		assert.True(t, categories["user_activity"])
		assert.False(t, categories["engagement_metrics"])
	})

	t.Run("returns defaults when no preference row exists", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()

		// FindByOrganization returns nil, nil — no row exists
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(nil, nil)

		categories, err := svc.GetPreferences(context.Background(), true, orgID)

		require.NoError(t, err)
		assert.Equal(t, domain.DefaultMetricCategories(), categories)
	})

	t.Run("wraps repo error as internal error", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()

		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(nil, apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		categories, err := svc.GetPreferences(context.Background(), true, orgID)

		require.Error(t, err)
		assert.Nil(t, categories)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 500, appErr.HTTPStatus)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})
}

// ======================================================================
// TestAnalyticsService_UpdatePreferences
// ======================================================================

func TestAnalyticsService_UpdatePreferences(t *testing.T) {
	t.Run("updates preferences and returns response", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		userID := uuid.New()

		categories := map[string]bool{
			"user_activity":       true,
			"content_metrics":    true,
			"engagement_metrics": false,
			"system_metrics":     false,
		}

		preferenceRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardPreference")).Return(nil)

		categoriesJSON, _ := json.Marshal(categories)
		savedPref := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(categoriesJSON),
			UpdatedByUserID:  userID,
		}
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(savedPref, nil)

		resp, err := svc.UpdatePreferences(context.Background(), orgID, userID, categories)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, orgID.String(), resp.OrganizationID)
		assert.Equal(t, categories, resp.MetricCategories)
	})

	t.Run("filters invalid category keys", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		userID := uuid.New()

		// Include invalid keys alongside valid ones
		categories := map[string]bool{
			"user_activity":       true,
			"content_metrics":    true,
			"invalid_category":  true,
			"fake_metric":       true,
		}

		var capturedPref *domain.DashboardPreference
		preferenceRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardPreference")).Run(func(args mock.Arguments) {
			capturedPref = args.Get(1).(*domain.DashboardPreference)
		}).Return(nil)

		// Re-fetch should return only valid categories
		filteredCategories := map[string]bool{
			"user_activity":    true,
			"content_metrics": true,
		}
		categoriesJSON, _ := json.Marshal(filteredCategories)
		savedPref := &domain.DashboardPreference{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			MetricCategories: datatypes.JSON(categoriesJSON),
			UpdatedByUserID:  userID,
		}
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(savedPref, nil)

		resp, err := svc.UpdatePreferences(context.Background(), orgID, userID, categories)

		require.NoError(t, err)
		assert.NotNil(t, resp)
		// Verify the upserted preference only contains valid keys
		require.NotNil(t, capturedPref)
		var upsertedCategories map[string]bool
		err = json.Unmarshal(capturedPref.MetricCategories, &upsertedCategories)
		require.NoError(t, err)
		assert.Equal(t, 2, len(upsertedCategories))
		assert.False(t, upsertedCategories["invalid_category"])
		assert.False(t, upsertedCategories["fake_metric"])
	})

	t.Run("wraps upsert error as internal error", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		userID := uuid.New()
		categories := domain.DefaultMetricCategories()

		preferenceRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardPreference")).Return(apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		resp, err := svc.UpdatePreferences(context.Background(), orgID, userID, categories)

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 500, appErr.HTTPStatus)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("wraps find error as internal error", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		userID := uuid.New()
		categories := domain.DefaultMetricCategories()

		preferenceRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardPreference")).Return(nil)
		preferenceRepo.On("FindByOrganization", mock.Anything, orgID).Return(nil, apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		resp, err := svc.UpdatePreferences(context.Background(), orgID, userID, categories)

		require.Error(t, err)
		assert.Nil(t, resp)
		appErr := apperrors.GetAppError(err)
		assert.Equal(t, 500, appErr.HTTPStatus)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})
}

// ======================================================================
// TestAnalyticsService_handleEvent
// ======================================================================

func TestAnalyticsService_handleEvent(t *testing.T) {
	t.Run("creates metric event from mapped event", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		actorID := uuid.New()
		resourceID := uuid.New()

		// Set up EventBus subscription to test handleEvent via event processing
		eventBus := domain.NewEventBus(256)

		var capturedEvent *domain.MetricEvent
		metricEventRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.MetricEvent")).Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*domain.MetricEvent)
		}).Return(nil)

		svc.SubscribeToEventBus(eventBus)
		eventBus.Start(context.Background())
		defer eventBus.Stop()

		eventBus.Publish(domain.WebhookEvent{
			Type:    "user.created",
			OrgID:   &orgID,
			Payload: map[string]interface{}{"id": resourceID.String(), "actor_id": actorID.String()},
		})

		// Wait for async processing
		time.Sleep(300 * time.Millisecond)

		metricEventRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.MetricEvent"))
		require.NotNil(t, capturedEvent)
		assert.Equal(t, "user.created", capturedEvent.EventType)
		assert.Equal(t, domain.MetricResourceUser, capturedEvent.ResourceType)
		assert.Equal(t, resourceID.String(), capturedEvent.ResourceID)
		assert.Equal(t, actorID, capturedEvent.ActorID)
		assert.Equal(t, orgID, *capturedEvent.OrganizationID)
	})

	t.Run("ignores unmapped event types", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()

		eventBus := domain.NewEventBus(256)

		svc.SubscribeToEventBus(eventBus)
		eventBus.Start(context.Background())
		defer eventBus.Stop()

		// Publish an event type that has no analytics mapping
		eventBus.Publish(domain.WebhookEvent{
			Type:    "unknown.event.type",
			OrgID:   &orgID,
			Payload: map[string]interface{}{"data": "test"},
		})

		// Wait for async processing
		time.Sleep(300 * time.Millisecond)

		// No Create call should have been made
		metricEventRepo.AssertNotCalled(t, "Create")
	})

	t.Run("logs error when create fails", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		orgID := uuid.New()
		actorID := uuid.New()
		resourceID := uuid.New()

		eventBus := domain.NewEventBus(256)

		// Create returns an error
		metricEventRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.MetricEvent")).Return(apperrors.NewAppError("INTERNAL_ERROR", "db error", 500))

		svc.SubscribeToEventBus(eventBus)
		eventBus.Start(context.Background())
		defer eventBus.Stop()

		eventBus.Publish(domain.WebhookEvent{
			Type:    "user.created",
			OrgID:   &orgID,
			Payload: map[string]interface{}{"id": resourceID.String(), "actor_id": actorID.String()},
		})

		// Wait for async processing
		time.Sleep(300 * time.Millisecond)

		// Create was called but returned error — service should have logged it
		metricEventRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.MetricEvent"))
		// No panic, test passes — the error was logged
	})
}

// ======================================================================
// TestAnalyticsService_GenerateTimeSeriesDataPoints
// ======================================================================

func TestAnalyticsService_GenerateTimeSeriesDataPoints(t *testing.T) {
	t.Run("generates daily data points with zero fill", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

		// Only January 2nd has data
		points := []*domain.MetricTimeSeriesPoint{
			{Date: "2025-01-02", Count: 10, Value: 0},
		}

		result := svc.GenerateTimeSeriesDataPoints(points, from, to, "daily", []string{"user.created"})

		// Should have 3 data points: Jan 1 (0), Jan 2 (10), Jan 3 (0)
		assert.Len(t, result, 3)
		assert.Equal(t, 0.0, result[0].Value)   // Jan 1 — zero fill
		assert.Equal(t, 10.0, result[1].Value) // Jan 2 — from data
		assert.Equal(t, 0.0, result[2].Value)   // Jan 3 — zero fill
	})

	t.Run("generates hourly data points", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		from := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
		to := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

		points := []*domain.MetricTimeSeriesPoint{
			{Date: "2025-01-01T11:00", Count: 5, Value: 0},
		}

		result := svc.GenerateTimeSeriesDataPoints(points, from, to, "hourly", []string{"user.created"})

		// Should have 3 data points: 10:00 (0), 11:00 (5), 12:00 (0)
		assert.Len(t, result, 3)
	})

	t.Run("generates weekly data points", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		// Start on a Monday
		from := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)  // Monday Jan 6
		to := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)    // Monday Jan 20

		points := []*domain.MetricTimeSeriesPoint{
			{Date: "2025-01-13", Count: 8, Value: 0},
		}

		result := svc.GenerateTimeSeriesDataPoints(points, from, to, "weekly", []string{"user.created"})

		// Should have at least the week containing Jan 13
		assert.NotEmpty(t, result)
		// Check that at least one point has the value from our data
		found := false
		for _, dp := range result {
			if dp.Value == 8.0 {
				found = true
			}
		}
		assert.True(t, found, "should find the data point with value 8")
	})

	t.Run("generates monthly data points", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

		points := []*domain.MetricTimeSeriesPoint{
			{Date: "2025-02-01", Count: 42, Value: 0},
		}

		result := svc.GenerateTimeSeriesDataPoints(points, from, to, "monthly", []string{"user.created"})

		// Should have 3 data points: Jan (0), Feb (42), Mar (0)
		assert.Len(t, result, 3)
		assert.Equal(t, 0.0, result[0].Value)
		assert.Equal(t, 42.0, result[1].Value)
		assert.Equal(t, 0.0, result[2].Value)
	})

	t.Run("returns empty slice for empty input when range is zero", func(t *testing.T) {
		metricEventRepo := newASvcMockMetricEventRepo()
		dashboardRepo := newASvcMockDashboardMetricRepo()
		preferenceRepo := newASvcMockDashboardPrefRepo()
		enforcer := newAnalyticsTestEnforcer(t)
		auditRepo := newASvcMockAuditLogRepo()
		svc := newAnalyticsTestService(metricEventRepo, dashboardRepo, preferenceRepo, enforcer, auditRepo)

		// from > to — no range
		from := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
		to := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		result := svc.GenerateTimeSeriesDataPoints(nil, from, to, "daily", []string{"user.created"})

		// Empty slice (not nil)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})
}
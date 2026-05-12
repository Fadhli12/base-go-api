package unit

import (
	"context"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ======================================================================
// Shared Mock implementations for analytics tests
// ======================================================================

// MockMetricEventRepository implements repository.MetricEventRepository
type MockMetricEventRepository struct {
	mock.Mock
	mu     sync.Mutex
	events map[uuid.UUID]*domain.MetricEvent
}

func newMockMetricEventRepository() *MockMetricEventRepository {
	return &MockMetricEventRepository{
		events: make(map[uuid.UUID]*domain.MetricEvent),
	}
}

// Compile-time interface check
var _ repository.MetricEventRepository = (*MockMetricEventRepository)(nil)

func (m *MockMetricEventRepository) Create(ctx context.Context, event *domain.MetricEvent) error {
	args := m.Called(ctx, event)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.events[event.ID] = event
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockMetricEventRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.MetricEvent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MetricEvent), args.Error(1)
}

func (m *MockMetricEventRepository) FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters repository.MetricEventFilters, limit, offset int) ([]*domain.MetricEvent, int64, error) {
	args := m.Called(ctx, hasOrgID, orgID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.MetricEvent), args.Get(1).(int64), args.Error(2)
}

func (m *MockMetricEventRepository) CountByTypeAndPeriod(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (map[string]int64, error) {
	args := m.Called(ctx, eventTypes, hasOrgID, orgID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockMetricEventRepository) CountDistinctActorsByPeriod(ctx context.Context, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (int64, error) {
	args := m.Called(ctx, hasOrgID, orgID, from, to)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMetricEventRepository) FindTimeSeries(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time, periodType string) ([]*domain.MetricTimeSeriesPoint, error) {
	args := m.Called(ctx, eventTypes, hasOrgID, orgID, from, to, periodType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MetricTimeSeriesPoint), args.Error(1)
}

func (m *MockMetricEventRepository) ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	args := m.Called(ctx, retentionDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMetricEventRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMetricEventRepository) FindDistinctOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

// MockDashboardMetricRepository implements repository.DashboardMetricRepository
type MockDashboardMetricRepository struct {
	mock.Mock
}

func newMockDashboardMetricRepository() *MockDashboardMetricRepository {
	return &MockDashboardMetricRepository{}
}

// Compile-time interface check
var _ repository.DashboardMetricRepository = (*MockDashboardMetricRepository)(nil)

func (m *MockDashboardMetricRepository) Upsert(ctx context.Context, metric *domain.DashboardMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *MockDashboardMetricRepository) FindByTypeAndPeriod(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (*domain.DashboardMetric, error) {
	args := m.Called(ctx, metricType, periodType, hasOrgID, orgID, periodStart, periodEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DashboardMetric), args.Error(1)
}

func (m *MockDashboardMetricRepository) FindByTypesAndPeriod(ctx context.Context, metricTypes []string, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*domain.DashboardMetric, error) {
	args := m.Called(ctx, metricTypes, periodType, hasOrgID, orgID, periodStart, periodEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DashboardMetric), args.Error(1)
}

func (m *MockDashboardMetricRepository) DeleteByTypeAndPeriodBefore(ctx context.Context, metricType, periodType string, before time.Time) error {
	args := m.Called(ctx, metricType, periodType, before)
	return args.Error(0)
}

func (m *MockDashboardMetricRepository) FindMaxCalculatedAt(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID) (*time.Time, error) {
	args := m.Called(ctx, metricType, periodType, hasOrgID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

// MockDashboardPreferenceRepository implements repository.DashboardPreferenceRepository
type MockDashboardPreferenceRepository struct {
	mock.Mock
	mu          sync.Mutex
	preferences map[uuid.UUID]*domain.DashboardPreference
}

func newMockDashboardPreferenceRepository() *MockDashboardPreferenceRepository {
	return &MockDashboardPreferenceRepository{
		preferences: make(map[uuid.UUID]*domain.DashboardPreference),
	}
}

// Compile-time interface check
var _ repository.DashboardPreferenceRepository = (*MockDashboardPreferenceRepository)(nil)

func (m *MockDashboardPreferenceRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.DashboardPreference, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DashboardPreference), args.Error(1)
}

func (m *MockDashboardPreferenceRepository) Upsert(ctx context.Context, preference *domain.DashboardPreference) error {
	args := m.Called(ctx, preference)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.preferences[preference.OrganizationID] = preference
		m.mu.Unlock()
	}
	return args.Error(0)
}
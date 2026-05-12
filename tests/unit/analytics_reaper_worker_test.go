package unit

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ======================================================================
// AnalyticsReaper Tests
// ======================================================================

func TestAnalyticsReaper_StartStop(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	reaper.Stop()
}

func TestAnalyticsReaper_ArchiveOldMetricEvents(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	metricEventRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(5), nil)

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	metricEventRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestAnalyticsReaper_CustomRetention(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  30,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	metricEventRepo.On("ArchiveOlderThan", mock.Anything, 30).Return(int64(3), nil)

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	metricEventRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 30)
}

func TestAnalyticsReaper_DefaultRetention(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  0,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	metricEventRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(0), nil)

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	metricEventRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestAnalyticsReaper_StopWithoutStart(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	reaper.Stop()
}

func TestAnalyticsReaper_DefaultInterval(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 0,
	}
	logger := slog.Default()

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	reaper.Stop()
}

func TestAnalyticsReaper_ArchiveError(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	metricEventRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(0), assert.AnError)

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	metricEventRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestAnalyticsReaper_MultipleTicks(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	cfg := config.AnalyticsConfig{
		RetentionDays:  90,
		ReaperInterval: 50 * time.Millisecond,
	}
	logger := slog.Default()

	metricEventRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(1), nil)

	reaper := service.NewAnalyticsReaper(metricEventRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(300 * time.Millisecond)

	reaper.Stop()

	metricEventRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
	assert.GreaterOrEqual(t, len(metricEventRepo.Calls), 2)
}

// ======================================================================
// AggregationWorker Tests
// ======================================================================

func TestAggregationWorker_StartStop(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	worker.Stop()
}

func TestAggregationWorker_AggregationCycle(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	// Global-only aggregation (no org IDs)
	metricEventRepo.On("FindDistinctOrganizationIDs", mock.Anything).Return([]uuid.UUID{}, nil)

	pastTime := time.Now().AddDate(0, 0, -60)
	dashboardRepo.On("FindMaxCalculatedAt", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), false, uuid.Nil).Return(&pastTime, nil)

	metricEventRepo.On("CountByTypeAndPeriod", mock.Anything, mock.AnythingOfType("[]string"), false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(map[string]int64{
		domain.MetricEventTypeUserCreated: 10,
		domain.MetricEventTypeUserDeleted: 2,
	}, nil)

	dashboardRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardMetric")).Return(nil)

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	worker.Stop()

	metricEventRepo.AssertCalled(t, "CountByTypeAndPeriod", mock.Anything, mock.AnythingOfType("[]string"), false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"))
	dashboardRepo.AssertCalled(t, "Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardMetric"))
}

func TestAggregationWorker_SkipsCurrentPeriod(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	// Global-only aggregation (no org IDs)
	metricEventRepo.On("FindDistinctOrganizationIDs", mock.Anything).Return([]uuid.UUID{}, nil)

	// Set watermark to today - this means we already calculated up to today,
	// so the next period to aggregate would be today (current period) and should be skipped
	watermark := time.Now()
	dashboardRepo.On("FindMaxCalculatedAt", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), false, uuid.Nil).Return(&watermark, nil)

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	worker.Stop()

	dashboardRepo.AssertNotCalled(t, "Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardMetric"))
}

func TestAggregationWorker_ResumeFromWatermark(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	// Global-only aggregation (no org IDs)
	metricEventRepo.On("FindDistinctOrganizationIDs", mock.Anything).Return([]uuid.UUID{}, nil)

	watermark := time.Now().AddDate(0, 0, -10)
	dashboardRepo.On("FindMaxCalculatedAt", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), false, uuid.Nil).Return(&watermark, nil)

	metricEventRepo.On("CountByTypeAndPeriod", mock.Anything, mock.AnythingOfType("[]string"), false, uuid.Nil, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(map[string]int64{
		domain.MetricEventTypeUserCreated: 5,
	}, nil)

	dashboardRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardMetric")).Return(nil)

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	worker.Stop()

	dashboardRepo.AssertCalled(t, "Upsert", mock.Anything, mock.AnythingOfType("*domain.DashboardMetric"))
}

func TestAggregationWorker_StopWithoutStart(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	worker.Stop()
}

func TestAggregationWorker_DefaultInterval(t *testing.T) {
	metricEventRepo := newMockMetricEventRepository()
	dashboardRepo := newMockDashboardMetricRepository()
	cfg := config.AnalyticsConfig{
		AggregationInterval: 0,
	}
	logger := slog.Default()

	worker := service.NewAggregationWorker(metricEventRepo, dashboardRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	worker.Stop()
}
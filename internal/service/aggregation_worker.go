package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/google/uuid"
)

// AggregationWorker pre-computes dashboard metrics on a configurable interval.
// It processes completed periods (never the current incomplete period) and
// uses the calculated_at watermark to resume from where it left off.
type AggregationWorker struct {
	metricEventRepo  repository.MetricEventRepository
	dashboardRepo   repository.DashboardMetricRepository
	config          config.AnalyticsConfig
	log            *slog.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewAggregationWorker creates a new AggregationWorker instance.
func NewAggregationWorker(
	metricEventRepo repository.MetricEventRepository,
	dashboardRepo repository.DashboardMetricRepository,
	config config.AnalyticsConfig,
	log *slog.Logger,
) *AggregationWorker {
	return &AggregationWorker{
		metricEventRepo: metricEventRepo,
		dashboardRepo:  dashboardRepo,
		config:         config,
		log:            log,
	}
}

// Start begins the aggregation worker goroutine with a ticker.
func (w *AggregationWorker) Start(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)

	interval := w.config.AggregationInterval
	if interval == 0 {
		interval = 60 * time.Second
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		w.log.Info("aggregation worker started",
			slog.Duration("interval", interval),
		)

		for {
			select {
			case <-w.ctx.Done():
				w.log.Info("aggregation worker stopped")
				return
			case <-ticker.C:
				w.run()
			}
		}
	}()
}

// Stop cancels the aggregation worker context and waits for the goroutine to finish.
func (w *AggregationWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	w.log.Info("aggregation worker shutdown complete")
}

// Trigger executes a single aggregation cycle immediately.
// This is used by the manual trigger endpoint (POST /aggregate).
func (w *AggregationWorker) Trigger(ctx context.Context) {
	w.log.Info("manual aggregation trigger received")
	w.run()
}

// run executes a single aggregation cycle across all period types.
func (w *AggregationWorker) run() {
	periodTypes := []string{domain.MetricPeriodDaily, domain.MetricPeriodWeekly, domain.MetricPeriodMonthly}

	for _, periodType := range periodTypes {
		if err := w.aggregatePeriod(w.ctx, periodType); err != nil {
			w.log.Error("aggregation worker failed",
				slog.String("period_type", periodType),
				slog.String("error", err.Error()),
			)
		}
	}
}

// aggregatePeriod processes unprocessed periods for a given period type.
// It iterates over distinct organization IDs (including NULL for global metrics)
// and aggregates metrics for each org/period combination.
func (w *AggregationWorker) aggregatePeriod(ctx context.Context, periodType string) error {
	now := time.Now()

	// Determine the end of the current period (don't aggregate current incomplete period)
	var currentPeriodStart time.Time
	switch periodType {
	case domain.MetricPeriodDaily:
		currentPeriodStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case domain.MetricPeriodWeekly:
		weekday := now.Weekday()
		daysSinceMonday := int(weekday)
		if weekday == time.Sunday {
			daysSinceMonday = 7
		}
		daysSinceMonday--
		currentPeriodStart = now.AddDate(0, 0, -daysSinceMonday).Truncate(24 * time.Hour)
	case domain.MetricPeriodMonthly:
		currentPeriodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}

	// Get list of distinct org IDs to process
	// Always include global (nil) metrics, then add per-organization metrics
	orgIDs := []struct {
		hasOrgID bool
		orgID    uuid.UUID
	}{
		{hasOrgID: false, orgID: uuid.Nil},
	}

	distinctOrgIDs, err := w.metricEventRepo.FindDistinctOrganizationIDs(ctx)
	if err != nil {
		w.log.Error("failed to get distinct organization IDs for aggregation",
			slog.String("error", err.Error()),
		)
		// Continue with global-only aggregation rather than failing entirely
	} else {
		for _, orgID := range distinctOrgIDs {
			orgIDs = append(orgIDs, struct {
				hasOrgID bool
				orgID    uuid.UUID
			}{hasOrgID: true, orgID: orgID})
		}
	}

	// Aggregate metrics for each org
	for _, org := range orgIDs {
		if err := w.aggregateForOrg(ctx, periodType, org.hasOrgID, org.orgID, currentPeriodStart); err != nil {
			return err
		}
	}

	return nil
}

// aggregateForOrg aggregates metrics for a specific organization context.
func (w *AggregationWorker) aggregateForOrg(
	ctx context.Context,
	periodType string,
	hasOrgID bool,
	orgID uuid.UUID,
	currentPeriodStart time.Time,
) error {
	// Define metric categories and their event types
	metricCategories := map[string][]string{
		domain.MetricCategoryUserActivity: {domain.MetricEventTypeUserCreated, domain.MetricEventTypeUserDeleted},
		domain.MetricCategoryContentMetrics: {
			domain.MetricEventTypeInvoiceCreated,
			domain.MetricEventTypeInvoicePaid,
			domain.MetricEventTypeNewsPublished,
			domain.MetricEventTypeMediaUploaded,
			domain.MetricEventTypeFileVersioned,
		},
		domain.MetricCategoryEngagementMetrics: {domain.MetricEventTypeCommentCreated},
		domain.MetricCategorySystemMetrics: {domain.MetricEventTypeLoginSuccess, domain.MetricEventTypeLoginFailed},
	}

	now := time.Now()

	// Process each metric category
	for metricType, eventTypes := range metricCategories {
		// Find the watermark (last calculated_at) for this metric type and org
		watermark, err := w.dashboardRepo.FindMaxCalculatedAt(ctx, metricType, periodType, hasOrgID, orgID)
		if err != nil {
			w.log.Error("failed to get aggregation watermark",
				slog.String("metric_type", metricType),
				slog.String("period_type", periodType),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Determine the period to aggregate
		var periodStart, periodEnd time.Time
		switch periodType {
		case domain.MetricPeriodDaily:
			if watermark != nil {
				periodStart = watermark.AddDate(0, 0, 1)
			} else {
				periodStart = now.AddDate(0, 0, -30) // Start from 30 days ago if no watermark
			}
			periodEnd = periodStart.AddDate(0, 0, 1)
		case domain.MetricPeriodWeekly:
			if watermark != nil {
				periodStart = watermark.AddDate(0, 0, 7)
			} else {
				// Start from 12 weeks ago
				weekday := now.Weekday()
				daysSinceMonday := int(weekday)
				if weekday == time.Sunday {
					daysSinceMonday = 7
				}
				daysSinceMonday--
				periodStart = now.AddDate(0, 0, -daysSinceMonday-7*12).Truncate(24 * time.Hour)
			}
			periodEnd = periodStart.AddDate(0, 0, 7)
		case domain.MetricPeriodMonthly:
			if watermark != nil {
				periodStart = watermark.AddDate(0, 1, 0)
			} else {
				periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -12, 0)
			}
			periodEnd = time.Date(periodStart.Year(), periodStart.Month()+1, 1, 0, 0, 0, 0, periodStart.Location())
		}

		// Skip if this period is the current (incomplete) period
		if !periodEnd.Before(currentPeriodStart) && !periodEnd.Equal(currentPeriodStart) {
			// Period is current or future — skip
			if periodStart.Equal(currentPeriodStart) || periodStart.After(currentPeriodStart) {
				continue
			}
		}

		// Query raw events for this period
		counts, err := w.metricEventRepo.CountByTypeAndPeriod(ctx, eventTypes, hasOrgID, orgID, periodStart, periodEnd)
		if err != nil {
			w.log.Error("failed to count events for aggregation",
				slog.String("metric_type", metricType),
				slog.String("period_type", periodType),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Sum total value across event types
		var totalValue float64
		for _, count := range counts {
			totalValue += float64(count)
		}

		// Upsert dashboard metric
		orgIDPtr := &orgID
		if !hasOrgID {
			orgIDPtr = nil
		}

		metric := &domain.DashboardMetric{
			MetricType:     metricType,
			PeriodType:     periodType,
			PeriodStart:    periodStart,
			PeriodEnd:     periodEnd,
			Value:         totalValue,
			OrganizationID: orgIDPtr,
			CalculatedAt:  now,
		}

		if err := w.dashboardRepo.Upsert(ctx, metric); err != nil {
			w.log.Error("failed to upsert dashboard metric",
				slog.String("metric_type", metricType),
				slog.String("period_type", periodType),
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	return nil
}
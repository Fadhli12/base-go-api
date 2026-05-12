package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DashboardMetricRepository defines the data access interface for dashboard metrics.
type DashboardMetricRepository interface {
	// Upsert creates or updates a dashboard metric using ON CONFLICT DO UPDATE.
	Upsert(ctx context.Context, metric *domain.DashboardMetric) error
	FindByTypeAndPeriod(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (*domain.DashboardMetric, error)
	FindByTypesAndPeriod(ctx context.Context, metricTypes []string, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*domain.DashboardMetric, error)
	DeleteByTypeAndPeriodBefore(ctx context.Context, metricType, periodType string, before time.Time) error
	FindMaxCalculatedAt(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID) (*time.Time, error)
}

// --- DashboardMetricRepository implementation ---

type dashboardMetricRepository struct {
	db *gorm.DB
}

// NewDashboardMetricRepository creates a new GORM-backed DashboardMetricRepository.
func NewDashboardMetricRepository(db *gorm.DB) DashboardMetricRepository {
	return &dashboardMetricRepository{db: db}
}

// Upsert creates or updates a dashboard metric using ON CONFLICT DO UPDATE.
func (r *dashboardMetricRepository) Upsert(ctx context.Context, metric *domain.DashboardMetric) error {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "metric_type"},
			{Name: "period_type"},
			{Name: "period_start"},
			{Name: "organization_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"value", "metadata", "calculated_at", "updated_at"}),
	}).Create(metric)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	return nil
}

// FindByTypeAndPeriod finds a single metric by type, period type, and org scope.
func (r *dashboardMetricRepository) FindByTypeAndPeriod(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) (*domain.DashboardMetric, error) {
	var metric domain.DashboardMetric

	db := r.db.WithContext(ctx).
		Where("metric_type = ? AND period_type = ? AND period_start = ? AND period_end = ?",
			metricType, periodType, periodStart, periodEnd)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	} else {
		db = db.Where("organization_id IS NULL")
	}

	if err := db.First(&metric).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &metric, nil
}

// FindByTypesAndPeriod finds multiple metrics by types for a period range.
func (r *dashboardMetricRepository) FindByTypesAndPeriod(ctx context.Context, metricTypes []string, periodType string, hasOrgID bool, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*domain.DashboardMetric, error) {
	var metrics []*domain.DashboardMetric

	db := r.db.WithContext(ctx).
		Where("metric_type IN ? AND period_type = ? AND period_start >= ? AND period_end <= ?",
			metricTypes, periodType, periodStart, periodEnd)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	} else {
		db = db.Where("organization_id IS NULL")
	}

	if err := db.Find(&metrics).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return metrics, nil
}

// DeleteByTypeAndPeriodBefore deletes metrics older than a cutoff.
func (r *dashboardMetricRepository) DeleteByTypeAndPeriodBefore(ctx context.Context, metricType, periodType string, before time.Time) error {
	result := r.db.WithContext(ctx).
		Where("metric_type = ? AND period_type = ? AND period_end < ?",
			metricType, periodType, before).
		Delete(&domain.DashboardMetric{})

	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	return nil
}

// FindMaxCalculatedAt returns the maximum calculated_at for a metric type and period.
// Returns nil (not an error) when no row exists.
func (r *dashboardMetricRepository) FindMaxCalculatedAt(ctx context.Context, metricType, periodType string, hasOrgID bool, orgID uuid.UUID) (*time.Time, error) {
	var maxTime *time.Time

	db := r.db.WithContext(ctx).Model(&domain.DashboardMetric{}).
		Select("MAX(calculated_at)").
		Where("metric_type = ? AND period_type = ?", metricType, periodType)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	} else {
		db = db.Where("organization_id IS NULL")
	}

	if err := db.Scan(&maxTime).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return maxTime, nil
}
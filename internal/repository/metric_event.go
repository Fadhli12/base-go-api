package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MetricEventFilters holds optional filtering criteria for metric event queries.
type MetricEventFilters struct {
	EventType      string
	ResourceType   string
	OrganizationID *uuid.UUID
	From           *time.Time
	To             *time.Time
}

// MetricEventRepository defines the data access interface for metric events.
type MetricEventRepository interface {
	// Create inserts a new metric event. Uses ON CONFLICT DO NOTHING for idempotent ingestion.
	Create(ctx context.Context, event *domain.MetricEvent) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.MetricEvent, error)
	FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters MetricEventFilters, limit, offset int) ([]*domain.MetricEvent, int64, error)
	CountByTypeAndPeriod(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (map[string]int64, error)
	CountDistinctActorsByPeriod(ctx context.Context, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (int64, error)
	FindTimeSeries(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time, periodType string) ([]*domain.MetricTimeSeriesPoint, error)
	FindDistinctOrganizationIDs(ctx context.Context) ([]uuid.UUID, error)
	ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// --- MetricEventRepository implementation ---

type metricEventRepository struct {
	db *gorm.DB
}

// NewMetricEventRepository creates a new GORM-backed MetricEventRepository.
func NewMetricEventRepository(db *gorm.DB) MetricEventRepository {
	return &metricEventRepository{db: db}
}

func (r *metricEventRepository) Create(ctx context.Context, event *domain.MetricEvent) error {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_type"}, {Name: "resource_id"}, {Name: "date"}, {Name: "hour"}},
		DoNothing: true,
	}).Create(event)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	return nil
}

func (r *metricEventRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.MetricEvent, error) {
	var event domain.MetricEvent
	if err := r.db.WithContext(ctx).
		Where("archived_at IS NULL").
		First(&event, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &event, nil
}

func (r *metricEventRepository) FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters MetricEventFilters, limit, offset int) ([]*domain.MetricEvent, int64, error) {
	var events []*domain.MetricEvent
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Where("archived_at IS NULL")

	// Organization scoping
	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	// Optional filters
	if filters.EventType != "" {
		db = db.Where("event_type = ?", filters.EventType)
	}
	if filters.ResourceType != "" {
		db = db.Where("resource_type = ?", filters.ResourceType)
	}
	if filters.From != nil {
		db = db.Where("date >= ?", *filters.From)
	}
	if filters.To != nil {
		db = db.Where("date <= ?", *filters.To)
	}

	// Count total matching records
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	// Paginate
	query := db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&events).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return events, total, nil
}

func (r *metricEventRepository) CountByTypeAndPeriod(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (map[string]int64, error) {
	type result struct {
		EventType string
		Count     int64
	}

	var results []result
	db := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Select("event_type, COUNT(*) as count").
		Where("event_type IN ?", eventTypes).
		Where("archived_at IS NULL").
		Where("date >= ? AND date <= ?", from, to)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	if err := db.Group("event_type").Find(&results).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	counts := make(map[string]int64, len(results))
	for _, r := range results {
		counts[r.EventType] = r.Count
	}
	return counts, nil
}

func (r *metricEventRepository) CountDistinctActorsByPeriod(ctx context.Context, hasOrgID bool, orgID uuid.UUID, from, to time.Time) (int64, error) {
	var count int64

	db := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Where("archived_at IS NULL").
		Where("date >= ? AND date <= ?", from, to)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	if err := db.Select("COUNT(DISTINCT actor_id)").Count(&count).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}
	return count, nil
}

func (r *metricEventRepository) FindTimeSeries(ctx context.Context, eventTypes []string, hasOrgID bool, orgID uuid.UUID, from, to time.Time, periodType string) ([]*domain.MetricTimeSeriesPoint, error) {
	var points []*domain.MetricTimeSeriesPoint

	db := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Where("event_type IN ?", eventTypes).
		Where("archived_at IS NULL").
		Where("date >= ? AND date <= ?", from, to)

	if hasOrgID {
		db = db.Where("organization_id = ?", orgID)
	}

	switch periodType {
	case "daily":
		db = db.Select("TO_CHAR(date, 'YYYY-MM-DD') as date, NULL as hour, COUNT(*) as count, 0 as value").
			Group("date").
			Order("date ASC")
	case "hourly":
		db = db.Select("TO_CHAR(date, 'YYYY-MM-DD') as date, hour, COUNT(*) as count, 0 as value").
			Group("date, hour").
			Order("date ASC, hour ASC")
	case "weekly":
		db = db.Select("TO_CHAR(date_trunc('week', date), 'YYYY-MM-DD') as date, NULL as hour, COUNT(*) as count, 0 as value").
			Group("date_trunc('week', date)").
			Order("date_trunc('week', date) ASC")
	case "monthly":
		db = db.Select("TO_CHAR(date_trunc('month', date), 'YYYY-MM-DD') as date, NULL as hour, COUNT(*) as count, 0 as value").
			Group("date_trunc('month', date)").
			Order("date_trunc('month', date) ASC")
	default:
		return nil, apperrors.WrapInternal(fmt.Errorf("unsupported period type: %s", periodType))
	}

	if err := db.Scan(&points).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	return points, nil
}

func (r *metricEventRepository) FindDistinctOrganizationIDs(ctx context.Context) ([]uuid.UUID, error) {
	var orgIDs []uuid.UUID
	if err := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Where("organization_id IS NOT NULL AND archived_at IS NULL AND deleted_at IS NULL").
		Distinct("organization_id").
		Pluck("organization_id", &orgIDs).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return orgIDs, nil
}

func (r *metricEventRepository) ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Unscoped().
		Where("created_at < ? AND archived_at IS NULL AND deleted_at IS NULL", cutoff).
		Update("archived_at", gorm.Expr("NOW()"))

	if result.Error != nil {
		return 0, apperrors.WrapInternal(result.Error)
	}
	return result.RowsAffected, nil
}

func (r *metricEventRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.MetricEvent{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
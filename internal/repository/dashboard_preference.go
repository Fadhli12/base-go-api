package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DashboardPreferenceRepository defines the data access interface for dashboard preferences.
type DashboardPreferenceRepository interface {
	FindByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.DashboardPreference, error)
	Upsert(ctx context.Context, preference *domain.DashboardPreference) error
}

// --- DashboardPreferenceRepository implementation ---

type dashboardPreferenceRepository struct {
	db *gorm.DB
}

// NewDashboardPreferenceRepository creates a new GORM-backed DashboardPreferenceRepository.
func NewDashboardPreferenceRepository(db *gorm.DB) DashboardPreferenceRepository {
	return &dashboardPreferenceRepository{db: db}
}

// FindByOrganization returns nil (not an error) when no preference row exists for the organization.
// This is CRITICAL because the default behavior (no row = all categories visible) needs
// to distinguish between "not found" and "error".
func (r *dashboardPreferenceRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.DashboardPreference, error) {
	var pref domain.DashboardPreference
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		First(&pref).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &pref, nil
}

// Upsert inserts or updates a dashboard preference using ON CONFLICT DO UPDATE.
func (r *dashboardPreferenceRepository) Upsert(ctx context.Context, preference *domain.DashboardPreference) error {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "organization_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"metric_categories", "updated_by_user_id", "updated_at"}),
	}).Create(preference)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	return nil
}
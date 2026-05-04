package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SystemSettingsRepository defines the interface for system settings data access operations
type SystemSettingsRepository interface {
	// Upsert creates or updates system settings for an organization, handling soft-delete recovery
	Upsert(ctx context.Context, settings *domain.SystemSettings) error
	// FindByOrgID retrieves system settings for an organization
	// Returns errors.ErrNotFound if not found, indicating defaults should be used
	FindByOrgID(ctx context.Context, orgID uuid.UUID) (*domain.SystemSettings, error)
}

// systemSettingsRepository implements SystemSettingsRepository
type systemSettingsRepository struct {
	db *gorm.DB
}

// NewSystemSettingsRepository creates a new SystemSettingsRepository instance
func NewSystemSettingsRepository(db *gorm.DB) SystemSettingsRepository {
	return &systemSettingsRepository{db: db}
}

// Upsert creates or updates system settings using INSERT ... ON CONFLICT
// Handles soft-delete recovery by resetting deleted_at to NULL on conflict
// Ensures only one system settings record exists per organization
func (r *systemSettingsRepository) Upsert(ctx context.Context, settings *domain.SystemSettings) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"settings", "updated_at", "deleted_at"}),
	}).Create(settings).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByOrgID retrieves system settings for an organization
// Returns errors.ErrNotFound if not found, indicating defaults should be used
func (r *systemSettingsRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID) (*domain.SystemSettings, error) {
	var settings domain.SystemSettings
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &settings, nil
}

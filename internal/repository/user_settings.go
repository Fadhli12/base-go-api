package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserSettingsRepository defines the interface for user settings data access operations
type UserSettingsRepository interface {
	// Upsert creates or updates user settings, handling soft-delete recovery
	Upsert(ctx context.Context, settings *domain.UserSettings) error
	// FindByUserIDAndOrgID retrieves user settings for a specific user in an organization
	FindByUserIDAndOrgID(ctx context.Context, userID, orgID uuid.UUID) (*domain.UserSettings, error)
	// FindByOrgID returns paginated user settings for all users in an organization
	FindByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserSettings, int64, error)
}

// userSettingsRepository implements UserSettingsRepository
type userSettingsRepository struct {
	db *gorm.DB
}

// NewUserSettingsRepository creates a new UserSettingsRepository instance
func NewUserSettingsRepository(db *gorm.DB) UserSettingsRepository {
	return &userSettingsRepository{db: db}
}

// Upsert creates or updates user settings using INSERT ... ON CONFLICT
// Handles soft-delete recovery by resetting deleted_at to NULL on conflict
func (r *userSettingsRepository) Upsert(ctx context.Context, settings *domain.UserSettings) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"settings", "updated_at", "deleted_at"}),
	}).Create(settings).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByUserIDAndOrgID retrieves user settings for a specific user in an organization
// Returns errors.ErrNotFound if not found, indicating defaults should be used
func (r *userSettingsRepository) FindByUserIDAndOrgID(ctx context.Context, userID, orgID uuid.UUID) (*domain.UserSettings, error) {
	var settings domain.UserSettings
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ? AND deleted_at IS NULL", userID, orgID).
		First(&settings).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &settings, nil
}

// FindByOrgID returns paginated user settings for all users in an organization, ordered by created_at DESC
func (r *userSettingsRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserSettings, int64, error) {
	var settings []*domain.UserSettings
	var total int64

	base := r.db.WithContext(ctx).
		Model(&domain.UserSettings{}).
		Where("organization_id = ? AND deleted_at IS NULL", orgID)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&settings).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return settings, total, nil
}

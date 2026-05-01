package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationPreferenceRepository defines the interface for notification preference data access operations
type NotificationPreferenceRepository interface {
	// Upsert inserts or updates a notification preference (INSERT ... ON CONFLICT)
	Upsert(ctx context.Context, pref *domain.NotificationPreference) error
	// FindByUserID returns all active notification preferences for a user
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.NotificationPreference, error)
	// FindByUserIDAndType returns the preference for a specific notification type
	FindByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) (*domain.NotificationPreference, error)
	// DeleteByUserIDAndType soft-deletes the preference for a specific notification type
	DeleteByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) error
}

// notificationPreferenceRepository implements NotificationPreferenceRepository
type notificationPreferenceRepository struct {
	db *gorm.DB
}

// NewNotificationPreferenceRepository creates a new NotificationPreferenceRepository instance
func NewNotificationPreferenceRepository(db *gorm.DB) NotificationPreferenceRepository {
	return &notificationPreferenceRepository{db: db}
}

// Upsert inserts a new preference or updates it if (user_id, notification_type) already exists.
// Uses explicit column map to force GORM to include zero-value fields (e.g. push_enabled=false),
// which GORM otherwise skips during CREATE, causing the DB DEFAULT (true) to be used instead.
func (r *notificationPreferenceRepository) Upsert(ctx context.Context, pref *domain.NotificationPreference) error {
	// Build explicit values map to bypass GORM's zero-value skipping for bool fields.
	// GORM's Create() skips fields with Go zero values (false for bool, "" for string, etc.),
	// which means push_enabled=false would be omitted from the INSERT, and the PostgreSQL
	// column DEFAULT (true) would be used instead.
	values := map[string]interface{}{
		"user_id":           pref.UserID,
		"notification_type": pref.NotificationType,
		"email_enabled":     pref.EmailEnabled,
		"push_enabled":      pref.PushEnabled,
		"deleted_at":        pref.DeletedAt,
	}

	err := r.db.WithContext(ctx).
		Model(&domain.NotificationPreference{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "notification_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"email_enabled", "push_enabled", "updated_at", "deleted_at"}),
		}).
		Create(values).Error
	if err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByUserID returns all active notification preferences for a user
func (r *notificationPreferenceRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.NotificationPreference, error) {
	var prefs []*domain.NotificationPreference
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&prefs).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return prefs, nil
}

// FindByUserIDAndType returns the preference for a specific notification type
func (r *notificationPreferenceRepository) FindByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) (*domain.NotificationPreference, error) {
	var pref domain.NotificationPreference
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND notification_type = ?", userID, notifType).
		First(&pref).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &pref, nil
}

// DeleteByUserIDAndType soft-deletes the preference for a specific notification type
func (r *notificationPreferenceRepository) DeleteByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND notification_type = ?", userID, notifType).
		Delete(&domain.NotificationPreference{})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

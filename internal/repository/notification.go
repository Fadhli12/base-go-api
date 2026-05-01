package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationRepository defines the interface for notification data access operations.
// Notification records are never hard-deleted; use ArchiveByID to hide them from active views.
type NotificationRepository interface {
	// Create inserts a new notification into the database
	Create(ctx context.Context, notification *domain.Notification) error
	// CreateBatch inserts multiple notifications in a single operation
	CreateBatch(ctx context.Context, notifications []*domain.Notification) error
	// FindByID finds a notification by its UUID
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	// FindByUserID returns paginated active (non-archived) notifications for a user
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error)
	// FindUnreadByUserID returns paginated unread active notifications for a user
	FindUnreadByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error)
	// CountUnreadByUserID returns the number of unread active notifications for a user
	CountUnreadByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	// MarkAsRead sets read_at for the notification owned by the given user
	MarkAsRead(ctx context.Context, id, userID uuid.UUID) error
	// MarkAllAsRead sets read_at on all unread active notifications for a user.
	// If notifType is non-nil, only notifications of that type are updated.
	// Returns the number of rows affected.
	MarkAllAsRead(ctx context.Context, userID uuid.UUID, notifType *string) (int64, error)
	// ArchiveByID sets archived_at on the notification owned by the given user
	ArchiveByID(ctx context.Context, id, userID uuid.UUID) error
}

// notificationRepository implements NotificationRepository
type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new NotificationRepository instance
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

// Create inserts a new notification into the database
func (r *notificationRepository) Create(ctx context.Context, notification *domain.Notification) error {
	if err := r.db.WithContext(ctx).Create(notification).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// CreateBatch inserts multiple notifications in a single operation
func (r *notificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&notifications).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a notification by its UUID
func (r *notificationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var notification domain.Notification
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&notification).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &notification, nil
}

// FindByUserID returns paginated active (non-archived) notifications for a user, ordered newest first
func (r *notificationRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	var notifications []*domain.Notification
	var total int64

	base := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND archived_at IS NULL", userID)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND archived_at IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return notifications, total, nil
}

// FindUnreadByUserID returns paginated unread active notifications for a user, ordered newest first
func (r *notificationRepository) FindUnreadByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	var notifications []*domain.Notification
	var total int64

	base := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND read_at IS NULL AND archived_at IS NULL", userID)

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND read_at IS NULL AND archived_at IS NULL", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return notifications, total, nil
}

// CountUnreadByUserID returns the number of unread active notifications for a user
func (r *notificationRepository) CountUnreadByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND read_at IS NULL AND archived_at IS NULL", userID).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

// MarkAsRead sets read_at for the notification owned by the given user
func (r *notificationRepository) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL AND archived_at IS NULL", id, userID).
		Update("read_at", time.Now())
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkAllAsRead sets read_at on all unread active notifications for a user.
// If notifType is non-nil, only notifications of that type are updated.
// Returns the number of rows affected.
func (r *notificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID, notifType *string) (int64, error) {
	query := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("user_id = ? AND read_at IS NULL AND archived_at IS NULL", userID)

	if notifType != nil {
		query = query.Where("type = ?", *notifType)
	}

	result := query.Update("read_at", time.Now())
	if result.Error != nil {
		return 0, errors.WrapInternal(result.Error)
	}
	return result.RowsAffected, nil
}

// ArchiveByID sets archived_at on the notification owned by the given user
func (r *notificationRepository) ArchiveByID(ctx context.Context, id, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Notification{}).
		Where("id = ? AND user_id = ? AND archived_at IS NULL", id, userID).
		Update("archived_at", time.Now())
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

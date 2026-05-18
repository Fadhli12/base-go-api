package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IdempotencyRepository defines the data access interface for idempotency records.
type IdempotencyRepository interface {
	// Create persists a new idempotency record.
	Create(ctx context.Context, record *domain.IdempotencyRecord) error
	// FindByKey locates an active (non-deleted) idempotency record by its key, user, method, and path.
	FindByKey(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error)
	// FindByID retrieves an idempotency record by its primary key.
	FindByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error)
	// UpdateStatus updates the status and cached response fields of an existing record.
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, responseStatusCode int, responseBody string, responseHeaders map[string]string) error
	// CleanupExpired soft-deletes records that have expired past the retention window.
	CleanupExpired(ctx context.Context, retentionDays int) (int64, error)
	// ListByUser returns paginated idempotency records for a specific user.
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error)
	// SoftDelete marks an idempotency record as deleted by setting deleted_at.
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// --- IdempotencyRepository implementation ---

type idempotencyRepository struct {
	db *gorm.DB
}

// NewIdempotencyRepository creates a new GORM-backed IdempotencyRepository.
func NewIdempotencyRepository(db *gorm.DB) IdempotencyRepository {
	return &idempotencyRepository{db: db}
}

// Create persists a new idempotency record.
func (r *idempotencyRepository) Create(ctx context.Context, record *domain.IdempotencyRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByKey locates an active idempotency record by its key, user, method, and path.
// GORM soft-delete scope (deleted_at IS NULL) is automatically applied via gorm.DeletedAt field.
func (r *idempotencyRepository) FindByKey(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error) {
	var record domain.IdempotencyRecord
	if err := r.db.WithContext(ctx).
		Where("idempotency_key = ? AND user_id = ? AND http_method = ? AND request_path = ?", key, userID, method, path).
		First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &record, nil
}

// FindByID retrieves an idempotency record by its primary key.
// GORM soft-delete scope is automatically applied.
func (r *idempotencyRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error) {
	var record domain.IdempotencyRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &record, nil
}

// UpdateStatus updates the status and cached response fields of an existing record.
func (r *idempotencyRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, responseStatusCode int, responseBody string, responseHeaders map[string]string) error {
	updates := map[string]interface{}{
		"status":               status,
		"response_status_code": responseStatusCode,
		"response_body":        responseBody,
		"response_body_size":   len(responseBody),
	}
	if responseHeaders != nil {
		updates["response_headers"] = domain.MapStringString(responseHeaders)
	}

	result := r.db.WithContext(ctx).Model(&domain.IdempotencyRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// CleanupExpired soft-deletes records that have expired past the retention window.
// Records with expires_at older than cutoff are soft-deleted by setting deleted_at.
func (r *idempotencyRepository) CleanupExpired(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result := r.db.WithContext(ctx).Model(&domain.IdempotencyRecord{}).
		Unscoped().
		Where("expires_at < ? AND deleted_at IS NULL", cutoff).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		return 0, apperrors.WrapInternal(result.Error)
	}
	return result.RowsAffected, nil
}

// ListByUser returns paginated idempotency records for a specific user, ordered by creation time descending.
func (r *idempotencyRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error) {
	var records []*domain.IdempotencyRecord
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.IdempotencyRecord{}).
		Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	offset := (page - 1) * pageSize
	if err := db.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return records, total, nil
}

// SoftDelete marks an idempotency record as deleted by setting deleted_at.
// GORM soft-delete via DeletedAt field.
func (r *idempotencyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.IdempotencyRecord{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
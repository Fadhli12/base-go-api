package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailQueueRepository defines the interface for email queue data access operations
// NOTE: EmailQueue does NOT support soft delete - it's a permanent audit trail
type EmailQueueRepository interface {
	// Queue operations
	Create(ctx context.Context, email *domain.EmailQueue) error
	Update(ctx context.Context, email *domain.EmailQueue) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailQueue, error)
	FindByStatus(ctx context.Context, status domain.EmailStatus, limit, offset int) ([]*domain.EmailQueue, int64, error)
	FindByRecipient(ctx context.Context, toAddress string, limit, offset int) ([]*domain.EmailQueue, int64, error)
	GetNextBatch(ctx context.Context, batchSize int) ([]*domain.EmailQueue, error)

	// Status transitions
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	MarkSent(ctx context.Context, id uuid.UUID, provider, messageID string) error
	MarkDelivered(ctx context.Context, id uuid.UUID) error
	MarkBounced(ctx context.Context, id uuid.UUID, reason string) error
	MarkFailed(ctx context.Context, id uuid.UUID, err error) error

	// Retry management
	IncrementAttempts(ctx context.Context, id uuid.UUID) error
	CanRetry(ctx context.Context, id uuid.UUID) (bool, error)
}

// emailQueueRepository implements EmailQueueRepository interface
type emailQueueRepository struct {
	db *gorm.DB
}

// NewEmailQueueRepository creates a new EmailQueueRepository instance
func NewEmailQueueRepository(db *gorm.DB) EmailQueueRepository {
	return &emailQueueRepository{db: db}
}

// Create inserts a new email into the queue
func (r *emailQueueRepository) Create(ctx context.Context, email *domain.EmailQueue) error {
	if err := r.db.WithContext(ctx).Create(email).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// Update updates an existing email in the queue
func (r *emailQueueRepository) Update(ctx context.Context, email *domain.EmailQueue) error {
	result := r.db.WithContext(ctx).Save(email)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// FindByID finds an email queue entry by its UUID
func (r *emailQueueRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailQueue, error) {
	var email domain.EmailQueue
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&email).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &email, nil
}

// FindByStatus retrieves emails by status with pagination
func (r *emailQueueRepository) FindByStatus(ctx context.Context, status domain.EmailStatus, limit, offset int) ([]*domain.EmailQueue, int64, error) {
	var emails []*domain.EmailQueue
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("status = ?", status).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&emails).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return emails, total, nil
}

// FindByRecipient retrieves emails sent to a specific address with pagination
func (r *emailQueueRepository) FindByRecipient(ctx context.Context, toAddress string, limit, offset int) ([]*domain.EmailQueue, int64, error) {
	var emails []*domain.EmailQueue
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("to_address = ?", toAddress).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	err := r.db.WithContext(ctx).
		Where("to_address = ?", toAddress).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&emails).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return emails, total, nil
}

// GetNextBatch retrieves the next batch of queued emails for processing
func (r *emailQueueRepository) GetNextBatch(ctx context.Context, batchSize int) ([]*domain.EmailQueue, error) {
	var emails []*domain.EmailQueue
	err := r.db.WithContext(ctx).
		Where("status = ?", domain.EmailStatusQueued).
		Where("attempts < max_attempts").
		Order("created_at ASC").
		Limit(batchSize).
		Find(&emails).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return emails, nil
}

// MarkProcessing marks an email as processing
func (r *emailQueueRepository) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ? AND status = ?", id, domain.EmailStatusQueued).
		Update("status", domain.EmailStatusProcessing)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkSent marks an email as sent with provider details
func (r *emailQueueRepository) MarkSent(ctx context.Context, id uuid.UUID, provider, messageID string) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     domain.EmailStatusSent,
			"provider":   provider,
			"message_id": messageID,
			"sent_at":    time.Now(),
		})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkDelivered marks an email as delivered
func (r *emailQueueRepository) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       domain.EmailStatusDelivered,
			"delivered_at": time.Now(),
		})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkBounced marks an email as bounced with reason
func (r *emailQueueRepository) MarkBounced(ctx context.Context, id uuid.UUID, reason string) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        domain.EmailStatusBounced,
			"bounced_at":    time.Now(),
			"bounce_reason": reason,
		})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkFailed marks an email as failed with error
func (r *emailQueueRepository) MarkFailed(ctx context.Context, id uuid.UUID, err error) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     domain.EmailStatusFailed,
			"last_error": err.Error(),
		})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// IncrementAttempts increments the attempt counter for retry logic
func (r *emailQueueRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.EmailQueue{}).
		Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + ?", 1))
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// CanRetry checks if an email can be retried
func (r *emailQueueRepository) CanRetry(ctx context.Context, id uuid.UUID) (bool, error) {
	var email domain.EmailQueue
	err := r.db.WithContext(ctx).
		Select("attempts, max_attempts").
		Where("id = ?", id).
		First(&email).Error
	if err == gorm.ErrRecordNotFound {
		return false, errors.ErrNotFound
	}
	if err != nil {
		return false, errors.WrapInternal(err)
	}
	return email.Attempts < email.MaxAttempts, nil
}
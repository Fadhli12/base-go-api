package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"gorm.io/gorm"
)

// EmailBounceRepository defines the interface for email bounce data access operations
// NOTE: EmailBounce is INSERT-ONLY for compliance - no updates or deletes
type EmailBounceRepository interface {
	// Insert-only operations
	Create(ctx context.Context, bounce *domain.EmailBounce) error

	// Query operations
	FindByID(ctx context.Context, id string) (*domain.EmailBounce, error)
	FindByEmail(ctx context.Context, email string, limit, offset int) ([]*domain.EmailBounce, int64, error)
	FindByType(ctx context.Context, bounceType domain.BounceType, limit, offset int) ([]*domain.EmailBounce, int64, error)
	FindByMessageID(ctx context.Context, messageID string) (*domain.EmailBounce, error)

	// Analytics
	CountByType(ctx context.Context) (map[domain.BounceType]int64, error)
	CountByEmail(ctx context.Context, email string) (int64, error)
}

// emailBounceRepository implements EmailBounceRepository interface
type emailBounceRepository struct {
	db *gorm.DB
}

// NewEmailBounceRepository creates a new EmailBounceRepository instance
func NewEmailBounceRepository(db *gorm.DB) EmailBounceRepository {
	return &emailBounceRepository{db: db}
}

// Create inserts a new bounce record (insert-only for compliance)
func (r *emailBounceRepository) Create(ctx context.Context, bounce *domain.EmailBounce) error {
	if err := r.db.WithContext(ctx).Create(bounce).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a bounce record by its UUID
func (r *emailBounceRepository) FindByID(ctx context.Context, id string) (*domain.EmailBounce, error) {
	var bounce domain.EmailBounce
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&bounce).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &bounce, nil
}

// FindByEmail retrieves bounce records for a specific email address with pagination
func (r *emailBounceRepository) FindByEmail(ctx context.Context, email string, limit, offset int) ([]*domain.EmailBounce, int64, error) {
	var bounces []*domain.EmailBounce
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.EmailBounce{}).
		Where("email = ?", email).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination (most recent first)
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&bounces).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return bounces, total, nil
}

// FindByType retrieves bounce records by type with pagination
func (r *emailBounceRepository) FindByType(ctx context.Context, bounceType domain.BounceType, limit, offset int) ([]*domain.EmailBounce, int64, error) {
	var bounces []*domain.EmailBounce
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.EmailBounce{}).
		Where("bounce_type = ?", bounceType).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination (most recent first)
	err := r.db.WithContext(ctx).
		Where("bounce_type = ?", bounceType).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&bounces).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return bounces, total, nil
}

// FindByMessageID finds a bounce record by provider message ID
func (r *emailBounceRepository) FindByMessageID(ctx context.Context, messageID string) (*domain.EmailBounce, error) {
	var bounce domain.EmailBounce
	err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		First(&bounce).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &bounce, nil
}

// CountByType returns count of bounces grouped by type
func (r *emailBounceRepository) CountByType(ctx context.Context) (map[domain.BounceType]int64, error) {
	type countResult struct {
		BounceType domain.BounceType
		Count      int64
	}

	var results []countResult
	err := r.db.WithContext(ctx).
		Model(&domain.EmailBounce{}).
		Select("bounce_type, count(*) as count").
		Group("bounce_type").
		Scan(&results).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	// Convert to map
	counts := make(map[domain.BounceType]int64)
	for _, r := range results {
		counts[r.BounceType] = r.Count
	}

	return counts, nil
}

// CountByEmail returns the count of bounces for a specific email address
func (r *emailBounceRepository) CountByEmail(ctx context.Context, email string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.EmailBounce{}).
		Where("email = ?", email).
		Count(&count).Error
	if err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}
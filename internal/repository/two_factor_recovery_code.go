package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TwoFactorRecoveryCodeRepository defines the interface for recovery code data access
type TwoFactorRecoveryCodeRepository interface {
	Create(ctx context.Context, code *domain.TwoFactorRecoveryCode) error
	FindByUserAndHash(ctx context.Context, userID uuid.UUID, codeHash string) (*domain.TwoFactorRecoveryCode, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	DeleteAllByUser(ctx context.Context, userID uuid.UUID) error
	CountUnusedByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

// twoFactorRecoveryCodeRepository implements TwoFactorRecoveryCodeRepository interface
type twoFactorRecoveryCodeRepository struct {
	db *gorm.DB
}

// NewTwoFactorRecoveryCodeRepository creates a new TwoFactorRecoveryCodeRepository instance
func NewTwoFactorRecoveryCodeRepository(db *gorm.DB) TwoFactorRecoveryCodeRepository {
	return &twoFactorRecoveryCodeRepository{
		db: db,
	}
}

// Create inserts a new recovery code into the database
func (r *twoFactorRecoveryCodeRepository) Create(ctx context.Context, code *domain.TwoFactorRecoveryCode) error {
	if err := r.db.WithContext(ctx).Create(code).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByUserAndHash finds an unused recovery code by user ID and code hash
func (r *twoFactorRecoveryCodeRepository) FindByUserAndHash(ctx context.Context, userID uuid.UUID, codeHash string) (*domain.TwoFactorRecoveryCode, error) {
	var code domain.TwoFactorRecoveryCode
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, codeHash).
		First(&code).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &code, nil
}

// MarkUsed marks a recovery code as used by setting used_at
func (r *twoFactorRecoveryCodeRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&domain.TwoFactorRecoveryCode{}).
		Where("id = ?", id).
		Update("used_at", now)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// DeleteAllByUser soft deletes all recovery codes for a user
func (r *twoFactorRecoveryCodeRepository) DeleteAllByUser(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&domain.TwoFactorRecoveryCode{})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	return nil
}

// CountUnusedByUser counts the number of unused recovery codes for a user
func (r *twoFactorRecoveryCodeRepository) CountUnusedByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.TwoFactorRecoveryCode{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}
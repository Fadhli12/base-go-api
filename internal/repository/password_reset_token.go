package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PasswordResetTokenRepository defines the interface for password reset token data access operations
type PasswordResetTokenRepository interface {
	// Create inserts a new password reset token into the database
	Create(ctx context.Context, token *domain.PasswordResetToken) error

	// FindByHash finds a password reset token by its hash
	FindByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error)

	// FindValidByUserID finds all valid (unused and not expired) tokens for a user
	FindValidByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.PasswordResetToken, error)

	// MarkUsed marks a password reset token as used
	MarkUsed(ctx context.Context, hash string) error

	// RevokeAllByUser revokes all password reset tokens for a specific user
	// This should be called when a password is successfully reset to invalidate any remaining tokens
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) error

	// DeleteExpired removes all expired tokens (for cleanup job)
	DeleteExpired(ctx context.Context) error
}

// passwordResetTokenRepository implements PasswordResetTokenRepository interface
type passwordResetTokenRepository struct {
	db *gorm.DB
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository instance
func NewPasswordResetTokenRepository(db *gorm.DB) PasswordResetTokenRepository {
	return &passwordResetTokenRepository{
		db: db,
	}
}

// Create inserts a new password reset token into the database
func (r *passwordResetTokenRepository) Create(ctx context.Context, token *domain.PasswordResetToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByHash finds a password reset token by its hash
func (r *passwordResetTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	var token domain.PasswordResetToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &token, nil
}

// FindValidByUserID finds all valid (unused and not expired) tokens for a user
func (r *passwordResetTokenRepository) FindValidByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.PasswordResetToken, error) {
	var tokens []*domain.PasswordResetToken
	now := time.Now()

	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL AND expires_at > ?", userID, now).
		Find(&tokens).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return tokens, nil
}

// MarkUsed marks a password reset token as used
func (r *passwordResetTokenRepository) MarkUsed(ctx context.Context, hash string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.PasswordResetToken{}).
		Where("token_hash = ?", hash).
		Update("used_at", now)

	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// RevokeAllByUser revokes all password reset tokens for a specific user
func (r *passwordResetTokenRepository) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	err := r.db.WithContext(ctx).Model(&domain.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", now).Error
	if err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// DeleteExpired removes all tokens that have expired (for cleanup job)
func (r *passwordResetTokenRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&domain.PasswordResetToken{}).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}
package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshTokenRepository defines the interface for refresh token data access operations
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	MarkRevoked(ctx context.Context, hash string) error
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshToken, error)
}

// refreshTokenRepository implements RefreshTokenRepository interface
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new RefreshTokenRepository instance
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{
		db: db,
	}
}

// Create inserts a new refresh token into the database
func (r *refreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByHash finds a refresh token by its hash
func (r *refreshTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &token, nil
}

// MarkRevoked marks a refresh token as revoked
func (r *refreshTokenRepository) MarkRevoked(ctx context.Context, hash string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("token_hash = ?", hash).
		Update("revoked_at", now)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// RevokeAllByUser revokes all refresh tokens for a specific user
func (r *refreshTokenRepository) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	err := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
	if err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// DeleteExpired deletes all tokens that have expired (for cleanup job)
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&domain.RefreshToken{}).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByUserID finds all refresh tokens for a specific user
func (r *refreshTokenRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshToken, error) {
	var tokens []*domain.RefreshToken
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return tokens, nil
}

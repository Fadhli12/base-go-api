package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthAccountRepository defines the interface for OAuth account data access operations.
// OAuthAccount supports soft delete via gorm.DeletedAt.
type OAuthAccountRepository interface {
	Create(ctx context.Context, account *domain.OAuthAccount) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.OAuthAccount, error)
	FindByUserAndProvider(ctx context.Context, userID, providerID uuid.UUID) (*domain.OAuthAccount, error)
	FindByProviderAndProviderUserID(ctx context.Context, providerID uuid.UUID, providerUserID string) (*domain.OAuthAccount, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthAccount, error)
	CountAuthMethodsByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// oauthAccountRepository implements OAuthAccountRepository interface.
type oauthAccountRepository struct {
	db *gorm.DB
}

// NewOAuthAccountRepository creates a new OAuthAccountRepository instance.
func NewOAuthAccountRepository(db *gorm.DB) OAuthAccountRepository {
	return &oauthAccountRepository{db: db}
}

// Create inserts a new OAuth account into the database.
func (r *oauthAccountRepository) Create(ctx context.Context, account *domain.OAuthAccount) error {
	if err := r.db.WithContext(ctx).Create(account).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByID finds an OAuth account by its UUID, preloading the associated Provider.
func (r *oauthAccountRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.OAuthAccount, error) {
	var account domain.OAuthAccount
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Where("id = ?", id).
		First(&account).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &account, nil
}

// FindByUserAndProvider finds an OAuth account by user ID and provider ID.
// Returns ErrNotFound if the user has no account with the given provider.
func (r *oauthAccountRepository) FindByUserAndProvider(ctx context.Context, userID, providerID uuid.UUID) (*domain.OAuthAccount, error) {
	var account domain.OAuthAccount
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Where("user_id = ? AND provider_id = ? AND deleted_at IS NULL", userID, providerID).
		First(&account).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &account, nil
}

// FindByProviderAndProviderUserID finds an OAuth account by provider ID and the external user ID.
// This is used during OAuth callback to find an existing linked account.
// Uses explicit deleted_at IS NULL clause for safety with joins.
func (r *oauthAccountRepository) FindByProviderAndProviderUserID(ctx context.Context, providerID uuid.UUID, providerUserID string) (*domain.OAuthAccount, error) {
	var account domain.OAuthAccount
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Where("provider_id = ? AND provider_user_id = ? AND deleted_at IS NULL", providerID, providerUserID).
		First(&account).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &account, nil
}

// FindByUserID retrieves all non-deleted OAuth accounts for a user, preloading Provider info.
func (r *oauthAccountRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.OAuthAccount, error) {
	var accounts []*domain.OAuthAccount
	err := r.db.WithContext(ctx).
		Preload("Provider").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Find(&accounts).Error
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return accounts, nil
}

// CountAuthMethodsByUserID counts total auth methods for a user.
// Returns the sum of:
//   - Number of non-deleted OAuth accounts linked to the user
//   - 1 if the user has a real password (password_hash does NOT start with 'oauth-')
//
// This is used to enforce the "at least one auth method" rule during unlink.
func (r *oauthAccountRepository) CountAuthMethodsByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	// Count linked OAuth providers (non-deleted)
	var oauthCount int64
	if err := r.db.WithContext(ctx).Model(&domain.OAuthAccount{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&oauthCount).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}

	// Check if user has a real password (not an OAuth placeholder)
	var hasPassword int64
	if err := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ? AND password_hash NOT LIKE ?", userID, "oauth-%").
		Count(&hasPassword).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}

	return oauthCount + hasPassword, nil
}

// SoftDelete performs a soft delete on an OAuth account by its ID.
func (r *oauthAccountRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.OAuthAccount{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
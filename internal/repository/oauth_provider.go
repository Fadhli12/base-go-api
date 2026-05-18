package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthProviderRepository defines the interface for OAuth provider data access operations.
// OAuthProvider supports soft delete via gorm.DeletedAt.
type OAuthProviderRepository interface {
	Create(ctx context.Context, provider *domain.OAuthProvider) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.OAuthProvider, error)
	FindByName(ctx context.Context, name string) (*domain.OAuthProvider, error)
	FindEnabled(ctx context.Context, orgID *uuid.UUID) ([]*domain.OAuthProvider, error)
	FindAll(ctx context.Context, orgID *uuid.UUID, limit, offset int) ([]*domain.OAuthProvider, int64, error)
	Update(ctx context.Context, provider *domain.OAuthProvider) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// oauthProviderRepository implements OAuthProviderRepository interface.
type oauthProviderRepository struct {
	db *gorm.DB
}

// NewOAuthProviderRepository creates a new OAuthProviderRepository instance.
func NewOAuthProviderRepository(db *gorm.DB) OAuthProviderRepository {
	return &oauthProviderRepository{db: db}
}

// Create inserts a new OAuth provider into the database.
func (r *oauthProviderRepository) Create(ctx context.Context, provider *domain.OAuthProvider) error {
	if err := r.db.WithContext(ctx).Create(provider).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByID finds an OAuth provider by its UUID.
// Automatically filters out soft-deleted records.
func (r *oauthProviderRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.OAuthProvider, error) {
	var provider domain.OAuthProvider
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Where("id = ?", id).
		First(&provider).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &provider, nil
}

// FindByName finds an OAuth provider by its unique name.
// Uses explicit WHERE clause for unique constraint lookup.
func (r *oauthProviderRepository) FindByName(ctx context.Context, name string) (*domain.OAuthProvider, error) {
	var provider domain.OAuthProvider
	err := r.db.WithContext(ctx).
		Where("name = ? AND deleted_at IS NULL", name).
		First(&provider).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &provider, nil
}

// FindEnabled retrieves all enabled OAuth providers.
// If orgID is nil or uuid.Nil, returns only global providers (organization_id IS NULL).
// If orgID is a real UUID, returns both global providers AND org-scoped providers for that org.
// Only returns providers where is_enabled = true AND deleted_at IS NULL.
func (r *oauthProviderRepository) FindEnabled(ctx context.Context, orgID *uuid.UUID) ([]*domain.OAuthProvider, error) {
	var providers []*domain.OAuthProvider

	db := r.db.WithContext(ctx).Model(&domain.OAuthProvider{}).
		Where("is_enabled = ? AND deleted_at IS NULL", true)

	if orgID == nil || *orgID == uuid.Nil {
		db = db.Where("organization_id IS NULL")
	} else {
		db = db.Where("organization_id IS NULL OR organization_id = ?", orgID)
	}

	if err := db.Preload("Organization").Find(&providers).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	return providers, nil
}

// FindAll retrieves OAuth providers with pagination and optional organization filter.
// If orgID is nil, returns global providers (organization_id IS NULL).
// If orgID is provided, returns org-scoped providers for that organization.
// Returns providers alongside total count for pagination.
func (r *oauthProviderRepository) FindAll(ctx context.Context, orgID *uuid.UUID, limit, offset int) ([]*domain.OAuthProvider, int64, error) {
	var providers []*domain.OAuthProvider
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.OAuthProvider{})

	if orgID == nil {
		db = db.Where("organization_id IS NULL")
	} else {
		db = db.Where("organization_id = ?", orgID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	err := db.Preload("Organization").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&providers).Error
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return providers, total, nil
}

// Update updates an existing OAuth provider in the database.
func (r *oauthProviderRepository) Update(ctx context.Context, provider *domain.OAuthProvider) error {
	result := r.db.WithContext(ctx).Save(provider)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on an OAuth provider by its ID.
func (r *oauthProviderRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.OAuthProvider{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
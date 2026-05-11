package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FeatureFlagRepository interface {
	Create(ctx context.Context, flag *domain.FeatureFlag) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.FeatureFlag, error)
	FindByKey(ctx context.Context, key string) (*domain.FeatureFlag, error)
	FindAll(ctx context.Context, limit, offset int) ([]*domain.FeatureFlag, int64, error)
	Update(ctx context.Context, flag *domain.FeatureFlag) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type featureFlagRepository struct {
	db *gorm.DB
}

func NewFeatureFlagRepository(db *gorm.DB) FeatureFlagRepository {
	return &featureFlagRepository{db: db}
}

func (r *featureFlagRepository) Create(ctx context.Context, flag *domain.FeatureFlag) error {
	if err := r.db.WithContext(ctx).Create(flag).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *featureFlagRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.FeatureFlag, error) {
	var flag domain.FeatureFlag
	if err := r.db.WithContext(ctx).First(&flag, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &flag, nil
}

func (r *featureFlagRepository) FindByKey(ctx context.Context, key string) (*domain.FeatureFlag, error) {
	var flag domain.FeatureFlag
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&flag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &flag, nil
}

func (r *featureFlagRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.FeatureFlag, int64, error) {
	var flags []*domain.FeatureFlag
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.FeatureFlag{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	query := db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&flags).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return flags, total, nil
}

func (r *featureFlagRepository) Update(ctx context.Context, flag *domain.FeatureFlag) error {
	result := r.db.WithContext(ctx).Save(flag)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *featureFlagRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.FeatureFlag{}).Where("id = ?", id).Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
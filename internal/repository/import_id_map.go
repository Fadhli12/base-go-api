package repository

import (
	"context"
	stderrors "errors"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImportIDMapRepository interface {
	Create(ctx context.Context, mapping *domain.ImportIDMap) error
	FindByJobAndEntityType(ctx context.Context, jobID uuid.UUID, entityType string) ([]*domain.ImportIDMap, error)
	FindByExternalID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (*domain.ImportIDMap, error)
	BatchCreate(ctx context.Context, mappings []*domain.ImportIDMap) error
	ResolveUUID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error)
}

type importIDMapRepository struct {
	db *gorm.DB
}

func NewImportIDMapRepository(db *gorm.DB) ImportIDMapRepository {
	return &importIDMapRepository{db: db}
}

func (r *importIDMapRepository) Create(ctx context.Context, mapping *domain.ImportIDMap) error {
	if err := r.db.WithContext(ctx).Create(mapping).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *importIDMapRepository) FindByJobAndEntityType(ctx context.Context, jobID uuid.UUID, entityType string) ([]*domain.ImportIDMap, error) {
	var mappings []*domain.ImportIDMap
	err := r.db.WithContext(ctx).
		Where("job_id = ? AND entity_type = ?", jobID, entityType).
		Find(&mappings).Error
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return mappings, nil
}

func (r *importIDMapRepository) FindByExternalID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (*domain.ImportIDMap, error) {
	var mapping domain.ImportIDMap
	err := r.db.WithContext(ctx).
		Where("job_id = ? AND entity_type = ? AND external_id = ?", jobID, entityType, externalID).
		First(&mapping).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return &mapping, nil
}

func (r *importIDMapRepository) BatchCreate(ctx context.Context, mappings []*domain.ImportIDMap) error {
	if len(mappings) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(mappings, 100).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *importIDMapRepository) ResolveUUID(ctx context.Context, jobID uuid.UUID, entityType string, externalID uuid.UUID) (uuid.UUID, error) {
	mapping, err := r.FindByExternalID(ctx, jobID, entityType, externalID)
	if err != nil {
		if stderrors.Is(err, apperrors.ErrNotFound) {
			return uuid.Nil, nil
		}
		return uuid.Nil, apperrors.WrapInternal(err)
	}
	return mapping.InternalID, nil
}
package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TagRepository interface {
	Create(ctx context.Context, tag *domain.Tag) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error)
	FindBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*domain.Tag, error)
	FindByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, sort, order string) ([]*domain.Tag, int64, error)
	Update(ctx context.Context, tag *domain.Tag) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	IncrementUsageCount(ctx context.Context, id uuid.UUID) error
	DecrementUsageCount(ctx context.Context, id uuid.UUID) error
	FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Tag, error)
	Autocomplete(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]*domain.Tag, error)
}

type tagRepository struct {
	db *gorm.DB
}

func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{db: db}
}

func (r *tagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	if err := r.db.WithContext(ctx).Create(tag).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *tagRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	var tag domain.Tag
	if err := r.db.WithContext(ctx).First(&tag, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &tag, nil
}

func (r *tagRepository) FindBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*domain.Tag, error) {
	var tag domain.Tag
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND slug = ?", orgID, slug).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &tag, nil
}

func (r *tagRepository) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Tag, error) {
	var tag domain.Tag
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND name = ?", orgID, name).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &tag, nil
}

func (r *tagRepository) FindByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, sort, order string) ([]*domain.Tag, int64, error) {
	var tags []*domain.Tag
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Tag{}).Where("organization_id = ?", orgID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	allowedSorts := map[string]string{
		"name":       "name",
		"usage_count": "usage_count",
		"created_at": "created_at",
	}
	sortCol, ok := allowedSorts[sort]
	if !ok {
		sortCol = "name"
	}

	orderDir := "ASC"
	if order == "desc" || order == "DESC" {
		orderDir = "DESC"
	}

	query := db.Order(sortCol + " " + orderDir)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&tags).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return tags, total, nil
}

func (r *tagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	if err := r.db.WithContext(ctx).Save(tag).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *tagRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&domain.Tag{}).Where("id = ?", id).Delete(&domain.Tag{}).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *tagRepository) IncrementUsageCount(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&domain.Tag{}).Where("id = ? AND usage_count >= 0", id).
		Update("usage_count", gorm.Expr("usage_count + 1")).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *tagRepository) DecrementUsageCount(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Model(&domain.Tag{}).Where("id = ? AND usage_count > 0", id).
		Update("usage_count", gorm.Expr("usage_count - 1")).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *tagRepository) Autocomplete(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]*domain.Tag, error) {
	var tags []*domain.Tag
	db := r.db.WithContext(ctx).Where("organization_id = ?", orgID)

	if query != "" {
		db = db.Where("name ILIKE ? OR slug ILIKE ?", "%"+query+"%", "%"+query+"%")
	}

	if limit <= 0 {
		limit = 10
	}

	orderClause := "name ASC"
	if query == "" {
		orderClause = "usage_count DESC, name ASC"
	}

	if err := db.Order(orderClause).Limit(limit).Find(&tags).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	return tags, nil
}

type EntityTagRepository interface {
	Create(ctx context.Context, entityTag *domain.EntityTag) error
	FindByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.EntityTag, error)
	FindByEntityAndTag(ctx context.Context, orgID uuid.UUID, entityType string, entityID, tagID uuid.UUID) (*domain.EntityTag, error)
	DeleteByEntityAndTag(ctx context.Context, id uuid.UUID) error
	ListTagDetailsByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.Tag, error)
}

type entityTagRepository struct {
	db *gorm.DB
}

func NewEntityTagRepository(db *gorm.DB) EntityTagRepository {
	return &entityTagRepository{db: db}
}

func (r *entityTagRepository) Create(ctx context.Context, entityTag *domain.EntityTag) error {
	if err := r.db.WithContext(ctx).Create(entityTag).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *entityTagRepository) FindByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.EntityTag, error) {
	var entityTags []*domain.EntityTag
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND entity_type = ? AND entity_id = ?", orgID, entityType, entityID).
		Find(&entityTags).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return entityTags, nil
}

func (r *entityTagRepository) FindByEntityAndTag(ctx context.Context, orgID uuid.UUID, entityType string, entityID, tagID uuid.UUID) (*domain.EntityTag, error) {
	var entityTag domain.EntityTag
	if err := r.db.WithContext(ctx).
		Where("organization_id = ? AND entity_type = ? AND entity_id = ? AND tag_id = ?", orgID, entityType, entityID, tagID).
		First(&entityTag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &entityTag, nil
}

func (r *entityTagRepository) DeleteByEntityAndTag(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&domain.EntityTag{}, "id = ?", id).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *entityTagRepository) ListTagDetailsByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.Tag, error) {
	var tags []*domain.Tag
	if err := r.db.WithContext(ctx).
		Model(&domain.Tag{}).
		Joins("JOIN entity_tags ON entity_tags.tag_id = tags.id").
		Where("entity_tags.organization_id = ? AND entity_tags.entity_type = ? AND entity_tags.entity_id = ? AND tags.deleted_at IS NULL", orgID, entityType, entityID).
		Find(&tags).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return tags, nil
}
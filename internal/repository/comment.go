package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CommentRepository defines the data access interface for comments.
type CommentRepository interface {
	Create(ctx context.Context, comment *domain.Comment) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Comment, error)
	FindByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error)
	FindReplies(ctx context.Context, parentID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error)
	Update(ctx context.Context, comment *domain.Comment) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	CountByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID) (int64, error)
}

type commentRepository struct {
	db *gorm.DB
}

// NewCommentRepository creates a new GORM-backed CommentRepository.
func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) Create(ctx context.Context, comment *domain.Comment) error {
	if err := r.db.WithContext(ctx).Create(comment).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

func (r *commentRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Comment, error) {
	var comment domain.Comment
	if err := r.db.WithContext(ctx).First(&comment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &comment, nil
}

func (r *commentRepository) FindByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error) {
	var comments []*domain.Comment
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Comment{}).
		Where("commentable_type = ? AND commentable_id = ?", commentableType, commentableID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	query := db.Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&comments).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return comments, total, nil
}

func (r *commentRepository) FindReplies(ctx context.Context, parentID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error) {
	var comments []*domain.Comment
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Comment{}).
		Where("parent_id = ?", parentID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	query := db.Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Offset(offset).Find(&comments).Error; err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return comments, total, nil
}

func (r *commentRepository) Update(ctx context.Context, comment *domain.Comment) error {
	result := r.db.WithContext(ctx).Save(comment)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *commentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.Comment{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *commentRepository) CountByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Comment{}).
		Where("commentable_type = ? AND commentable_id = ?", commentableType, commentableID).
		Count(&count).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}
	return count, nil
}
package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NewsRepository defines the interface for news data access operations
type NewsRepository interface {
	// Create inserts a new news article into the database
	Create(ctx context.Context, news *domain.News) error
	// FindByID finds a news article by its UUID
	FindByID(ctx context.Context, id uuid.UUID) (*domain.News, error)
	// FindBySlug finds a news article by its slug
	FindBySlug(ctx context.Context, slug string) (*domain.News, error)
	// FindByStatus finds all news articles with a specific status
	FindByStatus(ctx context.Context, status domain.NewsStatus, limit, offset int) ([]domain.News, error)
	// FindByAuthorID finds all news articles by an author
	FindByAuthorID(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]domain.News, error)
	// FindAll finds all news articles with pagination
	FindAll(ctx context.Context, limit, offset int) ([]domain.News, error)
	// Update updates an existing news article in the database
	Update(ctx context.Context, news *domain.News) error
	// SoftDelete performs a soft delete on a news article by its ID
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// CountByStatus counts news articles by status
	CountByStatus(ctx context.Context, status domain.NewsStatus) (int64, error)
	// CountByAuthorID counts news articles by author
	CountByAuthorID(ctx context.Context, authorID uuid.UUID) (int64, error)
	// CountAll counts all news articles
	CountAll(ctx context.Context) (int64, error)
}

// newsRepository implements NewsRepository interface
type newsRepository struct {
	db *gorm.DB
}

// NewNewsRepository creates a new NewsRepository instance
func NewNewsRepository(db *gorm.DB) NewsRepository {
	return &newsRepository{
		db: db,
	}
}

// Create inserts a new news article into the database
func (r *newsRepository) Create(ctx context.Context, news *domain.News) error {
	if err := r.db.WithContext(ctx).Create(news).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a news article by its UUID
func (r *newsRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.News, error) {
	var news domain.News
	if err := r.db.WithContext(ctx).Preload("Author").First(&news, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &news, nil
}

// FindBySlug finds a news article by its slug
func (r *newsRepository) FindBySlug(ctx context.Context, slug string) (*domain.News, error) {
	var news domain.News
	if err := r.db.WithContext(ctx).Preload("Author").Where("slug = ?", slug).First(&news).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &news, nil
}

// FindByStatus finds all news articles with a specific status
func (r *newsRepository) FindByStatus(ctx context.Context, status domain.NewsStatus, limit, offset int) ([]domain.News, error) {
	var news []domain.News
	query := r.db.WithContext(ctx).
		Preload("Author").
		Where("status = ?", status).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&news).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return news, nil
}

// FindByAuthorID finds all news articles by an author
func (r *newsRepository) FindByAuthorID(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]domain.News, error) {
	var news []domain.News
	query := r.db.WithContext(ctx).
		Preload("Author").
		Where("author_id = ?", authorID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&news).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return news, nil
}

// FindAll finds all news articles with pagination
func (r *newsRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.News, error) {
	var news []domain.News
	query := r.db.WithContext(ctx).
		Preload("Author").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&news).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return news, nil
}

// Update updates an existing news article in the database
func (r *newsRepository) Update(ctx context.Context, news *domain.News) error {
	result := r.db.WithContext(ctx).Save(news)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on a news article by its ID
func (r *newsRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.News{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// CountByStatus counts news articles by status
func (r *newsRepository) CountByStatus(ctx context.Context, status domain.NewsStatus) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.News{}).
		Where("status = ?", status).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

// CountByAuthorID counts news articles by author
func (r *newsRepository) CountByAuthorID(ctx context.Context, authorID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.News{}).
		Where("author_id = ?", authorID).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

// CountAll counts all news articles
func (r *newsRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.News{}).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

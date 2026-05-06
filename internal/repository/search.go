package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SearchParams contains search query, filters, pagination, and sorting
type SearchParams struct {
	Query    string                 // Full-text search query
	Filters  map[string]interface{} // Field-specific filters
	Page     int                   // Page number (1-based)
	PageSize int                   // Items per page
	SortBy   string                // Field to sort by
	SortDir  string                // Sort direction (asc/desc)
}

// SearchResult contains search results with optional highlights
type SearchResult[T any] struct {
	Items      []T                  // Result items
	Total      int64                // Total matching count
	Page       int                  // Current page
	PageSize   int                  // Items per page
	Highlights map[string][]string  // Field highlights from full-text search
}

// SearchRepository defines the interface for full-text search operations
type SearchRepository interface {
	Search(ctx context.Context, params SearchParams) (*SearchResult[map[string]interface{}], error)
	SearchByResource(ctx context.Context, resource string, params SearchParams) (*SearchResult[map[string]interface{}], error)
	HighlightMatches(ctx context.Context, query string, fields []string, text string) ([]string, error)
}

// SavedSearchRepository defines operations for saved search persistence.
type SavedSearchRepository interface {
	Create(ctx context.Context, search *domain.SavedSearch) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SavedSearch, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.SavedSearch, error)
	Update(ctx context.Context, search *domain.SavedSearch) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// savedSearchRepository implements SavedSearchRepository interface
type savedSearchRepository struct {
	db *gorm.DB
}

// NewSavedSearchRepository creates a new SavedSearchRepository instance
func NewSavedSearchRepository(db *gorm.DB) SavedSearchRepository {
	return &savedSearchRepository{
		db: db,
	}
}

// Create inserts a new saved search into the database
func (r *savedSearchRepository) Create(ctx context.Context, search *domain.SavedSearch) error {
	if err := r.db.WithContext(ctx).Create(search).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByUserID retrieves all saved searches for a user
func (r *savedSearchRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SavedSearch, error) {
	var searches []domain.SavedSearch
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&searches).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return searches, nil
}

// FindByID retrieves a saved search by its UUID
func (r *savedSearchRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.SavedSearch, error) {
	var search domain.SavedSearch
	if err := r.db.WithContext(ctx).First(&search, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &search, nil
}

// Update updates an existing saved search in the database
func (r *savedSearchRepository) Update(ctx context.Context, search *domain.SavedSearch) error {
	if err := r.db.WithContext(ctx).Save(search).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// SoftDelete performs a soft delete on a saved search by its ID
func (r *savedSearchRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.SavedSearch{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}
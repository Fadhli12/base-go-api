package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// NewsService handles news-related business logic
type NewsService struct {
	repo     repository.NewsRepository
	enforcer *permission.Enforcer
}

// NewNewsService creates a new NewsService instance
func NewNewsService(repo repository.NewsRepository, enforcer *permission.Enforcer) *NewsService {
	return &NewsService{
		repo:     repo,
		enforcer: enforcer,
	}
}

// Create creates a new news article
func (s *NewsService) Create(ctx context.Context, authorID uuid.UUID, title, content, excerpt string, tags, metadata datatypes.JSON) (*domain.News, error) {
	// Validate required fields
	if strings.TrimSpace(title) == "" {
		return nil, errors.NewAppError("VALIDATION_ERROR", "Title is required", 422)
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.NewAppError("VALIDATION_ERROR", "Content is required", 422)
	}

	// Generate slug from title
	slug := generateSlug(title)

	news := &domain.News{
		AuthorID: authorID,
		Title:    title,
		Slug:     slug,
		Content:  content,
		Excerpt:  excerpt,
		Status:   domain.NewsStatusDraft,
		Tags:     tags,
		Metadata: metadata,
	}

	if err := s.repo.Create(ctx, news); err != nil {
		return nil, err
	}

	return news, nil
}

// GetByID retrieves a news article by ID with permission check
func (s *NewsService) GetByID(ctx context.Context, userID, id uuid.UUID, isAdmin bool) (*domain.News, error) {
	news, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ownership check: if not admin, can only access own news
	if !isAdmin && !news.IsOwnedBy(userID) {
		return nil, errors.ErrNotFound
	}

	return news, nil
}

// GetBySlug retrieves a news article by slug (public access for published)
func (s *NewsService) GetBySlug(ctx context.Context, slug string) (*domain.News, error) {
	news, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Only allow access to published news via slug
	if news.Status != domain.NewsStatusPublished {
		return nil, errors.ErrNotFound
	}

	return news, nil
}

// GetBySlugWithAuth retrieves a news article by slug with auth check
func (s *NewsService) GetBySlugWithAuth(ctx context.Context, userID uuid.UUID, slug string, isAdmin bool) (*domain.News, error) {
	news, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Ownership check for non-published news
	if news.Status != domain.NewsStatusPublished {
		if !isAdmin && !news.IsOwnedBy(userID) {
			return nil, errors.ErrNotFound
		}
	}

	return news, nil
}

// ListByAuthor retrieves all news articles for a specific author
func (s *NewsService) ListByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]domain.News, int64, error) {
	news, err := s.repo.FindByAuthorID(ctx, authorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.CountByAuthorID(ctx, authorID)
	if err != nil {
		return nil, 0, err
	}

	return news, count, nil
}

// ListByStatus retrieves all news articles with a specific status
func (s *NewsService) ListByStatus(ctx context.Context, status domain.NewsStatus, limit, offset int) ([]domain.News, int64, error) {
	if !domain.IsValidNewsStatus(status) {
		return nil, 0, errors.NewAppError("VALIDATION_ERROR", "Invalid status", 422)
	}

	news, err := s.repo.FindByStatus(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.CountByStatus(ctx, status)
	if err != nil {
		return nil, 0, err
	}

	return news, count, nil
}

// ListAll retrieves all news articles with pagination (admin only)
func (s *NewsService) ListAll(ctx context.Context, limit, offset int) ([]domain.News, int64, error) {
	news, err := s.repo.FindAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.CountAll(ctx)
	if err != nil {
		return nil, 0, err
	}

	return news, count, nil
}

// Update updates a news article with ownership check
func (s *NewsService) Update(ctx context.Context, userID, id uuid.UUID, title, content, excerpt string, tags, metadata datatypes.JSON, status domain.NewsStatus, isAdmin bool) (*domain.News, error) {
	// Fetch news first
	news, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ownership check
	if !isAdmin && !news.IsOwnedBy(userID) {
		return nil, errors.ErrNotFound
	}

	// Validate status transition if status is being changed
	if status != "" && status != news.Status {
		if !domain.IsValidNewsStatus(status) {
			return nil, errors.NewAppError("VALIDATION_ERROR", "Invalid status", 422)
		}
		if !news.CanTransitionTo(status) {
			return nil, errors.NewAppError("VALIDATION_ERROR",
				fmt.Sprintf("Cannot transition from %s to %s", news.Status, status), 422)
		}
		news.Status = status

		// Set published_at when transitioning to published
		if status == domain.NewsStatusPublished && news.PublishedAt == nil {
			news.SetPublishedAt()
		}
	}

	// Update fields
	if strings.TrimSpace(title) != "" {
		news.Title = title
		// Update slug when title changes
		news.Slug = generateSlug(title)
	}
	if strings.TrimSpace(content) != "" {
		news.Content = content
	}
	if excerpt != "" {
		news.Excerpt = excerpt
	}
	if tags != nil {
		news.Tags = tags
	}
	if metadata != nil {
		news.Metadata = metadata
	}

	if err := s.repo.Update(ctx, news); err != nil {
		return nil, err
	}

	return news, nil
}

// Delete soft-deletes a news article with ownership check
func (s *NewsService) Delete(ctx context.Context, userID, id uuid.UUID, isAdmin bool) error {
	// Fetch news first to check ownership
	news, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Ownership check
	if !isAdmin && !news.IsOwnedBy(userID) {
		return errors.ErrNotFound
	}

	return s.repo.SoftDelete(ctx, id)
}

// UpdateStatus updates the status of a news article
func (s *NewsService) UpdateStatus(ctx context.Context, userID, id uuid.UUID, status domain.NewsStatus, isAdmin bool) error {
	if !domain.IsValidNewsStatus(status) {
		return errors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("Invalid status: %s", status), 422)
	}

	news, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Ownership check
	if !isAdmin && !news.IsOwnedBy(userID) {
		return errors.ErrNotFound
	}

	// Validate status transition
	if !news.CanTransitionTo(status) {
		return errors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("Cannot transition from %s to %s", news.Status, status), 422)
	}

	news.Status = status

	// Set published_at when transitioning to published
	if status == domain.NewsStatusPublished && news.PublishedAt == nil {
		news.SetPublishedAt()
	}

	return s.repo.Update(ctx, news)
}

// CheckPermission checks if a user has permission for an action on news
func (s *NewsService) CheckPermission(ctx context.Context, userID uuid.UUID, action string) (bool, error) {
	if s.enforcer == nil {
		// If no enforcer, default to allow (for backward compatibility)
		return true, nil
	}

	return s.enforcer.Enforce(userID.String(), "default", "news", action)
}

// generateSlug creates a URL-friendly slug from a title
func generateSlug(title string) string {
	// Simple slug generation - lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove special characters
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

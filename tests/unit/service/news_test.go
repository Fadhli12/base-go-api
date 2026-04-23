package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockNewsRepository is a mock implementation of repository.NewsRepository
type MockNewsRepository struct {
	mock.Mock
}

var _ repository.NewsRepository = (*MockNewsRepository)(nil)

func (m *MockNewsRepository) Create(ctx context.Context, news *domain.News) error {
	args := m.Called(ctx, news)
	return args.Error(0)
}

func (m *MockNewsRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.News, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.News), args.Error(1)
}

func (m *MockNewsRepository) FindBySlug(ctx context.Context, slug string) (*domain.News, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.News), args.Error(1)
}

func (m *MockNewsRepository) FindByStatus(ctx context.Context, status domain.NewsStatus, limit, offset int) ([]domain.News, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.News), args.Error(1)
}

func (m *MockNewsRepository) FindByAuthorID(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]domain.News, error) {
	args := m.Called(ctx, authorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.News), args.Error(1)
}

func (m *MockNewsRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.News, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.News), args.Error(1)
}

func (m *MockNewsRepository) Update(ctx context.Context, news *domain.News) error {
	args := m.Called(ctx, news)
	return args.Error(0)
}

func (m *MockNewsRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNewsRepository) CountByStatus(ctx context.Context, status domain.NewsStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNewsRepository) CountByAuthorID(ctx context.Context, authorID uuid.UUID) (int64, error) {
	args := m.Called(ctx, authorID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNewsRepository) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func TestNewsService_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	authorID := uuid.New()
	title := "Test News Title"
	content := "This is the test news content."
	excerpt := "Test excerpt"

	// Test successful creation
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.News")).Return(nil).Once()

	news, err := newsSvc.Create(ctx, authorID, title, content, excerpt, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, title, news.Title)
	assert.Equal(t, content, news.Content)
	assert.Equal(t, excerpt, news.Excerpt)
	assert.Equal(t, domain.NewsStatusDraft, news.Status)
	assert.Equal(t, authorID, news.AuthorID)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_Create_Validation(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	authorID := uuid.New()

	tests := []struct {
		name    string
		title   string
		content string
		wantErr bool
		errCode string
	}{
		{
			name:    "empty title",
			title:   "",
			content: "Content",
			wantErr: true,
			errCode: "VALIDATION_ERROR",
		},
		{
			name:    "whitespace only title",
			title:   "   ",
			content: "Content",
			wantErr: true,
			errCode: "VALIDATION_ERROR",
		},
		{
			name:    "empty content",
			title:   "Title",
			content: "",
			wantErr: true,
			errCode: "VALIDATION_ERROR",
		},
		{
			name:    "whitespace only content",
			title:   "Title",
			content: "   ",
			wantErr: true,
			errCode: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newsSvc.Create(ctx, authorID, tt.title, tt.content, "", nil, nil)
			if tt.wantErr {
				require.Error(t, err)
				appErr := errors.GetAppError(err)
				require.NotNil(t, appErr)
				assert.Equal(t, tt.errCode, appErr.Code)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewsService_GetByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	expectedNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Test News",
		Content:  "Test Content",
		Status:   domain.NewsStatusDraft,
	}

	// Test successful retrieval (owner)
	mockRepo.On("FindByID", ctx, newsID).Return(expectedNews, nil).Once()

	news, err := newsSvc.GetByID(ctx, userID, newsID, false)
	require.NoError(t, err)
	assert.Equal(t, expectedNews.ID, news.ID)
	assert.Equal(t, expectedNews.Title, news.Title)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_GetByID_NotOwner(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	authorID := uuid.New()
	newsID := uuid.New()

	expectedNews := &domain.News{
		ID:       newsID,
		AuthorID: authorID, // Different from userID
		Title:    "Test News",
		Content:  "Test Content",
	}

	// Test retrieval by non-owner (should fail)
	mockRepo.On("FindByID", ctx, newsID).Return(expectedNews, nil).Once()

	news, err := newsSvc.GetByID(ctx, userID, newsID, false)
	require.Error(t, err)
	assert.Nil(t, news)
	assert.Equal(t, errors.ErrNotFound, err)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_GetByID_AdminAccess(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	adminID := uuid.New()
	authorID := uuid.New()
	newsID := uuid.New()

	expectedNews := &domain.News{
		ID:       newsID,
		AuthorID: authorID, // Different from adminID
		Title:    "Test News",
		Content:  "Test Content",
	}

	// Test retrieval by admin (should succeed even if not owner)
	mockRepo.On("FindByID", ctx, newsID).Return(expectedNews, nil).Once()

	news, err := newsSvc.GetByID(ctx, adminID, newsID, true)
	require.NoError(t, err)
	assert.Equal(t, expectedNews.ID, news.ID)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_GetBySlug(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	slug := "test-news-article"
	expectedNews := &domain.News{
		ID:      uuid.New(),
		Title:   "Test News Article",
		Slug:    slug,
		Content: "Content",
		Status:  domain.NewsStatusPublished,
	}

	mockRepo.On("FindBySlug", ctx, slug).Return(expectedNews, nil).Once()

	news, err := newsSvc.GetBySlug(ctx, slug)
	require.NoError(t, err)
	assert.Equal(t, expectedNews.Slug, news.Slug)
	assert.Equal(t, expectedNews.Status, news.Status)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_GetBySlug_NotPublished(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	slug := "draft-news-article"
	expectedNews := &domain.News{
		ID:      uuid.New(),
		Title:   "Draft News",
		Slug:    slug,
		Content: "Content",
		Status:  domain.NewsStatusDraft,
	}

	mockRepo.On("FindBySlug", ctx, slug).Return(expectedNews, nil).Once()

	// Public access to draft article should fail
	news, err := newsSvc.GetBySlug(ctx, slug)
	require.Error(t, err)
	assert.Nil(t, news)
	assert.Equal(t, errors.ErrNotFound, err)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_ListByAuthor(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	authorID := uuid.New()
	expectedNews := []domain.News{
		{ID: uuid.New(), Title: "News 1", AuthorID: authorID},
		{ID: uuid.New(), Title: "News 2", AuthorID: authorID},
	}

	mockRepo.On("FindByAuthorID", ctx, authorID, 10, 0).Return(expectedNews, nil).Once()
	mockRepo.On("CountByAuthorID", ctx, authorID).Return(int64(2), nil).Once()

	news, total, err := newsSvc.ListByAuthor(ctx, authorID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, news, 2)
	assert.Equal(t, int64(2), total)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_ListAll(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	expectedNews := []domain.News{
		{ID: uuid.New(), Title: "News 1"},
		{ID: uuid.New(), Title: "News 2"},
		{ID: uuid.New(), Title: "News 3"},
	}

	mockRepo.On("FindAll", ctx, 10, 0).Return(expectedNews, nil).Once()
	mockRepo.On("CountAll", ctx).Return(int64(3), nil).Once()

	news, total, err := newsSvc.ListAll(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, news, 3)
	assert.Equal(t, int64(3), total)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_Update(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	existingNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Old Title",
		Content:  "Old Content",
		Status:   domain.NewsStatusDraft,
	}

	mockRepo.On("FindByID", ctx, newsID).Return(existingNews, nil).Once()
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.News")).Return(nil).Once()

	news, err := newsSvc.Update(ctx, userID, newsID, "New Title", "New Content", "", nil, nil, "", false)
	require.NoError(t, err)
	assert.Equal(t, "New Title", news.Title)
	assert.Equal(t, "New Content", news.Content)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_Update_StatusTransition(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	existingNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Test Title",
		Content:  "Test Content",
		Status:   domain.NewsStatusDraft,
	}

	mockRepo.On("FindByID", ctx, newsID).Return(existingNews, nil).Once()
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.News")).Return(nil).Once()

	// Transition from draft to published
	news, err := newsSvc.Update(ctx, userID, newsID, "", "", "", nil, nil, domain.NewsStatusPublished, false)
	require.NoError(t, err)
	assert.Equal(t, domain.NewsStatusPublished, news.Status)
	assert.NotNil(t, news.PublishedAt)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_Update_InvalidStatusTransition(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	existingNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Test Title",
		Content:  "Test Content",
		Status:   domain.NewsStatusArchived,
	}

	mockRepo.On("FindByID", ctx, newsID).Return(existingNews, nil).Once()

	// Try to transition from archived to published (invalid)
	_, err := newsSvc.Update(ctx, userID, newsID, "", "", "", nil, nil, domain.NewsStatusPublished, false)
	require.Error(t, err)
	appErr := errors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_Delete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	existingNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Test Title",
		Status:   domain.NewsStatusDraft,
	}

	mockRepo.On("FindByID", ctx, newsID).Return(existingNews, nil).Once()
	mockRepo.On("SoftDelete", ctx, newsID).Return(nil).Once()

	err := newsSvc.Delete(ctx, userID, newsID, false)
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	existingNews := &domain.News{
		ID:       newsID,
		AuthorID: userID,
		Title:    "Test Title",
		Status:   domain.NewsStatusDraft,
	}

	mockRepo.On("FindByID", ctx, newsID).Return(existingNews, nil).Once()
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.News")).Return(nil).Once()

	err := newsSvc.UpdateStatus(ctx, userID, newsID, domain.NewsStatusPublished, false)
	require.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestNewsService_UpdateStatus_InvalidStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)
	newsSvc := service.NewNewsService(mockRepo, nil)

	userID := uuid.New()
	newsID := uuid.New()

	err := newsSvc.UpdateStatus(ctx, userID, newsID, "invalid_status", false)
	require.Error(t, err)
	appErr := errors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestDomain_NewsStatus_Valid(t *testing.T) {
	tests := []struct {
		status domain.NewsStatus
		valid  bool
	}{
		{domain.NewsStatusDraft, true},
		{domain.NewsStatusPublished, true},
		{domain.NewsStatusArchived, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.valid, domain.IsValidNewsStatus(tt.status))
		})
	}
}

func TestDomain_News_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from     domain.NewsStatus
		to       domain.NewsStatus
		expected bool
	}{
		// From draft
		{domain.NewsStatusDraft, domain.NewsStatusPublished, true},
		{domain.NewsStatusDraft, domain.NewsStatusArchived, true},
		{domain.NewsStatusDraft, domain.NewsStatusDraft, false},
		// From published
		{domain.NewsStatusPublished, domain.NewsStatusArchived, true},
		{domain.NewsStatusPublished, domain.NewsStatusDraft, false},
		{domain.NewsStatusPublished, domain.NewsStatusPublished, false},
		// From archived
		{domain.NewsStatusArchived, domain.NewsStatusDraft, true},
		{domain.NewsStatusArchived, domain.NewsStatusPublished, false},
		{domain.NewsStatusArchived, domain.NewsStatusArchived, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			news := &domain.News{Status: tt.from}
			assert.Equal(t, tt.expected, news.CanTransitionTo(tt.to))
		})
	}
}

func TestDomain_News_IsOwnedBy(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()

	news := &domain.News{AuthorID: ownerID}

	assert.True(t, news.IsOwnedBy(ownerID))
	assert.False(t, news.IsOwnedBy(otherID))
}

func TestDomain_News_IsPublished(t *testing.T) {
	publishedNews := &domain.News{Status: domain.NewsStatusPublished}
	draftNews := &domain.News{Status: domain.NewsStatusDraft}

	assert.True(t, publishedNews.IsPublished())
	assert.False(t, draftNews.IsPublished())
}

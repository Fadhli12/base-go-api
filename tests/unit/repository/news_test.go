package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNewsRepository is a mock implementation of repository.NewsRepository
type MockNewsRepository struct {
	mock.Mock
}

// Ensure MockNewsRepository implements the interface
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

func TestMockNewsRepository_Create(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	news := &domain.News{
		Title:   "Test Title",
		Content: "Test Content",
		Slug:    "test-title",
	}

	mockRepo.On("Create", ctx, news).Return(nil)

	err := mockRepo.Create(ctx, news)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	id := uuid.New()
	expectedNews := &domain.News{
		ID:      id,
		Title:   "Test Title",
		Content: "Test Content",
		Slug:    "test-title",
	}

	mockRepo.On("FindByID", ctx, id).Return(expectedNews, nil)

	news, err := mockRepo.FindByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, expectedNews.ID, news.ID)
	assert.Equal(t, expectedNews.Title, news.Title)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	id := uuid.New()

	mockRepo.On("FindByID", ctx, id).Return(nil, errors.ErrNotFound)

	news, err := mockRepo.FindByID(ctx, id)
	assert.Error(t, err)
	assert.Nil(t, news)
	assert.Equal(t, errors.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindBySlug(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	slug := "test-news-article"
	expectedNews := &domain.News{
		ID:      uuid.New(),
		Title:   "Test News Article",
		Slug:    slug,
		Content: "This is a test news article.",
	}

	mockRepo.On("FindBySlug", ctx, slug).Return(expectedNews, nil)

	news, err := mockRepo.FindBySlug(ctx, slug)
	assert.NoError(t, err)
	assert.Equal(t, expectedNews.Slug, news.Slug)
	assert.Equal(t, expectedNews.Title, news.Title)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindByStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	status := domain.NewsStatusPublished
	expectedNews := []domain.News{
		{ID: uuid.New(), Title: "News 1", Status: status},
		{ID: uuid.New(), Title: "News 2", Status: status},
	}

	mockRepo.On("FindByStatus", ctx, status, 10, 0).Return(expectedNews, nil)

	news, err := mockRepo.FindByStatus(ctx, status, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, news, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindByAuthorID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	authorID := uuid.New()
	expectedNews := []domain.News{
		{ID: uuid.New(), Title: "Author News 1", AuthorID: authorID},
		{ID: uuid.New(), Title: "Author News 2", AuthorID: authorID},
	}

	mockRepo.On("FindByAuthorID", ctx, authorID, 10, 0).Return(expectedNews, nil)

	news, err := mockRepo.FindByAuthorID(ctx, authorID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, news, 2)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_FindAll(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	expectedNews := []domain.News{
		{ID: uuid.New(), Title: "News 1"},
		{ID: uuid.New(), Title: "News 2"},
		{ID: uuid.New(), Title: "News 3"},
	}

	mockRepo.On("FindAll", ctx, 10, 0).Return(expectedNews, nil)

	news, err := mockRepo.FindAll(ctx, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, news, 3)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_Update(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	news := &domain.News{
		ID:      uuid.New(),
		Title:   "Updated Title",
		Content: "Updated Content",
	}

	mockRepo.On("Update", ctx, news).Return(nil)

	err := mockRepo.Update(ctx, news)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_SoftDelete(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	id := uuid.New()

	mockRepo.On("SoftDelete", ctx, id).Return(nil)

	err := mockRepo.SoftDelete(ctx, id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_CountByStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	status := domain.NewsStatusPublished
	expectedCount := int64(5)

	mockRepo.On("CountByStatus", ctx, status).Return(expectedCount, nil)

	count, err := mockRepo.CountByStatus(ctx, status)
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, count)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_CountByAuthorID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	authorID := uuid.New()
	expectedCount := int64(3)

	mockRepo.On("CountByAuthorID", ctx, authorID).Return(expectedCount, nil)

	count, err := mockRepo.CountByAuthorID(ctx, authorID)
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, count)
	mockRepo.AssertExpectations(t)
}

func TestMockNewsRepository_CountAll(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockNewsRepository)

	expectedCount := int64(10)

	mockRepo.On("CountAll", ctx).Return(expectedCount, nil)

	count, err := mockRepo.CountAll(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, count)
	mockRepo.AssertExpectations(t)
}

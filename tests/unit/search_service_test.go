package unit

import (
	"context"
	"log/slog"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

type MockSavedSearchRepository struct {
	mock.Mock
}

func (m *MockSavedSearchRepository) Create(ctx context.Context, search *domain.SavedSearch) error {
	args := m.Called(ctx, search)
	return args.Error(0)
}

func (m *MockSavedSearchRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SavedSearch, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.SavedSearch), args.Error(1)
}

func (m *MockSavedSearchRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.SavedSearch, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SavedSearch), args.Error(1)
}

func (m *MockSavedSearchRepository) Update(ctx context.Context, search *domain.SavedSearch) error {
	args := m.Called(ctx, search)
	return args.Error(0)
}

func (m *MockSavedSearchRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestSearchService_CreateSavedSearch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("empty name returns validation error", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		ss, err := svc.CreateSavedSearch(ctx, userID, "", "golang", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
		assert.Nil(t, ss)
	})

	t.Run("name too long returns validation error", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		longName := ""
		for i := 0; i < 256; i++ {
			longName += "a"
		}
		ss, err := svc.CreateSavedSearch(ctx, userID, longName, "golang", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
		assert.Nil(t, ss)
	})

	t.Run("empty query_text returns validation error", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		ss, err := svc.CreateSavedSearch(ctx, userID, "my search", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query_text is required")
		assert.Nil(t, ss)
	})

	t.Run("max saved searches exceeded returns error", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearches := make([]domain.SavedSearch, 50)
		for i := range existingSearches {
			existingSearches[i] = domain.SavedSearch{ID: uuid.New()}
		}
		mockRepo.On("FindByUserID", ctx, userID).Return(existingSearches, nil)

		ss, err := svc.CreateSavedSearch(ctx, userID, "my search", "golang", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum of 50")
		assert.Nil(t, ss)
	})

	t.Run("successful creation", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		mockRepo.On("FindByUserID", ctx, userID).Return([]domain.SavedSearch{}, nil)
		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.CreateSavedSearch(ctx, userID, "my search", "golang", map[string]interface{}{"status": "published"})
		require.NoError(t, err)
		assert.NotNil(t, ss)
		assert.Equal(t, "my search", ss.Name)
		assert.Equal(t, "golang", ss.QueryText)
		mockRepo.AssertExpectations(t)
	})
}

func TestSearchService_ListSavedSearches(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("returns saved searches for user", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		expectedSearches := []domain.SavedSearch{
			{ID: uuid.New(), Name: "search 1", QueryText: "golang"},
			{ID: uuid.New(), Name: "search 2", QueryText: "python"},
		}
		mockRepo.On("FindByUserID", ctx, userID).Return(expectedSearches, nil)

		searches, err := svc.ListSavedSearches(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, searches, 2)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns error on repo failure", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		mockRepo.On("FindByUserID", ctx, userID).Return(nil, assert.AnError)

		searches, err := svc.ListSavedSearches(ctx, userID)
		require.Error(t, err)
		assert.Nil(t, searches)
	})
}

func TestSearchService_GetSavedSearch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	searchID := uuid.New()

	t.Run("returns saved search when owner", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		expectedSearch := &domain.SavedSearch{
			ID:     searchID,
			UserID: userID,
			Name:   "my search",
		}
		mockRepo.On("FindByID", ctx, searchID).Return(expectedSearch, nil)

		ss, err := svc.GetSavedSearch(ctx, userID, searchID)
		require.NoError(t, err)
		assert.Equal(t, searchID, ss.ID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns not found when not owner", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		otherUserID := uuid.New()
		expectedSearch := &domain.SavedSearch{
			ID:     searchID,
			UserID: otherUserID,
			Name:   "my search",
		}
		mockRepo.On("FindByID", ctx, searchID).Return(expectedSearch, nil)

		ss, err := svc.GetSavedSearch(ctx, userID, searchID)
		require.Error(t, err)
		assert.Nil(t, ss)
	})

	t.Run("returns not found when search does not exist", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		mockRepo.On("FindByID", ctx, searchID).Return(nil, assert.AnError)

		ss, err := svc.GetSavedSearch(ctx, userID, searchID)
		require.Error(t, err)
		assert.Nil(t, ss)
	})
}

func TestSearchService_UpdateSavedSearch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	searchID := uuid.New()

	t.Run("updates name when provided", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:        searchID,
			UserID:    userID,
			Name:      "old name",
			QueryText: "golang",
			Filters:   datatypes.JSON("{}"),
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.UpdateSavedSearch(ctx, userID, searchID, "new name", "", nil)
		require.NoError(t, err)
		assert.Equal(t, "new name", ss.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("updates query_text when provided", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:        searchID,
			UserID:    userID,
			Name:      "my search",
			QueryText: "old query",
			Filters:   datatypes.JSON("{}"),
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.UpdateSavedSearch(ctx, userID, searchID, "", "new query", nil)
		require.NoError(t, err)
		assert.Equal(t, "new query", ss.QueryText)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns not found when not owner", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		otherUserID := uuid.New()
		existingSearch := &domain.SavedSearch{
			ID:     searchID,
			UserID: otherUserID,
			Name:   "my search",
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)

		ss, err := svc.UpdateSavedSearch(ctx, userID, searchID, "new name", "", nil)
		require.Error(t, err)
		assert.Nil(t, ss)
	})

	t.Run("returns validation error for name too long", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:        searchID,
			UserID:    userID,
			Name:      "my search",
			QueryText: "golang",
			Filters:   datatypes.JSON("{}"),
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)

		longName := ""
		for i := 0; i < 256; i++ {
			longName += "a"
		}
		ss, err := svc.UpdateSavedSearch(ctx, userID, searchID, longName, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name must be under")
		assert.Nil(t, ss)
	})
}

func TestSearchService_DeleteSavedSearch(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	searchID := uuid.New()

	t.Run("deletes saved search when owner", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:     searchID,
			UserID: userID,
			Name:   "my search",
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)
		mockRepo.On("SoftDelete", ctx, searchID).Return(nil)

		err := svc.DeleteSavedSearch(ctx, userID, searchID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("returns not found when not owner", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		otherUserID := uuid.New()
		existingSearch := &domain.SavedSearch{
			ID:     searchID,
			UserID: otherUserID,
			Name:   "my search",
		}
		mockRepo.On("FindByID", ctx, searchID).Return(existingSearch, nil)

		err := svc.DeleteSavedSearch(ctx, userID, searchID)
		require.Error(t, err)
	})

	t.Run("returns error when search not found", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		mockRepo.On("FindByID", ctx, searchID).Return(nil, assert.AnError)

		err := svc.DeleteSavedSearch(ctx, userID, searchID)
		require.Error(t, err)
	})
}

func TestSearchService_CreateSavedSearch_TrimsWhitespace(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("name is trimmed", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		mockRepo.On("FindByUserID", ctx, userID).Return([]domain.SavedSearch{}, nil)
		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.CreateSavedSearch(ctx, userID, "  trimmed name  ", "golang", nil)
		require.NoError(t, err)
		assert.Equal(t, "trimmed name", ss.Name)
	})
}

func TestSearchService_UpdateSavedSearch_SkipsEmptyValues(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("skips empty name update", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "original",
			QueryText: "golang",
			Filters:   datatypes.JSON("{}"),
		}
		mockRepo.On("FindByID", ctx, existingSearch.ID).Return(existingSearch, nil)
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.UpdateSavedSearch(ctx, userID, existingSearch.ID, "  ", "", nil)
		require.NoError(t, err)
		assert.Equal(t, "original", ss.Name)
	})

	t.Run("skips empty query_text update", func(t *testing.T) {
		mockRepo := new(MockSavedSearchRepository)
		svc := service.NewSearchService(nil, mockRepo, slog.Default())

		existingSearch := &domain.SavedSearch{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "my search",
			QueryText: "original",
			Filters:   datatypes.JSON("{}"),
		}
		mockRepo.On("FindByID", ctx, existingSearch.ID).Return(existingSearch, nil)
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.SavedSearch")).Return(nil)

		ss, err := svc.UpdateSavedSearch(ctx, userID, existingSearch.ID, "", "  ", nil)
		require.NoError(t, err)
		assert.Equal(t, "original", ss.QueryText)
	})
}

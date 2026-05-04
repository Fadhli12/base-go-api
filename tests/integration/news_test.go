//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestNews_CRUD_Full tests complete CRUD operations for news
func TestNews_CRUD_Full(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	// Initialize components
	newsRepo := repository.NewNewsRepository(suite.DB)
	newsSvc := service.NewNewsService(newsRepo, nil)
	newsHandler := handler.NewNewsHandler(newsSvc, nil)

	// Create a test user
	authorID := uuid.New()
	authorEmail := "author@example.com"
	createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

	// Create Echo instance and register routes
	e := echo.New()
	api := e.Group("/api/v1")
	registerNewsRoutes(api, newsHandler, "test-secret")

	ctx := context.Background()

	t.Run("Create News Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		reqBody := map[string]interface{}{
			"title":   "Test News Article",
			"content": "This is a test news article content.",
			"excerpt": "Test excerpt",
			"tags":    []string{"test", "news"},
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/news", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		require.NotNil(t, response["data"])

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "Test News Article", data["title"])
		assert.Equal(t, "draft", data["status"])
		assert.Equal(t, authorID.String(), data["author_id"])
	})

	t.Run("Get News by ID", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create a news article first
		news := createTestNews(t, suite.DB, authorID, "Test Article", domain.NewsStatusDraft)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/news/"+news.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		require.NotNil(t, response["data"])

		data := response["data"].(map[string]interface{})
		assert.Equal(t, news.ID.String(), data["id"])
		assert.Equal(t, "Test Article", data["title"])
	})

	t.Run("Get News by Slug", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create a published news article
		news := createTestNews(t, suite.DB, authorID, "Published Article", domain.NewsStatusPublished)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/news/slug/"+news.Slug, nil)
		// No auth required for published articles

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		require.NotNil(t, response["data"])

		data := response["data"].(map[string]interface{})
		assert.Equal(t, news.Slug, data["slug"])
		assert.Equal(t, "published", data["status"])
	})

	t.Run("List News Articles", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create multiple news articles
		createTestNews(t, suite.DB, authorID, "Article 1", domain.NewsStatusDraft)
		createTestNews(t, suite.DB, authorID, "Article 2", domain.NewsStatusDraft)
		createTestNews(t, suite.DB, authorID, "Article 3", domain.NewsStatusPublished)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/news?limit=10&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		require.NotNil(t, response["data"])

		data := response["data"].(map[string]interface{})
		articles := data["data"].([]interface{})
		assert.GreaterOrEqual(t, len(articles), 3)
	})

	t.Run("Update News Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create a news article first
		news := createTestNews(t, suite.DB, authorID, "Original Title", domain.NewsStatusDraft)

		reqBody := map[string]interface{}{
			"title":   "Updated Title",
			"content": "Updated content",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/news/"+news.ID.String(), bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "Updated Title", data["title"])
	})

	t.Run("Update News Status", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create a draft news article
		news := createTestNews(t, suite.DB, authorID, "Draft Article", domain.NewsStatusDraft)

		reqBody := map[string]interface{}{
			"status": "published",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/news/"+news.ID.String()+"/status", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "published", data["status"])
		assert.NotNil(t, data["published_at"])
	})

	t.Run("Delete News Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create a news article first
		news := createTestNews(t, suite.DB, authorID, "Article to Delete", domain.NewsStatusDraft)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/news/"+news.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify the article is soft deleted
		var count int64
		suite.DB.WithContext(ctx).Unscoped().Model(&domain.News{}).Where("id = ?", news.ID).Count(&count)
		assert.Equal(t, int64(1), count)

		// Verify it's not accessible via normal query
		_, err := newsRepo.FindByID(ctx, news.ID)
		assert.Error(t, err)
	})
}

// TestNews_PermissionEnforcement tests permission and ownership enforcement
func TestNews_PermissionEnforcement(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	// Create enforcer (without actual Casbin for simplicity)
	newsRepo := repository.NewNewsRepository(suite.DB)
	newsSvc := service.NewNewsService(newsRepo, nil)
	newsHandler := handler.NewNewsHandler(newsSvc, nil)

	// Create test users
	authorID := uuid.New()
	authorEmail := "author@example.com"
	otherUserID := uuid.New()
	otherUserEmail := "other@example.com"
	createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)
	createTestUserWithMockHash(t, suite.DB, otherUserID, otherUserEmail)

	// Create Echo instance
	e := echo.New()
	api := e.Group("/api/v1")
	registerNewsRoutes(api, newsHandler, "test-secret")

	t.Run("Non-owner Cannot Access Other's Draft", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)
		createTestUserWithMockHash(t, suite.DB, otherUserID, otherUserEmail)

		// Create a draft article by author
		news := createTestNews(t, suite.DB, authorID, "Private Draft", domain.NewsStatusDraft)

		// Try to access by other user
		req := httptest.NewRequest(http.MethodGet, "/api/v1/news/"+news.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+generateTestToken(otherUserID.String(), otherUserEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Non-owner Can Access Published Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)
		createTestUserWithMockHash(t, suite.DB, otherUserID, otherUserEmail)

		// Create a published article by author
		news := createTestNews(t, suite.DB, authorID, "Public Article", domain.NewsStatusPublished)

		// Access by other user should work for published
		req := httptest.NewRequest(http.MethodGet, "/api/v1/news/slug/"+news.Slug, nil)
		// No auth required for published via slug

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Non-owner Cannot Update Other's Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)
		createTestUserWithMockHash(t, suite.DB, otherUserID, otherUserEmail)

		// Create an article by author
		news := createTestNews(t, suite.DB, authorID, "Author Article", domain.NewsStatusDraft)

		// Try to update by other user
		reqBody := map[string]interface{}{"title": "Hacked Title"}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/news/"+news.ID.String(), bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(otherUserID.String(), otherUserEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Non-owner Cannot Delete Other's Article", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)
		createTestUserWithMockHash(t, suite.DB, otherUserID, otherUserEmail)

		// Create an article by author
		news := createTestNews(t, suite.DB, authorID, "Author Article", domain.NewsStatusDraft)

		// Try to delete by other user
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/news/"+news.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+generateTestToken(otherUserID.String(), otherUserEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestNews_StatusTransitions tests status transition rules
func TestNews_StatusTransitions(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	newsRepo := repository.NewNewsRepository(suite.DB)
	newsSvc := service.NewNewsService(newsRepo, nil)
	newsHandler := handler.NewNewsHandler(newsSvc, nil)

	authorID := uuid.New()
	authorEmail := "author@example.com"
	createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

	e := echo.New()
	api := e.Group("/api/v1")
	registerNewsRoutes(api, newsHandler, "test-secret")

	t.Run("Draft to Published", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create draft
		news := createTestNews(t, suite.DB, authorID, "Draft", domain.NewsStatusDraft)

		// Transition to published
		reqBody := map[string]interface{}{"status": "published"}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/news/"+news.ID.String()+"/status", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Draft to Archived", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create draft
		news := createTestNews(t, suite.DB, authorID, "Draft to Archive", domain.NewsStatusDraft)

		// Transition to archived
		reqBody := map[string]interface{}{"status": "archived"}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/news/"+news.ID.String()+"/status", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Archived to Draft", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create archived
		news := createTestNews(t, suite.DB, authorID, "Archived", domain.NewsStatusArchived)

		// Transition to draft
		reqBody := map[string]interface{}{"status": "draft"}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/news/"+news.ID.String()+"/status", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Archived to Published - Invalid", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		// Create archived
		news := createTestNews(t, suite.DB, authorID, "Archived Invalid", domain.NewsStatusArchived)

		// Try to transition to published (invalid)
		reqBody := map[string]interface{}{"status": "published"}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/news/"+news.ID.String()+"/status", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

// TestNews_Validation tests input validation
func TestNews_Validation(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	newsRepo := repository.NewNewsRepository(suite.DB)
	newsSvc := service.NewNewsService(newsRepo, nil)
	newsHandler := handler.NewNewsHandler(newsSvc, nil)

	authorID := uuid.New()
	authorEmail := "author@example.com"
	createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

	e := echo.New()
	api := e.Group("/api/v1")
	registerNewsRoutes(api, newsHandler, "test-secret")

	t.Run("Create with Empty Title", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		reqBody := map[string]interface{}{
			"title":   "",
			"content": "Content",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/news", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Create with Empty Content", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		reqBody := map[string]interface{}{
			"title":   "Title",
			"content": "",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/news", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Update with Invalid Status", func(t *testing.T) {
		suite.SetupTest(t)
		createTestUserWithMockHash(t, suite.DB, authorID, authorEmail)

		news := createTestNews(t, suite.DB, authorID, "Test", domain.NewsStatusDraft)

		reqBody := map[string]interface{}{
			"status": "invalid_status",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/news/"+news.ID.String(), bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+generateTestToken(authorID.String(), authorEmail, "test-secret"))

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should reject update with invalid status
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// Helper functions

func createTestUserWithMockHash(t *testing.T, db *gorm.DB, id uuid.UUID, email string) {
	hash := "$2a$12$abcdefghijklmnopqrstuvwxyc12345678901234567890" // Mock bcrypt hash
	user := &domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: hash,
	}
	err := db.Create(user).Error
	require.NoError(t, err)
}

func createTestNews(t *testing.T, db *gorm.DB, authorID uuid.UUID, title string, status domain.NewsStatus) *domain.News {
	slug := "test-slug-" + uuid.New().String()[:8]
	news := &domain.News{
		ID:       uuid.New(),
		AuthorID: authorID,
		Title:    title,
		Slug:     slug,
		Content:  "Test content for " + title,
		Status:   status,
	}
	if status == domain.NewsStatusPublished {
		now := time.Now()
		news.PublishedAt = &now
	}
	err := db.Create(news).Error
	require.NoError(t, err)
	return news
}

func generateTestToken(userID, email, secret string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

func registerNewsRoutes(api *echo.Group, newsHandler *handler.NewsHandler, jwtSecret string) {
	news := api.Group("/news")

	// Public routes
	news.GET("/slug/:slug", newsHandler.GetBySlug)

	// Protected routes
	protected := news.Group("")
	protected.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))

	protected.POST("", newsHandler.Create)
	protected.GET("", newsHandler.List)
	protected.GET("/:id", newsHandler.GetByID)
	protected.PUT("/:id", newsHandler.Update)
	protected.DELETE("/:id", newsHandler.Delete)
	protected.PATCH("/:id/status", newsHandler.UpdateStatus)
}

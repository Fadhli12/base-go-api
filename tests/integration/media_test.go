//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/internal/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const mediaJWTSeret = "test-jwt-secret"

// TestMediaUpload tests the complete media upload flow
func TestMediaUpload(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, err := os.MkdirTemp("", "media-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageDriver, err := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})
	require.NoError(t, err)

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	t.Run("successful upload", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		err := writer.WriteField("collection", "test-collection")
		require.NoError(t, err)

		part, err := writer.CreateFormFile("file", "test-image.jpg")
		require.NoError(t, err)
		_, err = part.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01})
		require.NoError(t, err)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/models/news/"+uuid.New().String()+"/media", &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "id")
	})

	t.Run("unauthorized upload", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01})
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/models/news/"+uuid.New().String()+"/media", &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid model ID", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01})
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/models/news/invalid-uuid/media", &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestMediaDownload tests media download functionality
func TestMediaDownload(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, err := os.MkdirTemp("", "media-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageDriver, err := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})
	require.NoError(t, err)

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01}

	t.Run("download existing media", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)

		media := &domain.Media{
			ModelType:        "news",
			ModelID:          uuid.New(),
			CollectionName:   "default",
			Disk:             "local",
			Filename:         "test-file.jpg",
			OriginalFilename: "test-file.jpg",
			MimeType:         "image/jpeg",
			Size:             int64(len(jpegBytes)),
			Path:             "news/" + uuid.New().String() + "/test-file.jpg",
			UploadedByID:     user.ID,
		}
		err := suite.DB.Create(media).Error
		require.NoError(t, err)

		err = storageDriver.Store(nil, media.Path, bytes.NewReader(jpegBytes))
		require.NoError(t, err)

		signer := storage.NewSigner("test-signing-secret")
		signedURL, err := signer.Generate(media.ID.String(), media.Path, time.Hour)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1%s", signedURL.URL), nil)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	})

	t.Run("download non-existent media", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)

		signer := storage.NewSigner("test-signing-secret")
		signedURL, err := signer.Generate(uuid.New().String(), "news/fake/path.jpg", time.Hour)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1%s", signedURL.URL), nil)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestMediaDelete tests media deletion
func TestMediaDelete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, err := os.MkdirTemp("", "media-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageDriver, _ := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	t.Run("delete own media", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		media := &domain.Media{
			ModelType:        "news",
			ModelID:          uuid.New(),
			CollectionName:   "default",
			Disk:             "local",
			Filename:         "test.txt",
			OriginalFilename: "test.txt",
			MimeType:         "text/plain",
			Size:             100,
			Path:             "news/test.txt",
			UploadedByID:     user.ID,
		}
		suite.DB.Create(media)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/media/%s", media.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var deletedMedia domain.Media
		err := suite.DB.Unscoped().First(&deletedMedia, "id = ?", media.ID).Error
		require.NoError(t, err)
		assert.NotNil(t, deletedMedia.DeletedAt)
	})

	t.Run("delete non-existent media", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/media/%s", uuid.New()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestMediaList tests media listing
func TestMediaList(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, _ := os.MkdirTemp("", "media-test-*")
	defer os.RemoveAll(tempDir)

	storageDriver, _ := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	t.Run("list media for model", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		modelID := uuid.New()

		// Create multiple media items
		for i := 0; i < 3; i++ {
			media := &domain.Media{
				ModelType:        "news",
				ModelID:          modelID,
				CollectionName:   "default",
				Disk:             "local",
				Filename:         fmt.Sprintf("test%d.txt", i),
				OriginalFilename: fmt.Sprintf("test%d.txt", i),
				MimeType:         "text/plain",
				Size:             100,
				Path:             fmt.Sprintf("news/test%d.txt", i),
				UploadedByID:     user.ID,
			}
			suite.DB.Create(media)
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/models/news/%s/media", modelID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "data")
	})
}

// TestMediaSignedURL tests signed URL generation
func TestMediaSignedURL(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, _ := os.MkdirTemp("", "media-test-*")
	defer os.RemoveAll(tempDir)

	storageDriver, _ := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	t.Run("generate signed URL", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		user := createTestUserForMedia(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		media := &domain.Media{
			ModelType:        "news",
			ModelID:          uuid.New(),
			CollectionName:   "default",
			Disk:             "local",
			Filename:         "test.txt",
			OriginalFilename: "test.txt",
			MimeType:         "text/plain",
			Size:             100,
			Path:             "news/test.txt",
			UploadedByID:     user.ID,
		}
		suite.DB.Create(media)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/media/%s/url", media.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "url")
		assert.Contains(t, rec.Body.String(), "expires_at")
	})
}

// TestMediaAdminEndpoints tests admin-only endpoints
func TestMediaAdminEndpoints(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)

	e := echo.New()
	enforcer := setupTestEnforcer(t, suite.DB)

	tempDir, _ := os.MkdirTemp("", "media-test-*")
	defer os.RemoveAll(tempDir)

	storageDriver, _ := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})

	mediaRepo := repository.NewMediaRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})

	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)

	t.Run("admin stats", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		adminUser := createTestUserForMedia(t, suite.DB, enforcer)
		addMediaManagePermission(t, suite.DB, enforcer, adminUser.ID)
		token := generateTestTokenForMedia(adminUser.ID)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/media/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "total_files")
	})

	t.Run("admin cleanup", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		adminUser := createTestUserForMedia(t, suite.DB, enforcer)
		addMediaManagePermission(t, suite.DB, enforcer, adminUser.ID)
		token := generateTestTokenForMedia(adminUser.ID)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/media/cleanup?cutoff_hours=24", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Cleanup job queued")
	})
}

// createMediaTables creates the necessary media tables for testing
func createMediaTables(t *testing.T, db *gorm.DB) {
	migration := `
CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type VARCHAR(255) NOT NULL,
    model_id UUID NOT NULL,
    collection_name VARCHAR(255) DEFAULT 'default' NOT NULL,
    disk VARCHAR(50) DEFAULT 'local' NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    path VARCHAR(2000) NOT NULL,
    metadata JSONB DEFAULT '{}',
    custom_properties JSONB DEFAULT '{}',
    uploaded_by_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    orphaned_at TIMESTAMPTZ DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_media_model ON media(model_type, model_id);
CREATE INDEX IF NOT EXISTS idx_media_filename ON media(filename);
CREATE INDEX IF NOT EXISTS idx_media_uploaded_by ON media(uploaded_by_id);
CREATE INDEX IF NOT EXISTS idx_media_deleted_at ON media(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_orphaned_at ON media(orphaned_at) WHERE orphaned_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS media_conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    disk VARCHAR(50) DEFAULT 'local',
    path VARCHAR(2000) NOT NULL,
    size BIGINT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE(media_id, name)
);

CREATE INDEX IF NOT EXISTS idx_media_conversions_media_id ON media_conversions(media_id);

CREATE TABLE IF NOT EXISTS media_downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id),
    downloaded_by_id UUID REFERENCES users(id),
    downloaded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ip_address VARCHAR(45),
    user_agent VARCHAR(2000)
);

CREATE INDEX IF NOT EXISTS idx_media_downloads_media_id ON media_downloads(media_id);
CREATE INDEX IF NOT EXISTS idx_media_downloads_downloaded_at ON media_downloads(downloaded_at);
`
	result := db.Exec(migration)
	require.NoError(t, result.Error, "Failed to create media tables")
}

// createTestUserForMedia creates a test user
func createTestUserForMedia(t *testing.T, db *gorm.DB, enforcer *permission.Enforcer) *domain.User {
	user := &domain.User{
		Email:        fmt.Sprintf("test-%s@example.com", uuid.New().String()),
		PasswordHash: "$2a$12$hashedpassword",
	}
	err := db.Create(user).Error
	require.NoError(t, err)

	// Grant media:upload, media:view, media:download, media:list permissions so media endpoints succeed
	roleName := "media-user-" + user.ID.String()
	err = db.Exec(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, 'Media user', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, roleName).Error
	require.NoError(t, err)

	actions := []string{"upload", "view", "download", "list", "manage"}
	for _, action := range actions {
		permName := "media:" + action
		err = db.Exec(`
			INSERT INTO permissions (id, name, resource, action, scope, created_at, updated_at)
			VALUES (gen_random_uuid(), ?, 'media', ?, 'all', NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, permName, action).Error
		require.NoError(t, err)

		var permIDStr string
		err = db.Raw("SELECT id FROM permissions WHERE name = ?", permName).Scan(&permIDStr).Error
		require.NoError(t, err)
		permID, err := uuid.Parse(permIDStr)
		require.NoError(t, err)

		var roleIDStr string
		err = db.Raw("SELECT id FROM roles WHERE name = ?", roleName).Scan(&roleIDStr).Error
		require.NoError(t, err)
		roleID, err := uuid.Parse(roleIDStr)
		require.NoError(t, err)

		err = db.Exec(`
			INSERT INTO user_roles (user_id, role_id, assigned_at)
			VALUES (?, ?, NOW())
			ON CONFLICT DO NOTHING
		`, user.ID, roleID).Error
		require.NoError(t, err)
		err = db.Exec(`
			INSERT INTO role_permissions (role_id, permission_id, assigned_at)
			VALUES (?, ?, NOW())
			ON CONFLICT DO NOTHING
		`, roleID, permID).Error
		require.NoError(t, err)

		if enforcer != nil {
			_ = enforcer.AddPolicy(roleName, "default", "media", action)
		}
	}

	if enforcer != nil {
		_ = enforcer.AddRoleForUser(user.ID.String(), roleName, "default")
		// In-memory policies already added above; LoadPolicy() would lose them
		// because casbin_rule is empty after SetupTest truncation.
	}

	return user
}

// generateTestTokenForMedia creates a real JWT token for testing
func generateTestTokenForMedia(userID uuid.UUID) string {
	token, _ := auth.GenerateAccessTokenWithClaims(
		userID.String(),
		"test@example.com",
		mediaJWTSeret,
		15*time.Minute,
		"",
		"",
	)
	return token
}

// addMediaManagePermission creates a role with media:manage permission for the user
func addMediaManagePermission(t *testing.T, db *gorm.DB, enforcer *permission.Enforcer, userID uuid.UUID) {
	roleName := "media-admin-" + userID.String()
	err := db.Exec(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, 'Media admin', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, roleName).Error
	require.NoError(t, err)

	permName := "media:manage"
	err = db.Exec(`
		INSERT INTO permissions (id, name, resource, action, scope, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, 'media', 'manage', 'all', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, permName).Error
	require.NoError(t, err)

	var roleIDStr string
	err = db.Raw("SELECT id FROM roles WHERE name = ?", roleName).Scan(&roleIDStr).Error
	require.NoError(t, err)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	var permIDStr string
	err = db.Raw("SELECT id FROM permissions WHERE name = ?", permName).Scan(&permIDStr).Error
	require.NoError(t, err)
	permID, err := uuid.Parse(permIDStr)
	require.NoError(t, err)

	err = db.Exec(`
		INSERT INTO user_roles (user_id, role_id, assigned_at)
		VALUES (?, ?, NOW())
		ON CONFLICT DO NOTHING
	`, userID, roleID).Error
	require.NoError(t, err)

	err = db.Exec(`
		INSERT INTO role_permissions (role_id, permission_id, assigned_at)
		VALUES (?, ?, NOW())
		ON CONFLICT DO NOTHING
	`, roleID, permID).Error
	require.NoError(t, err)

	if enforcer != nil {
		_ = enforcer.AddRoleForUser(userID.String(), roleName, "default")
		_ = enforcer.AddPolicy(roleName, "default", "media", "manage")
		// Do NOT call LoadPolicy() here: truncation from SetupTest() wiped casbin_rule,
		// and reloading from DB would discard the in-memory policies we just added.
	}
}

// setupTestEnforcer creates a test permission enforcer
func setupTestEnforcer(t *testing.T, db *gorm.DB) *permission.Enforcer {
	enforcer, err := permission.NewEnforcer(db)
	require.NoError(t, err)
	return enforcer
}

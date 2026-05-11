//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/http/response"
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

func createMediaVersionTables(t *testing.T, db *gorm.DB) {
	migration := `
	CREATE TABLE IF NOT EXISTS media_versions (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		media_id        UUID NOT NULL,
		version         INTEGER NOT NULL,
		filename        VARCHAR(255) NOT NULL,
		original_filename VARCHAR(500) NOT NULL,
		mime_type       VARCHAR(100) NOT NULL,
		size            BIGINT NOT NULL,
		file_path       VARCHAR(2000) NOT NULL,
		checksum        CHAR(64) NOT NULL,
		uploaded_by_id  UUID NOT NULL,
		created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
		deleted_at      TIMESTAMPTZ DEFAULT NULL,
		CONSTRAINT uq_media_versions_version UNIQUE (media_id, version)
	);

	CREATE INDEX IF NOT EXISTS idx_media_versions_media_id
		ON media_versions(media_id) WHERE deleted_at IS NULL;
	`

	result := db.Exec(migration)
	require.NoError(t, result.Error, "Failed to create media_versions table")

	// Add current_version column to media if it doesn't exist
	db.Exec("ALTER TABLE media ADD COLUMN IF NOT EXISTS current_version INTEGER NOT NULL DEFAULT 1")

	// Ensure foreign keys exist (create if not already present)
	db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_media_versions_media'
			) THEN
				ALTER TABLE media_versions
					ADD CONSTRAINT fk_media_versions_media
					FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE;
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_media_versions_user'
			) THEN
				ALTER TABLE media_versions
					ADD CONSTRAINT fk_media_versions_user
					FOREIGN KEY (uploaded_by_id) REFERENCES users(id) ON DELETE RESTRICT;
			END IF;
		END;
		$$;
	`)
}

func createTestUserWithVersionPerms(t *testing.T, db *gorm.DB, enforcer *permission.Enforcer) *domain.User {
	user := &domain.User{
		Email:        fmt.Sprintf("test-version-%s@example.com", uuid.New().String()),
		PasswordHash: "$2a$12$hashedpassword",
	}
	err := db.Create(user).Error
	require.NoError(t, err)

	roleName := "version-user-" + user.ID.String()
	err = db.Exec(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, 'Version user', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, roleName).Error
	require.NoError(t, err)

	actions := []struct {
		action   string
		resource string
	}{
		{"upload", "media"},
		{"view", "media"},
		{"download", "media"},
		{"view", "media_version"},
		{"upload", "media_version"},
		{"download", "media_version"},
		{"delete", "media_version"},
		{"manage", "media"},
	}
	for _, a := range actions {
		permName := a.resource + ":" + a.action
		err = db.Exec(`
			INSERT INTO permissions (id, name, resource, action, scope, created_at, updated_at)
			VALUES (gen_random_uuid(), ?, ?, ?, 'all', NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, permName, a.resource, a.action).Error
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
			_ = enforcer.AddPolicy(roleName, "default", a.resource, a.action)
		}
	}

	if enforcer != nil {
		_ = enforcer.AddRoleForUser(user.ID.String(), roleName, "default")
	}

	return user
}

func seedMediaWithVersions(t *testing.T, db *gorm.DB, userID uuid.UUID) uuid.UUID {
	mediaID := uuid.New()
	err := db.Exec(`
		INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
			mime_type, size, path, uploaded_by_id, current_version)
		VALUES (?, 'news', gen_random_uuid(), 'default', 'doc.pdf', 'original-doc.pdf',
			'application/pdf', 2048, '/test/path/doc.pdf', ?, 3)
	`, mediaID, userID).Error
	require.NoError(t, err)

	// Seed versions
	for v := 1; v <= 3; v++ {
		err = db.Exec(`
			INSERT INTO media_versions (id, media_id, version, filename, original_filename,
				mime_type, size, file_path, checksum, uploaded_by_id)
			VALUES (gen_random_uuid(), ?, ?, ?, 'original-v'+CAST(? AS VARCHAR)+'.pdf',
				'application/pdf', ?, '/test/path/v'+CAST(? AS VARCHAR)+'.pdf',
				LPAD(?, 64, '0'), ?)
		`, mediaID, v, fmt.Sprintf("v%d.pdf", v), v, 1024*int64(v), v,
			fmt.Sprintf("%064d", v), userID).Error
		require.NoError(t, err)
	}

	return mediaID
}

func setupMediaVersionHandler(t *testing.T, suite *TestSuite, enforcer *permission.Enforcer) (*echo.Echo, *handler.MediaVersionHandler, *handler.MediaHandler) {
	e := echo.New()

	tempDir, err := os.MkdirTemp("", "media-version-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	storageDriver, err := storage.NewDriver(storage.Config{
		Type:      "local",
		LocalPath: tempDir,
		BaseURL:   "http://localhost:8080/storage",
	})
	require.NoError(t, err)

	mediaRepo := repository.NewMediaRepository(suite.DB)
	versionRepo := repository.NewMediaVersionRepository(suite.DB)
	mediaService := service.NewMediaService(mediaRepo, enforcer, storageDriver, "test-signing-secret")
	auditLogRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditLogRepo, service.AuditServiceConfig{BufferSize: 10})
	versionService := service.NewVersionService(mediaRepo, versionRepo, storageDriver, "test-signing-secret", auditService, enforcer)

	versionHandler := handler.NewMediaVersionHandler(versionService, mediaService, enforcer)
	mediaHandler := handler.NewMediaHandler(mediaService, auditService, enforcer, "test-signing-secret")

	v1 := e.Group("/api/v1")
	mediaHandler.RegisterRoutes(v1, mediaJWTSeret)
	versionHandler.RegisterRoutes(v1, mediaJWTSeret)

	return e, versionHandler, mediaHandler
}

func TestListVersions(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("list versions with seeded data", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var envelope response.Envelope
		err := json.Unmarshal(rec.Body.Bytes(), &envelope)
		require.NoError(t, err)

		dataBytes, err := json.Marshal(envelope.Data)
		require.NoError(t, err)

		var history domain.VersionHistoryResponse
		err = json.Unmarshal(dataBytes, &history)
		require.NoError(t, err)

		assert.Equal(t, mediaID, history.MediaID)
		assert.Equal(t, 3, history.CurrentVersion)
		assert.Equal(t, int64(3), history.Total)
		assert.Len(t, history.Versions, 3)
		// Versions should be ordered DESC
		assert.Equal(t, 3, history.Versions[0].Version)
		assert.True(t, history.Versions[0].IsCurrent)
	})

	t.Run("list versions with unauthorized user", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'f.pdf', 'f.pdf',
				'application/pdf', 100, '/test/f.pdf', gen_random_uuid())
		`, mediaID).Error
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions", mediaID.String()), nil)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("list versions for nonexistent media", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions", uuid.New().String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestGetVersion(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("get specific version", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/2", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var envelope response.Envelope
		err := json.Unmarshal(rec.Body.Bytes(), &envelope)
		require.NoError(t, err)

		dataBytes, err := json.Marshal(envelope.Data)
		require.NoError(t, err)

		var ver domain.MediaVersionResponse
		err = json.Unmarshal(dataBytes, &ver)
		require.NoError(t, err)

		assert.Equal(t, 2, ver.Version)
		assert.Equal(t, mediaID, ver.MediaID)
		assert.False(t, ver.IsCurrent) // current is 3
	})

	t.Run("get version not found", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/99", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("get version with invalid version number", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/abc", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestUploadVersionIntegration(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("upload new version", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		// Create base media first
		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id, current_version)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'base.pdf', 'base.pdf',
				'application/pdf', 1024, '/test/base.pdf', ?, 1)
		`, mediaID, user.ID).Error
		require.NoError(t, err)

		// Upload a new version
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, err := writer.CreateFormFile("file", "v2.pdf")
		require.NoError(t, err)
		_, err = part.Write([]byte("%PDF-1.4 test content for v2"))
		require.NoError(t, err)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/media/%s/versions", mediaID.String()), &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "id")

		// Verify media version was updated
		var m domain.Media
		err = suite.DB.First(&m, "id = ?", mediaID).Error
		require.NoError(t, err)
		assert.Equal(t, 2, m.CurrentVersion)
	})
}

func TestDownloadVersionIntegration(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("download version file", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id, current_version)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'base.pdf', 'base.pdf',
				'application/pdf', 1024, '/test/base.pdf', ?, 1)
		`, mediaID, user.ID).Error
		require.NoError(t, err)

		// Upload a version first
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, err := writer.CreateFormFile("file", "v2.pdf")
		require.NoError(t, err)
		_, err = part.Write([]byte("%PDF-1.4 test content for v2"))
		require.NoError(t, err)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/media/%s/versions", mediaID.String()), &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Download the version
		req = httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/2/download", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "v2.pdf")
		assert.Equal(t, "21", rec.Header().Get("Content-Length"))
	})

	t.Run("download without permission", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id, current_version)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'base.pdf', 'base.pdf',
				'application/pdf', 1024, '/test/base.pdf', gen_random_uuid(), 1)
		`, mediaID).Error
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/1/download", mediaID.String()), nil)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestGetVersionSignedURLIntegration(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("get signed url for version", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)

		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id, current_version)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'base.pdf', 'base.pdf',
				'application/pdf', 1024, '/test/base.pdf', ?, 1)
		`, mediaID, user.ID).Error
		require.NoError(t, err)

		// Upload a version first
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		part, err := writer.CreateFormFile("file", "v2.pdf")
		require.NoError(t, err)
		_, err = part.Write([]byte("%PDF-1.4 test content for v2"))
		require.NoError(t, err)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost,
			fmt.Sprintf("/api/v1/media/%s/versions", mediaID.String()), &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Get signed URL for the version
		req = httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/v1/media/%s/versions/2/url?expires_in=3600", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var envelope response.Envelope
		err = json.Unmarshal(rec.Body.Bytes(), &envelope)
		require.NoError(t, err)

		dataBytes, err := json.Marshal(envelope.Data)
		require.NoError(t, err)

		var signedURL handler.SignedURLResponse
		err = json.Unmarshal(dataBytes, &signedURL)
		require.NoError(t, err)

		assert.NotEmpty(t, signedURL.URL)
		assert.NotZero(t, signedURL.ExpiresAt)
		assert.Equal(t, 3600, signedURL.ExpiresIn)
	})
}

func TestDeleteVersion(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	createMediaTables(t, suite.DB)
	createMediaVersionTables(t, suite.DB)

	e, _, _ := setupMediaVersionHandler(t, suite, nil)

	t.Run("delete specific version", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		// Delete version 2
		req := httptest.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/v1/media/%s/versions/2", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify version 2 is soft-deleted in DB
		var mv domain.MediaVersion
		err := suite.DB.Unscoped().Where("media_id = ? AND version = ?", mediaID, 2).First(&mv).Error
		require.NoError(t, err)
		assert.True(t, mv.DeletedAt.Valid)
		assert.False(t, mv.DeletedAt.Time.IsZero())
	})

	t.Run("delete current version is rejected", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		// Attempt to delete current version (3)
		req := httptest.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/v1/media/%s/versions/3", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("delete nonexistent version returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		enforcer := setupTestEnforcer(t, suite.DB)
		user := createTestUserWithVersionPerms(t, suite.DB, enforcer)
		token := generateTestTokenForMedia(user.ID)
		mediaID := seedMediaWithVersions(t, suite.DB, user.ID)

		req := httptest.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/v1/media/%s/versions/99", mediaID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("delete requires authentication", func(t *testing.T) {
		suite.SetupTest(t)
		createMediaTables(t, suite.DB)
		createMediaVersionTables(t, suite.DB)

		mediaID := uuid.New()
		err := suite.DB.Exec(`
			INSERT INTO media (id, model_type, model_id, collection_name, filename, original_filename,
				mime_type, size, path, uploaded_by_id, current_version)
			VALUES (?, 'news', gen_random_uuid(), 'default', 'f.pdf', 'f.pdf',
				'application/pdf', 100, '/test/f.pdf', gen_random_uuid(), 1)
		`, mediaID).Error
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete,
			fmt.Sprintf("/api/v1/media/%s/versions/1", mediaID.String()), nil)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

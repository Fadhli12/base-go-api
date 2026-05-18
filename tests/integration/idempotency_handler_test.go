//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// insertTestIdempotencyRecord creates a completed idempotency record directly in the database.
func insertTestIdempotencyRecord(t *testing.T, db *gorm.DB, userID uuid.UUID, key, method, path string) *domain.IdempotencyRecord {
	t.Helper()
	record := &domain.IdempotencyRecord{
		ID:                 uuid.New(),
		IdempotencyKey:     key,
		UserID:             userID,
		HTTPMethod:         method,
		RequestPath:        path,
		RequestHash:        fmt.Sprintf("%x", uuid.New())[:64], // 64-char hex string
		Status:             domain.IdempotencyStatusCompleted,
		ResponseStatusCode: 200,
		ResponseBody:        `{"ok":true}`,
		ResponseBodySize:    11,
		ResponseHeaders:     domain.MapStringString{"Content-Type": "application/json"},
		ExpiresAt:           time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, db.Create(record).Error, "should insert test idempotency record")
	return record
}

// setupIdempotencyHandler creates a fully-wired IdempotencyHandler for integration tests.
// Returns the handler and the enforcer (for adding permissions).
func setupIdempotencyHandler(t *testing.T, db *gorm.DB) (*handler.IdempotencyHandler, *permission.Enforcer) {
	t.Helper()

	idempotencyRepo := repository.NewIdempotencyRepository(db)
	cfg := config.DefaultIdempotencyConfig()
	idempotencySvc := service.NewIdempotencyService(idempotencyRepo, cfg, slog.Default())

	enforcer, err := permission.NewEnforcer(db)
	require.NoError(t, err, "should create enforcer")

	auditRepo := repository.NewAuditLogRepository(db)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())

	idempotencyHandler := handler.NewIdempotencyHandler(idempotencySvc, auditService, enforcer)
	return idempotencyHandler, enforcer
}

// setupIdempotencyTestServer creates a full HTTP server with idempotency routes registered.
func setupIdempotencyTestServer(t *testing.T, idempotencyHandler *handler.IdempotencyHandler) *echo.Echo {
	t.Helper()

	e := echo.New()
	jwtSecret := "test-secret-key-min-32-chars-long!"

	api := e.Group("/api/v1")
	idempotencyHandler.RegisterRoutes(api, jwtSecret)

	return e
}

// =============================================================================
// LIST TESTS
// =============================================================================

// TestIdempotencyHandler_List_Success verifies GET /api/v1/idempotency/keys returns 200 with paginated results.
func TestIdempotencyHandler_List_Success(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:view"})

	// Insert test records
	for i := 0; i < 3; i++ {
		insertTestIdempotencyRecord(t, suite.DB, userID,
			fmt.Sprintf("test-key-%d", i), "POST", "/api/v1/invoices")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/idempotency/keys?page=1&per_page=10", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	err := json.Unmarshal(rec.Body.Bytes(), &env)
	require.NoError(t, err)
	require.NotNil(t, env.Data, "response should have data")

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "data should be a map")

	meta, ok := dataMap["meta"].(map[string]interface{})
	require.True(t, ok, "meta should be a map")
	assert.Equal(t, float64(3), meta["total"])
}

// TestIdempotencyHandler_List_Pagination verifies pagination parameters work correctly.
func TestIdempotencyHandler_List_Pagination(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:view"})

	// Insert 5 records
	for i := 0; i < 5; i++ {
		insertTestIdempotencyRecord(t, suite.DB, userID,
			fmt.Sprintf("page-key-%d", i), "POST", "/api/v1/test")
	}

	// Request page 1 with per_page=2
	req := httptest.NewRequest(http.MethodGet, "/api/v1/idempotency/keys?page=1&per_page=2", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)

	meta, ok := dataMap["meta"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(5), meta["total"])
	assert.Equal(t, float64(1), meta["page"])
	assert.Equal(t, float64(2), meta["per_page"])
}

// TestIdempotencyHandler_List_DefaultPagination verifies default pagination when no params provided.
func TestIdempotencyHandler_List_DefaultPagination(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:view"})

	// Insert a single record
	insertTestIdempotencyRecord(t, suite.DB, userID, "default-key", "GET", "/api/v1/test")

	// Request without pagination params
	req := httptest.NewRequest(http.MethodGet, "/api/v1/idempotency/keys", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)

	meta, ok := dataMap["meta"].(map[string]interface{})
	require.True(t, ok)
	// Default page=1, per_page=20 (from DefaultIdempotencyConfig)
	assert.Equal(t, float64(1), meta["total"])
	assert.Equal(t, float64(1), meta["page"])
	assert.Equal(t, float64(20), meta["per_page"])
}

// TestIdempotencyHandler_List_MaxPagination verifies per_page > 100 is capped to 100.
func TestIdempotencyHandler_List_MaxPagination(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:view"})

	// Insert a record to have data
	insertTestIdempotencyRecord(t, suite.DB, userID, "max-page-key", "GET", "/api/v1/test")

	// Request with per_page=500 (over max)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/idempotency/keys?per_page=500", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)

	meta, ok := dataMap["meta"].(map[string]interface{})
	require.True(t, ok)
	// per_page should be capped to 100
	assert.Equal(t, float64(100), meta["per_page"])
}

// =============================================================================
// GET BY ID TESTS
// =============================================================================

// TestIdempotencyHandler_GetByID_Success verifies GET /api/v1/idempotency/keys/:id returns 200.
func TestIdempotencyHandler_GetByID_Success(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:view"})

	record := insertTestIdempotencyRecord(t, suite.DB, userID, "getbyid-key", "POST", "/api/v1/invoices")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/idempotency/keys/%s", record.ID), nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Data)
}

// TestIdempotencyHandler_GetByID_NotFound verifies GET with valid UUID but no record returns 404.
func TestIdempotencyHandler_GetByID_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:view"})

	// Use a random UUID that doesn't exist
	nonexistentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/idempotency/keys/%s", nonexistentID), nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// TestIdempotencyHandler_GetByID_InvalidID verifies GET with non-UUID path param returns 400.
func TestIdempotencyHandler_GetByID_InvalidID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:view"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/idempotency/keys/not-a-uuid", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_ID", env.Error.Code)
}

// =============================================================================
// TRIGGER CLEANUP TESTS
// =============================================================================

// TestIdempotencyHandler_TriggerCleanup_Success verifies POST /api/v1/idempotency/cleanup returns 202.
func TestIdempotencyHandler_TriggerCleanup_Success(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/idempotency/cleanup", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Data)

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "accepted", dataMap["status"])
	assert.Equal(t, float64(30), dataMap["retention_days"]) // default from config
}

// TestIdempotencyHandler_TriggerCleanup_WithRetentionDays verifies POST with ?retention_days=7 uses custom retention.
func TestIdempotencyHandler_TriggerCleanup_WithRetentionDays(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:manage"})

	// Insert an expired record to verify it gets cleaned up
	expiredRecord := &domain.IdempotencyRecord{
		ID:                 uuid.New(),
		IdempotencyKey:     "expired-key-cleanup",
		UserID:             userID,
		HTTPMethod:         "POST",
		RequestPath:        "/api/v1/test",
		RequestHash:        fmt.Sprintf("%x", uuid.New())[:64],
		Status:             domain.IdempotencyStatusCompleted,
		ResponseStatusCode: 200,
		ResponseBody:        `{"expired":true}`,
		ResponseBodySize:    16,
		ExpiresAt:           time.Now().AddDate(0, 0, -60), // 60 days ago = expired
	}
	require.NoError(t, suite.DB.Create(expiredRecord).Error)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/idempotency/cleanup?retention_days=7", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "accepted", dataMap["status"])
	assert.Equal(t, float64(7), dataMap["retention_days"])
	assert.Equal(t, float64(1), dataMap["cleaned_count"]) // should clean the expired record
}

// TestIdempotencyHandler_TriggerCleanup_InvalidRetentionDays verifies POST with ?retention_days=0 uses default.
func TestIdempotencyHandler_TriggerCleanup_InvalidRetentionDays(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/idempotency/cleanup?retention_days=0", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	// retention_days=0 is not > 0, so it falls back to default (30)
	assert.Equal(t, float64(30), dataMap["retention_days"])
}

// TestIdempotencyHandler_TriggerCleanup_NegativeRetentionDays verifies POST with ?retention_days=-1 uses default.
func TestIdempotencyHandler_TriggerCleanup_NegativeRetentionDays(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/idempotency/cleanup?retention_days=-1", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	// retention_days=-1 is not > 0, so it falls back to default (30)
	assert.Equal(t, float64(30), dataMap["retention_days"])
}

// =============================================================================
// DELETE TESTS
// =============================================================================

// TestIdempotencyHandler_Delete_Success verifies DELETE /api/v1/idempotency/keys/:id returns 200.
func TestIdempotencyHandler_Delete_Success(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:manage"})

	record := insertTestIdempotencyRecord(t, suite.DB, userID, "delete-key-success", "POST", "/api/v1/invoices")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/idempotency/keys/%s", record.ID), nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Data)

	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "deleted", dataMap["status"])
}

// TestIdempotencyHandler_Delete_NotFound verifies DELETE with non-existent ID returns 404.
func TestIdempotencyHandler_Delete_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	nonexistentID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/idempotency/keys/%s", nonexistentID), nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// TestIdempotencyHandler_Delete_InvalidID verifies DELETE with invalid UUID returns 400.
func TestIdempotencyHandler_Delete_InvalidID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/idempotency/keys/not-a-uuid", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_ID", env.Error.Code)
}

// TestIdempotencyHandler_Delete_DifferentUserForbidden verifies DELETE of another user's record returns 403.
func TestIdempotencyHandler_Delete_DifferentUserForbidden(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	// Create user A who owns the record
	_, userIDA, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:manage"})

	// Insert record owned by user A
	record := insertTestIdempotencyRecord(t, suite.DB, userIDA, "owner-key-delete", "POST", "/api/v1/test")

	// Create user B who tries to delete user A's record
	tokenB, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{"idempotency:manage"})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/idempotency/keys/%s", record.ID), nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+tokenB)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var env response.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotNil(t, env.Error)
	assert.Equal(t, "FORBIDDEN", env.Error.Code)
}

// TestIdempotencyHandler_Delete_AlreadyDeleted verifies DELETE of an already-deleted record returns 404.
func TestIdempotencyHandler_Delete_AlreadyDeleted(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	idempotencyHandler, enforcer := setupIdempotencyHandler(t, suite.DB)
	e := setupIdempotencyTestServer(t, idempotencyHandler)

	token, userID, _, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, []string{"idempotency:manage"})

	record := insertTestIdempotencyRecord(t, suite.DB, userID, "already-deleted-key", "POST", "/api/v1/test")

	// First delete - should succeed
	req1 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/idempotency/keys/%s", record.ID), nil)
	req1.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second delete - should return 404 (soft-deleted)
	req2 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/idempotency/keys/%s", record.ID), nil)
	req2.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}
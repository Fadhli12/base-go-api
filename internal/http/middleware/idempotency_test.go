package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

// MockCacheDriver implements cache.Driver for testing.
type MockCacheDriver struct {
	mock.Mock
}

func (m *MockCacheDriver) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheDriver) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	args := m.Called(ctx, key, value, ttlSeconds)
	return args.Error(0)
}

func (m *MockCacheDriver) SetNX(ctx context.Context, key string, value []byte, ttlSeconds int) (bool, error) {
	args := m.Called(ctx, key, value, ttlSeconds)
	return args.Bool(0), args.Error(1)
}

func (m *MockCacheDriver) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheDriver) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockCacheDriver) Clear(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *MockCacheDriver) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockIdempotencyRepository implements repository.IdempotencyRepository for testing.
type MockIdempotencyRepository struct {
	mock.Mock
}

func (m *MockIdempotencyRepository) Create(ctx context.Context, record *domain.IdempotencyRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockIdempotencyRepository) FindByKey(ctx context.Context, key string, userID uuid.UUID, method, path string) (*domain.IdempotencyRecord, error) {
	args := m.Called(ctx, key, userID, method, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IdempotencyRecord), args.Error(1)
}

func (m *MockIdempotencyRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.IdempotencyRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IdempotencyRecord), args.Error(1)
}

func (m *MockIdempotencyRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, responseStatusCode int, responseBody string, responseHeaders map[string]string) error {
	args := m.Called(ctx, id, status, responseStatusCode, responseBody, responseHeaders)
	return args.Error(0)
}

func (m *MockIdempotencyRepository) CleanupExpired(ctx context.Context, retentionDays int) (int64, error) {
	args := m.Called(ctx, retentionDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockIdempotencyRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.IdempotencyRecord, int64, error) {
	args := m.Called(ctx, userID, page, pageSize)
	return args.Get(0).([]*domain.IdempotencyRecord), args.Get(1).(int64), args.Error(2)
}

func (m *MockIdempotencyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// --- Helpers ---

// setupIdempotencyMiddleware creates a fully configured IdempotencyMiddleware with mocks.
func setupIdempotencyMiddleware(t *testing.T) (*IdempotencyMiddleware, *MockCacheDriver, *MockIdempotencyRepository, *service.IdempotencyService) {
	t.Helper()

	mockCache := new(MockCacheDriver)
	mockRepo := new(MockIdempotencyRepository)
	cfg := config.DefaultIdempotencyConfig()
	logger := newTestLogger(t)

	idemSvc := service.NewIdempotencyService(mockRepo, cfg, logger)

	mw := NewIdempotencyMiddleware(mockCache, idemSvc, cfg, logger)
	return mw, mockCache, mockRepo, idemSvc
}

// newTestLogger returns a discard logger for tests.
func newTestLogger(t *testing.T) *slog.Logger {
	// Use slog.Default() for tests; it goes to stderr which is fine
	return slog.Default()
}

// setupEchoContext creates an echo.Context with a request ready for idempotency testing.
func setupEchoContext(method, path string, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

// setupEchoContextWithUser creates an echo.Context with JWT claims (user context).
func setupEchoContextWithUser(method, path, body string, userID uuid.UUID) (echo.Context, *httptest.ResponseRecorder) {
	c, rec := setupEchoContext(method, path, body)
	c.Set("user", &auth.Claims{UserID: userID.String(), Email: "test@example.com"})
	c.Set("id", userID)
	return c, rec
}

// setupEchoContextWithUserAndOrg creates an echo.Context with JWT claims and organization ID.
func setupEchoContextWithUserAndOrg(method, path, body string, userID, orgID uuid.UUID) (echo.Context, *httptest.ResponseRecorder) {
	c, rec := setupEchoContextWithUser(method, path, body, userID)
	c.Set("organization_id", orgID)
	return c, rec
}

// idempotencyKeyLocked returns a Redis lock value with the given status.
func idempotencyKeyLocked(status string) []byte {
	data := map[string]string{"status": status}
	b, _ := json.Marshal(data)
	return b
}

// idempotencyKeyLockedWithHash returns a Redis lock value with status and request hash.
func idempotencyKeyLockedWithHash(status, hash string) []byte {
	data := map[string]string{"status": status, "request_hash": hash}
	b, _ := json.Marshal(data)
	return b
}

// cachedResponseJSON returns the Redis record value for a cached response.
func cachedResponseJSON(statusCode int, body string, headers map[string]string) []byte {
	cached := cachedResponse{
		StatusCode: statusCode,
		Body:       []byte(body),
		Headers:    headers,
	}
	b, _ := json.Marshal(cached)
	return b
}

// --- Key Validation Tests ---

func TestIdempotencyMiddleware_SkipsWhenNoKeyHeader(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "next handler should be called when no Idempotency-Key header")
}

func TestIdempotencyMiddleware_SkipsGETRequests(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodGet, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-123")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "GET requests should pass through regardless of idempotency key")
}

func TestIdempotencyMiddleware_SkipsDELETERequests(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodDelete, "/api/v1/test/123", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-123")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "DELETE requests should pass through regardless of idempotency key")
}

func TestIdempotencyMiddleware_SkipsWhenDisabled(t *testing.T) {
	mockCache := new(MockCacheDriver)
	mockRepo := new(MockIdempotencyRepository)
	cfg := config.DefaultIdempotencyConfig()
	cfg.Enabled = false
	logger := newTestLogger(t)

	idemSvc := service.NewIdempotencyService(mockRepo, cfg, logger)
	mw := NewIdempotencyMiddleware(mockCache, idemSvc, cfg, logger)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-123")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "next handler should be called when idempotency is disabled")
}

func TestIdempotencyMiddleware_SkipsWhenCacheNil(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	cfg := config.DefaultIdempotencyConfig()
	logger := newTestLogger(t)

	idemSvc := service.NewIdempotencyService(mockRepo, cfg, logger)
	mw := NewIdempotencyMiddleware(nil, idemSvc, cfg, logger)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-123")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "next handler should be called when cache driver is nil (fail-open)")
}

func TestIdempotencyMiddleware_RejectsInvalidKeyFormat(t *testing.T) {
	mw, mockCache, _, _ := setupIdempotencyMiddleware(t)

	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	// Keys with special characters that don't match ^[a-zA-Z0-9_-]+$
	c, _ := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "key with spaces!")

	// Lock key lookup should not happen for invalid keys
	mockCache.AssertNotCalled(t, "Get")

	err := handler(c)

	require.NoError(t, err)
	// The middleware returns a 400 response via c.JSON, not an error
}

func TestIdempotencyMiddleware_RejectsKeyTooLong(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	// Key longer than 128 chars
	longKey := strings.Repeat("a", 129)

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, longKey)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- SETNX Guard Tests ---

func TestIdempotencyMiddleware_AcquiresLockAndProcessesRequest(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"id": "new-resource-123"})
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-001")

	// Redis returns nil for lock key (no prior request)
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(nil, nil)

	// SETNX succeeds (lock acquired)
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(true, nil)

	// Store result calls (async, in goroutine)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil).Maybe()
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Wait briefly for async goroutine to complete
	time.Sleep(100 * time.Millisecond)
	mockCache.AssertExpectations(t)
}

func TestIdempotencyMiddleware_ConflictingRequestReturns409(t *testing.T) {
	mw, mockCache, _, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-002")

	// Redis returns nil for lock key (no prior state)
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(nil, nil)

	// SETNX fails (another request already acquired the lock)
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(false, nil)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "5", rec.Header().Get(RetryAfterHeader))
}

func TestIdempotencyMiddleware_SetNXErrorFailsOpen(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-003")

	// Redis returns nil for lock key
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(nil, nil)

	// SETNX returns an error (fail-open)
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(false, assert.AnError)

	// Store result calls (async) - use Maybe since fail-open proceeds
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil).Maybe()
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, called, "next handler should be called when SETNX fails (fail-open)")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Replay Tests ---

func TestIdempotencyMiddleware_ProcessingRequestReturns409(t *testing.T) {
	mw, mockCache, _, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-004")

	// Redis returns a lock key with status=processing
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(idempotencyKeyLocked(domain.IdempotencyStatusProcessing), nil)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "5", rec.Header().Get(RetryAfterHeader))
}

func TestIdempotencyMiddleware_ReplaysCompletedFromRedisCache(t *testing.T) {
	mw, mockCache, _, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-005")

	// Redis returns a lock key with status=completed
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(idempotencyKeyLocked(domain.IdempotencyStatusCompleted), nil)

	// Redis returns cached response in record key
	cachedResp := cachedResponseJSON(http.StatusCreated, `{"id":"resource-005"}`, map[string]string{
		"Content-Type": "application/json",
	})
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyRecordSuffix)
	})).Return(cachedResp, nil)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "true", rec.Header().Get(IdempotencyReplayedHeader))
	assert.Equal(t, "create-resource-005", rec.Header().Get(IdempotencyKeyResponseHeader))
}

func TestIdempotencyMiddleware_ReplaysCompletedFromDB(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-006")

	// Redis returns a lock key with status=completed
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(idempotencyKeyLocked(domain.IdempotencyStatusCompleted), nil)

	// Redis record key returns error (cache miss) -> fall back to DB
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyRecordSuffix)
	})).Return(nil, assert.AnError)

	// DB returns a completed record
	dbRecord := &domain.IdempotencyRecord{
		IdempotencyKey:     "create-resource-006",
		UserID:             userID,
		HTTPMethod:         "post",
		RequestPath:        "/api/v1/resources",
		Status:             domain.IdempotencyStatusCompleted,
		ResponseStatusCode: http.StatusCreated,
		ResponseBody:       `{"id":"resource-006"}`,
		ResponseHeaders:    domain.MapStringString{"Content-Type": "application/json"},
	}
	mockRepo.On("FindByKey", mock.Anything, "create-resource-006", userID, "POST", "/api/v1/resources").Return(dbRecord, nil)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "true", rec.Header().Get(IdempotencyReplayedHeader))
	assert.Equal(t, "create-resource-006", rec.Header().Get(IdempotencyKeyResponseHeader))
}

func TestIdempotencyMiddleware_ReplayFromDBNotFoundReturns409(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-007")

	// Redis returns a lock key with status=completed
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(idempotencyKeyLocked(domain.IdempotencyStatusCompleted), nil)

	// Redis record key returns error -> fall back to DB
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyRecordSuffix)
	})).Return(nil, assert.AnError)

	// DB returns ErrNotFound (record doesn't exist)
	appErr := apperrors.ErrNotFound
	mockRepo.On("FindByKey", mock.Anything, "create-resource-007", userID, "POST", "/api/v1/resources").Return(nil, appErr)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

// --- Fail-Open Tests ---

func TestIdempotencyMiddleware_GetLockErrorFailsOpen(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-008")

	// Redis Get returns an error (fail-open)
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(nil, assert.AnError)

	// SETNX should be attempted (since lock read failed, we try to acquire)
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(true, nil)

	// Store result calls (async)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil).Maybe()
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, called, "next handler should be called when Redis Get fails (fail-open)")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIdempotencyMiddleware_RecordKeyGetErrorFallsBackToDB(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-009")

	// Redis returns completed lock
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(idempotencyKeyLocked(domain.IdempotencyStatusCompleted), nil)

	// Redis record key Get returns error -> fall back to DB
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyRecordSuffix)
	})).Return(nil, assert.AnError)

	// DB returns a completed record
	dbRecord := &domain.IdempotencyRecord{
		IdempotencyKey:     "create-resource-009",
		UserID:             userID,
		HTTPMethod:         "post",
		RequestPath:        "/api/v1/resources",
		Status:             domain.IdempotencyStatusCompleted,
		ResponseStatusCode: http.StatusOK,
		ResponseBody:       `{"result":"from-db"}`,
		ResponseHeaders:    domain.MapStringString{"Content-Type": "application/json"},
	}
	mockRepo.On("FindByKey", mock.Anything, "create-resource-009", userID, "POST", "/api/v1/resources").Return(dbRecord, nil)

	err := handler(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "true", rec.Header().Get(IdempotencyReplayedHeader))
}

// --- Response Writer Tests ---

func TestIdempotencyResponseWriter_CapturesStatusCode(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &idempotencyResponseWriter{
		ResponseWriter: rec,
		maxSize:        4096 + 1024,
	}

	rw.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, rw.statusCode)
	assert.True(t, rw.wrote)
}

func TestIdempotencyResponseWriter_CapturesBody(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &idempotencyResponseWriter{
		ResponseWriter: rec,
		maxSize:        4096 + 1024,
	}

	body := []byte(`{"id":"123","name":"test"}`)
	n, err := rw.Write(body)

	require.NoError(t, err)
	assert.Equal(t, len(body), n)
	assert.Equal(t, body, rw.body.Bytes())
}

func TestIdempotencyResponseWriter_CapturesHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.Header().Set("Location", "/api/v1/resources/123")
	rec.Header().Set("X-Request-Id", "req-abc-123")
	rec.Header().Set("X-Custom-Header", "should-not-be-captured")

	rw := &idempotencyResponseWriter{
		ResponseWriter: rec,
		maxSize:        4096 + 1024,
	}

	rw.WriteHeader(http.StatusCreated)

	assert.Equal(t, "application/json", rw.headers["Content-Type"])
	assert.Equal(t, "/api/v1/resources/123", rw.headers["Location"])
	assert.Equal(t, "req-abc-123", rw.headers["X-Request-Id"])
	// X-Custom-Header is NOT in replayHeaders set, so it should NOT be captured
	_, hasCustom := rw.headers["X-Custom-Header"]
	assert.False(t, hasCustom, "headers not in replayHeaders set should not be captured")
}

func TestIdempotencyResponseWriter_DefaultStatusCode200(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &idempotencyResponseWriter{
		ResponseWriter: rec,
		maxSize:        4096 + 1024,
	}

	// Write body without calling WriteHeader explicitly
	_, err := rw.Write([]byte("hello"))
	require.NoError(t, err)

	// statusCode should be 0 before Write, then set to 200 after Write triggers implicit header
	assert.Equal(t, http.StatusOK, rw.statusCode)
}

func TestIsWebSocketRequest(t *testing.T) {
	t.Run("websocket upgrade header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Header.Set("Upgrade", "websocket")
		assert.True(t, isWebSocketRequest(req))
	})

	t.Run("connection upgrade header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Header.Set("Connection", "Upgrade")
		assert.True(t, isWebSocketRequest(req))
	})

	t.Run("connection upgrade lowercase", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Header.Set("Connection", "keep-alive, upgrade")
		assert.True(t, isWebSocketRequest(req))
	})

	t.Run("normal request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		assert.False(t, isWebSocketRequest(req))
	})

	t.Run("empty headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
		assert.False(t, isWebSocketRequest(req))
	})
}

// --- Additional Edge Case Tests ---

func TestIdempotencyMiddleware_SkipsOPTIONSRequests(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodOptions, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-opt")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "OPTIONS requests should pass through regardless of idempotency key")
}

func TestIdempotencyMiddleware_SkipsHEADRequests(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, _ := setupEchoContextWithUser(http.MethodHead, "/api/v1/test", "", uuid.New())
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-head")
	err := handler(c)

	require.NoError(t, err)
	assert.True(t, called, "HEAD requests should pass through regardless of idempotency key")
}

func TestIdempotencyMiddleware_RejectsInvalidKeyWithSpecialChars(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	handler := mw.Middleware()(func(c echo.Context) error {
		return c.String(http.StatusOK, "should not be called")
	})

	// Key with characters not matching ^[a-zA-Z0-9_-]+$
	invalidKeys := []string{
		"key with spaces",
		"key@with#special",
		"key/with/slashes",
		"key.with.dots", // dots are not in the pattern
		"key:colon",
	}

	for _, key := range invalidKeys {
		t.Run("invalid_key_"+key[:4], func(t *testing.T) {
			c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/test", "", uuid.New())
			c.Request().Header.Set(IdempotencyKeyHeader, key)

			err := handler(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestIdempotencyMiddleware_SkipsWhenNoUserID(t *testing.T) {
	mw, _, _, _ := setupIdempotencyMiddleware(t)

	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	// No user in context (unauthenticated)
	c, _ := setupEchoContext(http.MethodPost, "/api/v1/test", `{"name":"test"}`)
	c.Request().Header.Set(IdempotencyKeyHeader, "test-key-no-user")

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, called, "should pass through when no userID (unauthenticated)")
}

func TestIdempotencyMiddleware_UnmarshalLockErrorFailsOpen(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-unmarshal")

	// Redis returns invalid JSON in the lock key (unmarshal fails -> fail-open)
	invalidLockValue := []byte("this-is-not-json")
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(invalidLockValue, nil)

	// After unmarshal error, middleware falls through to SETNX attempt
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(true, nil)

	// Store result calls (async)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil).Maybe()
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, called, "should pass through and process request when lock unmarshal fails")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIdempotencyMiddleware_ExpiredKeyWithNilLockValue(t *testing.T) {
	mw, mockCache, mockRepo, _ := setupIdempotencyMiddleware(t)

	userID := uuid.New()
	called := false
	handler := mw.Middleware()(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})

	c, rec := setupEchoContextWithUser(http.MethodPost, "/api/v1/resources", `{"name":"test"}`, userID)
	c.Request().Header.Set(IdempotencyKeyHeader, "create-resource-expired")

	// Redis Get returns nil value with nil error (key doesn't exist)
	mockCache.On("Get", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	})).Return(nil, nil)

	// SETNX succeeds (lock acquired)
	mockCache.On("SetNX", mock.Anything, mock.MatchedBy(func(key string) bool {
		return strings.HasSuffix(key, IdempotencyLockSuffix)
	}), mock.Anything, mock.Anything).Return(true, nil)

	// Store result calls (async)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.IdempotencyRecord")).Return(nil).Maybe()
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, called, "should process request when lock key not found in Redis")
	assert.Equal(t, http.StatusOK, rec.Code)
}


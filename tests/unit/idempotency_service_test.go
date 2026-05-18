package unit

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ======================================================================
// Mock implementations
// ======================================================================

// MockIdempotencyRepository implements repository.IdempotencyRepository
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
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.IdempotencyRecord), args.Get(1).(int64), args.Error(2)
}

func (m *MockIdempotencyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ======================================================================
// Helpers
// ======================================================================

func newTestIdempotencyRecord() *domain.IdempotencyRecord {
	userID := uuid.New()
	return &domain.IdempotencyRecord{
		ID:             uuid.New(),
		IdempotencyKey: "test-key-123",
		UserID:         userID,
		HTTPMethod:     "POST",
		RequestPath:    "/api/v1/invoices",
		RequestHash:    "a1b2c3d4e5f6",
		Status:         domain.IdempotencyStatusProcessing,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func newTestIdempotencyConfig() config.IdempotencyConfig {
	return config.IdempotencyConfig{
		Enabled:         true,
		MaxKeyLength:    128,
		KeyPattern:     `^[a-zA-Z0-9_-]+$`,
		DefaultTTL:      24 * time.Hour,
		MinTTL:          1 * time.Hour,
		MaxTTL:          72 * time.Hour,
		GuardTTL:        5 * time.Minute,
		ReaperInterval:  100 * time.Millisecond, // fast for testing
		RetentionDays:   30,
		DefaultPageSize: 20,
		MaxPageSize:     100,
	}
}

// ======================================================================
// CreateRecord tests
// ======================================================================

func TestIdempotencyService_CreateRecord_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	record := newTestIdempotencyRecord()
	repo.On("Create", mock.Anything, record).Return(nil)

	err := svc.CreateRecord(context.Background(), record)

	require.NoError(t, err)
	repo.AssertCalled(t, "Create", mock.Anything, record)
}

func TestIdempotencyService_CreateRecord_RepositoryError(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	record := newTestIdempotencyRecord()
	dbErr := errors.New("database connection failed")
	repo.On("Create", mock.Anything, record).Return(dbErr)

	err := svc.CreateRecord(context.Background(), record)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 500, appErr.HTTPStatus)
	repo.AssertCalled(t, "Create", mock.Anything, record)
}

// ======================================================================
// FindRecord tests
// ======================================================================

func TestIdempotencyService_FindRecord_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	record := newTestIdempotencyRecord()
	repo.On("FindByKey", mock.Anything, "test-key-123", record.UserID, "POST", "/api/v1/invoices").Return(record, nil)

	result, err := svc.FindRecord(context.Background(), "test-key-123", record.UserID, "POST", "/api/v1/invoices")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, record.ID, result.ID)
	assert.Equal(t, "test-key-123", result.IdempotencyKey)
	repo.AssertCalled(t, "FindByKey", mock.Anything, "test-key-123", record.UserID, "POST", "/api/v1/invoices")
}

func TestIdempotencyService_FindRecord_NotFound(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	userID := uuid.New()
	repo.On("FindByKey", mock.Anything, "nonexistent", userID, "GET", "/api/v1/test").Return(nil, apperrors.ErrNotFound)

	result, err := svc.FindRecord(context.Background(), "nonexistent", userID, "GET", "/api/v1/test")

	assert.Nil(t, result)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// FindRecordByID tests
// ======================================================================

func TestIdempotencyService_FindRecordByID_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	record := newTestIdempotencyRecord()
	repo.On("FindByID", mock.Anything, record.ID).Return(record, nil)

	result, err := svc.FindRecordByID(context.Background(), record.ID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, record.ID, result.ID)
}

func TestIdempotencyService_FindRecordByID_NotFound(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, apperrors.ErrNotFound)

	result, err := svc.FindRecordByID(context.Background(), id)

	assert.Nil(t, result)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// UpdateRecord tests
// ======================================================================

func TestIdempotencyService_UpdateRecord_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	headers := map[string]string{"Content-Type": "application/json"}
	repo.On("UpdateStatus", mock.Anything, id, "completed", 200, `{"status":"ok"}`, headers).Return(nil)

	err := svc.UpdateRecord(context.Background(), id, "completed", 200, `{"status":"ok"}`, headers)

	require.NoError(t, err)
	repo.AssertCalled(t, "UpdateStatus", mock.Anything, id, "completed", 200, `{"status":"ok"}`, headers)
}

func TestIdempotencyService_UpdateRecord_RepositoryError(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	dbErr := errors.New("update failed")
	repo.On("UpdateStatus", mock.Anything, id, "completed", 200, "", map[string]string(nil)).Return(dbErr)

	err := svc.UpdateRecord(context.Background(), id, "completed", 200, "", nil)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 500, appErr.HTTPStatus)
}

// ======================================================================
// ListByUser tests
// ======================================================================

func TestIdempotencyService_ListByUser_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	userID := uuid.New()
	record := newTestIdempotencyRecord()
	record.UserID = userID
	records := []*domain.IdempotencyRecord{record}
	total := int64(1)

	repo.On("ListByUser", mock.Anything, userID, 1, 20).Return(records, total, nil)

	result, count, err := svc.ListByUser(context.Background(), userID, 1, 20)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, total, count)
	assert.Equal(t, record.ID, result[0].ID)
	repo.AssertCalled(t, "ListByUser", mock.Anything, userID, 1, 20)
}

func TestIdempotencyService_ListByUser_RepositoryError(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	userID := uuid.New()
	dbErr := errors.New("query failed")
	repo.On("ListByUser", mock.Anything, userID, 1, 20).Return(nil, int64(0), dbErr)

	result, count, err := svc.ListByUser(context.Background(), userID, 1, 20)

	assert.Nil(t, result)
	assert.Equal(t, int64(0), count)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 500, appErr.HTTPStatus)
}

// ======================================================================
// CleanupExpired tests
// ======================================================================

func TestIdempotencyService_CleanupExpired_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	repo.On("CleanupExpired", mock.Anything, 7).Return(int64(10), nil)

	cleaned, err := svc.CleanupExpired(7)

	require.NoError(t, err)
	assert.Equal(t, int64(10), cleaned)
	repo.AssertCalled(t, "CleanupExpired", mock.Anything, 7)
}

func TestIdempotencyService_CleanupExpired_DefaultRetentionDays(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	cfg.RetentionDays = 30
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	// When days <= 0, should use config default (30)
	repo.On("CleanupExpired", mock.Anything, 30).Return(int64(5), nil)

	cleaned, err := svc.CleanupExpired(0)

	require.NoError(t, err)
	assert.Equal(t, int64(5), cleaned)
	repo.AssertCalled(t, "CleanupExpired", mock.Anything, 30)
}

// ======================================================================
// Reaper tests
// ======================================================================

func TestIdempotencyService_Reaper_StartStop(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should not panic
	svc.StartReaper(ctx)

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	svc.StopReaper()
}

func TestIdempotencyService_Reaper_CleanupCalled(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	cfg.ReaperInterval = 100 * time.Millisecond
	cfg.RetentionDays = 30
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	repo.On("CleanupExpired", mock.Anything, 30).Return(int64(2), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartReaper(ctx)

	// Wait for at least one tick
	time.Sleep(250 * time.Millisecond)

	svc.StopReaper()

	repo.AssertCalled(t, "CleanupExpired", mock.Anything, 30)
}

// ======================================================================
// Delete tests
// ======================================================================

func TestIdempotencyService_Delete_Success(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	repo.On("SoftDelete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)

	require.NoError(t, err)
	repo.AssertCalled(t, "SoftDelete", mock.Anything, id)
}

func TestIdempotencyService_Delete_NotFound(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	repo.On("SoftDelete", mock.Anything, id).Return(apperrors.ErrNotFound)

	err := svc.Delete(context.Background(), id)

	require.Error(t, err)
	// Service wraps repo error with WrapInternal, so ErrNotFound (404) becomes 500
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 500, appErr.HTTPStatus)
	repo.AssertCalled(t, "SoftDelete", mock.Anything, id)
}

func TestIdempotencyService_Delete_RepositoryError(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	id := uuid.New()
	dbErr := errors.New("database connection failed")
	repo.On("SoftDelete", mock.Anything, id).Return(dbErr)

	err := svc.Delete(context.Background(), id)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 500, appErr.HTTPStatus)
}

// ======================================================================
// Config accessor test
// ======================================================================

func TestIdempotencyService_Config(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()
	svc := service.NewIdempotencyService(repo, cfg, logger)

	result := svc.Config()

	assert.Equal(t, cfg.Enabled, result.Enabled)
	assert.Equal(t, cfg.MaxKeyLength, result.MaxKeyLength)
	assert.Equal(t, cfg.RetentionDays, result.RetentionDays)
	assert.Equal(t, cfg.ReaperInterval, result.ReaperInterval)
}

// ======================================================================
// Constructor test
// ======================================================================

func TestNewIdempotencyService(t *testing.T) {
	repo := new(MockIdempotencyRepository)
	cfg := newTestIdempotencyConfig()
	logger := slog.Default()

	svc := service.NewIdempotencyService(repo, cfg, logger)

	require.NotNil(t, svc)
}
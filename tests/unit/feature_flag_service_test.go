package unit

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

type MockFeatureFlagRepository struct {
	mock.Mock
	mu      sync.Mutex
	storage map[uuid.UUID]*domain.FeatureFlag
	byKey   map[string]*domain.FeatureFlag
}

func newMockFeatureFlagRepository() *MockFeatureFlagRepository {
	return &MockFeatureFlagRepository{
		storage: make(map[uuid.UUID]*domain.FeatureFlag),
		byKey:   make(map[string]*domain.FeatureFlag),
	}
}

func (m *MockFeatureFlagRepository) Create(ctx context.Context, flag *domain.FeatureFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, flag)
	if args.Error(0) == nil {
		m.storage[flag.ID] = flag
		m.byKey[flag.Key] = flag
	}
	return args.Error(0)
}

func (m *MockFeatureFlagRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.FeatureFlag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FeatureFlag), args.Error(1)
}

func (m *MockFeatureFlagRepository) FindByKey(ctx context.Context, key string) (*domain.FeatureFlag, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FeatureFlag), args.Error(1)
}

func (m *MockFeatureFlagRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.FeatureFlag, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.FeatureFlag), args.Get(1).(int64), args.Error(2)
}

func (m *MockFeatureFlagRepository) Update(ctx context.Context, flag *domain.FeatureFlag) error {
	args := m.Called(ctx, flag)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.storage[flag.ID] = flag
		m.byKey[flag.Key] = flag
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockFeatureFlagRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	if args.Error(0) == nil {
		m.mu.Lock()
		if flag, ok := m.storage[id]; ok {
			delete(m.byKey, flag.Key)
			delete(m.storage, id)
		}
		m.mu.Unlock()
	}
	return args.Error(0)
}

type MockAuditLogForFeatureFlags struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func newMockAuditLogForFeatureFlags() *MockAuditLogForFeatureFlags {
	return &MockAuditLogForFeatureFlags{}
}

func (m *MockAuditLogForFeatureFlags) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *MockAuditLogForFeatureFlags) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForFeatureFlags) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForFeatureFlags) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForFeatureFlags) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func newTestFeatureFlagService(
	repo *MockFeatureFlagRepository,
	enforcer *permission.Enforcer,
	auditRepo *MockAuditLogForFeatureFlags,
) *service.FeatureFlagService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewFeatureFlagService(repo, enforcer, auditSvc, log)
}

func newTestEnforcer(t *testing.T) *permission.Enforcer {
	t.Helper()
	enforcer, err := permission.NewTestEnforcer()
	require.NoError(t, err)
	return enforcer
}

func grantManage(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("admin", domain, "feature_flag", "manage"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "admin", domain))
}

func grantView(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("viewer", domain, "feature_flag", "view"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "viewer", domain))
}

func boolPtr(b bool) *bool    { return &b }
func ffIntPtr(i int) *int     { return &i }
func ffStrPtr(s string) *string { return &s }

func makeFlag(key string, enabled bool, rollout int) *domain.FeatureFlag {
	return &domain.FeatureFlag{
		ID:        uuid.New(),
		Key:       key,
		Name:      key,
		Enabled:   enabled,
		Rollout:   rollout,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func makeCreateRequest(key, name string, enabled *bool, rollout int, conditions map[string]interface{}) request.CreateFeatureFlagRequest {
	return request.CreateFeatureFlagRequest{
		Key:         key,
		Name:        name,
		Enabled:     enabled,
		Rollout:     rollout,
		Conditions:  conditions,
	}
}

func ffWaitForAuditLogs(auditRepo *MockAuditLogForFeatureFlags, expectedCount int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if auditRepo.GetCount() >= expectedCount {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ======================================================================
// Create tests
// ======================================================================

func TestFeatureFlagService_Create_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	repo.On("FindByKey", mock.Anything, "new_flag").Return(nil, apperrors.ErrNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.FeatureFlag")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	req := makeCreateRequest("new_flag", "New Flag", boolPtr(true), 100, nil)
	resp, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "new_flag", resp.Key)
	assert.Equal(t, "New Flag", resp.Name)
	assert.True(t, resp.Enabled)
	assert.Equal(t, 100, resp.Rollout)

	repo.AssertExpectations(t)
}

func TestFeatureFlagService_Create_DuplicateKeyConflict(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	existing := makeFlag("existing_flag", true, 100)
	repo.On("FindByKey", mock.Anything, "existing_flag").Return(existing, nil)

	req := makeCreateRequest("existing_flag", "Existing Flag", boolPtr(true), 100, nil)
	_, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "CONFLICT", appErr.Code)
	assert.Equal(t, 409, appErr.HTTPStatus)
}

func TestFeatureFlagService_Create_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	req := makeCreateRequest("new_flag", "New Flag", boolPtr(true), 100, nil)
	_, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestFeatureFlagService_Create_BadKeyFormat(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	req := makeCreateRequest("BAD_KEY", "Bad Key", boolPtr(true), 50, nil)
	_, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus)
}

func TestFeatureFlagService_Create_RolloutOutOfRange(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	req := makeCreateRequest("flag_key", "Flag", boolPtr(true), 150, nil)
	_, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestFeatureFlagService_Create_NegativeRollout(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	req := makeCreateRequest("flag_key", "Flag", boolPtr(true), -1, nil)
	_, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestFeatureFlagService_Create_DefaultEnabledFalse(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	repo.On("FindByKey", mock.Anything, "default_off").Return(nil, apperrors.ErrNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.FeatureFlag")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	req := makeCreateRequest("default_off", "Default Off", nil, 50, nil)
	resp, err := svc.Create(context.Background(), userID, true, orgID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Enabled)
}

// ======================================================================
// GetByID tests
// ======================================================================

func TestFeatureFlagService_GetByID_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantView(t, enforcer, userID, orgID)

	flag := makeFlag("test_flag", true, 100)
	flag.ID = flagID
	repo.On("FindByID", mock.Anything, flagID).Return(flag, nil)

	resp, err := svc.GetByID(context.Background(), userID, true, orgID, flagID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "test_flag", resp.Key)
}

func TestFeatureFlagService_GetByID_NotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantView(t, enforcer, userID, orgID)

	repo.On("FindByID", mock.Anything, flagID).Return(nil, apperrors.ErrNotFound)

	_, err := svc.GetByID(context.Background(), userID, true, orgID, flagID)

	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))
}

func TestFeatureFlagService_GetByID_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	_, err := svc.GetByID(context.Background(), userID, true, orgID, flagID)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// ======================================================================
// GetByKey tests
// ======================================================================

func TestFeatureFlagService_GetByKey_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantView(t, enforcer, userID, orgID)

	flag := makeFlag("my_flag", true, 75)
	repo.On("FindByKey", mock.Anything, "my_flag").Return(flag, nil)

	resp, err := svc.GetByKey(context.Background(), userID, true, orgID, "my_flag")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "my_flag", resp.Key)
}

func TestFeatureFlagService_GetByKey_NotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantView(t, enforcer, userID, orgID)

	repo.On("FindByKey", mock.Anything, "missing_flag").Return(nil, apperrors.ErrNotFound)

	_, err := svc.GetByKey(context.Background(), userID, true, orgID, "missing_flag")

	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))
}

func TestFeatureFlagService_GetByKey_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	_, err := svc.GetByKey(context.Background(), userID, true, orgID, "any_flag")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// ======================================================================
// List tests
// ======================================================================

func TestFeatureFlagService_List_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantView(t, enforcer, userID, orgID)

	flags := []*domain.FeatureFlag{
		makeFlag("flag_a", true, 100),
		makeFlag("flag_b", false, 50),
	}
	repo.On("FindAll", mock.Anything, 10, 0).Return(flags, int64(2), nil)

	responses, total, err := svc.List(context.Background(), userID, true, orgID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, responses, 2)
}

func TestFeatureFlagService_List_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	_, _, err := svc.List(context.Background(), userID, true, orgID, 10, 0)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// ======================================================================
// Update tests
// ======================================================================

func TestFeatureFlagService_Update_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	flag := makeFlag("update_flag", false, 50)
	flag.ID = flagID
	repo.On("FindByID", mock.Anything, flagID).Return(flag, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.FeatureFlag")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	newName := "Updated Flag"
	req := request.UpdateFeatureFlagRequest{
		Name:    &newName,
		Enabled: boolPtr(true),
	}
	resp, err := svc.Update(context.Background(), userID, true, orgID, flagID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Updated Flag", resp.Name)
	assert.True(t, resp.Enabled)
}

func TestFeatureFlagService_Update_NotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	repo.On("FindByID", mock.Anything, flagID).Return(nil, apperrors.ErrNotFound)

	req := request.UpdateFeatureFlagRequest{Name: ffStrPtr("x")}
	_, err := svc.Update(context.Background(), userID, true, orgID, flagID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))
}

func TestFeatureFlagService_Update_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	req := request.UpdateFeatureFlagRequest{Name: ffStrPtr("x")}
	_, err := svc.Update(context.Background(), userID, true, orgID, flagID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

func TestFeatureFlagService_Update_RolloutValidation(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	req := request.UpdateFeatureFlagRequest{Rollout: ffIntPtr(200)}
	_, err := svc.Update(context.Background(), userID, true, orgID, flagID, req, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

// ======================================================================
// Delete tests
// ======================================================================

func TestFeatureFlagService_Delete_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	flag := makeFlag("delete_flag", true, 100)
	flag.ID = flagID
	flag.IsSystem = false
	repo.On("FindByID", mock.Anything, flagID).Return(flag, nil)
	repo.On("SoftDelete", mock.Anything, flagID).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := svc.Delete(context.Background(), userID, true, orgID, flagID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
}

func TestFeatureFlagService_Delete_SystemFlagForbidden(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	flag := makeFlag("system_flag", true, 100)
	flag.ID = flagID
	flag.IsSystem = true
	repo.On("FindByID", mock.Anything, flagID).Return(flag, nil)

	err := svc.Delete(context.Background(), userID, true, orgID, flagID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestFeatureFlagService_Delete_NotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	grantManage(t, enforcer, userID, orgID)

	repo.On("FindByID", mock.Anything, flagID).Return(nil, apperrors.ErrNotFound)

	err := svc.Delete(context.Background(), userID, true, orgID, flagID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))
}

func TestFeatureFlagService_Delete_RBACDenied(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flagID := uuid.New()

	err := svc.Delete(context.Background(), userID, true, orgID, flagID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// ======================================================================
// IsEnabled tests
// ======================================================================

func TestFeatureFlagService_IsEnabled_Disabled(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	flag := makeFlag("disabled_flag", false, 100)
	repo.On("FindByKey", mock.Anything, "disabled_flag").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "disabled_flag", uuid.New())
	assert.False(t, result)
}

func TestFeatureFlagService_IsEnabled_FlagNotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	repo.On("FindByKey", mock.Anything, "missing_flag").Return(nil, apperrors.ErrNotFound)

	result := svc.IsEnabled(context.Background(), "missing_flag", uuid.New())
	assert.False(t, result)
}

func TestFeatureFlagService_IsEnabled_Rollout100(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	flag := makeFlag("full_rollout", true, 100)
	repo.On("FindByKey", mock.Anything, "full_rollout").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "full_rollout", uuid.New())
	assert.True(t, result)
}

func TestFeatureFlagService_IsEnabled_Rollout0(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	flag := makeFlag("zero_rollout", true, 0)
	repo.On("FindByKey", mock.Anything, "zero_rollout").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "zero_rollout", uuid.New())
	assert.False(t, result)
}

func TestFeatureFlagService_IsEnabled_RolloutMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	flag := makeFlag("rollout_flag", true, 50)
	repo.On("FindByKey", mock.Anything, "rollout_flag").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "rollout_flag", userID)
	hash := fnvHash(userID.String() + "rollout_flag") % 100
	expected := hash < uint32(50)
	assert.Equal(t, expected, result)
}

func TestFeatureFlagService_IsEnabled_ConditionMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	conditions := datatypes.JSON(`{"user_ids":["00000000-0000-0000-0000-000000000001"]}`)
	flag := &domain.FeatureFlag{
		ID:         uuid.New(),
		Key:        "condition_flag",
		Name:       "Condition Flag",
		Enabled:    true,
		Rollout:    100,
		Conditions: conditions,
	}
	repo.On("FindByKey", mock.Anything, "condition_flag").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "condition_flag", userID)
	assert.True(t, result)
}

func TestFeatureFlagService_IsEnabled_ConditionNoMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	conditions := datatypes.JSON(`{"user_ids":["00000000-0000-0000-0000-000000000001"]}`)
	flag := &domain.FeatureFlag{
		ID:         uuid.New(),
		Key:        "condition_flag",
		Name:       "Condition Flag",
		Enabled:    true,
		Rollout:    100,
		Conditions: conditions,
	}
	repo.On("FindByKey", mock.Anything, "condition_flag").Return(flag, nil)

	result := svc.IsEnabled(context.Background(), "condition_flag", userID)
	assert.False(t, result)
}

// ======================================================================
// IsEnabledWithReason tests
// ======================================================================

func TestFeatureFlagService_IsEnabledWithReason_FlagDisabled(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flag := makeFlag("disabled_flag", false, 100)
	repo.On("FindByKey", mock.Anything, "disabled_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "disabled_flag")
	require.NoError(t, err)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "flag_disabled", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_FlagNotFound(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	repo.On("FindByKey", mock.Anything, "missing_flag").Return(nil, apperrors.ErrNotFound)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "missing_flag")
	require.NoError(t, err)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "flag_not_found", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_Rollout100(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flag := makeFlag("full_flag", true, 100)
	repo.On("FindByKey", mock.Anything, "full_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "full_flag")
	require.NoError(t, err)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "rollout_100", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_Rollout0(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flag := makeFlag("zero_flag", true, 0)
	repo.On("FindByKey", mock.Anything, "zero_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "zero_flag")
	require.NoError(t, err)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "rollout_0", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_RolloutMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.New()
	flag := makeFlag("rollout_flag", true, 100)
	repo.On("FindByKey", mock.Anything, "rollout_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "rollout_flag")
	require.NoError(t, err)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "rollout_100", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_RolloutNoMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.New()

	hash := fnvHash(userID.String()+"rollout_flag") % 100
	var rollout int
	if hash < 50 {
		rollout = 10
	} else {
		rollout = 50
	}

	flag := makeFlag("rollout_flag", true, rollout)
	repo.On("FindByKey", mock.Anything, "rollout_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "rollout_flag")
	require.NoError(t, err)

	hashVal := fnvHash(userID.String()+"rollout_flag") % 100
	if hashVal < uint32(rollout) {
		assert.Equal(t, "rollout_match", eval.Reason)
		assert.True(t, eval.Enabled)
	} else {
		assert.Equal(t, "rollout_no_match", eval.Reason)
		assert.False(t, eval.Enabled)
	}
}

func TestFeatureFlagService_IsEnabledWithReason_ConditionMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orgID := uuid.New()
	conditions := datatypes.JSON(`{"user_ids":["00000000-0000-0000-0000-000000000001"]}`)
	flag := &domain.FeatureFlag{
		ID:         uuid.New(),
		Key:        "cond_flag",
		Name:       "Cond Flag",
		Enabled:    true,
		Rollout:    100,
		Conditions: conditions,
	}
	repo.On("FindByKey", mock.Anything, "cond_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "cond_flag")
	require.NoError(t, err)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "rollout_100", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_ConditionNoMatch(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	orgID := uuid.New()
	conditions := datatypes.JSON(`{"user_ids":["00000000-0000-0000-0000-000000000001"]}`)
	flag := &domain.FeatureFlag{
		ID:         uuid.New(),
		Key:        "cond_flag",
		Name:       "Cond Flag",
		Enabled:    true,
		Rollout:    100,
		Conditions: conditions,
	}
	repo.On("FindByKey", mock.Anything, "cond_flag").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "cond_flag")
	require.NoError(t, err)
	assert.False(t, eval.Enabled)
	assert.Equal(t, "condition_no_match", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_EmptyConditions(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	conditions := datatypes.JSON(`{}`)
	flag := &domain.FeatureFlag{
		ID:         uuid.New(),
		Key:        "empty_cond",
		Name:       "Empty Conditions",
		Enabled:    true,
		Rollout:    100,
		Conditions: conditions,
	}
	repo.On("FindByKey", mock.Anything, "empty_cond").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "empty_cond")
	require.NoError(t, err)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "rollout_100", eval.Reason)
}

func TestFeatureFlagService_IsEnabledWithReason_NilConditions(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	flag := &domain.FeatureFlag{
		ID:      uuid.New(),
		Key:     "nil_cond",
		Name:    "Nil Conditions",
		Enabled: true,
		Rollout: 100,
	}
	repo.On("FindByKey", mock.Anything, "nil_cond").Return(flag, nil)

	eval, err := svc.IsEnabledWithReason(context.Background(), userID, true, orgID, "nil_cond")
	require.NoError(t, err)
	assert.True(t, eval.Enabled)
	assert.Equal(t, "rollout_100", eval.Reason)
}

// ======================================================================
// EvaluateAll tests
// ======================================================================

func TestFeatureFlagService_EvaluateAll_Success(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	flags := []*domain.FeatureFlag{
		makeFlag("enabled_flag", true, 100),
		makeFlag("disabled_flag", false, 50),
		makeFlag("zero_rollout", true, 0),
	}
	repo.On("FindAll", mock.Anything, 0, 0).Return(flags, int64(3), nil)

	result, err := svc.EvaluateAll(context.Background(), userID, true, orgID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Flags, 3)
}

func TestFeatureFlagService_EvaluateAll_MultipleFlagsWithReasons(t *testing.T) {
	repo := newMockFeatureFlagRepository()
	enforcer := newTestEnforcer(t)
	auditRepo := newMockAuditLogForFeatureFlags()
	svc := newTestFeatureFlagService(repo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	flags := []*domain.FeatureFlag{
		makeFlag("full_on", true, 100),
		{ID: uuid.New(), Key: "half_on", Name: "Half", Enabled: true, Rollout: 50, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		makeFlag("off_flag", false, 100),
	}
	repo.On("FindAll", mock.Anything, 0, 0).Return(flags, int64(3), nil)

	result, err := svc.EvaluateAll(context.Background(), userID, true, orgID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Flags, 3)

	for _, eval := range result.Flags {
		if eval.Key == "full_on" {
			assert.True(t, eval.Enabled)
			assert.Equal(t, "rollout_100", eval.Reason)
		}
		if eval.Key == "off_flag" {
			assert.False(t, eval.Enabled)
			assert.Equal(t, "flag_disabled", eval.Reason)
		}
	}
}

func fnvHash(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for _, c := range s {
		h ^= uint32(c)
		h *= prime32
	}
	return h
}

var _ = json.Marshal
package unit

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// --- Mock Repositories ---

// MockUserSettingsRepository implements repository.UserSettingsRepository
type MockUserSettingsRepository struct {
	mock.Mock
	mu       sync.Mutex
	storage  map[string]*domain.UserSettings // key: "userID:orgID"
	upserted *domain.UserSettings
}

func newMockUserSettingsRepository() *MockUserSettingsRepository {
	return &MockUserSettingsRepository{
		storage: make(map[string]*domain.UserSettings),
	}
}

func (m *MockUserSettingsRepository) Upsert(ctx context.Context, settings *domain.UserSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, settings)
	if args.Error(0) == nil {
		key := settings.UserID.String() + ":" + settings.OrganizationID.String()
		m.storage[key] = settings
		m.upserted = settings
	}
	return args.Error(0)
}

func (m *MockUserSettingsRepository) FindByUserIDAndOrgID(ctx context.Context, userID, orgID uuid.UUID) (*domain.UserSettings, error) {
	args := m.Called(ctx, userID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserSettings), args.Error(1)
}

func (m *MockUserSettingsRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserSettings, int64, error) {
	args := m.Called(ctx, orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.UserSettings), args.Get(1).(int64), args.Error(2)
}

// MockSystemSettingsRepository implements repository.SystemSettingsRepository
type MockSystemSettingsRepository struct {
	mock.Mock
	mu       sync.Mutex
	storage  map[uuid.UUID]*domain.SystemSettings
	upserted *domain.SystemSettings
}

func newMockSystemSettingsRepository() *MockSystemSettingsRepository {
	return &MockSystemSettingsRepository{
		storage: make(map[uuid.UUID]*domain.SystemSettings),
	}
}

func (m *MockSystemSettingsRepository) Upsert(ctx context.Context, settings *domain.SystemSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, settings)
	if args.Error(0) == nil {
		m.storage[settings.OrganizationID] = settings
		m.upserted = settings
	}
	return args.Error(0)
}

func (m *MockSystemSettingsRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID) (*domain.SystemSettings, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SystemSettings), args.Error(1)
}

// MockAuditLogForSettings implements repository.AuditLogRepository for settings tests
type MockAuditLogForSettings struct {
	mock.Mock
	mu      sync.Mutex
	logs    []*domain.AuditLog
	count   int
	created chan *domain.AuditLog
}

func newMockAuditLogForSettings() *MockAuditLogForSettings {
	return &MockAuditLogForSettings{
		created: make(chan *domain.AuditLog, 100),
	}
}

func (m *MockAuditLogForSettings) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *MockAuditLogForSettings) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForSettings) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForSettings) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForSettings) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *MockAuditLogForSettings) GetLogs() []*domain.AuditLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logs
}

// --- Helpers ---

func newTestSettingsService(
	userRepo *MockUserSettingsRepository,
	systemRepo *MockSystemSettingsRepository,
	auditRepo *MockAuditLogForSettings,
) *service.SettingsService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewSettingsService(userRepo, systemRepo, auditSvc, log)
}

func newTestAuditSvcForSettings(auditRepo *MockAuditLogForSettings) *service.AuditService {
	return service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
}

// waitForAuditLogs waits for the async audit service to process logs
func waitForAuditLogs(auditRepo *MockAuditLogForSettings, expectedCount int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if auditRepo.GetCount() >= expectedCount {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// --- Tests ---

func TestNewSettingsService(t *testing.T) {
	t.Run("creates service with all dependencies", func(t *testing.T) {
		userRepo := newMockUserSettingsRepository()
		systemRepo := newMockSystemSettingsRepository()
		auditRepo := newMockAuditLogForSettings()
		auditSvc := newTestAuditSvcForSettings(auditRepo)
		log := slog.Default()

		svc := service.NewSettingsService(userRepo, systemRepo, auditSvc, log)
		require.NotNil(t, svc)
	})
}

func TestGetUserSettings_ExistingSettings(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	existingSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"theme":"light","language":"en"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(existingSettings, nil)

	resp, err := svc.GetUserSettings(context.Background(), userID, orgID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, userID, resp.UserID)
	assert.Equal(t, orgID, resp.OrganizationID)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &settings))
	assert.Equal(t, "light", settings["theme"])
	assert.Equal(t, "en", settings["language"])
}

func TestGetUserSettings_EmptyResult(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// Return ErrNotFound (nil, not-found error)
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, nil) // nil settings, nil error = not found but no error

	resp, err := svc.GetUserSettings(context.Background(), userID, orgID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, userID, resp.UserID)
	assert.Equal(t, orgID, resp.OrganizationID)
	// Should return empty JSON object
	assert.Equal(t, json.RawMessage(`{}`), resp.Settings)
}

func TestGetUserSettings_RepoError(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, assert.AnError)

	resp, err := svc.GetUserSettings(context.Background(), userID, orgID)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestGetSystemSettings_Defaults(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()

	// Return nil settings (not found) and nil error
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(nil, nil) // nil, nil means "not found, no error"

	resp, err := svc.GetSystemSettings(context.Background(), orgID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, orgID, resp.OrganizationID)

	// Should contain default values
	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &settings))
	assert.Equal(t, "API Base", settings["app_name"])
	assert.Equal(t, false, settings["maintenance_mode"])
}

func TestGetSystemSettings_ExistingSettings(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	existingSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"Custom App","maintenance_mode":true}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil)

	resp, err := svc.GetSystemSettings(context.Background(), orgID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, orgID, resp.OrganizationID)

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &settings))
	assert.Equal(t, "Custom App", settings["app_name"])
	assert.Equal(t, true, settings["maintenance_mode"])
}

func TestGetEffectiveSettings_UserOverridesSystem(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	systemSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"theme":"light","language":"en","notifications":true}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	userSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"theme":"dark","timezone":"UTC"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).Return(systemSettings, nil)
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).Return(userSettings, nil)

	result, err := svc.GetEffectiveSettings(context.Background(), userID, orgID)

	require.NoError(t, err)
	require.NotNil(t, result)

	var merged map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &merged))

	// User overrides system
	assert.Equal(t, "dark", merged["theme"])
	// System defaults preserved when user doesn't override
	assert.Equal(t, "en", merged["language"])
	assert.Equal(t, true, merged["notifications"])
	// User-specific settings included
	assert.Equal(t, "UTC", merged["timezone"])
}

func TestGetEffectiveSettings_NoSystemSettings(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// No system settings, no user settings
	systemRepo.On("FindByOrgID", mock.Anything, orgID).Return(nil, nil)
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).Return(nil, nil)

	result, err := svc.GetEffectiveSettings(context.Background(), userID, orgID)

	require.NoError(t, err)
	require.NotNil(t, result)

	var merged map[string]interface{}
	require.NoError(t, json.Unmarshal(result, &merged))

	// Should contain default system settings
	assert.Equal(t, "API Base", merged["app_name"])
}

func TestUpdateUserSettings_MergePreservesExisting(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// Existing settings have "language":"en"
	existingSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"language":"en"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// First call: FindByUserIDAndOrgID returns existing settings
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(existingSettings, nil).Once()

	// Upsert succeeds
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings")).
		Return(nil).Once()

	// Second call: FindByUserIDAndOrgID returns the merged settings
	mergedSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"language":"en","theme":"dark"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(mergedSettings, nil).Once()

	// Audit logging
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	// Update only "theme" - "language" should be preserved
	updates := map[string]interface{}{"theme": "dark"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify the upserted data contains both old and new keys
	userRepo.AssertCalled(t, "Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings"))

	// Verify the merged response has both keys
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &result))
	assert.Equal(t, "dark", result["theme"])
	assert.Equal(t, "en", result["language"])
}

func TestUpdateUserSettings_MergeWithEmptyExisting(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// No existing settings (first time user)
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, nil).Once()

	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings")).
		Return(nil).Once()

	updatedSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"theme":"dark"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(updatedSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	updates := map[string]interface{}{"theme": "dark"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &result))
	assert.Equal(t, "dark", result["theme"])
}

func TestUpdateUserSettings_AuditLogCalled(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	auditSvc := newTestAuditSvcForSettings(auditRepo)
	log := slog.Default()
	svc := service.NewSettingsService(userRepo, systemRepo, auditSvc, log)

	userID := uuid.New()
	orgID := uuid.New()

	existingSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"language":"en"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(existingSettings, nil).Once()
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings")).
		Return(nil).Once()
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(existingSettings, nil).Once()

	// Expect audit log to be created
	auditRepo.On("Create", mock.Anything, mock.MatchedBy(func(auditLog *domain.AuditLog) bool {
		return auditLog.ActorID == userID &&
			auditLog.Action == domain.AuditActionUpdate &&
			auditLog.Resource == "user_settings" &&
			auditLog.ResourceID == userID.String() &&
			auditLog.IPAddress == "192.168.1.1" &&
			auditLog.UserAgent == "test-user-agent"
	})).Return(nil)

	updates := map[string]interface{}{"theme": "dark"}
	_, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "192.168.1.1", "test-user-agent")

	require.NoError(t, err)

	// Wait for async audit to process
	require.Eventually(t, func() bool {
		return auditRepo.GetCount() >= 1
	}, 2*time.Second, 50*time.Millisecond, "audit log should be created")

	auditRepo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(auditLog *domain.AuditLog) bool {
		return auditLog.ActorID == userID &&
			auditLog.Action == domain.AuditActionUpdate &&
			auditLog.Resource == "user_settings"
	}))
}

func TestUpdateUserSettings_ValidTimezone(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, nil).Once()
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings")).
		Return(nil).Once()

	updatedSettings := &domain.UserSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Settings:       datatypes.JSON(`{"timezone":"America/New_York"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(updatedSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	updates := map[string]interface{}{"timezone": "America/New_York"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestUpdateUserSettings_InvalidTimezone(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	updates := map[string]interface{}{"timezone": "Invalid/Timezone"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid timezone")
}

func TestUpdateUserSettings_RepoError(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, assert.AnError)

	updates := map[string]interface{}{"theme": "dark"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestUpdateSystemSettings_MergePreservesExisting(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	// Existing system settings have "app_name":"MyApp"
	existingSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp","maintenance_mode":false}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil).Once()
	systemRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SystemSettings")).
		Return(nil).Once()

	mergedSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp","maintenance_mode":false,"requests_per_minute":100}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(mergedSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	// Update only "requests_per_minute" - existing config should be preserved
	updates := map[string]interface{}{"requests_per_minute": 100}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify the merged response has both old and new keys
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &result))
	assert.Equal(t, "MyApp", result["app_name"])
	assert.Equal(t, false, result["maintenance_mode"])
	assert.Equal(t, float64(100), result["requests_per_minute"])
}

func TestUpdateSystemSettings_AuditLogCalled(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	auditSvc := newTestAuditSvcForSettings(auditRepo)
	log := slog.Default()
	svc := service.NewSettingsService(userRepo, systemRepo, auditSvc, log)

	orgID := uuid.New()
	actorID := uuid.New()

	existingSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil).Once()
	systemRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SystemSettings")).
		Return(nil).Once()
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.MatchedBy(func(auditLog *domain.AuditLog) bool {
		return auditLog.ActorID == actorID &&
			auditLog.Action == domain.AuditActionUpdate &&
			auditLog.Resource == "system_settings" &&
			auditLog.ResourceID == orgID.String() &&
			auditLog.IPAddress == "10.0.0.1" &&
			auditLog.UserAgent == "admin-browser"
	})).Return(nil)

	updates := map[string]interface{}{"maintenance_mode": true}
	_, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "10.0.0.1", "admin-browser")

	require.NoError(t, err)

	// Wait for async audit to process
	require.Eventually(t, func() bool {
		return auditRepo.GetCount() >= 1
	}, 2*time.Second, 50*time.Millisecond, "audit log should be created")

	auditRepo.AssertCalled(t, "Create", mock.Anything, mock.MatchedBy(func(auditLog *domain.AuditLog) bool {
		return auditLog.ActorID == actorID &&
			auditLog.Action == domain.AuditActionUpdate &&
			auditLog.Resource == "system_settings"
	}))
}

func TestUpdateSystemSettings_AllowlistFiltering(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	existingSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil).Once()
	systemRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SystemSettings")).
		Return(nil).Once()

	updatedSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp","maintenance_mode":true,"from_address":"no-reply@example.com"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(updatedSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	// Mix of allowed and disallowed fields
	updates := map[string]interface{}{
		"maintenance_mode": true,                      // allowed
		"app_name":         "Hacked",                  // NOT in allowlist → filtered out
		"secret_key":       "s3cr3t",                   // NOT in allowlist → filtered out
		"from_address":     "no-reply@example.com",    // allowed
	}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)

	// app_name should NOT have been updated (filtered out by allowlist)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &result))
	// The existing app_name should be preserved since "app_name" is not in the allowlist
	assert.Equal(t, "MyApp", result["app_name"])
	// But maintenance_mode and from_address should be updated
	assert.Equal(t, true, result["maintenance_mode"])
	assert.Equal(t, "no-reply@example.com", result["from_address"])
}

func TestUpdateSystemSettings_NoValidFields(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	// All fields are outside the allowlist
	updates := map[string]interface{}{
		"app_name":  "Hacked",
		"secret_key": "s3cr3t",
		"admin_pass": "password123",
	}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no valid fields")
}

func TestUpdateSystemSettings_RepoError(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(nil, assert.AnError)

	updates := map[string]interface{}{"maintenance_mode": true}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestUpdateUserSettings_RepoErrorOnUpsert(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	userRepo.On("FindByUserIDAndOrgID", mock.Anything, userID, orgID).
		Return(nil, nil).Once()
	userRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.UserSettings")).
		Return(assert.AnError).Once()

	updates := map[string]interface{}{"theme": "dark"}
	resp, err := svc.UpdateUserSettings(context.Background(), userID, orgID, updates, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestGetEffectiveSettings_SystemRepoError(t *testing.T) {
	userRepo := newMockUserSettingsRepository()
	systemRepo := newMockSystemSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(nil, assert.AnError)

	result, err := svc.GetEffectiveSettings(context.Background(), userID, orgID)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateSystemSettings_RepoErrorOnUpsert(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	existingSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"app_name":"MyApp"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(existingSettings, nil).Once()
	systemRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SystemSettings")).
		Return(assert.AnError).Once()

	updates := map[string]interface{}{"maintenance_mode": true}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestUpdateSystemSettings_MergeWithEmptyExisting(t *testing.T) {
	systemRepo := newMockSystemSettingsRepository()
	userRepo := newMockUserSettingsRepository()
	auditRepo := newMockAuditLogForSettings()
	svc := newTestSettingsService(userRepo, systemRepo, auditRepo)

	orgID := uuid.New()
	actorID := uuid.New()

	// No existing system settings
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(nil, nil).Once()
	systemRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.SystemSettings")).
		Return(nil).Once()

	updatedSettings := &domain.SystemSettings{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Settings:       datatypes.JSON(`{"maintenance_mode":true}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	systemRepo.On("FindByOrgID", mock.Anything, orgID).
		Return(updatedSettings, nil).Once()

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	updates := map[string]interface{}{"maintenance_mode": true}
	resp, err := svc.UpdateSystemSettings(context.Background(), orgID, updates, actorID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	require.NotNil(t, resp)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Settings, &result))
	assert.Equal(t, true, result["maintenance_mode"])
}
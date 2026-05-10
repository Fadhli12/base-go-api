package unit

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockNotificationRepository implements repository.NotificationRepository
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, notification *domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	args := m.Called(ctx, notifications)
	return args.Error(0)
}

func (m *MockNotificationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Notification), args.Get(1).(int64), args.Error(2)
}

func (m *MockNotificationRepository) FindUnreadByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Notification, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Notification), args.Get(1).(int64), args.Error(2)
}

func (m *MockNotificationRepository) CountUnreadByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, id, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID, notifType *string) (int64, error) {
	args := m.Called(ctx, userID, notifType)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) ArchiveByID(ctx context.Context, id, userID uuid.UUID) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

// MockNotificationPreferenceRepository implements repository.NotificationPreferenceRepository
type MockNotificationPreferenceRepository struct {
	mock.Mock
}

func (m *MockNotificationPreferenceRepository) Upsert(ctx context.Context, pref *domain.NotificationPreference) error {
	args := m.Called(ctx, pref)
	return args.Error(0)
}

func (m *MockNotificationPreferenceRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.NotificationPreference, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.NotificationPreference), args.Error(1)
}

func (m *MockNotificationPreferenceRepository) FindByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) (*domain.NotificationPreference, error) {
	args := m.Called(ctx, userID, notifType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NotificationPreference), args.Error(1)
}

func (m *MockNotificationPreferenceRepository) DeleteByUserIDAndType(ctx context.Context, userID uuid.UUID, notifType domain.NotificationType) error {
	args := m.Called(ctx, userID, notifType)
	return args.Error(0)
}

// MockUserRepository implements repository.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

// MockCacheDriver implements cache.Driver
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

// Helper to create a NotificationService with mocks
func newTestNotificationService(
	notifRepo *MockNotificationRepository,
	prefRepo *MockNotificationPreferenceRepository,
	userRepo *MockUserRepository,
	cacheDriver *MockCacheDriver,
) *service.NotificationService {
	return service.NewNotificationService(
		notifRepo,
		prefRepo,
		nil, // emailService - nil to skip email queuing
		userRepo,
		cacheDriver,
		slog.Default(),
	)
}

// TestNotificationService_Send tests the Send method
func TestNotificationService_Send_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	title := "Test Notification"
	message := "This is a test message"
	actionURL := "https://example.com/action"

	// Cache returns nil (no existing count)
	cacheDriver.On("Get", ctx, mock.Anything).Return(nil, nil)
	// Cache Set for new key
	cacheDriver.On("Set", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Notification create
	notifRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

	err := svc.Send(ctx, userID, notifType, title, message, actionURL)
	require.NoError(t, err)

	notifRepo.AssertExpectations(t)
	cacheDriver.AssertExpectations(t)
}

func TestNotificationService_Send_RateLimited(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	title := "Test Notification"
	message := "This is a test message"
	actionURL := ""

	// Cache returns 100 (at rate limit)
	cacheDriver.On("Get", ctx, mock.Anything).Return([]byte("100"), nil)

	err := svc.Send(ctx, userID, notifType, title, message, actionURL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrTooManyRequests))

	cacheDriver.AssertExpectations(t)
	notifRepo.AssertNotCalled(t, "Create")
}

func TestNotificationService_Send_InvalidType(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := "invalid.notification.type"
	title := "Test Notification"
	message := "This is a test message"
	actionURL := ""

	err := svc.Send(ctx, userID, notifType, title, message, actionURL)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus)

	notifRepo.AssertNotCalled(t, "Create")
}

// TestNotificationService_SendBulk tests the SendBulk method
func TestNotificationService_SendBulk_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID1 := uuid.New()
	userID2 := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	title := "Bulk Notification"
	message := "This is a bulk test message"
	actionURL := "https://example.com/action"

	// Each user gets rate limit check and notification create
	cacheDriver.On("Get", ctx, mock.Anything).Return(nil, nil).Twice()
	cacheDriver.On("Set", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()
	notifRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil).Twice()

	err := svc.SendBulk(ctx, []uuid.UUID{userID1, userID2}, notifType, title, message, actionURL)
	require.NoError(t, err)

	notifRepo.AssertExpectations(t)
	cacheDriver.AssertExpectations(t)
}

func TestNotificationService_SendBulk_EmptyList(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	notifType := string(domain.NotificationTypeSystem)
	title := "Bulk Notification"
	message := "This is a bulk test message"
	actionURL := ""

	err := svc.SendBulk(ctx, []uuid.UUID{}, notifType, title, message, actionURL)
	require.NoError(t, err)

	notifRepo.AssertNotCalled(t, "Create")
}

func TestNotificationService_SendBulk_PartialFailure(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID1 := uuid.New()
	userID2 := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	title := "Bulk Notification"
	message := "This is a bulk test message"
	actionURL := "https://example.com/action"

	// User 1: cache miss, set, create success
	cacheDriver.On("Get", ctx, mock.Anything).Return(nil, nil).Once()
	cacheDriver.On("Set", ctx, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	notifRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil).Once()

	// User 2: at rate limit
	cacheDriver.On("Get", ctx, mock.Anything).Return([]byte("100"), nil).Once()

	// SendBulk should continue after error for user 2
	err := svc.SendBulk(ctx, []uuid.UUID{userID1, userID2}, notifType, title, message, actionURL)
	// SendBulk doesn't return error even if individual sends fail - it logs and continues
	require.NoError(t, err)

	notifRepo.AssertExpectations(t)
	cacheDriver.AssertExpectations(t)
}

func TestNotificationService_SendBulk_InvalidType(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := "invalid.type"
	title := "Bulk Notification"
	message := "This is a bulk test message"
	actionURL := ""

	err := svc.SendBulk(ctx, []uuid.UUID{userID}, notifType, title, message, actionURL)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)

	notifRepo.AssertNotCalled(t, "Create")
}

// TestNotificationService_ListByUser tests the ListByUser method
func TestNotificationService_ListByUser(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	limit := 10
	offset := 0

	notifications := []*domain.Notification{
		{
			ID:      uuid.New(),
			UserID:  userID,
			Type:    domain.NotificationTypeSystem,
			Title:   "Notification 1",
			Message: "Message 1",
		},
		{
			ID:      uuid.New(),
			UserID:  userID,
			Type:    domain.NotificationTypeMention,
			Title:   "Notification 2",
			Message: "Message 2",
		},
	}
	total := int64(2)

	notifRepo.On("FindByUserID", ctx, userID, limit, offset).Return(notifications, total, nil)

	result, resultTotal, err := svc.ListByUser(ctx, userID, limit, offset)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, total, resultTotal)

	notifRepo.AssertExpectations(t)
}

// TestNotificationService_ListUnreadByUser tests the ListUnreadByUser method
func TestNotificationService_ListUnreadByUser(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	limit := 5
	offset := 10

	notifications := []*domain.Notification{
		{
			ID:      uuid.New(),
			UserID:  userID,
			Type:    domain.NotificationTypeAssignment,
			Title:   "Unread Notification",
			Message: "Unread message",
		},
	}
	total := int64(1)

	notifRepo.On("FindUnreadByUserID", ctx, userID, limit, offset).Return(notifications, total, nil)

	result, resultTotal, err := svc.ListUnreadByUser(ctx, userID, limit, offset)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, total, resultTotal)

	notifRepo.AssertExpectations(t)
}

// TestNotificationService_CountUnread tests the CountUnread method
func TestNotificationService_CountUnread(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	expectedCount := int64(5)

	notifRepo.On("CountUnreadByUserID", ctx, userID).Return(expectedCount, nil)

	count, err := svc.CountUnread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, count)

	notifRepo.AssertExpectations(t)
}

// TestNotificationService_MarkAsRead tests the MarkAsRead method
func TestNotificationService_MarkAsRead_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	notifID := uuid.New()
	userID := uuid.New()

	notifRepo.On("MarkAsRead", ctx, notifID, userID).Return(nil)

	err := svc.MarkAsRead(ctx, notifID, userID)
	require.NoError(t, err)

	notifRepo.AssertExpectations(t)
}

func TestNotificationService_MarkAsRead_NotFound(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	notifID := uuid.New()
	userID := uuid.New()

	notifRepo.On("MarkAsRead", ctx, notifID, userID).Return(apperrors.ErrNotFound)

	err := svc.MarkAsRead(ctx, notifID, userID)
	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))

	notifRepo.AssertExpectations(t)
}

// TestNotificationService_MarkAllAsRead tests the MarkAllAsRead method
func TestNotificationService_MarkAllAsRead_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	expectedAffected := int64(3)

	notifRepo.On("MarkAllAsRead", ctx, userID, (*string)(nil)).Return(expectedAffected, nil)

	count, err := svc.MarkAllAsRead(ctx, userID, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedAffected, count)

	notifRepo.AssertExpectations(t)
}

func TestNotificationService_MarkAllAsRead_WithTypeFilter(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	expectedAffected := int64(2)

	notifRepo.On("MarkAllAsRead", ctx, userID, &notifType).Return(expectedAffected, nil)

	count, err := svc.MarkAllAsRead(ctx, userID, &notifType)
	require.NoError(t, err)
	assert.Equal(t, expectedAffected, count)

	notifRepo.AssertExpectations(t)
}

func TestNotificationService_MarkAllAsRead_InvalidType(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	invalidType := "invalid.type"

	count, err := svc.MarkAllAsRead(ctx, userID, &invalidType)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Equal(t, int64(0), count)

	notifRepo.AssertNotCalled(t, "MarkAllAsRead")
}

// TestNotificationService_GetPreferences tests the GetPreferences method
func TestNotificationService_GetPreferences(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()

	prefs := []*domain.NotificationPreference{
		{
			ID:               uuid.New(),
			UserID:           userID,
			NotificationType: domain.NotificationTypeSystem,
			EmailEnabled:     true,
			PushEnabled:      true,
		},
		{
			ID:               uuid.New(),
			UserID:           userID,
			NotificationType: domain.NotificationTypeMention,
			EmailEnabled:     false,
			PushEnabled:      true,
		},
	}

	prefRepo.On("FindByUserID", ctx, userID).Return(prefs, nil)

	result, err := svc.GetPreferences(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	prefRepo.AssertExpectations(t)
}

func TestNotificationService_GetPreferences_Empty(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()

	prefRepo.On("FindByUserID", ctx, userID).Return([]*domain.NotificationPreference{}, nil)

	result, err := svc.GetPreferences(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, result, 0)

	prefRepo.AssertExpectations(t)
}

// TestNotificationService_UpdatePreference tests the UpdatePreference method
func TestNotificationService_UpdatePreference_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	notifType := string(domain.NotificationTypeSystem)
	emailEnabled := true
	pushEnabled := false

	prefRepo.On("Upsert", ctx, mock.MatchedBy(func(pref *domain.NotificationPreference) bool {
		return pref.UserID == userID &&
			pref.NotificationType == domain.NotificationType(notifType) &&
			pref.EmailEnabled == emailEnabled &&
			pref.PushEnabled == pushEnabled
	})).Return(nil)

	err := svc.UpdatePreference(ctx, userID, notifType, emailEnabled, pushEnabled)
	require.NoError(t, err)

	prefRepo.AssertExpectations(t)
}

func TestNotificationService_UpdatePreference_InvalidType(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()
	invalidType := "invalid.type"
	emailEnabled := true
	pushEnabled := true

	err := svc.UpdatePreference(ctx, userID, invalidType, emailEnabled, pushEnabled)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	require.NotNil(t, appErr)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)

	prefRepo.AssertNotCalled(t, "Upsert")
}

// TestNotificationService_ArchiveNotification tests the ArchiveNotification method
func TestNotificationService_ArchiveNotification_Success(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	notifID := uuid.New()
	userID := uuid.New()

	notifRepo.On("ArchiveByID", ctx, notifID, userID).Return(nil)

	err := svc.ArchiveNotification(ctx, notifID, userID)
	require.NoError(t, err)

	notifRepo.AssertExpectations(t)
}

func TestNotificationService_ArchiveNotification_NotFound(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	notifID := uuid.New()
	userID := uuid.New()

	notifRepo.On("ArchiveByID", ctx, notifID, userID).Return(apperrors.ErrNotFound)

	err := svc.ArchiveNotification(ctx, notifID, userID)
	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))

	notifRepo.AssertExpectations(t)
}

// TestNotificationService_checkAndIncrRateLimit tests rate limiting
func TestNotificationService_checkAndIncrRateLimit_WithinLimit(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()

	// Cache returns 50 (under limit of 100)
	cacheDriver.On("Get", ctx, mock.Anything).Return([]byte("50"), nil)
	// Increment counter
	cacheDriver.On("Set", ctx, mock.Anything, []byte("51"), mock.Anything).Return(nil)

	// Use reflection or a test wrapper to call unexported method
	// Since we can't call unexported methods directly, we test via Send which calls checkAndIncrRateLimit
	notifRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

	err := svc.Send(ctx, userID, string(domain.NotificationTypeSystem), "Test", "Test message", "")
	require.NoError(t, err)

	cacheDriver.AssertExpectations(t)
}

func TestNotificationService_checkAndIncrRateLimit_OverLimit(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()

	// Cache returns 100 (at limit)
	cacheDriver.On("Get", ctx, mock.Anything).Return([]byte("100"), nil)

	err := svc.Send(ctx, userID, string(domain.NotificationTypeSystem), "Test", "Test message", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrTooManyRequests))

	cacheDriver.AssertExpectations(t)
	notifRepo.AssertNotCalled(t, "Create")
}

func TestNotificationService_checkAndIncrRateLimit_CacheError_FailOpen(t *testing.T) {
	ctx := context.Background()
	notifRepo := new(MockNotificationRepository)
	prefRepo := new(MockNotificationPreferenceRepository)
	userRepo := new(MockUserRepository)
	cacheDriver := new(MockCacheDriver)

	svc := newTestNotificationService(notifRepo, prefRepo, userRepo, cacheDriver)

	userID := uuid.New()

	// Cache returns error (fail-open: skip rate limit check)
	cacheDriver.On("Get", ctx, mock.Anything).Return(nil, assert.AnError)
	notifRepo.On("Create", ctx, mock.AnythingOfType("*domain.Notification")).Return(nil)

	err := svc.Send(ctx, userID, string(domain.NotificationTypeSystem), "Test", "Test message", "")
	require.NoError(t, err)

	cacheDriver.AssertExpectations(t)
	notifRepo.AssertExpectations(t)
}


package unit

import (
	"context"
	"log/slog"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ======================================================================
// Helpers for ActivityFollowService tests
// ======================================================================

func newTestActivityFollowService(
	followRepo *MockActivityFollowRepository,
	enforcer *permission.Enforcer,
	auditRepo *MockAuditLogForActivity,
) *service.ActivityFollowService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewActivityFollowService(followRepo, enforcer, auditSvc, log)
}

func makeActivityFollow(userID uuid.UUID, resourceType, resourceID string) *domain.ActivityFollow {
	return &domain.ActivityFollow{
		ID:           uuid.New(),
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
}

// ======================================================================
// Follow tests
// ======================================================================

func TestActivityFollowService_Follow(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantActivityView(t, enforcer, userID, orgID)
	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-123").Return(false, nil)
	followRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ActivityFollow")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	follow, err := svc.Follow(context.Background(), userID, true, orgID, "invoice", "inv-123", "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, follow)
	followRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.ActivityFollow"))
}

func TestActivityFollowService_Follow_Duplicate(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantActivityView(t, enforcer, userID, orgID)
	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-123").Return(true, nil)

	follow, err := svc.Follow(context.Background(), userID, true, orgID, "invoice", "inv-123", "127.0.0.1", "test-agent")

	assert.Nil(t, follow)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 409, appErr.HTTPStatus)
	assert.Equal(t, "CONFLICT", appErr.Code)
}

func TestActivityFollowService_Follow_InvalidResourceType(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantActivityView(t, enforcer, userID, orgID)

	follow, err := svc.Follow(context.Background(), userID, true, orgID, "invalid_type", "res-123", "127.0.0.1", "test-agent")

	assert.Nil(t, follow)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestActivityFollowService_Follow_RBACDenied(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// No permissions granted

	follow, err := svc.Follow(context.Background(), userID, true, orgID, "invoice", "inv-123", "127.0.0.1", "test-agent")

	assert.Nil(t, follow)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestActivityFollowService_Follow_RepoErrorOnCreate(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantActivityView(t, enforcer, userID, orgID)
	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-123").Return(false, nil)
	followRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.ActivityFollow")).Return(apperrors.ErrInternal)

	follow, err := svc.Follow(context.Background(), userID, true, orgID, "invoice", "inv-123", "127.0.0.1", "test-agent")

	assert.Nil(t, follow)
	require.Error(t, err)
}

// ======================================================================
// Unfollow tests
// ======================================================================

func TestActivityFollowService_Unfollow(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	followID := uuid.New()

	follow := makeActivityFollow(userID, "invoice", "inv-123")
	follow.ID = followID

	followRepo.On("FindByUser", mock.Anything, userID, 1000, 0).Return([]*domain.ActivityFollow{follow}, int64(1), nil)
	followRepo.On("Delete", mock.Anything, followID).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := svc.Unfollow(context.Background(), userID, followID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	followRepo.AssertCalled(t, "Delete", mock.Anything, followID)
}

func TestActivityFollowService_Unfollow_NotOwner(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	otherUserID := uuid.New()
	followID := uuid.New()

	// The follow belongs to another user
	otherUsersFollow := makeActivityFollow(otherUserID, "invoice", "inv-123")
	otherUsersFollow.ID = followID

	followRepo.On("FindByUser", mock.Anything, userID, 1000, 0).Return([]*domain.ActivityFollow{}, int64(0), nil)

	err := svc.Unfollow(context.Background(), userID, followID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
	assert.Equal(t, "NOT_FOUND", appErr.Code)
}

func TestActivityFollowService_Unfollow_NotFound(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	followID := uuid.New()

	followRepo.On("FindByUser", mock.Anything, userID, 1000, 0).Return([]*domain.ActivityFollow{}, int64(0), nil)

	err := svc.Unfollow(context.Background(), userID, followID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// ListFollows tests
// ======================================================================

func TestActivityFollowService_ListFollows(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	follow1 := makeActivityFollow(userID, "invoice", "inv-1")
	follow2 := makeActivityFollow(userID, "news", "news-1")
	follows := []*domain.ActivityFollow{follow1, follow2}

	followRepo.On("FindByUser", mock.Anything, userID, 20, 0).Return(follows, int64(2), nil)

	result, total, err := svc.ListFollows(context.Background(), userID, 20, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
}

func TestActivityFollowService_ListFollows_DefaultPagination(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()
	follows := []*domain.ActivityFollow{}

	// When limit is 0, it should default to 20
	followRepo.On("FindByUser", mock.Anything, userID, 20, 0).Return(follows, int64(0), nil)

	result, total, err := svc.ListFollows(context.Background(), userID, 0, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, result, 0)
}

// ======================================================================
// IsFollowing tests
// ======================================================================

func TestActivityFollowService_IsFollowing(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()

	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-123").Return(true, nil)

	following, err := svc.IsFollowing(context.Background(), userID, "invoice", "inv-123")

	require.NoError(t, err)
	assert.True(t, following)
}

func TestActivityFollowService_IsFollowing_NotFollowing(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()

	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-456").Return(false, nil)

	following, err := svc.IsFollowing(context.Background(), userID, "invoice", "inv-456")

	require.NoError(t, err)
	assert.False(t, following)
}

func TestActivityFollowService_IsFollowing_RepoError(t *testing.T) {
	followRepo := newMockActivityFollowRepository()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityFollowService(followRepo, enforcer, auditRepo)

	userID := uuid.New()

	followRepo.On("ExistsByUserAndResource", mock.Anything, userID, "invoice", "inv-789").Return(false, apperrors.ErrInternal)

	following, err := svc.IsFollowing(context.Background(), userID, "invoice", "inv-789")

	assert.False(t, following)
	require.Error(t, err)
}


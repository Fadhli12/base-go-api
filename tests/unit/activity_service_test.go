package unit

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// ======================================================================
// Mock implementations
// ======================================================================

// MockActivityRepository implements repository.ActivityRepository
type MockActivityRepository struct {
	mock.Mock
	mu         sync.Mutex
	activities map[uuid.UUID]*domain.Activity
}

func newMockActivityRepository() *MockActivityRepository {
	return &MockActivityRepository{
		activities: make(map[uuid.UUID]*domain.Activity),
	}
}

func (m *MockActivityRepository) Create(ctx context.Context, activity *domain.Activity) error {
	args := m.Called(ctx, activity)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.activities[activity.ID] = activity
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockActivityRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Activity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Activity), args.Error(1)
}

func (m *MockActivityRepository) FindByOrganization(ctx context.Context, hasOrgID bool, orgID uuid.UUID, filters repository.ActivityFilters, limit, offset int) ([]*domain.Activity, int64, error) {
	args := m.Called(ctx, hasOrgID, orgID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Activity), args.Get(1).(int64), args.Error(2)
}

func (m *MockActivityRepository) FindByResource(ctx context.Context, resourceType, resourceID string, hasOrgID bool, orgID uuid.UUID, limit, offset int) ([]*domain.Activity, int64, error) {
	args := m.Called(ctx, resourceType, resourceID, hasOrgID, orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Activity), args.Get(1).(int64), args.Error(2)
}

func (m *MockActivityRepository) ArchiveOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	args := m.Called(ctx, retentionDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockActivityRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.activities, id)
		m.mu.Unlock()
	}
	return args.Error(0)
}

// MockActivityReadRepository implements repository.ActivityReadRepository
type MockActivityReadRepository struct {
	mock.Mock
}

func newMockActivityReadRepository() *MockActivityReadRepository {
	return &MockActivityReadRepository{}
}

func (m *MockActivityReadRepository) Upsert(ctx context.Context, read *domain.ActivityRead) error {
	args := m.Called(ctx, read)
	return args.Error(0)
}

func (m *MockActivityReadRepository) FindByUserAndActivityIDs(ctx context.Context, userID uuid.UUID, activityIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	args := m.Called(ctx, userID, activityIDs)
	if args.Get(0) == nil {
		return make(map[uuid.UUID]bool), args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]bool), args.Error(1)
}

func (m *MockActivityReadRepository) DeleteByUserAndActivity(ctx context.Context, userID, activityID uuid.UUID) error {
	args := m.Called(ctx, userID, activityID)
	return args.Error(0)
}

func (m *MockActivityReadRepository) CountUnreadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID, filters repository.ActivityFilters) (int64, error) {
	args := m.Called(ctx, userID, hasOrgID, orgID, filters)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockActivityReadRepository) MarkAllReadByUser(ctx context.Context, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID, hasOrgID, orgID)
	return args.Get(0).(int64), args.Error(1)
}

// MockActivityFollowRepository implements repository.ActivityFollowRepository
type MockActivityFollowRepository struct {
	mock.Mock
}

func newMockActivityFollowRepository() *MockActivityFollowRepository {
	return &MockActivityFollowRepository{}
}

func (m *MockActivityFollowRepository) Create(ctx context.Context, follow *domain.ActivityFollow) error {
	args := m.Called(ctx, follow)
	return args.Error(0)
}

func (m *MockActivityFollowRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockActivityFollowRepository) FindByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ActivityFollow, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.ActivityFollow), args.Get(1).(int64), args.Error(2)
}

func (m *MockActivityFollowRepository) FindByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (*domain.ActivityFollow, error) {
	args := m.Called(ctx, userID, resourceType, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ActivityFollow), args.Error(1)
}

func (m *MockActivityFollowRepository) FindByUserAndResourceIDs(ctx context.Context, userID uuid.UUID, resources []repository.FollowResource) (map[string]bool, error) {
	args := m.Called(ctx, userID, resources)
	if args.Get(0) == nil {
		return make(map[string]bool), args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockActivityFollowRepository) ExistsByUserAndResource(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) (bool, error) {
	args := m.Called(ctx, userID, resourceType, resourceID)
	return args.Bool(0), args.Error(1)
}

// MockUserRepositoryForActivity implements repository.UserRepository
type MockUserRepositoryForActivity struct {
	mock.Mock
}

func newMockUserRepositoryForActivity() *MockUserRepositoryForActivity {
	return &MockUserRepositoryForActivity{}
}

func (m *MockUserRepositoryForActivity) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForActivity) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForActivity) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForActivity) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

func (m *MockUserRepositoryForActivity) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForActivity) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockAuditLogForActivity implements repository.AuditLogRepository
type MockAuditLogForActivity struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func newMockAuditLogForActivity() *MockAuditLogForActivity {
	return &MockAuditLogForActivity{}
}

func (m *MockAuditLogForActivity) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *MockAuditLogForActivity) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForActivity) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForActivity) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForActivity) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// ======================================================================
// Helpers
// ======================================================================

func newTestActivityEnforcer(t *testing.T) *permission.Enforcer {
	t.Helper()
	enforcer, err := permission.NewTestEnforcer()
	require.NoError(t, err)
	return enforcer
}

func grantActivityView(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("viewer", domain, "activity", "view"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "viewer", domain))
}

func grantActivityManage(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("manager", domain, "activity", "manage"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "manager", domain))
}

func newTestActivityService(
	activityRepo *MockActivityRepository,
	readRepo *MockActivityReadRepository,
	followRepo *MockActivityFollowRepository,
	userRepo *MockUserRepositoryForActivity,
	enforcer *permission.Enforcer,
	auditRepo *MockAuditLogForActivity,
) *service.ActivityService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditSvc, log)
}

func makeActivity(actorID, orgID uuid.UUID, actionType, resourceType, resourceID string) *domain.Activity {
	return &domain.Activity{
		ID:             uuid.New(),
		ActorID:        actorID,
		ActionType:     actionType,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		OrganizationID: &orgID,
		Metadata:       datatypes.JSON(`{"description":"test"}`),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func makeUserForActivity(id uuid.UUID, email string) *domain.User {
	return &domain.User{
		ID:    id,
		Email: email,
	}
}

// ======================================================================
// ListByOrganization tests
// ======================================================================

func TestActivityService_ListByOrganization(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	actorID := uuid.New()

	activity := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
	activities := []*domain.Activity{activity}
	total := int64(1)

	grantActivityView(t, enforcer, userID, orgID)

	activityRepo.On("FindByOrganization", mock.Anything, true, orgID, mock.AnythingOfType("repository.ActivityFilters"), 20, 0).Return(activities, total, nil)
	userRepo.On("FindByID", mock.Anything, actorID).Return(makeUserForActivity(actorID, "actor@test.com"), nil)
	readRepo.On("FindByUserAndActivityIDs", mock.Anything, userID, mock.AnythingOfType("[]uuid.UUID")).Return(map[uuid.UUID]bool{activity.ID: true}, nil)
	followRepo.On("FindByUserAndResourceIDs", mock.Anything, userID, mock.AnythingOfType("[]repository.FollowResource")).Return(map[string]bool{"user:" + activity.ResourceID: true}, nil)
	readRepo.On("CountUnreadByUser", mock.Anything, userID, true, orgID, mock.AnythingOfType("repository.ActivityFilters")).Return(int64(0), nil)

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, repository.ActivityFilters{}, 20, 0)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, total, resp.Total)
	assert.Len(t, resp.Activities, 1)
	assert.Equal(t, "actor@test.com", resp.Activities[0].ActorName)
	assert.True(t, resp.Activities[0].IsRead)
	assert.True(t, resp.Activities[0].IsFollowing)
}

func TestActivityService_ListByOrganization_BatchReadStatus(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	actorID := uuid.New()

	activity1 := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceUser, "res1")
	activity2 := makeActivity(actorID, orgID, domain.ActivityActionUpdated, domain.ActivityResourceInvoice, "res2")
	activities := []*domain.Activity{activity1, activity2}

	grantActivityView(t, enforcer, userID, orgID)

	// Only activity1 is read
	readMap := map[uuid.UUID]bool{activity1.ID: true}

	activityRepo.On("FindByOrganization", mock.Anything, true, orgID, mock.AnythingOfType("repository.ActivityFilters"), 20, 0).Return(activities, int64(2), nil)
	userRepo.On("FindByID", mock.Anything, actorID).Return(makeUserForActivity(actorID, "actor@test.com"), nil)
	readRepo.On("FindByUserAndActivityIDs", mock.Anything, userID, mock.AnythingOfType("[]uuid.UUID")).Return(readMap, nil)
	followRepo.On("FindByUserAndResourceIDs", mock.Anything, userID, mock.AnythingOfType("[]repository.FollowResource")).Return(map[string]bool{}, nil)
	readRepo.On("CountUnreadByUser", mock.Anything, userID, true, orgID, mock.AnythingOfType("repository.ActivityFilters")).Return(int64(1), nil)

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, repository.ActivityFilters{}, 20, 0)

	require.NoError(t, err)
	assert.Len(t, resp.Activities, 2)
	// activity1 is read
	assert.True(t, resp.Activities[0].IsRead)
	// activity2 is NOT read
	assert.False(t, resp.Activities[1].IsRead)
}

func TestActivityService_ListByOrganization_BatchFollowStatus(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	actorID := uuid.New()

	activity := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceNews, "news-123")

	grantActivityView(t, enforcer, userID, orgID)

	followMap := map[string]bool{"news:news-123": true}

	activityRepo.On("FindByOrganization", mock.Anything, true, orgID, mock.AnythingOfType("repository.ActivityFilters"), 20, 0).Return([]*domain.Activity{activity}, int64(1), nil)
	userRepo.On("FindByID", mock.Anything, actorID).Return(makeUserForActivity(actorID, "actor@test.com"), nil)
	readRepo.On("FindByUserAndActivityIDs", mock.Anything, userID, mock.AnythingOfType("[]uuid.UUID")).Return(map[uuid.UUID]bool{}, nil)
	followRepo.On("FindByUserAndResourceIDs", mock.Anything, userID, mock.AnythingOfType("[]repository.FollowResource")).Return(followMap, nil)
	readRepo.On("CountUnreadByUser", mock.Anything, userID, true, orgID, mock.AnythingOfType("repository.ActivityFilters")).Return(int64(1), nil)

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, repository.ActivityFilters{}, 20, 0)

	require.NoError(t, err)
	assert.True(t, resp.Activities[0].IsFollowing)
}

func TestActivityService_ListByOrganization_RBACDenied(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	// No permissions granted

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, repository.ActivityFilters{}, 20, 0)

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

func TestActivityService_ListByOrganization_EmptyResult(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantActivityView(t, enforcer, userID, orgID)

	activityRepo.On("FindByOrganization", mock.Anything, true, orgID, mock.AnythingOfType("repository.ActivityFilters"), 20, 0).Return([]*domain.Activity{}, int64(0), nil)

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, repository.ActivityFilters{}, 20, 0)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Len(t, resp.Activities, 0)
}

func TestActivityService_ListByOrganization_UnreadOnly(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	actorID := uuid.New()

	activity := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceUser, "res1")
	activities := []*domain.Activity{activity}

	grantActivityView(t, enforcer, userID, orgID)

	unreadFilters := repository.ActivityFilters{UnreadOnly: true}

	activityRepo.On("FindByOrganization", mock.Anything, true, orgID, unreadFilters, 20, 0).Return(activities, int64(5), nil)
	userRepo.On("FindByID", mock.Anything, actorID).Return(makeUserForActivity(actorID, "actor@test.com"), nil)
	readRepo.On("FindByUserAndActivityIDs", mock.Anything, userID, mock.AnythingOfType("[]uuid.UUID")).Return(map[uuid.UUID]bool{}, nil)
	followRepo.On("FindByUserAndResourceIDs", mock.Anything, userID, mock.AnythingOfType("[]repository.FollowResource")).Return(map[string]bool{}, nil)

	resp, err := svc.ListByOrganization(context.Background(), userID, true, orgID, unreadFilters, 20, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.UnreadCount)
	// When UnreadOnly, total IS the unread count
	assert.Equal(t, int64(5), resp.Total)
}

// ======================================================================
// FindByID tests
// ======================================================================

func TestActivityService_FindByID(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	actorID := uuid.New()
	orgID := uuid.New()
	activityID := uuid.New()

	activity := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceUser, "res1")
	activity.ID = activityID

	activityRepo.On("FindByID", mock.Anything, activityID).Return(activity, nil)

	result, err := svc.FindByID(context.Background(), activityID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, activityID, result.ID)
	assert.Equal(t, domain.ActivityActionCreated, result.ActionType)
}

func TestActivityService_FindByID_NotFound(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	activityID := uuid.New()

	activityRepo.On("FindByID", mock.Anything, activityID).Return(nil, apperrors.ErrNotFound)

	result, err := svc.FindByID(context.Background(), activityID)

	assert.Nil(t, result)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// CountUnread tests
// ======================================================================

func TestActivityService_CountUnread(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	readRepo.On("CountUnreadByUser", mock.Anything, userID, true, orgID, mock.AnythingOfType("repository.ActivityFilters")).Return(int64(7), nil)

	count, err := svc.CountUnread(context.Background(), userID, true, orgID, repository.ActivityFilters{})

	require.NoError(t, err)
	assert.Equal(t, int64(7), count)
}

// ======================================================================
// MarkAllRead tests
// ======================================================================

func TestActivityService_MarkAllRead(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	readRepo.On("MarkAllReadByUser", mock.Anything, userID, true, orgID).Return(int64(5), nil)

	count, err := svc.MarkAllRead(context.Background(), userID, true, orgID)

	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

// ======================================================================
// MarkAsRead tests
// ======================================================================

func TestActivityService_MarkAsRead(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	activityID := uuid.New()
	actorID := uuid.New()

	activity := makeActivity(actorID, orgID, domain.ActivityActionCreated, domain.ActivityResourceUser, "res1")
	activity.ID = activityID

	activityRepo.On("FindByID", mock.Anything, activityID).Return(activity, nil)
	readRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.ActivityRead")).Return(nil)

	err := svc.MarkAsRead(context.Background(), userID, activityID)

	require.NoError(t, err)
	readRepo.AssertCalled(t, "Upsert", mock.Anything, mock.AnythingOfType("*domain.ActivityRead"))
}

func TestActivityService_MarkAsRead_ActivityNotFound(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	activityID := uuid.New()

	activityRepo.On("FindByID", mock.Anything, activityID).Return(nil, apperrors.ErrNotFound)

	err := svc.MarkAsRead(context.Background(), userID, activityID)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// SoftDelete tests
// ======================================================================

func TestActivityService_SoftDelete(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	activityID := uuid.New()

	grantActivityManage(t, enforcer, userID, orgID)
	activityRepo.On("SoftDelete", mock.Anything, activityID).Return(nil)

	err := svc.SoftDelete(context.Background(), userID, true, orgID, activityID)

	require.NoError(t, err)
	activityRepo.AssertCalled(t, "SoftDelete", mock.Anything, activityID)
}

func TestActivityService_SoftDelete_Forbidden(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	activityID := uuid.New()

	// No manage permission granted — only view
	grantActivityView(t, enforcer, userID, orgID)

	err := svc.SoftDelete(context.Background(), userID, true, orgID, activityID)

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// ======================================================================
// SubscribeToEventBus tests
// ======================================================================

func TestActivityService_SubscribeToEventBus(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	eventBus := domain.NewEventBus(256)

	userID := uuid.New()
	orgID := uuid.New()

	activityRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Activity")).Return(nil)

	svc.SubscribeToEventBus(eventBus)
	eventBus.Start(context.Background())
	defer eventBus.Stop()

	// Publish an event
	eventBus.Publish(domain.WebhookEvent{
		Type:    "user.created",
		OrgID:   &orgID,
		Payload: map[string]interface{}{"user_id": userID.String(), "id": userID.String()},
	})

	// Wait for the event to be processed
	time.Sleep(200 * time.Millisecond)

	activityRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Activity"))
}

// ======================================================================
// HandleEvent tests
// ======================================================================

func TestActivityService_HandleEvent(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	eventBus := domain.NewEventBus(256)

	userID := uuid.New()
	orgID := uuid.New()

	activityRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Activity")).Return(nil)

	svc.SubscribeToEventBus(eventBus)
	eventBus.Start(context.Background())
	defer eventBus.Stop()

	eventBus.Publish(domain.WebhookEvent{
		Type:    "user.created",
		OrgID:   &orgID,
		Payload: map[string]interface{}{"user_id": userID.String(), "id": userID.String()},
	})

	// Wait for async processing
	time.Sleep(300 * time.Millisecond)

	activityRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Activity"))
}

func TestActivityService_HandleEvent_UnknownEvent(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	eventBus := domain.NewEventBus(256)

	orgID := uuid.New()

	svc.SubscribeToEventBus(eventBus)
	eventBus.Start(context.Background())
	defer eventBus.Stop()

	// Unknown events should be logged but not create activities
	eventBus.Publish(domain.WebhookEvent{
		Type:    "order.shipped",
		OrgID:   &orgID,
		Payload: map[string]interface{}{},
	})

	// Wait for async processing
	time.Sleep(300 * time.Millisecond)

	activityRepo.AssertNotCalled(t, "Create")
}

func TestActivityService_HandleEvent_MetadataConstruction(t *testing.T) {
	activityRepo := newMockActivityRepository()
	readRepo := newMockActivityReadRepository()
	followRepo := newMockActivityFollowRepository()
	userRepo := newMockUserRepositoryForActivity()
	enforcer := newTestActivityEnforcer(t)
	auditRepo := newMockAuditLogForActivity()
	svc := newTestActivityService(activityRepo, readRepo, followRepo, userRepo, enforcer, auditRepo)

	eventBus := domain.NewEventBus(256)

	userID := uuid.New()
	orgID := uuid.New()
	invoiceID := uuid.New()

	var mu sync.Mutex
	var capturedActivity *domain.Activity
	activityRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Activity")).Run(func(args mock.Arguments) {
		mu.Lock()
		capturedActivity = args.Get(1).(*domain.Activity)
		mu.Unlock()
	}).Return(nil)

	svc.SubscribeToEventBus(eventBus)
	eventBus.Start(context.Background())
	defer eventBus.Stop()

	eventBus.Publish(domain.WebhookEvent{
		Type:  "invoice.created",
		OrgID: &orgID,
		Payload: map[string]interface{}{
			"user_id":     userID.String(),
			"id":          invoiceID.String(),
			"amount":      99.99,
			"description": "Test invoice",
		},
	})

	// Wait for async processing
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return capturedActivity != nil
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, domain.ActivityActionCreated, capturedActivity.ActionType)
	assert.Equal(t, domain.ActivityResourceInvoice, capturedActivity.ResourceType)
	assert.Equal(t, invoiceID.String(), capturedActivity.ResourceID)
	assert.Equal(t, userID, capturedActivity.ActorID)
	assert.NotNil(t, capturedActivity.Metadata)
}


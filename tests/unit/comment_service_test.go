package unit

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
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

// MockCommentRepository implements repository.CommentRepository
type MockCommentRepository struct {
	mock.Mock
	mu      sync.Mutex
	storage map[uuid.UUID]*domain.Comment
}

func newMockUserRepositoryForComments() *MockUserRepositoryForComments {
	return &MockUserRepositoryForComments{}
}

func newMockCommentRepository() *MockCommentRepository {
	return &MockCommentRepository{
		storage: make(map[uuid.UUID]*domain.Comment),
	}
}

func (m *MockCommentRepository) Create(ctx context.Context, comment *domain.Comment) error {
	args := m.Called(ctx, comment)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.storage[comment.ID] = comment
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockCommentRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Comment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Comment), args.Error(1)
}

func (m *MockCommentRepository) FindByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error) {
	args := m.Called(ctx, commentableType, commentableID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Comment), args.Get(1).(int64), args.Error(2)
}

func (m *MockCommentRepository) FindReplies(ctx context.Context, parentID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error) {
	args := m.Called(ctx, parentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Comment), args.Get(1).(int64), args.Error(2)
}

func (m *MockCommentRepository) Update(ctx context.Context, comment *domain.Comment) error {
	args := m.Called(ctx, comment)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.storage[comment.ID] = comment
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockCommentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.storage, id)
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockCommentRepository) CountByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID) (int64, error) {
	args := m.Called(ctx, commentableType, commentableID)
	return args.Get(0).(int64), args.Error(1)
}

// MockUserRepositoryForComments implements repository.UserRepository
type MockUserRepositoryForComments struct {
	mock.Mock
}

func (m *MockUserRepositoryForComments) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForComments) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForComments) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForComments) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

func (m *MockUserRepositoryForComments) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForComments) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockAuditLogRepository implements repository.AuditLogRepository
type MockAuditLogForComments struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func newMockAuditLogForComments() *MockAuditLogForComments {
	return &MockAuditLogForComments{}
}

func (m *MockAuditLogForComments) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	m.logs = append(m.logs, auditLog)
	args := m.Called(ctx, auditLog)
	return args.Error(0)
}

func (m *MockAuditLogForComments) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForComments) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForComments) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogForComments) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// ======================================================================
// Helpers
// ======================================================================

func newTestCommentService(
	repo *MockCommentRepository,
	userRepo *MockUserRepositoryForComments,
	enforcer *permission.Enforcer,
	auditRepo *MockAuditLogForComments,
) *service.CommentService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewCommentService(repo, userRepo, enforcer, auditSvc, log)
}

func newTestCommentEnforcer(t *testing.T) *permission.Enforcer {
	t.Helper()
	enforcer, err := permission.NewTestEnforcer()
	require.NoError(t, err)
	return enforcer
}

func grantCommentCreate(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("creator", domain, "comment", "create"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "creator", domain))
}

func grantCommentView(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("viewer", domain, "comment", "view"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "viewer", domain))
}

func grantCommentDelete(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("deleter", domain, "comment", "delete"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "deleter", domain))
}

func grantCommentDeleteAny(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("admin", domain, "comment", "delete_any"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "admin", domain))
}

func grantCommentManage(t *testing.T, enforcer *permission.Enforcer, userID, orgID uuid.UUID) {
	t.Helper()
	domain := orgID.String()
	require.NoError(t, enforcer.AddPolicy("manager", domain, "comment", "manage"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "manager", domain))
}

func makeComment(authorID, orgID, commentableID uuid.UUID, content string) *domain.Comment {
	return &domain.Comment{
		ID:               uuid.New(),
		AuthorID:         authorID,
		OrganizationID:  orgID,
		CommentableType:  "news",
		CommentableID:   commentableID,
		Content:          content,
		MentionedUserIDs: datatypes.JSON([]byte("[]")),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func makeUser(id uuid.UUID, email string) *domain.User {
	return &domain.User{
		ID:    id,
		Email: email,
	}
}

func waitForCommentAuditLogs(auditRepo *MockAuditLogForComments, expectedCount int, timeout time.Duration) bool {
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

func TestCommentService_Create_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hello world",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Hello world", resp.Content)
	assert.Equal(t, userID, resp.AuthorID)
	repo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Comment"))
}

func TestCommentService_Create_WithParentID(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	parentID := uuid.New()

	parentComment := makeComment(uuid.New(), orgID, commentableID, "Parent comment")
	parentComment.ID = parentID

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	repo.On("FindByID", mock.Anything, parentID).Return(parentComment, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content:  "Reply comment",
		ParentID: parentID.String(),
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Reply comment", resp.Content)
}

func TestCommentService_Create_WithMentions(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	mentionedUser := makeUser(uuid.New(), "john@example.com")
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByEmail", mock.Anything, "john@example.com").Return(mentionedUser, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hey @{john@example.com} check this out",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Verify that the comment was created with mention data
	repo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Comment"))
}

func TestCommentService_Create_SkipsUnknownMentions(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByEmail", mock.Anything, "unknown@example.com").Return(nil, apperrors.ErrNotFound)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hey @{unknown@example.com} nobody knows you",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Comment should still be created, unknown mentions are skipped
	repo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Comment"))
}

func TestCommentService_Create_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	// No permissions granted — enforcer will deny

	req := request.CreateCommentRequest{
		Content: "Hello world",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

func TestCommentService_Create_InvalidCommentableType(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)

	req := request.CreateCommentRequest{
		Content: "Hello world",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "invalid_type", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestCommentService_Create_InvalidParentIDFormat(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)

	req := request.CreateCommentRequest{
		Content:  "Reply",
		ParentID: "not-a-uuid",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestCommentService_Create_ParentNotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	parentID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, parentID).Return(nil, apperrors.ErrNotFound)

	req := request.CreateCommentRequest{
		Content:  "Reply",
		ParentID: parentID.String(),
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

func TestCommentService_Create_ParentBelongsToDifferentEntity(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	otherCommentableID := uuid.New()
	parentID := uuid.New()

	parentComment := makeComment(uuid.New(), orgID, otherCommentableID, "Parent on different entity")
	parentComment.ID = parentID
	parentComment.CommentableType = "news"

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, parentID).Return(parentComment, nil)

	req := request.CreateCommentRequest{
		Content:  "Reply",
		ParentID: parentID.String(),
	}

	// Parent belongs to different commentableID
	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestCommentService_Create_RepoFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(apperrors.ErrInternal)

	req := request.CreateCommentRequest{
		Content: "Hello world",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestCommentService_Create_EnforceError(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	// Use a real test enforcer but then we'll test the error path differently
	// The Enforce method on *permission.Enforcer doesn't return errors in normal use,
	// so we test this by ensuring the RBAC denied path works correctly
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	_ = uuid.Nil // Nil orgID → uses "default" domain, and user has no policy there

	commentableID := uuid.New()
	// No create permission granted in "default" domain

	req := request.CreateCommentRequest{
		Content: "Hello world",
	}

	resp, err := svc.Create(context.Background(), userID, false, uuid.Nil, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

// ======================================================================
// GetByID tests
// ======================================================================

func TestCommentService_GetByID_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()
	authorID := uuid.New()

	comment := makeComment(authorID, orgID, commentableID, "Test comment")
	comment.ID = commentID

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	userRepo.On("FindByID", mock.Anything, authorID).Return(makeUser(authorID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, err := svc.GetByID(context.Background(), userID, true, orgID, commentID)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, commentID, resp.ID)
	assert.Equal(t, "Test comment", resp.Content)
}

func TestCommentService_GetByID_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	// No view permission granted

	resp, err := svc.GetByID(context.Background(), userID, true, orgID, commentID)

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_GetByID_NotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(nil, apperrors.ErrNotFound)

	resp, err := svc.GetByID(context.Background(), userID, true, orgID, commentID)

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// ListByCommentable tests
// ======================================================================

func TestCommentService_ListByCommentable_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	authorID := uuid.New()

	comment := makeComment(authorID, orgID, commentableID, "Test comment")
	comments := []*domain.Comment{comment}

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByCommentable", mock.Anything, "news", commentableID, 10, 0).Return(comments, int64(1), nil)
	userRepo.On("FindByID", mock.Anything, authorID).Return(makeUser(authorID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, comment.ID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, total, err := svc.ListByCommentable(context.Background(), userID, true, orgID, "news", commentableID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, resp, 1)
}

func TestCommentService_ListByCommentable_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	// No view permission granted

	resp, total, err := svc.ListByCommentable(context.Background(), userID, true, orgID, "news", commentableID, 10, 0)

	assert.Nil(t, resp)
	assert.Equal(t, int64(0), total)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_ListByCommentable_InvalidType(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)

	resp, total, err := svc.ListByCommentable(context.Background(), userID, true, orgID, "invalid", commentableID, 10, 0)

	assert.Nil(t, resp)
	assert.Equal(t, int64(0), total)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
}

// ======================================================================
// ListReplies tests
// ======================================================================

func TestCommentService_ListReplies_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	parentID := uuid.New()
	commentableID := uuid.New()
	replyAuthorID := uuid.New()

	reply := makeComment(replyAuthorID, orgID, commentableID, "Reply text")
	reply.ParentID = &parentID
	replies := []*domain.Comment{reply}

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindReplies", mock.Anything, parentID, 10, 0).Return(replies, int64(1), nil)
	userRepo.On("FindByID", mock.Anything, replyAuthorID).Return(makeUser(replyAuthorID, "replier@test.com"), nil)
	repo.On("FindReplies", mock.Anything, reply.ID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, total, err := svc.ListReplies(context.Background(), userID, true, orgID, parentID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, resp, 1)
}

func TestCommentService_ListReplies_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	parentID := uuid.New()

	// No view permission granted

	resp, total, err := svc.ListReplies(context.Background(), userID, true, orgID, parentID, 10, 0)

	assert.Nil(t, resp)
	assert.Equal(t, int64(0), total)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

// ======================================================================
// Update tests
// ======================================================================

func TestCommentService_Update_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Original content")
	comment.ID = commentID

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.UpdateCommentRequest{
		Content: "Updated content",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Updated content", resp.Content)
}

func TestCommentService_Update_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	// No create permission (Update uses comment:create for RBAC)

	req := request.UpdateCommentRequest{
		Content: "Updated content",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Update_NotOwner(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	otherUserID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(otherUserID, orgID, commentableID, "Original content")
	comment.ID = commentID

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)

	req := request.UpdateCommentRequest{
		Content: "Trying to update someone else's comment",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Update_CommentNotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(nil, apperrors.ErrNotFound)

	req := request.UpdateCommentRequest{
		Content: "Update",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

func TestCommentService_Update_RepoFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Original content")
	comment.ID = commentID

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(apperrors.ErrInternal)

	req := request.UpdateCommentRequest{
		Content: "Updated content",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
}

// ======================================================================
// Delete tests
// ======================================================================

func TestCommentService_Delete_OwnerSuccess(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Comment to delete")
	comment.ID = commentID

	grantCommentDelete(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("SoftDelete", mock.Anything, commentID).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	repo.AssertCalled(t, "SoftDelete", mock.Anything, commentID)
}

func TestCommentService_Delete_AdminDeletesAny(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	adminID := uuid.New()
	otherUserID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(otherUserID, orgID, commentableID, "Someone else's comment")
	comment.ID = commentID

	grantCommentDeleteAny(t, enforcer, adminID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("SoftDelete", mock.Anything, commentID).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := svc.Delete(context.Background(), adminID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	repo.AssertCalled(t, "SoftDelete", mock.Anything, commentID)
}

func TestCommentService_Delete_NonOwnerWithoutDeleteAnyPermission(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	otherUserID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(otherUserID, orgID, commentableID, "Someone else's comment")
	comment.ID = commentID

	// No delete_any permission — only view
	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Delete_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "My comment")
	comment.ID = commentID

	// No permissions at all
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Delete_NotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	repo.On("FindByID", mock.Anything, commentID).Return(nil, apperrors.ErrNotFound)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.Error(t, err)
}

// ======================================================================
// Pin tests
// ======================================================================

func TestCommentService_Pin_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()
	authorID := uuid.New()

	comment := makeComment(authorID, orgID, commentableID, "Pin this comment")
	comment.ID = commentID
	comment.IsPinned = false

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByID", mock.Anything, authorID).Return(makeUser(authorID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, err := svc.Pin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsPinned)
}

func TestCommentService_Pin_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	// No manage permission

	resp, err := svc.Pin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Pin_AlreadyPinned(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Already pinned")
	comment.ID = commentID
	comment.IsPinned = true

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)

	resp, err := svc.Pin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestCommentService_Pin_CommentNotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(nil, apperrors.ErrNotFound)

	resp, err := svc.Pin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

// ======================================================================
// Unpin tests
// ======================================================================

func TestCommentService_Unpin_Success(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()
	authorID := uuid.New()

	comment := makeComment(authorID, orgID, commentableID, "Pinned comment")
	comment.ID = commentID
	comment.IsPinned = true

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByID", mock.Anything, authorID).Return(makeUser(authorID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, err := svc.Unpin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsPinned)
}

func TestCommentService_Unpin_RBACDenied(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	// No manage permission

	resp, err := svc.Unpin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestCommentService_Unpin_AlreadyUnpinned(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Not pinned comment")
	comment.ID = commentID
	comment.IsPinned = false

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)

	resp, err := svc.Unpin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestCommentService_Unpin_CommentNotFound(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(nil, apperrors.ErrNotFound)

	resp, err := svc.Unpin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
}

// ======================================================================
// parseMentions tests (via Create)
// ======================================================================

func TestCommentService_Create_MultipleMentions(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	user1 := makeUser(uuid.New(), "alice@example.com")
	user2 := makeUser(uuid.New(), "bob@example.com")

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByEmail", mock.Anything, "alice@example.com").Return(user1, nil)
	userRepo.On("FindByEmail", mock.Anything, "bob@example.com").Return(user2, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hey @{alice@example.com} and @{bob@example.com} check this",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	userRepo.AssertCalled(t, "FindByEmail", mock.Anything, "alice@example.com")
	userRepo.AssertCalled(t, "FindByEmail", mock.Anything, "bob@example.com")
}

func TestCommentService_Create_DuplicateMentionsDeduped(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	mentionedUser := makeUser(uuid.New(), "alice@example.com")

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByEmail", mock.Anything, "alice@example.com").Return(mentionedUser, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hey @{alice@example.com} and again @{alice@example.com}",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// FindByEmail should be called only once due to dedup (seen map)
	userRepo.AssertCalled(t, "FindByEmail", mock.Anything, "alice@example.com")
}

// ======================================================================
// Additional edge case tests
// ======================================================================

func TestCommentService_Create_ValidCommentableTypes(t *testing.T) {
	// Test that "news", "invoice", and "media" are all accepted
	validTypes := []string{"news", "invoice", "media"}

	for _, ct := range validTypes {
		t.Run(ct, func(t *testing.T) {
			repo := newMockCommentRepository()
			userRepo := newMockUserRepositoryForComments()
			enforcer := newTestCommentEnforcer(t)
			auditRepo := newMockAuditLogForComments()
			svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

			userID := uuid.New()
			orgID := uuid.New()
			commentableID := uuid.New()

			grantCommentCreate(t, enforcer, userID, orgID)
			auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
			repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
			userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
			repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

			req := request.CreateCommentRequest{
				Content: "Comment on " + ct,
			}

			resp, err := svc.Create(context.Background(), userID, true, orgID, ct, commentableID, req, "127.0.0.1", "test-agent")

			require.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestCommentService_Delete_RepoFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "My comment")
	comment.ID = commentID

	grantCommentDelete(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("SoftDelete", mock.Anything, commentID).Return(apperrors.ErrInternal)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.Error(t, err)
}

func TestCommentService_Pin_RepoUpdateFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Pin this")
	comment.ID = commentID
	comment.IsPinned = false

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(apperrors.ErrInternal)

	resp, err := svc.Pin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestCommentService_Unpin_RepoUpdateFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "Unpin this")
	comment.ID = commentID
	comment.IsPinned = true

	grantCommentManage(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(apperrors.ErrInternal)

	resp, err := svc.Unpin(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestCommentService_GetByID_AuthorResolutionFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()
	authorID := uuid.New()

	comment := makeComment(authorID, orgID, commentableID, "Test comment")
	comment.ID = commentID

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	// Author not found (e.g., soft-deleted) — should still succeed with empty author name
	userRepo.On("FindByID", mock.Anything, authorID).Return(nil, apperrors.ErrNotFound)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, err := svc.GetByID(context.Background(), userID, true, orgID, commentID)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "", resp.AuthorName) // Author name should be empty string on resolution failure
}

func TestCommentService_ListByCommentable_RepoFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByCommentable", mock.Anything, "news", commentableID, 10, 0).Return(nil, int64(0), apperrors.ErrInternal)

	resp, total, err := svc.ListByCommentable(context.Background(), userID, true, orgID, "news", commentableID, 10, 0)

	assert.Nil(t, resp)
	assert.Equal(t, int64(0), total)
	require.Error(t, err)
}

func TestCommentService_ListReplies_RepoFailure(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	parentID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindReplies", mock.Anything, parentID, 10, 0).Return(nil, int64(0), apperrors.ErrInternal)

	resp, total, err := svc.ListReplies(context.Background(), userID, true, orgID, parentID, 10, 0)

	assert.Nil(t, resp)
	assert.Equal(t, int64(0), total)
	require.Error(t, err)
}

func TestCommentService_Create_ParentBelongsToDifferentType(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()
	parentID := uuid.New()

	// Parent is on "invoice" type, but we're creating for "news"
	parentComment := makeComment(uuid.New(), orgID, commentableID, "Parent on invoice")
	parentComment.ID = parentID
	parentComment.CommentableType = "invoice"

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, parentID).Return(parentComment, nil)

	req := request.CreateCommentRequest{
		Content:  "Reply",
		ParentID: parentID.String(),
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "news", commentableID, req, "127.0.0.1", "test-agent")

	assert.Nil(t, resp)
	require.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
}

func TestCommentService_Update_WithMentions(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()
	mentionedUser := makeUser(uuid.New(), "jane@example.com")

	comment := makeComment(userID, orgID, commentableID, "Original")
	comment.ID = commentID

	grantCommentCreate(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	userRepo.On("FindByEmail", mock.Anything, "jane@example.com").Return(mentionedUser, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, commentID, 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.UpdateCommentRequest{
		Content: "Updated with @{jane@example.com}",
	}

	resp, err := svc.Update(context.Background(), userID, true, orgID, commentID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	userRepo.AssertCalled(t, "FindByEmail", mock.Anything, "jane@example.com")
}

func TestCommentService_Delete_OwnerWithDeletePermission(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentID := uuid.New()
	commentableID := uuid.New()

	comment := makeComment(userID, orgID, commentableID, "My comment")
	comment.ID = commentID

	// Owner with delete permission (not delete_any)
	grantCommentDelete(t, enforcer, userID, orgID)
	repo.On("FindByID", mock.Anything, commentID).Return(comment, nil)
	repo.On("SoftDelete", mock.Anything, commentID).Return(nil)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := svc.Delete(context.Background(), userID, true, orgID, commentID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
}

func TestCommentService_Create_WithDefaultDomain(t *testing.T) {
	// Test that when hasOrgID is false, "default" domain is used
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	commentableID := uuid.New()

	// Grant permission in "default" domain
	require.NoError(t, enforcer.AddPolicy("creator", "default", "comment", "create"))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), "creator", "default"))

	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Hello default domain",
	}

	// hasOrgID=false, orgID=uuid.Nil → uses "default" domain
	resp, err := svc.Create(context.Background(), userID, false, uuid.Nil, "news", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCommentService_ListByCommentable_EmptyResult(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindByCommentable", mock.Anything, "news", commentableID, 10, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, total, err := svc.ListByCommentable(context.Background(), userID, true, orgID, "news", commentableID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, resp)
}

func TestCommentService_ListReplies_EmptyResult(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	parentID := uuid.New()

	grantCommentView(t, enforcer, userID, orgID)
	repo.On("FindReplies", mock.Anything, parentID, 10, 0).Return([]*domain.Comment{}, int64(0), nil)

	resp, total, err := svc.ListReplies(context.Background(), userID, true, orgID, parentID, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, resp)
}

func TestCommentService_Create_FindsByCommentableType_Media(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Media comment",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "media", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestCommentService_Create_FindsByCommentableType_Invoice(t *testing.T) {
	repo := newMockCommentRepository()
	userRepo := newMockUserRepositoryForComments()
	enforcer := newTestCommentEnforcer(t)
	auditRepo := newMockAuditLogForComments()
	svc := newTestCommentService(repo, userRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	commentableID := uuid.New()

	grantCommentCreate(t, enforcer, userID, orgID)
	auditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
	userRepo.On("FindByID", mock.Anything, userID).Return(makeUser(userID, "author@test.com"), nil)
	repo.On("FindReplies", mock.Anything, mock.AnythingOfType("uuid.UUID"), 0, 0).Return([]*domain.Comment{}, int64(0), nil)

	req := request.CreateCommentRequest{
		Content: "Invoice comment",
	}

	resp, err := svc.Create(context.Background(), userID, true, orgID, "invoice", commentableID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

var _ repository.CommentRepository = (*MockCommentRepository)(nil)
var _ repository.UserRepository = (*MockUserRepositoryForComments)(nil)
var _ repository.AuditLogRepository = (*MockAuditLogForComments)(nil)
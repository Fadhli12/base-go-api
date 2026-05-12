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
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type MockTagRepository struct {
	mock.Mock
	mu      sync.Mutex
	storage map[uuid.UUID]*domain.Tag
}

func newMockTagRepository() *MockTagRepository {
	return &MockTagRepository{
		storage: make(map[uuid.UUID]*domain.Tag),
	}
}

func (m *MockTagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.storage[tag.ID] = tag
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockTagRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*domain.Tag, error) {
	args := m.Called(ctx, orgID, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, sort, order string) ([]*domain.Tag, int64, error) {
	args := m.Called(ctx, orgID, limit, offset, sort, order)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.Tag), args.Get(1).(int64), args.Error(2)
}

func (m *MockTagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	if args.Error(0) == nil {
		m.mu.Lock()
		m.storage[tag.ID] = tag
		m.mu.Unlock()
	}
	return args.Error(0)
}

func (m *MockTagRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepository) IncrementUsageCount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepository) DecrementUsageCount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepository) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Tag, error) {
	args := m.Called(ctx, orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepository) Autocomplete(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]*domain.Tag, error) {
	args := m.Called(ctx, orgID, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

type MockEntityTagRepository struct {
	mock.Mock
}

func newMockEntityTagRepository() *MockEntityTagRepository {
	return &MockEntityTagRepository{}
}

func (m *MockEntityTagRepository) Create(ctx context.Context, entityTag *domain.EntityTag) error {
	args := m.Called(ctx, entityTag)
	return args.Error(0)
}

func (m *MockEntityTagRepository) FindByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.EntityTag, error) {
	args := m.Called(ctx, orgID, entityType, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.EntityTag), args.Error(1)
}

func (m *MockEntityTagRepository) FindByEntityAndTag(ctx context.Context, orgID uuid.UUID, entityType string, entityID, tagID uuid.UUID) (*domain.EntityTag, error) {
	args := m.Called(ctx, orgID, entityType, entityID, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EntityTag), args.Error(1)
}

func (m *MockEntityTagRepository) DeleteByEntityAndTag(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEntityTagRepository) ListTagDetailsByEntity(ctx context.Context, orgID uuid.UUID, entityType string, entityID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(ctx, orgID, entityType, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

type MockAuditLogForTags struct {
	mock.Mock
	mu    sync.Mutex
	logs  []*domain.AuditLog
	count int
}

func newMockAuditLogForTags() *MockAuditLogForTags {
	return &MockAuditLogForTags{}
}

func (m *MockAuditLogForTags) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, auditLog)
	m.count++
	return nil
}

func (m *MockAuditLogForTags) GetCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *MockAuditLogForTags) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func (m *MockAuditLogForTags) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func (m *MockAuditLogForTags) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	return nil, nil
}

func newTestTagEnforcer(t *testing.T) *permission.Enforcer {
	t.Helper()
	enforcer, err := permission.NewTestEnforcer()
	require.NoError(t, err)
	return enforcer
}

func grantTagPermission(t *testing.T, enforcer *permission.Enforcer, role, domain, action string, userID uuid.UUID) {
	t.Helper()
	require.NoError(t, enforcer.AddPolicy(role, domain, "tag", action))
	require.NoError(t, enforcer.AddRoleForUser(userID.String(), role, domain))
}

func newTestTagService(tagRepo *MockTagRepository, entityTagRepo *MockEntityTagRepository, enforcer *permission.Enforcer, auditRepo *MockAuditLogForTags) *service.TagService {
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})
	log := slog.Default()
	return service.NewTagService(tagRepo, entityTagRepo, enforcer, auditSvc, log)
}

func makeTag(orgID uuid.UUID, name, slug string) *domain.Tag {
	return &domain.Tag{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Color:          "#FF5733",
		UsageCount:     0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func TestTagService_Create_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantTagPermission(t, enforcer, "creator", orgID.String(), "create", userID)
	tagRepo.On("FindByName", mock.Anything, orgID, "Test Tag").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("FindBySlug", mock.Anything, orgID, "test-tag").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(nil)

	req := request.CreateTagRequest{Name: "Test Tag", Color: "#FF5733"}

	resp, err := svc.Create(context.Background(), true, orgID, userID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Test Tag", resp.Name)
	assert.Equal(t, "test-tag", resp.Slug)
	assert.Equal(t, orgID, resp.OrganizationID)
	tagRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*domain.Tag"))
}

func TestTagService_Create_RBACDenied(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	req := request.CreateTagRequest{Name: "Test Tag"}

	_, err := svc.Create(context.Background(), true, orgID, userID, req, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

func TestTagService_Create_ConflictName(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	existing := makeTag(orgID, "Test Tag", "test-tag")

	grantTagPermission(t, enforcer, "creator", orgID.String(), "create", userID)
	tagRepo.On("FindByName", mock.Anything, orgID, "Test Tag").Return(existing, nil)

	req := request.CreateTagRequest{Name: "Test Tag"}

	_, err := svc.Create(context.Background(), true, orgID, userID, req, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 409, appErr.HTTPStatus)
	assert.Equal(t, "CONFLICT", appErr.Code)
}

func TestTagService_Create_SlugConflict(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	existing := makeTag(orgID, "Other Tag", "test-tag")

	grantTagPermission(t, enforcer, "creator", orgID.String(), "create", userID)
	tagRepo.On("FindByName", mock.Anything, orgID, "Test Tag").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("FindBySlug", mock.Anything, orgID, "test-tag").Return(existing, nil)
	tagRepo.On("FindBySlug", mock.Anything, orgID, "test-tag-2").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(nil)

	req := request.CreateTagRequest{Name: "Test Tag"}

	resp, err := svc.Create(context.Background(), true, orgID, userID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-tag-2", resp.Slug)
}

func TestTagService_GetByID_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag := makeTag(orgID, "Test", "test")

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)

	resp, err := svc.GetByID(context.Background(), true, orgID, userID, tag.ID)

	require.NoError(t, err)
	assert.Equal(t, "Test", resp.Name)
	assert.Equal(t, "test", resp.Slug)
}

func TestTagService_GetByID_RBACDenied(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	_, err := svc.GetByID(context.Background(), true, orgID, userID, uuid.New())

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

func TestTagService_GetBySlug_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag := makeTag(orgID, "Test", "test")

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("FindBySlug", mock.Anything, orgID, "test").Return(tag, nil)

	resp, err := svc.GetBySlug(context.Background(), true, orgID, userID, "test")

	require.NoError(t, err)
	assert.Equal(t, "test", resp.Slug)
}

func TestTagService_List_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{makeTag(orgID, "Alpha", "alpha"), makeTag(orgID, "Beta", "beta")}

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("FindByOrg", mock.Anything, orgID, 20, 0, "name", "ASC").Return(tags, int64(2), nil)

	resp, err := svc.List(context.Background(), true, orgID, userID, 20, 0, "name", "ASC")

	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	assert.Len(t, resp.Tags, 2)
}

func TestTagService_Update_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag := makeTag(orgID, "Old Name", "old-name")

	grantTagPermission(t, enforcer, "updater", orgID.String(), "update", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)
	tagRepo.On("FindByName", mock.Anything, orgID, "New Name").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("FindBySlug", mock.Anything, orgID, "new-name").Return(nil, apperrors.ErrNotFound)
	tagRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(nil)

	req := request.UpdateTagRequest{Name: "New Name"}

	resp, err := svc.Update(context.Background(), true, orgID, userID, tag.ID, req, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Equal(t, "New Name", resp.Name)
}

func TestTagService_Delete_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag := makeTag(orgID, "Test", "test")

	grantTagPermission(t, enforcer, "deleter", orgID.String(), "delete", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)
	tagRepo.On("SoftDelete", mock.Anything, tag.ID).Return(nil)

	err := svc.Delete(context.Background(), true, orgID, userID, tag.ID, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	tagRepo.AssertCalled(t, "SoftDelete", mock.Anything, tag.ID)
}

func TestTagService_Autocomplete_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{makeTag(orgID, "Testing", "testing")}

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("Autocomplete", mock.Anything, orgID, "test", 10).Return(tags, nil)

	resp, err := svc.Autocomplete(context.Background(), true, orgID, userID, "test", 10)

	require.NoError(t, err)
	assert.Len(t, resp.Tags, 1)
}

func TestTagService_Autocomplete_EmptyQuery(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{makeTag(orgID, "Popular", "popular")}

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("Autocomplete", mock.Anything, orgID, "", 10).Return(tags, nil)

	resp, err := svc.Autocomplete(context.Background(), true, orgID, userID, "", 10)

	require.NoError(t, err)
	assert.Len(t, resp.Tags, 1)
}

func TestTagService_AttachTags_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag1 := makeTag(orgID, "Tag1", "tag1")
	tag2 := makeTag(orgID, "Tag2", "tag2")

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	tagRepo.On("FindByID", mock.Anything, tag1.ID).Return(tag1, nil)
	tagRepo.On("FindByID", mock.Anything, tag2.ID).Return(tag2, nil)
	entityTagRepo.On("FindByEntityAndTag", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID"), tag1.ID).Return(nil, nil)
	entityTagRepo.On("FindByEntityAndTag", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID"), tag2.ID).Return(nil, nil)
	entityTagRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.EntityTag")).Return(nil)
	tagRepo.On("IncrementUsageCount", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	entityID := uuid.New()
	result, err := svc.AttachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tag1.ID, tag2.ID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Attached, 2)
	assert.Len(t, result.Skipped, 0)
	assert.Len(t, result.Errors, 0)
}

func TestTagService_AttachTags_SoftDeletedTag(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag1 := makeTag(orgID, "Deleted", "deleted")
	tag1.DeletedAt = gorm.DeletedAt{Valid: true, Time: time.Now()}

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	tagRepo.On("FindByID", mock.Anything, tag1.ID).Return(tag1, nil)

	entityID := uuid.New()
	result, err := svc.AttachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tag1.ID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Attached, 0)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "tag is soft-deleted", result.Errors[0].Message)
}

func TestTagService_AttachTags_Idempotent(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag1 := makeTag(orgID, "Existing", "existing")
	existingEntityTag := &domain.EntityTag{
		ID:             uuid.New(),
		EntityType:     "news",
		EntityID:       uuid.New(),
		TagID:          tag1.ID,
		OrganizationID: orgID,
		CreatedBy:      userID,
	}

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	tagRepo.On("FindByID", mock.Anything, tag1.ID).Return(tag1, nil)
	entityTagRepo.On("FindByEntityAndTag", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID"), tag1.ID).Return(existingEntityTag, nil)

	entityID := uuid.New()
	result, err := svc.AttachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tag1.ID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Skipped, 1)
	assert.Len(t, result.Attached, 0)
}

func TestTagService_AttachTags_InvalidEntityType(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)

	_, err := svc.AttachTags(context.Background(), true, orgID, userID, "invalid_type", uuid.New(), []uuid.UUID{uuid.New()}, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
}

func TestTagService_DetachTags_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tag1 := makeTag(orgID, "Tag1", "tag1")
	entityTag1 := &domain.EntityTag{ID: uuid.New(), TagID: tag1.ID}

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	entityTagRepo.On("FindByEntityAndTag", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID"), tag1.ID).Return(entityTag1, nil)
	entityTagRepo.On("DeleteByEntityAndTag", mock.Anything, entityTag1.ID).Return(nil)
	tagRepo.On("DecrementUsageCount", mock.Anything, tag1.ID).Return(nil)

	entityID := uuid.New()
	result, err := svc.DetachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tag1.ID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Detached, 1)
	assert.Len(t, result.Skipped, 0)
}

func TestTagService_DetachTags_Idempotent(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tagID := uuid.New()

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	entityTagRepo.On("FindByEntityAndTag", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID"), tagID).Return(nil, nil)

	entityID := uuid.New()
	result, err := svc.DetachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tagID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Skipped, 1)
	assert.Len(t, result.Detached, 0)
}

func TestTagService_ListEntityTags_Success(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{makeTag(orgID, "Tag1", "tag1"), makeTag(orgID, "Tag2", "tag2")}

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	entityTagRepo.On("ListTagDetailsByEntity", mock.Anything, orgID, "news", mock.AnythingOfType("uuid.UUID")).Return(tags, nil)

	entityID := uuid.New()
	result, err := svc.ListEntityTags(context.Background(), true, orgID, userID, "news", entityID)

	require.NoError(t, err)
	assert.Len(t, result.Tags, 2)
	assert.Equal(t, "news", result.EntityType)
}

func TestTagService_ListEntityTags_InvalidEntityType(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)

	_, err := svc.ListEntityTags(context.Background(), true, orgID, userID, "invalid", uuid.New())

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 422, appErr.HTTPStatus)
}

func TestDomain_GenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "Test Tag", "test-tag"},
		{"special chars", "Hello! World?", "hello-world"},
		{"multiple hyphens", "A   B   C", "a-b-c"},
		{"leading trailing hyphens", "--test--", "test"},
		{"numbers", "Version 2.0", "version-2-0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domain.GenerateSlug(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDomain_GenerateSlug_EmptyFallback(t *testing.T) {
	result := domain.GenerateSlug("!!!")
	assert.True(t, len(result) > 0)
	assert.True(t, result[:4] == "tag-")
}

func TestDomain_IsValidTaggableType(t *testing.T) {
	assert.True(t, domain.IsValidTaggableType("news"))
	assert.True(t, domain.IsValidTaggableType("invoice"))
	assert.True(t, domain.IsValidTaggableType("media"))
	assert.False(t, domain.IsValidTaggableType("user"))
	assert.False(t, domain.IsValidTaggableType("organization"))
	assert.False(t, domain.IsValidTaggableType(""))
}

func TestTagService_GetByID_CrossOrgReturnsNotFound(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	otherOrgID := uuid.New()
	tag := makeTag(otherOrgID, "Other Org Tag", "other-org-tag")

	grantTagPermission(t, enforcer, "viewer", orgID.String(), "view", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)

	_, err := svc.GetByID(context.Background(), true, orgID, userID, tag.ID)

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}

func TestTagService_AttachTags_CrossOrgRejected(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	otherOrgID := uuid.New()
	tag := makeTag(otherOrgID, "Cross Org", "cross-org")

	grantTagPermission(t, enforcer, "manager", orgID.String(), "manage", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)

	entityID := uuid.New()
	result, err := svc.AttachTags(context.Background(), true, orgID, userID, "news", entityID, []uuid.UUID{tag.ID}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.Len(t, result.Attached, 0)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "tag does not belong to this organization", result.Errors[0].Message)
}

func TestTagService_Delete_CrossOrgReturnsNotFound(t *testing.T) {
	tagRepo := newMockTagRepository()
	entityTagRepo := newMockEntityTagRepository()
	enforcer := newTestTagEnforcer(t)
	auditRepo := newMockAuditLogForTags()
	svc := newTestTagService(tagRepo, entityTagRepo, enforcer, auditRepo)

	userID := uuid.New()
	orgID := uuid.New()
	otherOrgID := uuid.New()
	tag := makeTag(otherOrgID, "Other", "other")

	grantTagPermission(t, enforcer, "deleter", orgID.String(), "delete", userID)
	tagRepo.On("FindByID", mock.Anything, tag.ID).Return(tag, nil)

	err := svc.Delete(context.Background(), true, orgID, userID, tag.ID, "127.0.0.1", "test-agent")

	assert.Error(t, err)
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, 404, appErr.HTTPStatus)
}
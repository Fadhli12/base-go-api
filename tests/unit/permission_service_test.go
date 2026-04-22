package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock PermissionRepository for testing
type MockPermissionRepository struct {
	mock.Mock
}

func (m *MockPermissionRepository) Create(ctx context.Context, permission *domain.Permission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockPermissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Permission), args.Error(1)
}

func (m *MockPermissionRepository) FindAll(ctx context.Context) ([]domain.Permission, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Permission), args.Error(1)
}

func (m *MockPermissionRepository) FindByResource(ctx context.Context, resource string) ([]domain.Permission, error) {
	args := m.Called(ctx, resource)
	return args.Get(0).([]domain.Permission), args.Error(1)
}

func (m *MockPermissionRepository) FindByName(ctx context.Context, name string) (*domain.Permission, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Permission), args.Error(1)
}

// TestPermissionService_Create_Success tests successful permission creation
func TestPermissionService_Create_Success(t *testing.T) {
	ctx := context.Background()
	permRepo := new(MockPermissionRepository)

	// Mock FindByName to return not found (permission doesn't exist)
	permRepo.On("FindByName", ctx, "invoice:create").Return(nil, errors.ErrNotFound)

	// Mock Create to succeed
	permRepo.On("Create", ctx, mock.AnythingOfType("*domain.Permission")).Return(nil)

	permSvc := service.NewPermissionService(permRepo)

	permission, err := permSvc.Create(ctx, "invoice:create", "invoice", "create", "all")
	require.NoError(t, err)
	assert.NotNil(t, permission)
	assert.Equal(t, "invoice:create", permission.Name)
	assert.Equal(t, "invoice", permission.Resource)
	assert.Equal(t, "create", permission.Action)
	assert.Equal(t, "all", permission.Scope)

	permRepo.AssertExpectations(t)
}

// TestPermissionService_Create_DuplicateName tests creation with existing name
func TestPermissionService_Create_DuplicateName(t *testing.T) {
	ctx := context.Background()
	permRepo := new(MockPermissionRepository)

	existingPerm := &domain.Permission{
		ID:       uuid.New(),
		Name:     "invoice:create",
		Resource: "invoice",
		Action:   "create",
	}

	// Mock FindByName to return existing permission
	permRepo.On("FindByName", ctx, "invoice:create").Return(existingPerm, nil)

	permSvc := service.NewPermissionService(permRepo)

	permission, err := permSvc.Create(ctx, "invoice:create", "invoice", "create", "all")
	require.Error(t, err)
	assert.Nil(t, permission)

	permRepo.AssertExpectations(t)
}

// TestPermissionService_Create_ValidationError tests creation with invalid input
func TestPermissionService_Create_ValidationError(t *testing.T) {
	ctx := context.Background()
	permRepo := new(MockPermissionRepository)
	permSvc := service.NewPermissionService(permRepo)

	// Empty name
	_, err := permSvc.Create(ctx, "", "invoice", "create", "all")
	assert.Error(t, err)

	// Empty resource
	_, err = permSvc.Create(ctx, "perm", "", "create", "all")
	assert.Error(t, err)

	// Empty action
	_, err = permSvc.Create(ctx, "perm", "invoice", "", "all")
	assert.Error(t, err)
}

// TestPermissionService_GetAll tests retrieving all permissions
func TestPermissionService_GetAll(t *testing.T) {
	ctx := context.Background()
	permRepo := new(MockPermissionRepository)

	permissions := []domain.Permission{
		{ID: uuid.New(), Name: "invoice:create", Resource: "invoice", Action: "create"},
		{ID: uuid.New(), Name: "invoice:read", Resource: "invoice", Action: "read"},
	}

	permRepo.On("FindAll", ctx).Return(permissions, nil)

	permSvc := service.NewPermissionService(permRepo)

	result, err := permSvc.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	permRepo.AssertExpectations(t)
}

// TestPermissionService_GetAll_Empty tests retrieving empty permissions list
func TestPermissionService_GetAll_Empty(t *testing.T) {
	ctx := context.Background()
	permRepo := new(MockPermissionRepository)

	permRepo.On("FindAll", ctx).Return([]domain.Permission{}, nil)

	permSvc := service.NewPermissionService(permRepo)

	result, err := permSvc.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, result)

	permRepo.AssertExpectations(t)
}
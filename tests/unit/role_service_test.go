package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock RoleRepository for testing
type MockRoleRepository struct {
	mock.Mock
}

func (m *MockRoleRepository) Create(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) FindAll(ctx context.Context) ([]domain.Role, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Role), args.Error(1)
}

func (m *MockRoleRepository) Update(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoleRepository) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

// Mock RolePermissionRepository for testing
type MockRolePermissionRepository struct {
	mock.Mock
}

func (m *MockRolePermissionRepository) Attach(ctx context.Context, roleID, permissionID uuid.UUID) error {
	args := m.Called(ctx, roleID, permissionID)
	return args.Error(0)
}

func (m *MockRolePermissionRepository) Detach(ctx context.Context, roleID, permissionID uuid.UUID) error {
	args := m.Called(ctx, roleID, permissionID)
	return args.Error(0)
}

func (m *MockRolePermissionRepository) FindByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	args := m.Called(ctx, roleID)
	return args.Get(0).([]domain.Permission), args.Error(1)
}

func (m *MockRolePermissionRepository) Sync(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	args := m.Called(ctx, roleID, permissionIDs)
	return args.Error(0)
}

// TestRoleService_Create_Success tests successful role creation
func TestRoleService_Create_Success(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	// Mock FindByName to return not found (role doesn't exist)
	roleRepo.On("FindByName", ctx, "admin").Return(nil, errors.ErrNotFound)

	// Mock Create to succeed
	roleRepo.On("Create", ctx, mock.AnythingOfType("*domain.Role")).Return(nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	role, err := roleSvc.Create(ctx, "admin", "Administrator role")
	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "admin", role.Name)
	assert.Equal(t, "Administrator role", role.Description)

	roleRepo.AssertExpectations(t)
}

// TestRoleService_Create_DuplicateName tests creation with existing name
func TestRoleService_Create_DuplicateName(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	existingRole := &domain.Role{
		ID:          uuid.New(),
		Name:        "admin",
		Description: "Existing admin role",
	}

	// Mock FindByName to return existing role
	roleRepo.On("FindByName", ctx, "admin").Return(existingRole, nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	role, err := roleSvc.Create(ctx, "admin", "New admin role")
	require.Error(t, err)
	assert.Nil(t, role)

	roleRepo.AssertExpectations(t)
}

// TestRoleService_Update tests updating a role
func TestRoleService_Update(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	existingRole := &domain.Role{
		ID:          uuid.New(),
		Name:        "admin",
		Description: "Old description",
	}

	// Mock FindByID to return existing role
	roleRepo.On("FindByID", ctx, existingRole.ID).Return(existingRole, nil)

	// Mock Update to succeed
	roleRepo.On("Update", ctx, mock.AnythingOfType("*domain.Role")).Return(nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	role, err := roleSvc.Update(ctx, existingRole.ID, "admin", "New description")
	require.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "New description", role.Description)

	roleRepo.AssertExpectations(t)
}

// TestRoleService_GetAll tests retrieving all roles
func TestRoleService_GetAll(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	roles := []domain.Role{
		{ID: uuid.New(), Name: "admin", Description: "Admin role"},
		{ID: uuid.New(), Name: "user", Description: "User role"},
	}

	roleRepo.On("FindAll", ctx).Return(roles, nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	result, err := roleSvc.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	roleRepo.AssertExpectations(t)
}

// TestRoleService_Delete_NonSystemRole tests deleting a non-system role
func TestRoleService_Delete_NonSystemRole(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	roleID := uuid.New()

	// Mock SoftDelete to succeed for non-system role
	roleRepo.On("SoftDelete", ctx, roleID).Return(nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	err := roleSvc.Delete(ctx, roleID)
	require.NoError(t, err)

	roleRepo.AssertExpectations(t)
}

// TestRoleService_AttachPermission tests attaching permission to role
func TestRoleService_AttachPermission(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	roleID := uuid.New()
	permID := uuid.New()

	existingRole := &domain.Role{ID: roleID, Name: "admin"}
	existingPerm := &domain.Permission{ID: permID, Name: "invoice:create"}

	// Mock FindByID to return existing role
	roleRepo.On("FindByID", ctx, roleID).Return(existingRole, nil)

	// Mock permission repository to return existing permission
	permRepo.On("FindByID", ctx, permID).Return(existingPerm, nil)

	// Mock Attach to succeed
	rolePermRepo.On("Attach", ctx, roleID, permID).Return(nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	err := roleSvc.AttachPermission(ctx, roleID, permID)
	require.NoError(t, err)

	roleRepo.AssertExpectations(t)
	permRepo.AssertExpectations(t)
	rolePermRepo.AssertExpectations(t)
}

// TestRoleService_DetachPermission tests detaching permission from role
func TestRoleService_DetachPermission(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	roleID := uuid.New()
	permID := uuid.New()

	// Mock Detach to succeed
	rolePermRepo.On("Detach", ctx, roleID, permID).Return(nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	err := roleSvc.DetachPermission(ctx, roleID, permID)
	require.NoError(t, err)

	rolePermRepo.AssertExpectations(t)
}

// TestRoleService_GetByID tests retrieving a role by ID
func TestRoleService_GetByID(t *testing.T) {
	ctx := context.Background()
	roleRepo := new(MockRoleRepository)
	rolePermRepo := new(MockRolePermissionRepository)
	permRepo := new(MockPermissionRepository)

	roleID := uuid.New()
	expectedRole := &domain.Role{
		ID:          roleID,
		Name:        "admin",
		Description: "Admin role",
	}

	roleRepo.On("FindByID", ctx, roleID).Return(expectedRole, nil)

	roleSvc := service.NewRoleService(roleRepo, rolePermRepo, permRepo)

	role, err := roleSvc.GetByID(ctx, roleID)
	require.NoError(t, err)
	assert.Equal(t, expectedRole.ID, role.ID)
	assert.Equal(t, expectedRole.Name, role.Name)

	roleRepo.AssertExpectations(t)
}
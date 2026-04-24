package unit

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserRepositoryForUserService for testing
type MockUserRepositoryForUserService struct {
	mock.Mock
}

func (m *MockUserRepositoryForUserService) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForUserService) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForUserService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryForUserService) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryForUserService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepositoryForUserService) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

// MockUserRoleRepositoryForUserService for testing
type MockUserRoleRepositoryForUserService struct {
	mock.Mock
}

func (m *MockUserRoleRepositoryForUserService) Assign(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, roleID, assignedBy)
	return args.Error(0)
}

func (m *MockUserRoleRepositoryForUserService) Remove(ctx context.Context, userID, roleID uuid.UUID) error {
	args := m.Called(ctx, userID, roleID)
	return args.Error(0)
}

func (m *MockUserRoleRepositoryForUserService) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

func (m *MockUserRoleRepositoryForUserService) FindByUserIDWithPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

// MockUserPermissionRepositoryForUserService for testing
type MockUserPermissionRepositoryForUserService struct {
	mock.Mock
}

func (m *MockUserPermissionRepositoryForUserService) Grant(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID, assignedBy)
	return args.Error(0)
}

func (m *MockUserPermissionRepositoryForUserService) Deny(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID, assignedBy)
	return args.Error(0)
}

func (m *MockUserPermissionRepositoryForUserService) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.UserPermission), args.Error(1)
}

func (m *MockUserPermissionRepositoryForUserService) FindByUserIDWithDetails(ctx context.Context, userID uuid.UUID) ([]repository.UserPermissionWithDetails, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.UserPermissionWithDetails), args.Error(1)
}

func (m *MockUserPermissionRepositoryForUserService) Remove(ctx context.Context, userID, permissionID uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID)
	return args.Error(0)
}

// TestUserService_AssignRole tests assigning role to user
func TestUserService_AssignRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForUserService)
	userRoleRepo := new(MockUserRoleRepositoryForUserService)
	userPermRepo := new(MockUserPermissionRepositoryForUserService)

	userID := uuid.New()
	roleID := uuid.New()
	assignedBy := uuid.New()

	userRoleRepo.On("Assign", ctx, userID, roleID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	err := userSvc.AssignRole(ctx, userID, roleID, assignedBy)
	require.NoError(t, err)

	userRoleRepo.AssertExpectations(t)
}

// TestUserService_RemoveRole tests removing role from user
func TestUserService_RemoveRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForUserService)
	userRoleRepo := new(MockUserRoleRepositoryForUserService)
	userPermRepo := new(MockUserPermissionRepositoryForUserService)

	userID := uuid.New()
	roleID := uuid.New()

	userRoleRepo.On("Remove", ctx, userID, roleID).Return(nil)

	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	err := userSvc.RemoveRole(ctx, userID, roleID)
	require.NoError(t, err)

	userRoleRepo.AssertExpectations(t)
}

// TestUserService_GetUserRoles tests getting user roles
func TestUserService_GetUserRoles(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForUserService)
	userRoleRepo := new(MockUserRoleRepositoryForUserService)
	userPermRepo := new(MockUserPermissionRepositoryForUserService)

	userID := uuid.New()
	roles := []domain.Role{
		{ID: uuid.New(), Name: "admin"},
		{ID: uuid.New(), Name: "user"},
	}

	userRoleRepo.On("FindByUserID", ctx, userID).Return(roles, nil)

	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	result, err := userSvc.GetUserRoles(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	userRoleRepo.AssertExpectations(t)
}

// TestUserService_GrantPermission tests granting permission to user
func TestUserService_GrantPermission(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForUserService)
	userRoleRepo := new(MockUserRoleRepositoryForUserService)
	userPermRepo := new(MockUserPermissionRepositoryForUserService)

	userID := uuid.New()
	permissionID := uuid.New()
	assignedBy := uuid.New()

	userPermRepo.On("Grant", ctx, userID, permissionID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	err := userSvc.GrantPermission(ctx, userID, permissionID, assignedBy)
	require.NoError(t, err)

	userPermRepo.AssertExpectations(t)
}

// TestUserService_DenyPermission tests denying permission to user
func TestUserService_DenyPermission(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryForUserService)
	userRoleRepo := new(MockUserRoleRepositoryForUserService)
	userPermRepo := new(MockUserPermissionRepositoryForUserService)

	userID := uuid.New()
	permissionID := uuid.New()
	assignedBy := uuid.New()

	userPermRepo.On("Deny", ctx, userID, permissionID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	err := userSvc.DenyPermission(ctx, userID, permissionID, assignedBy)
	require.NoError(t, err)

	userPermRepo.AssertExpectations(t)
}
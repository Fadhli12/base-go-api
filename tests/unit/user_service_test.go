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

// Mock UserRoleRepository for testing
type MockUserRoleRepository struct {
	mock.Mock
}

func (m *MockUserRoleRepository) Assign(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, roleID, assignedBy)
	return args.Error(0)
}

func (m *MockUserRoleRepository) Remove(ctx context.Context, userID, roleID uuid.UUID) error {
	args := m.Called(ctx, userID, roleID)
	return args.Error(0)
}

func (m *MockUserRoleRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

// Mock UserPermissionRepository for testing
type MockUserPermissionRepository struct {
	mock.Mock
}

func (m *MockUserPermissionRepository) Grant(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID, assignedBy)
	return args.Error(0)
}

func (m *MockUserPermissionRepository) Deny(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID, assignedBy)
	return args.Error(0)
}

func (m *MockUserPermissionRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.UserPermission), args.Error(1)
}

func (m *MockUserPermissionRepository) Remove(ctx context.Context, userID, permissionID uuid.UUID) error {
	args := m.Called(ctx, userID, permissionID)
	return args.Error(0)
}

// TestUserService_AssignRole tests assigning role to user
func TestUserService_AssignRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	roleID := uuid.New()
	assignedBy := uuid.New()

	userRoleRepo.On("Assign", ctx, userID, roleID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	err := userSvc.AssignRole(ctx, userID, roleID, assignedBy)
	require.NoError(t, err)

	userRoleRepo.AssertExpectations(t)
}

// TestUserService_RemoveRole tests removing role from user
func TestUserService_RemoveRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	roleID := uuid.New()

	userRoleRepo.On("Remove", ctx, userID, roleID).Return(nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	err := userSvc.RemoveRole(ctx, userID, roleID)
	require.NoError(t, err)

	userRoleRepo.AssertExpectations(t)
}

// TestUserService_GrantPermission tests granting permission to user
func TestUserService_GrantPermission(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	permID := uuid.New()
	assignedBy := uuid.New()

	userPermRepo.On("Grant", ctx, userID, permID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	err := userSvc.GrantPermission(ctx, userID, permID, assignedBy)
	require.NoError(t, err)

	userPermRepo.AssertExpectations(t)
}

// TestUserService_DenyPermission tests denying permission to user
func TestUserService_DenyPermission(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	permID := uuid.New()
	assignedBy := uuid.New()

	userPermRepo.On("Deny", ctx, userID, permID, assignedBy).Return(nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	err := userSvc.DenyPermission(ctx, userID, permID, assignedBy)
	require.NoError(t, err)

	userPermRepo.AssertExpectations(t)
}

// TestUserService_RemovePermission tests removing permission from user
func TestUserService_RemovePermission(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	permID := uuid.New()

	userPermRepo.On("Remove", ctx, userID, permID).Return(nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	err := userSvc.RemovePermission(ctx, userID, permID)
	require.NoError(t, err)

	userPermRepo.AssertExpectations(t)
}

// TestUserService_FindByID tests finding user by ID
func TestUserService_FindByID(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()
	expectedUser := &domain.User{
		ID:    userID,
		Email: "test@example.com",
	}

	userRepo.On("FindByID", ctx, userID).Return(expectedUser, nil)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	user, err := userSvc.FindByID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, expectedUser.ID, user.ID)
	assert.Equal(t, expectedUser.Email, user.Email)

	userRepo.AssertExpectations(t)
}

// TestUserService_FindByID_NotFound tests finding non-existent user
func TestUserService_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	userRoleRepo := new(MockUserRoleRepository)
	userPermRepo := new(MockUserPermissionRepository)

	userID := uuid.New()

	userRepo.On("FindByID", ctx, userID).Return(nil, errors.ErrNotFound)

	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, userPermRepo)

	user, err := userSvc.FindByID(ctx, userID)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, errors.ErrNotFound)

	userRepo.AssertExpectations(t)
}
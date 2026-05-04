package unit

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserRepository for testing AuthService
type MockUserRepositoryAuth struct {
	mock.Mock
}

func (m *MockUserRepositoryAuth) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryAuth) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryAuth) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepositoryAuth) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepositoryAuth) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepositoryAuth) FindAll(ctx context.Context) ([]domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.User), args.Error(1)
}

// MockRefreshTokenRepository for testing AuthService
type MockRefreshTokenRepositoryAuth struct {
	mock.Mock
}

func (m *MockRefreshTokenRepositoryAuth) Create(ctx context.Context, token *domain.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryAuth) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryAuth) FindByID(ctx context.Context, id uuid.UUID) (*domain.RefreshToken, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryAuth) MarkRevoked(ctx context.Context, hash string) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryAuth) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryAuth) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryAuth) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshToken, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepositoryAuth) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	args := m.Called(ctx, familyID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepositoryAuth) FindFamilyTokens(ctx context.Context, familyID uuid.UUID) ([]*domain.RefreshToken, error) {
	args := m.Called(ctx, familyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.RefreshToken), args.Error(1)
}

// MockPasswordResetTokenRepository for testing AuthService
type MockPasswordResetTokenRepositoryAuth struct {
	mock.Mock
}

func (m *MockPasswordResetTokenRepositoryAuth) Create(ctx context.Context, token *domain.PasswordResetToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockPasswordResetTokenRepositoryAuth) FindByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PasswordResetToken), args.Error(1)
}

func (m *MockPasswordResetTokenRepositoryAuth) FindValidByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.PasswordResetToken, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PasswordResetToken), args.Error(1)
}

func (m *MockPasswordResetTokenRepositoryAuth) MarkUsed(ctx context.Context, hash string) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockPasswordResetTokenRepositoryAuth) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockPasswordResetTokenRepositoryAuth) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockRoleRepository for testing AuthService
type MockRoleRepositoryAuth struct {
	mock.Mock
}

func (m *MockRoleRepositoryAuth) Create(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepositoryAuth) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepositoryAuth) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepositoryAuth) FindAll(ctx context.Context) ([]domain.Role, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

func (m *MockRoleRepositoryAuth) Update(ctx context.Context, role *domain.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepositoryAuth) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockUserRoleRepository for testing AuthService
type MockUserRoleRepositoryAuth struct {
	mock.Mock
}

func (m *MockUserRoleRepositoryAuth) Assign(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
	args := m.Called(ctx, userID, roleID, assignedBy)
	return args.Error(0)
}

func (m *MockUserRoleRepositoryAuth) Remove(ctx context.Context, userID, roleID uuid.UUID) error {
	args := m.Called(ctx, userID, roleID)
	return args.Error(0)
}

func (m *MockUserRoleRepositoryAuth) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

func (m *MockUserRoleRepositoryAuth) FindByUserIDWithPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Role), args.Error(1)
}

// Helper function to create AuthService with mocks
func newTestAuthService(
	userRepo *MockUserRepositoryAuth,
	refreshTokenRepo *MockRefreshTokenRepositoryAuth,
	resetTokenRepo *MockPasswordResetTokenRepositoryAuth,
	roleRepo *MockRoleRepositoryAuth,
	userRoleRepo *MockUserRoleRepositoryAuth,
) *service.AuthService {
	tokenService := service.NewTokenService(
		"test-secret-key-32-characters-long",
		"go-api-base",
		"go-api-base",
		15*time.Minute,
		24*time.Hour,
	)
	passwordHasher := service.NewPasswordHasher()

	return service.NewAuthService(
		userRepo,
		refreshTokenRepo,
		resetTokenRepo,
		tokenService,
		passwordHasher,
		nil, // emailService
		nil, // auditService
		5*time.Minute,
		roleRepo,
		userRoleRepo,
	)
}

// TestAuthService_Register_Success tests successful user registration
func TestAuthService_Register_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	// Mock FindByEmail to return not found (user doesn't exist)
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(nil, apperrors.ErrNotFound)

	// Mock Create to succeed
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	// Mock FindByName to return not found (viewer role doesn't exist)
	roleRepo.On("FindByName", ctx, "viewer").Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	req := &request.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	user, err := authSvc.Register(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NotEmpty(t, user.PasswordHash)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Register_DuplicateEmail tests registration with existing email
func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}

	// Mock FindByEmail to return existing user
	userRepo.On("FindByEmail", ctx, "existing@example.com").Return(existingUser, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	req := &request.RegisterRequest{
		Email:    "existing@example.com",
		Password: "password123",
	}

	user, err := authSvc.Register(ctx, req)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "already registered")

	userRepo.AssertExpectations(t)
}

// TestAuthService_Login_Success tests successful login
func TestAuthService_Login_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	passwordHasher := service.NewPasswordHasher()
	passwordHash, _ := passwordHasher.Hash("password123")

	existingUser := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
	}

	// Mock FindByEmail to return user
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(existingUser, nil)

	// Mock Create for refresh token
	refreshTokenRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	req := &request.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	user, accessToken, refreshToken, err := authSvc.Login(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	userRepo.AssertExpectations(t)
	refreshTokenRepo.AssertExpectations(t)
}

// TestAuthService_Login_UserNotFound tests login with non-existent user
func TestAuthService_Login_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	// Mock FindByEmail to return not found
	userRepo.On("FindByEmail", ctx, "nonexistent@example.com").Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	req := &request.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	user, accessToken, refreshToken, err := authSvc.Login(ctx, req)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Login_WrongPassword tests login with wrong password
func TestAuthService_Login_WrongPassword(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	passwordHasher := service.NewPasswordHasher()
	passwordHash, _ := passwordHasher.Hash("correctpassword")

	existingUser := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
	}

	// Mock FindByEmail to return user
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(existingUser, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	req := &request.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	user, accessToken, refreshToken, err := authSvc.Login(ctx, req)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	userRepo.AssertExpectations(t)
}
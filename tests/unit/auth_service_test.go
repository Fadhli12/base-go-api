package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock UserRepository for testing
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
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

// Mock RefreshTokenRepository for testing
type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepository) MarkRevoked(ctx context.Context, hash string) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.RefreshToken, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.RefreshToken), args.Error(1)
}

// TestAuthService_Register_Success tests successful user registration
func TestAuthService_Register_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	// Mock FindByEmail to return not found (user doesn't exist)
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(nil, errors.ErrNotFound)

	// Mock Create to succeed
	userRepo.On("Create", ctx, mock.AnythingOfType("*domain.User")).Return(nil)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	user, err := authSvc.Register(ctx, "test@example.com", "password123")
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NotEmpty(t, user.PasswordHash)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Register_DuplicateEmail tests registration with existing email
func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}

	// Mock FindByEmail to return existing user
	userRepo.On("FindByEmail", ctx, "existing@example.com").Return(existingUser, nil)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	user, err := authSvc.Register(ctx, "existing@example.com", "password123")
	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, errors.ErrDuplicate)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Login_Success tests successful login
func TestAuthService_Login_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	passwordHash, _ := service.HashPassword("password123")
	existingUser := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
	}

	// Mock FindByEmail to return user
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(existingUser, nil)

	// Mock Create for refresh token
	refreshTokenRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	user, accessToken, refreshToken, err := authSvc.Login(ctx, "test@example.com", "password123")
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
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	// Mock FindByEmail to return not found
	userRepo.On("FindByEmail", ctx, "nonexistent@example.com").Return(nil, errors.ErrNotFound)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	user, accessToken, refreshToken, err := authSvc.Login(ctx, "nonexistent@example.com", "password123")
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Login_WrongPassword tests login with wrong password
func TestAuthService_Login_WrongPassword(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	passwordHash, _ := service.HashPassword("correctpassword")
	existingUser := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: passwordHash,
	}

	// Mock FindByEmail to return user
	userRepo.On("FindByEmail", ctx, "test@example.com").Return(existingUser, nil)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	user, accessToken, refreshToken, err := authSvc.Login(ctx, "test@example.com", "wrongpassword")
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	userRepo.AssertExpectations(t)
}

// TestAuthService_Logout_Success tests successful logout
func TestAuthService_Logout_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	refreshTokenRepo := new(MockRefreshTokenRepository)

	userID := uuid.New()

	// Mock RevokeAllByUser to succeed
	refreshTokenRepo.On("RevokeAllByUser", ctx, userID).Return(nil)

	authSvc := service.NewAuthService(userRepo, refreshTokenRepo)

	err := authSvc.Logout(ctx, userID)
	require.NoError(t, err)

	refreshTokenRepo.AssertExpectations(t)
}

// TestPasswordHashing tests password hashing and verification
func TestPasswordHashing(t *testing.T) {
	password := "mySecretPassword123"

	// Test hashing
	hash, err := service.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Test verification with correct password
	err = service.Verify(hash, password)
	assert.NoError(t, err)

	// Test verification with wrong password
	err = service.Verify(hash, "wrongPassword")
	assert.Error(t, err)
}

// TestPasswordHashing_DifferentHashes tests that same password produces different hashes
func TestPasswordHashing_DifferentHashes(t *testing.T) {
	password := "mySecretPassword123"

	hash1, err := service.HashPassword(password)
	require.NoError(t, err)

	hash2, err := service.HashPassword(password)
	require.NoError(t, err)

	// Hashes should be different due to salt
	assert.NotEqual(t, hash1, hash2)

	// But both should verify correctly
	require.NoError(t, service.Verify(hash1, password))
	require.NoError(t, service.Verify(hash2, password))
}
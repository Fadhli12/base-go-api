//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/require"
)

// requireAppErrorCode checks that an error is an AppError with the expected code
func requireAppErrorCode(t *testing.T, err error, expectedCode string, msgAndArgs ...interface{}) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Expected AppError with code %s, got %T: %v", expectedCode, err, err)
	}
	require.Equal(t, expectedCode, appErr.Code, msgAndArgs...)
}

// createAuthServiceForTest creates an AuthService with real dependencies for testing
func createAuthServiceForTest(t *testing.T, suite *TestSuite) *service.AuthService {
	userRepo := repository.NewUserRepository(suite.DB)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)
	resetTokenRepo := repository.NewPasswordResetTokenRepository(suite.DB)
	tokenService := service.NewTokenService(
		"test-secret-key-min-32-characters-long",
		"go-api-base",
		"go-api-base",
		15*time.Minute,
		24*time.Hour,
	)
	passwordHasher := service.NewPasswordHasher()
	emailService := service.NewEmailService(nil, nil, nil, nil, nil, nil)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	return service.NewAuthService(userRepo, tokenRepo, resetTokenRepo, tokenService, passwordHasher, emailService, auditService, 15*time.Minute, roleRepo, userRoleRepo)
}

// createTestUser creates a test user and returns the user with plaintext password
func createTestUser(t *testing.T, authSvc *service.AuthService, ctx context.Context, email, password string) *domain.User {
	user, err := authSvc.Register(ctx, &request.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Failed to create test user")
	return user
}

// TestAuthRefresh_ValidToken tests that refresh with a valid token returns new access and refresh tokens
func TestAuthRefresh_ValidToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create and login user
	email := "refresh-valid@example.com"
	password := "password123"
	createTestUser(t, authSvc, ctx, email, password)

	user, accessToken, refreshToken, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")
	require.NotNil(t, user, "User should not be nil")
	require.NotEmpty(t, accessToken, "Access token should not be empty")
	require.NotEmpty(t, refreshToken, "Refresh token should not be empty")

	// Refresh the token
	newUser, newAccessToken, newRefreshToken, err := authSvc.Refresh(ctx, refreshToken)
	require.NoError(t, err, "Refresh should succeed")
	require.NotNil(t, newUser, "User should not be nil after refresh")
	require.NotEmpty(t, newAccessToken, "New access token should not be empty")
	require.NotEmpty(t, newRefreshToken, "New refresh token should not be empty")
	require.Equal(t, user.ID, newUser.ID, "User ID should match")
	require.NotEqual(t, accessToken, newAccessToken, "New access token should be different")
	require.NotEqual(t, refreshToken, newRefreshToken, "New refresh token should be different")
}

// TestAuthRefresh_OldTokenRevoked tests that the old refresh token is revoked after rotation
func TestAuthRefresh_OldTokenRevoked(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create and login user
	email := "refresh-revoked@example.com"
	password := "password123"
	createTestUser(t, authSvc, ctx, email, password)

	_, _, refreshToken, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")

	// First refresh should succeed
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.NoError(t, err, "First refresh should succeed")

	// Second refresh with same token should fail (token was revoked)
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.Error(t, err, "Second refresh with same token should fail")
	require.Contains(t, err.Error(), "Refresh token has been revoked", "Error should indicate invalid refresh token")
}

// TestAuthRefresh_InvalidToken tests that refresh with an invalid token returns an error
func TestAuthRefresh_InvalidToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Try to refresh with a completely invalid token string
	_, _, _, err := authSvc.Refresh(ctx, "invalid-token-string")
	require.Error(t, err, "Refresh with invalid token should fail")
	requireAppErrorCode(t, err, "INVALID_REFRESH_TOKEN", "Error code should be INVALID_REFRESH_TOKEN")
}

// TestAuthRefresh_ExpiredToken tests that refresh with an expired token returns an error
func TestAuthRefresh_ExpiredToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)

	// Create user
	email := "refresh-expired@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	// Create an expired refresh token manually
	// Generate a valid token hash (SHA256 for refresh tokens)
	tokenHash := auth.HashToken("expired-token-value")

	// Store expired token in database
	expiredToken := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}
	err := tokenRepo.Create(ctx, expiredToken)
	require.NoError(t, err, "Creating expired token should succeed")

	// Try to refresh with the expired token
	_, _, _, err = authSvc.Refresh(ctx, "expired-token-value")
	require.Error(t, err, "Refresh with expired token should fail")
	requireAppErrorCode(t, err, "INVALID_REFRESH_TOKEN", "Error code should be INVALID_REFRESH_TOKEN")
}

// TestAuthRefresh_RevokedToken tests that refresh with a revoked token returns an error
func TestAuthRefresh_RevokedToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)

	// Create and login user
	email := "refresh-revoked-token@example.com"
	password := "password123"
	_ = createTestUser(t, authSvc, ctx, email, password)

	// Login to get a refresh token
	_, _, refreshToken, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")

	// Manually revoke the token by marking it in the database
	tokenHash := auth.HashToken(refreshToken)
	err = tokenRepo.MarkRevoked(ctx, tokenHash)
	require.NoError(t, err, "Marking token as revoked should succeed")

	// Try to refresh with the revoked token
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.Error(t, err, "Refresh with revoked token should fail")
	requireAppErrorCode(t, err, "INVALID_REFRESH_TOKEN", "Error code should be INVALID_REFRESH_TOKEN")
}

// TestAuthRefresh_MultipleRefreshTokens tests that a user can have multiple valid refresh tokens
func TestAuthRefresh_MultipleRefreshTokens(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create user
	email := "refresh-multiple@example.com"
	password := "password123"
	createTestUser(t, authSvc, ctx, email, password)

	// Login twice to get two different refresh tokens
	_, _, refreshToken1, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "First login should succeed")

	_, _, refreshToken2, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Second login should succeed")

	require.NotEqual(t, refreshToken1, refreshToken2, "Tokens should be different")

	// Both tokens should be valid for refresh
	_, _, newRefresh1, err := authSvc.Refresh(ctx, refreshToken1)
	require.NoError(t, err, "First refresh should succeed")

	_, _, newRefresh2, err := authSvc.Refresh(ctx, refreshToken2)
	require.NoError(t, err, "Second refresh should succeed")

	// Both new tokens should be valid and different
	require.NotEmpty(t, newRefresh1, "New refresh token 1 should not be empty")
	require.NotEmpty(t, newRefresh2, "New refresh token 2 should not be empty")
	require.NotEqual(t, newRefresh1, newRefresh2, "New tokens should be different")
}

// TestAuthRefresh_NonExistentUser tests that refresh fails gracefully if user does not exist
func TestAuthRefresh_NonExistentUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create a user and login to get a valid refresh token
	email := "refresh-orphan@example.com"
	password := "password123"
	createTestUser(t, authSvc, ctx, email, password)

	_, _, refreshToken, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")

	// Soft-delete the user so the token exists but the user can't be found
	userRepo := repository.NewUserRepository(suite.DB)
	user, _ := userRepo.FindByEmail(ctx, email)
	require.NotNil(t, user, "User should exist")
	require.NoError(t, userRepo.SoftDelete(ctx, user.ID), "Should soft-delete user")

	// Try to refresh - should fail because user is deleted
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.Error(t, err, "Refresh with deleted user should fail")
}
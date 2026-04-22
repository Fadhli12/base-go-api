//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// createAuthServiceForTest creates an AuthService with real dependencies for testing
func createAuthServiceForTest(t *testing.T, suite *TestSuite) *service.AuthService {
	userRepo := repository.NewUserRepository(suite.DB)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)
	tokenService := service.NewTokenService("test-secret-key-min-32-characters-long", 15*time.Minute, 24*time.Hour)
	passwordHasher := service.NewPasswordHasher()
	return service.NewAuthService(userRepo, tokenRepo, tokenService, passwordHasher)
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
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")
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
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")
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
	// Generate a valid token hash
	tokenHash, err := auth.Hash("expired-token-value")
	require.NoError(t, err, "Hashing should succeed")

	// Store expired token in database
	expiredToken := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}
	err = tokenRepo.Create(ctx, expiredToken)
	require.NoError(t, err, "Creating expired token should succeed")

	// Try to refresh with the expired token
	_, _, _, err = authSvc.Refresh(ctx, "expired-token-value")
	require.Error(t, err, "Refresh with expired token should fail")
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")
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
	tokenHash, err := auth.Hash(refreshToken)
	require.NoError(t, err, "Hashing should succeed")
	err = tokenRepo.MarkRevoked(ctx, tokenHash)
	require.NoError(t, err, "Marking token as revoked should succeed")

	// Try to refresh with the revoked token
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.Error(t, err, "Refresh with revoked token should fail")
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")
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
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)

	// Create a token for a non-existent user ID
	nonExistentUserID := uuid.New()
	tokenHash, err := auth.Hash("orphan-token")
	require.NoError(t, err, "Hashing should succeed")

	orphanToken := &domain.RefreshToken{
		UserID:    nonExistentUserID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err = tokenRepo.Create(ctx, orphanToken)
	require.NoError(t, err, "Creating orphan token should succeed")

	// Try to refresh with the orphan token
	_, _, _, err = authSvc.Refresh(ctx, "orphan-token")
	require.Error(t, err, "Refresh with orphan token should fail")
	// The error could be either user not found or internal error
	require.Error(t, err, "Should return an error")
}
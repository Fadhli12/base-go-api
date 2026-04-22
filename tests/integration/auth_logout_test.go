//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAuthLogout_RevokesAllTokens tests that logout revokes all user's refresh tokens
func TestAuthLogout_RevokesAllTokens(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)

	// Create user
	email := "logout-user@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	// Login multiple times to create multiple refresh tokens
	_, _, _, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "First login should succeed")

	_, _, _, err = authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Second login should succeed")

	// Verify tokens exist and are not revoked
	tokens, err := tokenRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err, "Should find tokens")
	require.Len(t, tokens, 2, "Should have 2 tokens")

	for _, token := range tokens {
		require.Nil(t, token.RevokedAt, "Tokens should not be revoked before logout")
	}

	// Logout
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Logout should succeed")

	// Verify all tokens are now revoked
	tokens, err = tokenRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err, "Should find tokens")
	require.Len(t, tokens, 2, "Should still have 2 tokens")

	revokedCount := 0
	for _, token := range tokens {
		if token.IsRevoked() {
			revokedCount++
		}
	}
	require.Equal(t, 2, revokedCount, "All tokens should be revoked after logout")
}

// TestAuthLogout_RefreshFailsAfterLogout tests that refresh fails after logout
func TestAuthLogout_RefreshFailsAfterLogout(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create user and login
	email := "logout-refresh-fail@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	_, _, refreshToken, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")
	require.NotEmpty(t, refreshToken, "Refresh token should not be empty")

	// Verify refresh works before logout
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.NoError(t, err, "Refresh should succeed before logout")

	// Login again to get a new token (since we just used the old one)
	_, _, refreshToken, err = authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Second login should succeed")

	// Logout
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Logout should succeed")

	// Try to refresh with the token that was revoked by logout
	_, _, _, err = authSvc.Refresh(ctx, refreshToken)
	require.Error(t, err, "Refresh should fail after logout")
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")
}

// TestAuthLogout_CanLoginAgain tests that a user can login again after logout
func TestAuthLogout_CanLoginAgain(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)
	tokenService := service.NewTokenService("test-secret-key-min-32-characters-long", 15*time.Minute, 24*time.Hour)

	// Create user and login
	email := "logout-relogin@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	_, accessToken1, refreshToken1, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "First login should succeed")
	require.NotEmpty(t, accessToken1, "Access token should not be empty")
	require.NotEmpty(t, refreshToken1, "Refresh token should not be empty")

	// Logout
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Logout should succeed")

	// Login again
	_, accessToken2, refreshToken2, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Second login should succeed")
	require.NotEmpty(t, accessToken2, "New access token should not be empty")
	require.NotEmpty(t, refreshToken2, "New refresh token should not be empty")
	require.NotEqual(t, accessToken1, accessToken2, "Access tokens should be different")
	require.NotEqual(t, refreshToken1, refreshToken2, "Refresh tokens should be different")

	// Verify the new tokens work
	_, _, _, err = authSvc.Refresh(ctx, refreshToken2)
	require.NoError(t, err, "Refresh with new token should succeed after re-login")

	// Verify token expiry is set correctly
	require.Equal(t, 24*time.Hour, tokenService.GetRefreshExpiry(), "Refresh token expiry should be 24 hours")
}

// TestAuthLogout_MultipleSessions tests logging out one session doesn't affect others
// Note: Current implementation logs out ALL sessions for a user.
// This test documents that behavior and can be updated if session-specific logout is implemented.
func TestAuthLogout_MultipleSessions(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create user
	email := "logout-multi@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	// Simulate multiple sessions by logging in multiple times
	_, _, token1, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "First login should succeed")

	_, _, token2, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Second login should succeed")

	// Current behavior: Logout revokes ALL sessions
	// This test documents the expected behavior
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Logout should succeed")

	// Verify all sessions are revoked (current behavior)
	_, _, _, err = authSvc.Refresh(ctx, token1)
	require.Error(t, err, "First session should be revoked after logout")

	_, _, _, err = authSvc.Refresh(ctx, token2)
	require.Error(t, err, "Second session should be revoked after logout")
}

// TestAuthLogout_NoTokens tests that logout succeeds even when user has no tokens
func TestAuthLogout_NoTokens(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create user (no login, so no tokens)
	email := "logout-notokens@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	// Logout should still succeed (idempotent)
	err := authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Logout should succeed even with no tokens")
}

// TestAuthLogout_Idempotent tests that calling logout multiple times is safe
func TestAuthLogout_Idempotent(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create user and login
	email := "logout-idempotent@example.com"
	password := "password123"
	user := createTestUser(t, authSvc, ctx, email, password)

	_, _, _, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err, "Login should succeed")

	// First logout
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "First logout should succeed")

	// Second logout should also succeed (idempotent)
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Second logout should succeed (idempotent)")

	// Third logout
	err = authSvc.Logout(ctx, user.ID)
	require.NoError(t, err, "Third logout should succeed (idempotent)")
}

// TestAuthLogout_DifferentUsersIndependent tests that logging out one user doesn't affect another
func TestAuthLogout_DifferentUsersIndependent(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create two users
	email1 := "logout-user1@example.com"
	password1 := "password123"
	user1 := createTestUser(t, authSvc, ctx, email1, password1)

	email2 := "logout-user2@example.com"
	password2 := "password456"
	_ = createTestUser(t, authSvc, ctx, email2, password2)

	// Login both users
	_, _, token1, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email1,
		Password: password1,
	})
	require.NoError(t, err, "User1 login should succeed")

	_, _, token2, err := authSvc.Login(ctx, &request.LoginRequest{
		Email:    email2,
		Password: password2,
	})
	require.NoError(t, err, "User2 login should succeed")

	// Logout user1 only
	err = authSvc.Logout(ctx, user1.ID)
	require.NoError(t, err, "User1 logout should succeed")

	// User1's token should be revoked
	_, _, _, err = authSvc.Refresh(ctx, token1)
	require.Error(t, err, "User1's refresh should fail after logout")
	require.Contains(t, err.Error(), "INVALID_REFRESH_TOKEN", "Error should indicate invalid refresh token")

	// User2's token should still work
	_, _, newToken2, err := authSvc.Refresh(ctx, token2)
	require.NoError(t, err, "User2's refresh should still succeed")
	require.NotEmpty(t, newToken2, "User2's new token should not be empty")
}

// TestAuthLogout_InvalidUserID tests that logout handles non-existent users gracefully
func TestAuthLogout_InvalidUserID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	authSvc := createAuthServiceForTest(t, suite)

	// Create uuid for non-existent user
	nonExistentID := uuid.New()

	// Logout for non-existent user should still succeed (no tokens to revoke)
	err := authSvc.Logout(ctx, nonExistentID)
	// The current implementation doesn't check if user exists,
	// it just revokes tokens for that user ID (which there are none)
	require.NoError(t, err, "Logout should succeed for non-existent user (no-op)")
}
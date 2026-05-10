package unit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// -- TokenService tests --

func TestTokenService_GenerateAccessToken(t *testing.T) {
	ts := service.NewTokenService(
		"test-secret-32-characters-long-key",
		"test-issuer",
		"test-audience",
		15*time.Minute,
		24*time.Hour,
	)

	token, err := ts.GenerateAccessToken(uuid.New().String(), "test@example.com")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := auth.GetClaims(token, "test-secret-32-characters-long-key")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.NotEmpty(t, claims.UserID)
}

func TestTokenService_GenerateAccessToken_IncludesIssuerAndAudience(t *testing.T) {
	ts := service.NewTokenService(
		"test-secret-32-characters-long-key",
		"go-api-base",
		"api-users",
		15*time.Minute,
		24*time.Hour,
	)

	token, err := ts.GenerateAccessToken(uuid.New().String(), "user@example.com")
	require.NoError(t, err)

	claims, err := auth.GetClaims(token, "test-secret-32-characters-long-key")
	require.NoError(t, err)
	assert.Equal(t, "go-api-base", claims.Issuer)
}

func TestTokenService_GenerateRefreshToken(t *testing.T) {
	ts := service.NewTokenService("s", "i", "a", time.Minute, time.Hour)

	token, hash, err := ts.GenerateRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, hash)
	assert.Equal(t, 64, len(token))
	assert.Equal(t, auth.HashToken(token), hash)
}

func TestTokenService_GenerateRefreshToken_ProducesUniqueTokens(t *testing.T) {
	ts := service.NewTokenService("s", "i", "a", time.Minute, time.Hour)

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		token, hash, err := ts.GenerateRefreshToken()
		require.NoError(t, err)
		assert.False(t, seen[token], "duplicate token generated")
		assert.False(t, seen[hash], "duplicate hash generated")
		assert.Equal(t, auth.HashToken(token), hash)
		seen[token] = true
		seen[hash] = true
	}
}

func TestTokenService_GetRefreshExpiry(t *testing.T) {
	expiry := 7 * 24 * time.Hour
	ts := service.NewTokenService("s", "i", "a", time.Minute, expiry)
	assert.Equal(t, expiry, ts.GetRefreshExpiry())
}

// -- Refresh tests --

func TestAuthService_Refresh_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	sessionID := uuid.New()
	userID := uuid.New()
	familyID := uuid.New()

	rawToken, tokenHash, err := service.NewTokenService("s", "i", "a", time.Minute, time.Hour).GenerateRefreshToken()
	require.NoError(t, err)

	storedToken := &domain.RefreshToken{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	existingUser := &domain.User{
		ID:    userID,
		Email: "test@example.com",
	}

	refreshTokenRepo.On("FindByHash", ctx, tokenHash).Return(storedToken, nil)
	refreshTokenRepo.On("MarkRevoked", ctx, tokenHash).Return(nil)
	userRepo.On("FindByID", ctx, userID).Return(existingUser, nil)
	refreshTokenRepo.On("Create", ctx, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	user, accessToken, newRefreshToken, err := authSvc.Refresh(ctx, rawToken)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, newRefreshToken)
	assert.NotEqual(t, rawToken, newRefreshToken)

	refreshTokenRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestAuthService_Refresh_TokenNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	rawToken := "some-token-that-does-not-exist"
	tokenHash := auth.HashToken(rawToken)

	refreshTokenRepo.On("FindByHash", ctx, tokenHash).Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	user, accessToken, refreshToken, err := authSvc.Refresh(ctx, rawToken)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_Refresh_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	rawToken, tokenHash, err := service.NewTokenService("s", "i", "a", time.Minute, time.Hour).GenerateRefreshToken()
	require.NoError(t, err)

	expiredToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: tokenHash,
		FamilyID:  uuid.New(),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	refreshTokenRepo.On("FindByHash", ctx, tokenHash).Return(expiredToken, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	user, accessToken, refreshToken, err := authSvc.Refresh(ctx, rawToken)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_Refresh_RevokedToken_FamilyRevoked(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	now := time.Now()
	rawToken, tokenHash, err := service.NewTokenService("s", "i", "a", time.Minute, time.Hour).GenerateRefreshToken()
	require.NoError(t, err)

	userID := uuid.New()
	familyID := uuid.New()

	revokedToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		RevokedAt: &now,
	}

	refreshTokenRepo.On("FindByHash", ctx, tokenHash).Return(revokedToken, nil)
	refreshTokenRepo.On("RevokeFamily", ctx, familyID).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	user, accessToken, refreshToken, err := authSvc.Refresh(ctx, rawToken)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_Refresh_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	rawToken, tokenHash, err := service.NewTokenService("s", "i", "a", time.Minute, time.Hour).GenerateRefreshToken()
	require.NoError(t, err)

	validToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		FamilyID:  uuid.New(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	refreshTokenRepo.On("FindByHash", ctx, tokenHash).Return(validToken, nil)
	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	user, accessToken, refreshToken, err := authSvc.Refresh(ctx, rawToken)
	require.Error(t, err)
	assert.Nil(t, user)
	assert.Empty(t, accessToken)
	assert.Empty(t, refreshToken)

	refreshTokenRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// -- Logout tests --

func TestAuthService_Logout_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("RevokeAllByUser", ctx, userID).Return(nil)
	resetTokenRepo.On("RevokeAllByUser", ctx, userID).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.Logout(ctx, userID)
	require.NoError(t, err)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_Logout_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("RevokeAllByUser", ctx, userID).Return(apperrors.ErrInternal)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.Logout(ctx, userID)
	require.Error(t, err)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_Logout_ResetTokenRevokeFailureContinues(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("RevokeAllByUser", ctx, userID).Return(nil)
	resetTokenRepo.On("RevokeAllByUser", ctx, userID).Return(apperrors.ErrInternal)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.Logout(ctx, userID)
	require.NoError(t, err)

	refreshTokenRepo.AssertExpectations(t)
}

// -- GetActiveSessions tests --

func TestAuthService_GetActiveSessions_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "hash1",
			FamilyID:  uuid.New(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "hash2",
			FamilyID:  uuid.New(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	active, err := authSvc.GetActiveSessions(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_GetActiveSessions_FiltersExpired(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "valid",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "expired",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	active, err := authSvc.GetActiveSessions(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, "valid", active[0].TokenHash)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_GetActiveSessions_FiltersRevoked(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	now := time.Now()
	userID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "revoked",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			RevokedAt: &now,
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	active, err := authSvc.GetActiveSessions(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, active, 0)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_GetActiveSessions_EmptyList(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return([]*domain.RefreshToken{}, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	active, err := authSvc.GetActiveSessions(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, active, 0)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_GetActiveSessions_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(nil, apperrors.ErrInternal)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	active, err := authSvc.GetActiveSessions(ctx, userID)
	require.Error(t, err)
	assert.Nil(t, active)

	refreshTokenRepo.AssertExpectations(t)
}

// -- RevokeSession tests --

func TestAuthService_RevokeSession_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	sessionID := uuid.New()

	session := &domain.RefreshToken{
		ID:        sessionID,
		UserID:    userID,
		TokenHash: "session-hash",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	refreshTokenRepo.On("FindByID", ctx, sessionID).Return(session, nil)
	refreshTokenRepo.On("MarkRevoked", ctx, "session-hash").Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.RevokeSession(ctx, userID, sessionID)
	require.NoError(t, err)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeSession_NotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	sessionID := uuid.New()

	refreshTokenRepo.On("FindByID", ctx, sessionID).Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.RevokeSession(ctx, uuid.New(), sessionID)
	require.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeSession_WrongOwner(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	ownerID := uuid.New()
	otherUserID := uuid.New()
	sessionID := uuid.New()

	session := &domain.RefreshToken{
		ID:        sessionID,
		UserID:    ownerID,
		TokenHash: "hash",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	refreshTokenRepo.On("FindByID", ctx, sessionID).Return(session, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.RevokeSession(ctx, otherUserID, sessionID)
	require.Error(t, err)

	var appErr *apperrors.AppError
	require.True(t, apperrors.IsAppError(err))
	appErr = apperrors.GetAppError(err)
	assert.Equal(t, "FORBIDDEN", appErr.Code)

	refreshTokenRepo.AssertExpectations(t)
}

// -- RevokeAllOtherSessions tests --

func TestAuthService_RevokeAllOtherSessions_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	currentSessionID := uuid.New()
	otherSessionID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        currentSessionID,
			UserID:    userID,
			TokenHash: "current-hash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
		{
			ID:        otherSessionID,
			UserID:    userID,
			TokenHash: "other-hash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	currentSession := &domain.RefreshToken{
		ID:        currentSessionID,
		UserID:    userID,
		TokenHash: "current-hash",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)
	refreshTokenRepo.On("FindByID", ctx, currentSessionID).Return(currentSession, nil)
	refreshTokenRepo.On("MarkRevoked", ctx, mock.AnythingOfType("string")).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	count, err := authSvc.RevokeAllOtherSessions(ctx, userID, currentSessionID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeAllOtherSessions_NilCurrentID(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "hash-1",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "hash-2",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)
	refreshTokenRepo.On("MarkRevoked", ctx, mock.AnythingOfType("string")).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	count, err := authSvc.RevokeAllOtherSessions(ctx, userID, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeAllOtherSessions_AlreadyRevoked(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	now := time.Now()
	userID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "already-revoked",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			RevokedAt: &now,
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	count, err := authSvc.RevokeAllOtherSessions(ctx, userID, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeAllOtherSessions_CurrentSessionNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	unknownSessionID := uuid.New()

	sessions := []*domain.RefreshToken{
		{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "other-hash",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(sessions, nil)
	refreshTokenRepo.On("FindByID", ctx, unknownSessionID).Return(nil, apperrors.ErrNotFound)
	refreshTokenRepo.On("MarkRevoked", ctx, "other-hash").Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	count, err := authSvc.RevokeAllOtherSessions(ctx, userID, unknownSessionID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	refreshTokenRepo.AssertExpectations(t)
}

func TestAuthService_RevokeAllOtherSessions_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()

	refreshTokenRepo.On("FindByUserID", ctx, userID).Return(nil, apperrors.ErrInternal)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	count, err := authSvc.RevokeAllOtherSessions(ctx, userID, uuid.Nil)
	require.Error(t, err)
	assert.Equal(t, 0, count)

	refreshTokenRepo.AssertExpectations(t)
}

// -- RequestPasswordReset tests --

func TestAuthService_RequestPasswordReset_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "test@example.com",
	}

	userRepo.On("FindByEmail", ctx, "test@example.com").Return(existingUser, nil)
	resetTokenRepo.On("RevokeAllByUser", ctx, existingUser.ID).Return(nil)
	resetTokenRepo.On("Create", ctx, mock.AnythingOfType("*domain.PasswordResetToken")).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	token, err := authSvc.RequestPasswordReset(ctx, "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Len(t, token, 64)

	userRepo.AssertExpectations(t)
	resetTokenRepo.AssertExpectations(t)
}

func TestAuthService_RequestPasswordReset_UserNotFound_Silent(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userRepo.On("FindByEmail", ctx, "nonexistent@example.com").Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	token, err := authSvc.RequestPasswordReset(ctx, "nonexistent@example.com")
	require.NoError(t, err)
	assert.Empty(t, token)

	userRepo.AssertExpectations(t)
}

func TestAuthService_RequestPasswordReset_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userRepo.On("FindByEmail", ctx, "test@example.com").Return(nil, apperrors.ErrInternal)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	token, err := authSvc.RequestPasswordReset(ctx, "test@example.com")
	require.Error(t, err)
	assert.Empty(t, token)

	userRepo.AssertExpectations(t)
}

// -- ResetPassword tests --

func TestAuthService_ResetPassword_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	rawToken := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	hashedToken := auth.HashToken(rawToken)

	resetToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	existingUser := &domain.User{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: "old-hash",
	}

	resetTokenRepo.On("FindByHash", ctx, hashedToken).Return(resetToken, nil)
	userRepo.On("FindByID", ctx, userID).Return(existingUser, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*domain.User")).Return(nil)
	resetTokenRepo.On("MarkUsed", ctx, hashedToken).Return(nil)
	refreshTokenRepo.On("RevokeAllByUser", ctx, userID).Return(nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.ResetPassword(ctx, rawToken, "NewPassword123!")
	require.NoError(t, err)

	refreshTokenRepo.AssertExpectations(t)
	resetTokenRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestAuthService_ResetPassword_InvalidToken(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	rawToken := "invalid-token-that-does-not-exist"
	hashedToken := auth.HashToken(rawToken)

	resetTokenRepo.On("FindByHash", ctx, hashedToken).Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.ResetPassword(ctx, rawToken, "NewPassword123!")
	require.Error(t, err)

	var appErr *apperrors.AppError
	require.True(t, apperrors.IsAppError(err))
	appErr = apperrors.GetAppError(err)
	assert.Equal(t, "INVALID_RESET_TOKEN", appErr.Code)

	resetTokenRepo.AssertExpectations(t)
}

func TestAuthService_ResetPassword_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	rawToken := "expired-token-abcdef1234567890abcdef1234567890abcdef1234567890ab"
	hashedToken := auth.HashToken(rawToken)

	expiredResetToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	resetTokenRepo.On("FindByHash", ctx, hashedToken).Return(expiredResetToken, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.ResetPassword(ctx, rawToken, "NewPassword123!")
	require.Error(t, err)

	var appErr *apperrors.AppError
	require.True(t, apperrors.IsAppError(err))
	appErr = apperrors.GetAppError(err)
	assert.Equal(t, "INVALID_RESET_TOKEN", appErr.Code)

	resetTokenRepo.AssertExpectations(t)
}

func TestAuthService_ResetPassword_UsedToken(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	now := time.Now()
	rawToken := "used-token-abcdef1234567890abcdef1234567890abcdef1234567890ab"
	hashedToken := auth.HashToken(rawToken)

	usedResetToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		UsedAt:    &now,
	}

	resetTokenRepo.On("FindByHash", ctx, hashedToken).Return(usedResetToken, nil)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.ResetPassword(ctx, rawToken, "NewPassword123!")
	require.Error(t, err)

	var appErr *apperrors.AppError
	require.True(t, apperrors.IsAppError(err))
	appErr = apperrors.GetAppError(err)
	assert.Equal(t, "INVALID_RESET_TOKEN", appErr.Code)

	resetTokenRepo.AssertExpectations(t)
}

func TestAuthService_ResetPassword_UserNotFoundAfterTokenLookup(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	userID := uuid.New()
	rawToken := "orphan-token-abcdef1234567890abcdef1234567890abcdef123456789012"
	hashedToken := auth.HashToken(rawToken)

	validToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	resetTokenRepo.On("FindByHash", ctx, hashedToken).Return(validToken, nil)
	userRepo.On("FindByID", ctx, userID).Return(nil, apperrors.ErrNotFound)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	err := authSvc.ResetPassword(ctx, rawToken, "NewPassword123!")
	require.Error(t, err)

	resetTokenRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

// -- Auth Audit Tests (with async audit service) --

type MockAuditLogRepository struct {
	mock.Mock
	CreateCalled atomic.Bool
}

func (m *MockAuditLogRepository) Create(ctx context.Context, auditLog *domain.AuditLog) error {
	args := m.Called(ctx, auditLog)
	m.CreateCalled.Store(true)
	return args.Error(0)
}

func (m *MockAuditLogRepository) FindByActorID(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, actorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepository) FindByResource(ctx context.Context, resource, resourceID string, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, resource, resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepository) FindAll(ctx context.Context, limit, offset int) ([]domain.AuditLog, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AuditLog), args.Error(1)
}

func newTestAuthServiceWithAudit(
	userRepo *MockUserRepositoryAuth,
	refreshTokenRepo *MockRefreshTokenRepositoryAuth,
	resetTokenRepo *MockPasswordResetTokenRepositoryAuth,
	roleRepo *MockRoleRepositoryAuth,
	userRoleRepo *MockUserRoleRepositoryAuth,
	auditRepo repository.AuditLogRepository,
) *service.AuthService {
	tokenService := service.NewTokenService(
		"test-secret-key-32-characters-long",
		"go-api-base",
		"go-api-base",
		15*time.Minute,
		24*time.Hour,
	)
	passwordHasher := service.NewPasswordHasher()
	auditSvc := service.NewAuditService(auditRepo, service.AuditServiceConfig{BufferSize: 10})

	authSvc := service.NewAuthService(
		userRepo,
		refreshTokenRepo,
		resetTokenRepo,
		tokenService,
		passwordHasher,
		nil,
		auditSvc,
		5*time.Minute,
		roleRepo,
		userRoleRepo,
	)
	return authSvc
}

func TestAuthService_AuditLoginSuccess(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	mockAuditRepo := new(MockAuditLogRepository)
	mockAuditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	authSvc := newTestAuthServiceWithAudit(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo, mockAuditRepo)

	userID := uuid.New()
	authSvc.AuditLoginSuccess(ctx, userID, "test@example.com", "127.0.0.1", "Mozilla/5.0")

	require.Eventually(t, func() bool {
		return mockAuditRepo.CreateCalled.Load()
	}, 5*time.Second, 10*time.Millisecond)

	mockAuditRepo.AssertExpectations(t)
}

func TestAuthService_AuditLoginFailure(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	mockAuditRepo := new(MockAuditLogRepository)
	mockAuditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	authSvc := newTestAuthServiceWithAudit(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo, mockAuditRepo)

	authSvc.AuditLoginFailure(ctx, "bad@example.com", "10.0.0.1", "curl/7.0", "invalid_password")

	require.Eventually(t, func() bool {
		return mockAuditRepo.CreateCalled.Load()
	}, 5*time.Second, 10*time.Millisecond)

	mockAuditRepo.AssertExpectations(t)
}

func TestAuthService_AuditPasswordResetRequest(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	mockAuditRepo := new(MockAuditLogRepository)
	mockAuditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	authSvc := newTestAuthServiceWithAudit(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo, mockAuditRepo)

	userID := uuid.New()
	authSvc.AuditPasswordResetRequest(ctx, userID, "user@example.com", "192.168.1.1", "Chrome/120")

	require.Eventually(t, func() bool {
		return mockAuditRepo.CreateCalled.Load()
	}, 5*time.Second, 10*time.Millisecond)

	mockAuditRepo.AssertExpectations(t)
}

func TestAuthService_AuditPasswordResetComplete(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	mockAuditRepo := new(MockAuditLogRepository)
	mockAuditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	authSvc := newTestAuthServiceWithAudit(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo, mockAuditRepo)

	userID := uuid.New()
	authSvc.AuditPasswordResetComplete(ctx, userID, "user@example.com", "192.168.1.1", "Firefox/115")

	require.Eventually(t, func() bool {
		return mockAuditRepo.CreateCalled.Load()
	}, 5*time.Second, 10*time.Millisecond)

	mockAuditRepo.AssertExpectations(t)
}

func TestAuthService_AuditPotentialTokenReuse(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	mockAuditRepo := new(MockAuditLogRepository)
	mockAuditRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	authSvc := newTestAuthServiceWithAudit(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo, mockAuditRepo)

	userID := uuid.New()
	familyID := uuid.New()
	authSvc.AuditPotentialTokenReuse(ctx, userID, familyID)

	require.Eventually(t, func() bool {
		return mockAuditRepo.CreateCalled.Load()
	}, 5*time.Second, 10*time.Millisecond)

	mockAuditRepo.AssertExpectations(t)
}

func TestAuthService_AuditLoginSuccess_NilAuditService(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	assert.NotPanics(t, func() {
		authSvc.AuditLoginSuccess(ctx, uuid.New(), "test@example.com", "127.0.0.1", "agent")
	})
}

func TestAuthService_AuditLoginFailure_NilAuditService(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepositoryAuth)
	refreshTokenRepo := new(MockRefreshTokenRepositoryAuth)
	resetTokenRepo := new(MockPasswordResetTokenRepositoryAuth)
	roleRepo := new(MockRoleRepositoryAuth)
	userRoleRepo := new(MockUserRoleRepositoryAuth)

	authSvc := newTestAuthService(userRepo, refreshTokenRepo, resetTokenRepo, roleRepo, userRoleRepo)

	assert.NotPanics(t, func() {
		authSvc.AuditLoginFailure(ctx, "test@example.com", "127.0.0.1", "agent", "reason")
	})
}

// -- Auth Metrics tests --

func TestAuthMetrics_AllIncrementMethods(t *testing.T) {
	m := service.GetAuthMetrics()
	m.Reset()

	m.IncrementLoginSuccess()
	m.IncrementLoginSuccess()
	m.IncrementLoginFailed()
	m.IncrementLoginRateLimited()
	m.IncrementPasswordResetRequested()
	m.IncrementPasswordResetCompleted()
	m.IncrementPasswordResetFailed()
	m.IncrementTokenRefreshSuccess()
	m.IncrementTokenRefreshFailed()
	m.IncrementTokenReuseDetected()
	m.IncrementSessionRevoked()
	m.SetActiveSessions(5)

	snapshot := m.Snapshot()

	assert.Equal(t, int64(2), snapshot.LoginSuccess)
	assert.Equal(t, int64(1), snapshot.LoginFailed)
	assert.Equal(t, int64(1), snapshot.LoginRateLimited)
	assert.Equal(t, int64(1), snapshot.PasswordResetRequested)
	assert.Equal(t, int64(1), snapshot.PasswordResetCompleted)
	assert.Equal(t, int64(1), snapshot.PasswordResetFailed)
	assert.Equal(t, int64(1), snapshot.TokenRefreshSuccess)
	assert.Equal(t, int64(1), snapshot.TokenRefreshFailed)
	assert.Equal(t, int64(1), snapshot.TokenReuseDetected)
	assert.Equal(t, int64(5), snapshot.ActiveSessions)
	assert.Equal(t, int64(1), snapshot.SessionRevoked)
}

func TestAuthMetrics_Snapshot_IndependentOfSubsequentChanges(t *testing.T) {
	m := service.GetAuthMetrics()
	m.Reset()

	m.IncrementLoginSuccess()
	snapshot1 := m.Snapshot()
	m.IncrementLoginSuccess()
	snapshot2 := m.Snapshot()

	assert.Equal(t, int64(1), snapshot1.LoginSuccess)
	assert.Equal(t, int64(2), snapshot2.LoginSuccess)
}

func TestAuthMetrics_Reset(t *testing.T) {
	m := service.GetAuthMetrics()

	m.IncrementLoginSuccess()
	m.IncrementLoginFailed()
	m.IncrementTokenRefreshSuccess()
	m.IncrementSessionRevoked()
	m.SetActiveSessions(10)

	m.Reset()

	snapshot := m.Snapshot()
	assert.Equal(t, int64(0), snapshot.LoginSuccess)
	assert.Equal(t, int64(0), snapshot.LoginFailed)
	assert.Equal(t, int64(0), snapshot.TokenRefreshSuccess)
	assert.Equal(t, int64(0), snapshot.SessionRevoked)
	assert.Equal(t, int64(0), snapshot.ActiveSessions)
}

func TestAuthMetrics_Concurrency(t *testing.T) {
	m := service.GetAuthMetrics()
	m.Reset()

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				m.IncrementLoginSuccess()
				m.IncrementLoginFailed()
				m.IncrementTokenRefreshSuccess()
				m.IncrementTokenRefreshFailed()
				m.IncrementSessionRevoked()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	snapshot := m.Snapshot()
	assert.Equal(t, int64(100000), snapshot.LoginSuccess)
	assert.Equal(t, int64(100000), snapshot.LoginFailed)
	assert.Equal(t, int64(100000), snapshot.TokenRefreshSuccess)
	assert.Equal(t, int64(100000), snapshot.TokenRefreshFailed)
	assert.Equal(t, int64(100000), snapshot.SessionRevoked)
}

func TestAuthMetrics_GetAuthMetrics_Singleton(t *testing.T) {
	m1 := service.GetAuthMetrics()
	m2 := service.GetAuthMetrics()
	assert.Same(t, m1, m2)
}

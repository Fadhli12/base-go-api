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
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

func TestAuthLogin(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	// Run migrations
	suite.RunMigrations(t)

	// Create dependencies
	userRepo := repository.NewUserRepository(suite.DB)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)
	testSecret := "test-secret-key-min-32-chars-long!"
	tokenService := service.NewTokenService(
		testSecret,
		"go-api-base",
		"go-api-base",
		15*time.Minute,
		24*time.Hour,
	)
	passwordHasher := &passwordHasherTestService{}
	resetTokenRepo := repository.NewPasswordResetTokenRepository(suite.DB)
	emailService := service.NewEmailService(nil, nil, nil, nil, nil, nil)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	authService := service.NewAuthService(userRepo, tokenRepo, resetTokenRepo, tokenService, passwordHasher, emailService, auditService, 15*time.Minute, roleRepo, userRoleRepo)

	ctx := context.Background()

	// Helper to register a user for login tests
	registerUser := func(t *testing.T, email, password string) *domain.User {
		req := &request.RegisterRequest{
			Email:    email,
			Password: password,
		}
		user, err := authService.Register(ctx, req)
		require.NoError(t, err, "User registration should succeed")
		return user
	}

	t.Run("Login with valid credentials returns tokens", func(t *testing.T) {
		suite.SetupTest(t)

		email := "login@example.com"
		password := "ValidPassword123!"
		registerUser(t, email, password)

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}

		user, accessToken, refreshToken, err := authService.Login(ctx, req)

		require.NoError(t, err, "Login should succeed")
		require.NotNil(t, user, "User should not be nil")
		assert.NotEmpty(t, accessToken, "Access token should be returned")
		assert.NotEmpty(t, refreshToken, "Refresh token should be returned")
		assert.Equal(t, email, user.Email, "Email should match")
	})

	t.Run("Login with invalid email returns INVALID_CREDENTIALS error", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "SomePassword123!",
		}

		user, accessToken, refreshToken, err := authService.Login(ctx, req)

		require.Error(t, err, "Login should fail with nonexistent email")
		assert.Nil(t, user, "User should be nil on error")
		assert.Empty(t, accessToken, "Access token should be empty")
		assert.Empty(t, refreshToken, "Refresh token should be empty")

		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr, "Error should be AppError")
		assert.Equal(t, "INVALID_CREDENTIALS", appErr.Code, "Error code should be INVALID_CREDENTIALS")
		assert.Equal(t, 401, appErr.HTTPStatus, "HTTP status should be 401 Unauthorized")
	})

	t.Run("Login with wrong password returns INVALID_CREDENTIALS error", func(t *testing.T) {
		suite.SetupTest(t)

		email := "wrongpass@example.com"
		registerUser(t, email, "CorrectPassword123!")

		req := &request.LoginRequest{
			Email:    email,
			Password: "WrongPassword123!",
		}

		user, accessToken, refreshToken, err := authService.Login(ctx, req)

		require.Error(t, err, "Login should fail with wrong password")
		assert.Nil(t, user, "User should be nil on error")
		assert.Empty(t, accessToken, "Access token should be empty")
		assert.Empty(t, refreshToken, "Refresh token should be empty")

		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr, "Error should be AppError")
		assert.Equal(t, "INVALID_CREDENTIALS", appErr.Code, "Error code should be INVALID_CREDENTIALS")
		assert.Equal(t, 401, appErr.HTTPStatus, "HTTP status should be 401 Unauthorized")
	})

	t.Run("Access token is valid JWT with correct claims", func(t *testing.T) {
		suite.SetupTest(t)

		email := "jwtclaims@example.com"
		password := "Password123!"
		_ = registerUser(t, email, password)

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}

		user, accessToken, _, err := authService.Login(ctx, req)
		require.NoError(t, err, "Login should succeed")

		// Parse and validate the access token
		claims, err := auth.GetClaims(accessToken, testSecret)
		require.NoError(t, err, "Access token should be valid")

		// Verify claims
		assert.Equal(t, user.ID.String(), claims.UserID, "UserID claim should match")
		assert.Equal(t, user.Email, claims.Email, "Email claim should match")
		assert.Equal(t, user.ID.String(), claims.Subject, "Subject claim should match user ID")

		// Verify expiration is set
		assert.NotZero(t, claims.ExpiresAt, "Expiration should be set")
		assert.True(t, claims.ExpiresAt.Time.After(time.Now()), "Token should expire in the future")

		// Verify issued at is set
		assert.NotZero(t, claims.IssuedAt, "IssuedAt should be set")
		assert.True(t, claims.IssuedAt.Time.Before(time.Now()) || claims.IssuedAt.Time.Equal(time.Now()),
			"Token should be issued in the past or now")

		// Verify the token is a valid JWT structure
		parsedToken, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
			return []byte(testSecret), nil
		})
		require.NoError(t, err, "Token should parse successfully")
		assert.True(t, parsedToken.Valid, "Token should be valid")
		assert.IsType(t, &jwt.SigningMethodHMAC{}, parsedToken.Method,
			"Signing method should be HMAC")
		assert.Equal(t, "HS256", parsedToken.Method.Alg(), "Algorithm should be HS256")
	})

	t.Run("Refresh token is stored in database", func(t *testing.T) {
		suite.SetupTest(t)

		email := "refreshtoken@example.com"
		password := "Password123!"
		_ = registerUser(t, email, password)

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}

		user, _, refreshToken, err := authService.Login(ctx, req)
		require.NoError(t, err, "Login should succeed")
		require.NotEmpty(t, refreshToken, "Refresh token should be returned")

		// Hash the refresh token to look it up (same way the service does - SHA256)
		tokenHash := auth.HashToken(refreshToken)

		// Find the token in the database
		storedToken, err := tokenRepo.FindByHash(ctx, tokenHash)
		require.NoError(t, err, "Refresh token should be found in database")
		require.NotNil(t, storedToken, "Stored token should not be nil")

		// Verify token properties
		assert.Equal(t, user.ID, storedToken.UserID, "Token should belong to the user")
		assert.Equal(t, tokenHash, storedToken.TokenHash, "Token hash should match")
		assert.True(t, storedToken.ExpiresAt.After(time.Now()), "Token should not be expired")
		assert.Nil(t, storedToken.RevokedAt, "Token should not be revoked")
		assert.True(t, storedToken.IsValid(), "Token should be valid")
	})

	t.Run("Multiple logins create multiple refresh tokens", func(t *testing.T) {
		suite.SetupTest(t)

		email := "multiplelogin@example.com"
		password := "Password123!"
		registerUser(t, email, password)

		// Login multiple times
		refreshTokens := make([]string, 3)
		for i := 0; i < 3; i++ {
			req := &request.LoginRequest{
				Email:    email,
				Password: password,
			}
			_, _, refreshToken, err := authService.Login(ctx, req)
			require.NoError(t, err, "Login %d should succeed", i+1)
			refreshTokens[i] = refreshToken
		}

		// Verify all tokens are unique
		assert.NotEqual(t, refreshTokens[0], refreshTokens[1], "Refresh tokens should be unique")
		assert.NotEqual(t, refreshTokens[1], refreshTokens[2], "Refresh tokens should be unique")
		assert.NotEqual(t, refreshTokens[0], refreshTokens[2], "Refresh tokens should be unique")

		// Verify user has multiple refresh tokens
		user, err := userRepo.FindByEmail(ctx, email)
		require.NoError(t, err, "User should be found")

		tokens, err := tokenRepo.FindByUserID(ctx, user.ID)
		require.NoError(t, err, "Tokens should be found")
		assert.Len(t, tokens, 3, "User should have 3 refresh tokens")

		// Verify all tokens are valid
		for _, token := range tokens {
			assert.True(t, token.IsValid(), "Token should be valid")
		}
	})

	t.Run("Login updates user last activity", func(t *testing.T) {
		suite.SetupTest(t)

		email := "activity@example.com"
		password := "Password123!"
		registerUser(t, email, password)

		// Get user before login to check timestamps
		userBefore, err := userRepo.FindByEmail(ctx, email)
		require.NoError(t, err, "User should be found before login")
		createdAt := userBefore.CreatedAt

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}
		_, _, _, err = authService.Login(ctx, req)
		require.NoError(t, err, "Login should succeed")

		// User should still exist and have same email
		userAfter, err := userRepo.FindByEmail(ctx, email)
		require.NoError(t, err, "User should be found after login")
		assert.Equal(t, email, userAfter.Email, "Email should remain the same")
		assert.Equal(t, createdAt.Truncate(time.Second), userAfter.CreatedAt.Truncate(time.Second),
			"CreatedAt should not change on login")
	})

	t.Run("User ID in token matches registered user", func(t *testing.T) {
		suite.SetupTest(t)

		email := "useridmatch@example.com"
		password := "Password123!"
		registeredUser := registerUser(t, email, password)

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}

		_, accessToken, _, err := authService.Login(ctx, req)
		require.NoError(t, err, "Login should succeed")

		claims, err := auth.GetClaims(accessToken, testSecret)
		require.NoError(t, err, "Should be able to parse token")

		assert.Equal(t, registeredUser.ID.String(), claims.UserID,
			"UserID in token should match registered user's ID")
	})

	t.Run("Refresh token has correct expiry duration", func(t *testing.T) {
		suite.SetupTest(t)

		email := "expiry@example.com"
		password := "Password123!"
		registerUser(t, email, password)

		req := &request.LoginRequest{
			Email:    email,
			Password: password,
		}

		_, _, refreshToken, err := authService.Login(ctx, req)
		require.NoError(t, err, "Login should succeed")

		tokenHash := auth.HashToken(refreshToken)

		storedToken, err := tokenRepo.FindByHash(ctx, tokenHash)
		require.NoError(t, err, "Token should be found")

		// Verify expiry is approximately 24 hours from now (matching TokenService refreshExpiry config)
		expectedExpiry := time.Now().Add(24 * time.Hour)
		actualExpiry := storedToken.ExpiresAt

		// Allow 1 minute tolerance for test execution time
		diff := actualExpiry.Sub(expectedExpiry)
		assert.True(t, diff.Abs() < time.Minute,
			"Expiry should be approximately 24 hours from now, diff: %v", diff)
	})

	t.Run("Login fails validation with empty email", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.LoginRequest{
			Email:    "",
			Password: "Password123!",
		}

		err := req.Validate()
		assert.Error(t, err, "Validation should fail with empty email")
	})

	t.Run("Login fails validation with empty password", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.LoginRequest{
			Email:    "test@example.com",
			Password: "",
		}

		err := req.Validate()
		assert.Error(t, err, "Validation should fail with empty password")
	})
}
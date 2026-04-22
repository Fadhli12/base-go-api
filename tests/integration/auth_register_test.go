//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

// passwordHasherTestService implements PasswordHasher for testing
type passwordHasherTestService struct{}

func (p *passwordHasherTestService) Hash(password string) (string, error) {
	return auth.Hash(password)
}

func (p *passwordHasherTestService) Verify(hashedPassword, password string) error {
	return auth.Verify(hashedPassword, password)
}

func TestAuthRegister(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	// Run migrations
	suite.RunMigrations(t)

	// Create dependencies
	userRepo := repository.NewUserRepository(suite.DB)
	tokenRepo := repository.NewRefreshTokenRepository(suite.DB)
	tokenService := service.NewTokenService("test-secret-key-min-32-chars-long!", 15*time.Minute, 168*time.Hour)
	passwordHasher := &passwordHasherTestService{}
	authService := service.NewAuthService(userRepo, tokenRepo, tokenService, passwordHasher)

	ctx := context.Background()

	t.Run("TestSuite Setup Works", func(t *testing.T) {
		suite.SetupTest(t)

		// Verify database is accessible
		var count int64
		err := suite.DB.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error
		require.NoError(t, err, "Database should be accessible")
		assert.Equal(t, int64(0), count, "Users table should be empty after setup")
	})

	t.Run("Register with valid email and password succeeds", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.RegisterRequest{
			Email:    "test@example.com",
			Password: "ValidPassword123!",
		}

		user, err := authService.Register(ctx, req)

		require.NoError(t, err, "Registration should succeed")
		require.NotNil(t, user, "User should not be nil")
		assert.NotEmpty(t, user.ID, "User ID should be set")
		assert.Equal(t, "test@example.com", user.Email, "Email should match")
		assert.NotEmpty(t, user.PasswordHash, "Password hash should be set")
		assert.NotZero(t, user.CreatedAt, "CreatedAt should be set")
		assert.NotZero(t, user.UpdatedAt, "UpdatedAt should be set")
	})

	t.Run("Register with duplicate email returns EMAIL_EXISTS error", func(t *testing.T) {
		suite.SetupTest(t)

		// Register first user
		req1 := &request.RegisterRequest{
			Email:    "duplicate@example.com",
			Password: "Password123!",
		}
		_, err := authService.Register(ctx, req1)
		require.NoError(t, err, "First registration should succeed")

		// Try to register with same email
		req2 := &request.RegisterRequest{
			Email:    "duplicate@example.com",
			Password: "DifferentPassword123!",
		}
		user, err := authService.Register(ctx, req2)

		require.Error(t, err, "Registration should fail with duplicate email")
		assert.Nil(t, user, "User should be nil on error")

		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr, "Error should be AppError")
		assert.Equal(t, "EMAIL_EXISTS", appErr.Code, "Error code should be EMAIL_EXISTS")
		assert.Equal(t, 409, appErr.HTTPStatus, "HTTP status should be 409 Conflict")
	})

	t.Run("Register with invalid email returns validation error", func(t *testing.T) {
		suite.SetupTest(t)

		invalidEmails := []string{
			"notanemail",
			"missing@domain",
			"@missinglocal.com",
			"spaces in@email.com",
			"",
		}

		for _, email := range invalidEmails {
			req := &request.RegisterRequest{
				Email:    email,
				Password: "ValidPassword123!",
			}

			err := req.Validate()
			assert.Error(t, err, "Validation should fail for email: %s", email)
		}
	})

	t.Run("Register with weak password returns validation error", func(t *testing.T) {
		suite.SetupTest(t)

		weakPasswords := []struct {
			password string
			reason   string
		}{
			{"short", "too short (< 8 chars)"},
			{"", "empty"},
			{"    ", "whitespace only"},
		}

		for _, tc := range weakPasswords {
			req := &request.RegisterRequest{
				Email:    "test@example.com",
				Password: tc.password,
			}

			err := req.Validate()
			assert.Error(t, err, "Validation should fail for password: %s (%s)", tc.password, tc.reason)
		}
	})

	t.Run("Password is hashed and not stored in plaintext", func(t *testing.T) {
		suite.SetupTest(t)

		plainPassword := "MySecurePassword123!"
		req := &request.RegisterRequest{
			Email:    "hashcheck@example.com",
			Password: plainPassword,
		}

		user, err := authService.Register(ctx, req)
		require.NoError(t, err, "Registration should succeed")

		// Verify password is not stored in plaintext
		assert.NotEqual(t, plainPassword, user.PasswordHash, "Password should not be stored in plaintext")
		assert.True(t, len(user.PasswordHash) > 20, "Password hash should be reasonably long")

		// Verify it's a valid bcrypt hash
		assert.True(t, strings.HasPrefix(user.PasswordHash, "$2a$") || 
			strings.HasPrefix(user.PasswordHash, "$2b$"), 
			"Password hash should be bcrypt format")

		// Verify the password can be verified
		err = passwordHasher.Verify(user.PasswordHash, plainPassword)
		assert.NoError(t, err, "Password verification should succeed with correct password")

		// Verify wrong password fails
		err = passwordHasher.Verify(user.PasswordHash, "WrongPassword")
		assert.Error(t, err, "Password verification should fail with wrong password")
	})

	t.Run("User can be found after registration", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.RegisterRequest{
			Email:    "findable@example.com",
			Password: "Password123!",
		}

		registeredUser, err := authService.Register(ctx, req)
		require.NoError(t, err, "Registration should succeed")

		// Find by email
		foundUser, err := userRepo.FindByEmail(ctx, "findable@example.com")
		require.NoError(t, err, "User should be found by email")
		assert.Equal(t, registeredUser.ID, foundUser.ID, "User IDs should match")

		// Find by ID
		foundByID, err := userRepo.FindByID(ctx, registeredUser.ID)
		require.NoError(t, err, "User should be found by ID")
		assert.Equal(t, registeredUser.Email, foundByID.Email, "Emails should match")
	})

	t.Run("Multiple users can be registered", func(t *testing.T) {
		suite.SetupTest(t)

		users := []struct {
			email    string
			password string
		}{
			{"user1@example.com", "Password1!"},
			{"user2@example.com", "Password2!"},
			{"user3@example.com", "Password3!"},
		}

		for _, u := range users {
			req := &request.RegisterRequest{
				Email:    u.email,
				Password: u.password,
			}
			user, err := authService.Register(ctx, req)
			require.NoError(t, err, "Registration of %s should succeed", u.email)
			assert.NotEmpty(t, user.ID, "User ID should be set")
		}

		// Verify all users exist
		allUsers, err := userRepo.FindAll(ctx)
		require.NoError(t, err, "Should be able to list users")
		assert.Len(t, allUsers, 3, "Should have 3 users")
	})

	t.Run("Email is case-insensitive for duplicate check", func(t *testing.T) {
		suite.SetupTest(t)

		req := &request.RegisterRequest{
			Email:    "lowercase@example.com",
			Password: "Password123!",
		}
		_, err := authService.Register(ctx, req)
		require.NoError(t, err, "First registration should succeed")

		// Try with different case (database stores as-is, but unique constraint applies)
		req2 := &request.RegisterRequest{
			Email:    "LOWERCASE@example.com",
			Password: "Password123!",
		}
		_, err = authService.Register(ctx, req2)
		assert.Error(t, err, "Registration with different case should fail due to unique constraint")
	})
}
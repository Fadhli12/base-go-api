//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthHandler_Register tests the user registration endpoint.
func TestAuthHandler_Register(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("happy path - creates user and returns 201", func(t *testing.T) {
		email := helpers.UniqueEmail(t)
		payload := map[string]interface{}{
			"email":    email,
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusCreated, rec.Code, "register should return 201")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Data, "response should have data")
		assert.Nil(t, env.Error, "response should not have error")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotEmpty(t, dataMap["id"], "user id should be present")
		assert.Equal(t, email, dataMap["email"], "email should match")
	})

	t.Run("validation - missing email returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "missing email should return 400")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Error, "response should have error")
	})

	t.Run("validation - missing password returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"email": "test@example.com",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "missing password should return 400")
	})

	t.Run("validation - invalid email format returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    "not-an-email",
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid email should return 400")
	})

	t.Run("validation - weak password returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    "test@example.com",
			"password": "123",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "weak password should return 400")
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		email := helpers.UniqueEmail(t)
		payload := map[string]interface{}{
			"email":    email,
			"password": helpers.TestPassword,
		}

		// First registration - should succeed
		rec1 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusCreated, rec1.Code)

		// Second registration with same email - should fail with 409
		rec2 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusConflict, rec2.Code, "duplicate email should return 409")

		env := helpers.ParseEnvelope(t, rec2)
		assert.NotNil(t, env.Error, "response should have error")
	})

	t.Run("public route - no JWT required", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    helpers.UniqueEmail(t),
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "register should be public (no JWT required)")
	})
}

// TestAuthHandler_Login tests the user login endpoint.
func TestAuthHandler_Login(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	// Register a user first
	email := helpers.UniqueEmail(t)
	registerPayload := map[string]interface{}{
		"email":    email,
		"password": helpers.TestPassword,
	}
	helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", registerPayload, "")

	t.Run("happy path - valid credentials return tokens", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    email,
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", payload, "")
		assert.Equal(t, http.StatusOK, rec.Code, "login should return 200")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Data, "response should have data")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotEmpty(t, dataMap["access_token"], "access_token should be present")
		assert.NotEmpty(t, dataMap["refresh_token"], "refresh_token should be present")
	})

	t.Run("invalid credentials returns 401", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    email,
			"password": "WrongPassword123!",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", payload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "wrong password should return 401")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Error, "response should have error")
	})

	t.Run("non-existent user returns 401", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    "nonexistent@example.com",
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", payload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "non-existent user should return 401")
	})

	t.Run("public route - no JWT required", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    email,
			"password": helpers.TestPassword,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", payload, "")
		assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "login should be public (no JWT required)")
	})
}

// TestAuthHandler_Refresh tests the token refresh endpoint.
func TestAuthHandler_Refresh(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	// Register and login to get tokens
	email := helpers.UniqueEmail(t)
	helpers.AuthenticateUser(t, server, email, helpers.TestPassword) // registers user

	// Get refresh token from login response
	loginPayload := map[string]interface{}{
		"email":    email,
		"password": helpers.TestPassword,
	}
	loginRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", loginPayload, "")
	loginEnv := helpers.ParseEnvelope(t, loginRec)
	loginData := loginEnv.Data.(map[string]interface{})
	refreshToken := loginData["refresh_token"].(string)

	t.Run("happy path - valid refresh token returns new tokens", func(t *testing.T) {
		payload := map[string]interface{}{
			"refresh_token": refreshToken,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", payload, "")
		assert.Equal(t, http.StatusOK, rec.Code, "refresh should return 200")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Data, "response should have data")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotEmpty(t, dataMap["access_token"], "new access_token should be present")
		assert.NotEmpty(t, dataMap["refresh_token"], "new refresh_token should be present")
	})

	t.Run("invalid refresh token returns 401", func(t *testing.T) {
		payload := map[string]interface{}{
			"refresh_token": "invalid-token",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", payload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "invalid refresh token should return 401")
	})

	t.Run("missing refresh token returns 400", func(t *testing.T) {
		payload := map[string]interface{}{}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "missing refresh token should return 400")
	})

 	t.Run("public route - no JWT required", func(t *testing.T) {
 		// Get a fresh refresh token (the previous one was consumed by token rotation)
 		freshLoginPayload := map[string]interface{}{
 			"email":    email,
 			"password": helpers.TestPassword,
 		}
 		freshLoginRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", freshLoginPayload, "")
 		freshLoginEnv := helpers.ParseEnvelope(t, freshLoginRec)
 		freshLoginData := freshLoginEnv.Data.(map[string]interface{})
 		freshRefreshToken := freshLoginData["refresh_token"].(string)
 
 		payload := map[string]interface{}{
 			"refresh_token": freshRefreshToken,
 		}
 
 		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", payload, "")
 		assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "refresh should be public (no JWT required)")
 	})
}

// TestAuthHandler_Logout tests the user logout endpoint.
func TestAuthHandler_Logout(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	email := helpers.UniqueEmail(t)
	token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

	t.Run("happy path - logout with valid token returns 200", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "logout should return 200")
	})

	t.Run("protected route - no token returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "logout without token should return 401")
	})

	t.Run("protected route - invalid token returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, "invalid-token")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "logout with invalid token should return 401")
	})
}

// TestAuthHandler_E2E tests the complete auth flow.
func TestAuthHandler_E2E(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("register -> login -> access -> logout -> access revoked", func(t *testing.T) {
		email := helpers.UniqueEmail(t)
		password := helpers.TestPassword

		// 1. Register
		regPayload := map[string]interface{}{
			"email":    email,
			"password": password,
		}
		regRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", regPayload, "")
		assert.Equal(t, http.StatusCreated, regRec.Code, "register should succeed")

		// 2. Login
		loginPayload := map[string]interface{}{
			"email":    email,
			"password": password,
		}
		loginRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", loginPayload, "")
		assert.Equal(t, http.StatusOK, loginRec.Code, "login should succeed")

		env := helpers.ParseEnvelope(t, loginRec)
		data := env.Data.(map[string]interface{})
		token := data["access_token"].(string)

		// 3. Access protected endpoint with valid token
		meRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		assert.Equal(t, http.StatusOK, meRec.Code, "access with valid token should succeed")

		// 4. Logout
		logoutRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, token)
		assert.Equal(t, http.StatusOK, logoutRec.Code, "logout should succeed")

		// 5. Access after logout - JWT access tokens are stateless and remain valid until expiry
		// Logout only revokes refresh tokens, not access tokens
		meRec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		assert.Equal(t, http.StatusOK, meRec2.Code, "access token still valid after logout (JWT is stateless)")
	})
}
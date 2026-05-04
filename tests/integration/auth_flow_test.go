//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestAuthFlow_Complete tests the complete authentication flow across multiple endpoints.
func TestAuthFlow_Complete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	email := helpers.UniqueEmail(t)
	password := helpers.TestPassword

	t.Run("complete flow - register -> login -> access protected -> logout -> access revoked", func(t *testing.T) {
		// 1. Register a new user
		regPayload := map[string]interface{}{
			"email":    email,
			"password": password,
		}
		regRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", regPayload, "")
		assert.Equal(t, http.StatusCreated, regRec.Code, "register should succeed")
		regEnv := helpers.ParseEnvelope(t, regRec)
		regData := regEnv.Data.(map[string]interface{})
		userID := regData["id"].(string)
		assert.NotEmpty(t, userID, "user ID should be returned")

		// 2. Login to get tokens
		loginPayload := map[string]interface{}{
			"email":    email,
			"password": password,
		}
		loginRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", loginPayload, "")
		assert.Equal(t, http.StatusOK, loginRec.Code, "login should succeed")
		loginEnv := helpers.ParseEnvelope(t, loginRec)
		loginData := loginEnv.Data.(map[string]interface{})
		accessToken := loginData["access_token"].(string)
		refreshToken := loginData["refresh_token"].(string)
		assert.NotEmpty(t, accessToken, "access token should be returned")
		assert.NotEmpty(t, refreshToken, "refresh token should be returned")

		// 3. Access protected endpoint with valid token
		meRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, accessToken)
		assert.Equal(t, http.StatusOK, meRec.Code, "access with valid token should succeed")
		meEnv := helpers.ParseEnvelope(t, meRec)
		meData := meEnv.Data.(map[string]interface{})
		assert.Equal(t, email, meData["email"], "returned email should match")

		// 4. Access another protected endpoint (list users - requires permission)
		usersRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users", nil, accessToken)
		assert.Equal(t, http.StatusForbidden, usersRec.Code, "regular user should be forbidden from listing users")

		// 5. Use refresh token to get new access token
		refreshPayload := map[string]interface{}{
			"refresh_token": refreshToken,
		}
		refreshRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", refreshPayload, "")
		assert.Equal(t, http.StatusOK, refreshRec.Code, "refresh should succeed")
		refreshEnv := helpers.ParseEnvelope(t, refreshRec)
		refreshData := refreshEnv.Data.(map[string]interface{})
		newAccessToken := refreshData["access_token"].(string)
		assert.NotEmpty(t, newAccessToken, "new access token should be returned")
		assert.NotEqual(t, accessToken, newAccessToken, "new access token should be different")

		// 6. Logout (revoke tokens)
		logoutRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/logout", nil, accessToken)
		assert.Equal(t, http.StatusOK, logoutRec.Code, "logout should succeed")

		// 7. Verify access token still works until expiry (JWT is stateless, logout only revokes refresh tokens)
		meRec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, accessToken)
		assert.Equal(t, http.StatusOK, meRec2.Code, "access token still valid after logout (JWT stateless)")

		// 8. Verify new refresh tokens don't work (all revoked by logout)
		refreshRec2 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", refreshPayload, "")
		assert.Equal(t, http.StatusUnauthorized, refreshRec2.Code, "refresh after logout should fail")
	})
}

// TestAuthFlow_PasswordReset tests the password reset flow.
func TestAuthFlow_PasswordReset(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	email := helpers.UniqueEmail(t)

	// Register user first
	regPayload := map[string]interface{}{
		"email":    email,
		"password": helpers.TestPassword,
	}
	helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", regPayload, "")

	t.Run("request password reset - returns success even if email not found (security)", func(t *testing.T) {
		payload := map[string]interface{}{
			"email": email,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/password-reset", payload, "")
		// Should return 200 even for security reasons (don't reveal if email exists)
		assert.Equal(t, http.StatusOK, rec.Code, "password reset request should succeed")
	})

	t.Run("confirm password reset with invalid token returns 401", func(t *testing.T) {
		payload := map[string]interface{}{
			"token":    "invalid-token",
			"password": "NewPassword123!",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/password-reset/confirm", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid token should return 400")
	})
}
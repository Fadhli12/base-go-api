//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestUserHandler tests the user management endpoints.
func TestUserHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("GET /api/v1/me - get current user", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "get current user should return 200")
	})

	t.Run("GET /api/v1/users - list users (admin)", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list users should return 200")
	})

	t.Run("GET /api/v1/users/:id - get user by id", func(t *testing.T) {
		// First get current user to get an ID
		meRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		meEnv := helpers.ParseEnvelope(t, meRec)
		if user, ok := meEnv.Data.(map[string]interface{}); ok {
			userID := user["id"].(string)
			rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users/"+userID, nil, token)
			assert.Equal(t, http.StatusOK, rec.Code, "get user by id should return 200")
		}
	})

	t.Run("PUT /api/v1/users/:id - update user", func(t *testing.T) {
		// Get current user ID
		meRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		meEnv := helpers.ParseEnvelope(t, meRec)
		if user, ok := meEnv.Data.(map[string]interface{}); ok {
			userID := user["id"].(string)
			payload := map[string]interface{}{
				"name": "Updated Name",
			}
			rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/users/"+userID, payload, token)
			assert.Equal(t, http.StatusOK, rec.Code, "update user should return 200")
		}
	})

	t.Run("DELETE /api/v1/users/:id - delete user (soft delete)", func(t *testing.T) {
		// Create a user to delete
		regPayload := map[string]interface{}{
			"email":    helpers.UniqueEmail(t),
			"password": helpers.TestPassword,
		}
		regRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", regPayload, "")
		regEnv := helpers.ParseEnvelope(t, regRec)

		if newUser, ok := regEnv.Data.(map[string]interface{}); ok {
			userID := newUser["id"].(string)
			rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/users/"+userID, nil, token)
			assert.Equal(t, http.StatusOK, rec.Code, "delete user should return 200")
		}
	})

	t.Run("POST /api/v1/users - create user", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    helpers.UniqueEmail(t),
			"password": helpers.TestPassword,
			"name":     "New User",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/users", payload, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "create user should return 201")
	})

	t.Run("protected routes - unauthenticated returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated should return 401")
	})
}
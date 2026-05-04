//go:build integration
// +build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestRoleHandler tests the role management endpoints.
func TestRoleHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("GET /api/v1/roles - list all roles", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list roles should return 200")
	})

	t.Run("POST /api/v1/roles - create role", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        fmt.Sprintf("role-%d", time.Now().UnixNano()%100000),
			"description": "Test role description",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", payload, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "create role should return 201")
	})

	t.Run("GET /api/v1/roles/:id - get role by id", func(t *testing.T) {
		// Role GetByID endpoint not yet implemented
		// List returns all roles; individual get not available
		t.Skip("role GetByID endpoint not yet implemented")
	})

	t.Run("PUT /api/v1/roles/:id - update role", func(t *testing.T) {
		// First list roles to get one
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles", nil, token)
		env := helpers.ParseEnvelope(t, listRec)

		if roles, ok := env.Data.([]interface{}); ok && len(roles) > 0 {
			if role, ok := roles[0].(map[string]interface{}); ok {
				roleID := role["id"].(string)
				payload := map[string]interface{}{
					"name":        "updated-role-name",
					"description": "Updated description",
				}
				rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/roles/"+roleID, payload, token)
				assert.Equal(t, http.StatusOK, rec.Code, "update role should return 200")
			}
		}
	})

	t.Run("DELETE /api/v1/roles/:id - delete role", func(t *testing.T) {
		// Create a role to delete
		createPayload := map[string]interface{}{
			"name":        "temp-role-to-delete",
			"description": "Will be deleted",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)

		if role, ok := env.Data.(map[string]interface{}); ok {
			roleID := role["id"].(string)
			rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
			assert.Equal(t, http.StatusOK, rec.Code, "delete role should return 200")
		}
	})

	t.Run("protected routes - unauthenticated returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated should return 401")
	})
}
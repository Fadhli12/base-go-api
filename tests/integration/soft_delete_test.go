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

// TestSoftDeleteBehavior tests that soft delete works correctly.
func TestSoftDeleteBehavior(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("delete sets deleted_at not removes row", func(t *testing.T) {
		// Create a role to delete
		createPayload := map[string]interface{}{
			"name":        "SoftDelete Test Role",
			"description": "Will be soft deleted",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// Delete the role
		deleteRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusOK, deleteRec.Code)

		// Verify the row still exists in database but with deleted_at set
		var count int64
		err := suite.DB.Raw("SELECT COUNT(*) FROM roles WHERE id = ?", roleID).Scan(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "soft deleted row should still exist in database")

		var deletedCount int64
		err = suite.DB.Raw("SELECT COUNT(*) FROM roles WHERE id = ? AND deleted_at IS NOT NULL", roleID).Scan(&deletedCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), deletedCount, "soft deleted row should have deleted_at set")
	})

	t.Run("soft deleted resource returns 404 on GET", func(t *testing.T) {
		// Create a role to delete
		createPayload := map[string]interface{}{
			"name":        "Another SoftDelete Role",
			"description": "Will be soft deleted",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// Delete the role
		deleteRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusOK, deleteRec.Code)

		// Try to get the deleted role - should return 404
		getRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusNotFound, getRec.Code, "soft deleted resource should return 404")
	})

	t.Run("soft deleted resource not in list", func(t *testing.T) {
		// Create and delete a role
		createPayload := map[string]interface{}{
			"name":        "List Exclusion Role",
			"description": "Will be hidden from list",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// Delete the role
		helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)

		// List all roles - deleted role should not appear
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)

		listEnv := helpers.ParseEnvelope(t, listRec)
		if roles, ok := listEnv.Data.([]interface{}); ok {
			for _, role := range roles {
				if r, ok := role.(map[string]interface{}); ok {
					assert.NotEqual(t, roleID, r["id"], "deleted role should not appear in list")
				}
			}
		}
	})

	t.Run("cannot delete same resource twice", func(t *testing.T) {
		// Create a role to delete
		createPayload := map[string]interface{}{
			"name":        "Double Delete Role",
			"description": "Will be deleted twice",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// First delete - should succeed
		deleteRec1 := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusOK, deleteRec1.Code)

		// Second delete - should return 404 (already deleted)
		deleteRec2 := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusNotFound, deleteRec2.Code, "deleting already deleted resource should return 404")
	})
}
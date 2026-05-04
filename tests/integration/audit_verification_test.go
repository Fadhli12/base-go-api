//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditLogVerification tests that mutations create audit log entries.
func TestAuditLogVerification(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	// Get initial audit log count
	var initialCount int64
	err := suite.DB.Raw("SELECT COUNT(*) FROM audit_logs").Scan(&initialCount).Error
	require.NoError(t, err)

	t.Run("POST creates audit log entry", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        "Test Role",
			"description": "Created for audit test",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", payload, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Small delay to ensure async audit log is written
		time.Sleep(100 * time.Millisecond)

		var newCount int64
		err := suite.DB.Raw("SELECT COUNT(*) FROM audit_logs").Scan(&newCount).Error
		require.NoError(t, err)
		assert.Greater(t, newCount, initialCount, "POST should create audit log entry")
	})

	t.Run("PUT creates audit log entry", func(t *testing.T) {
		// First create a role to update
		createPayload := map[string]interface{}{
			"name":        "Role to Update",
			"description": "Will be updated",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// Update the role
		updatePayload := map[string]interface{}{
			"name":        "Updated Role Name",
			"description": "Successfully updated",
		}
		updateRec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/roles/"+roleID, updatePayload, token)
		assert.Equal(t, http.StatusOK, updateRec.Code)

		// Small delay for async audit
		time.Sleep(100 * time.Millisecond)

		// Verify audit log for update
		var updateCount int64
		err := suite.DB.Raw("SELECT COUNT(*) FROM audit_logs WHERE action = 'update'").Scan(&updateCount).Error
		require.NoError(t, err)
		assert.Greater(t, updateCount, int64(0), "UPDATE should create audit log entry")
	})

	t.Run("DELETE creates audit log entry", func(t *testing.T) {
		// First create a role to delete
		createPayload := map[string]interface{}{
			"name":        "Role to Delete",
			"description": "Will be deleted",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		// Delete the role
		deleteRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/roles/"+roleID, nil, token)
		assert.Equal(t, http.StatusOK, deleteRec.Code)

		// Small delay for async audit
		time.Sleep(100 * time.Millisecond)

		// Verify audit log for delete
		var deleteCount int64
		err := suite.DB.Raw("SELECT COUNT(*) FROM audit_logs WHERE action = 'delete'").Scan(&deleteCount).Error
		require.NoError(t, err)
		assert.Greater(t, deleteCount, int64(0), "DELETE should create audit log entry")
	})

	t.Run("audit log contains actor information", func(t *testing.T) {
		// Get the current user ID from token context
		meRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
		meEnv := helpers.ParseEnvelope(t, meRec)
		currentUserID := meEnv.Data.(map[string]interface{})["id"].(string)

		// Create a new role
		payload := map[string]interface{}{
			"name":        "Audit Check Role",
			"description": "Testing audit actor",
		}
		helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", payload, token)

		// Small delay for async audit
		time.Sleep(100 * time.Millisecond)

		// Find the audit log entry
		var actorID string
		err := suite.DB.Raw(`
			SELECT actor_id::text FROM audit_logs
			WHERE resource = 'role' AND action = 'create'
			ORDER BY created_at DESC LIMIT 1
		`).Scan(&actorID).Error
		require.NoError(t, err)
		assert.Equal(t, currentUserID, actorID, "audit log should record correct actor")
	})

	t.Run("audit log contains change details", func(t *testing.T) {
		// Create and update a role
		createPayload := map[string]interface{}{
			"name":        "Before Update",
			"description": "Initial",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/roles", createPayload, token)
		env := helpers.ParseEnvelope(t, createRec)
		roleID := env.Data.(map[string]interface{})["id"].(string)

		updatePayload := map[string]interface{}{
			"name":        "After Update",
			"description": "Changed",
		}
		helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/roles/"+roleID, updatePayload, token)

		// Small delay
		time.Sleep(100 * time.Millisecond)

		// Check audit log has before/after state
		var logCount int64
		err := suite.DB.Raw(`
			SELECT COUNT(*) FROM audit_logs
			WHERE resource = 'role' AND resource_id = ? AND action = 'update'
		`, roleID).Scan(&logCount).Error
		require.NoError(t, err)
		assert.Greater(t, logCount, int64(0), "update should create audit log with state changes")
	})
}
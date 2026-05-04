//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestPermissionHandler tests the permission management endpoints.
func TestPermissionHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	// Create admin user with permissions
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("GET /api/v1/permissions - list all permissions (admin)", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/permissions", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "admin should be able to list permissions")
	})

	t.Run("GET /api/v1/permissions - unauthenticated returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/permissions", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated should return 401")
	})
}
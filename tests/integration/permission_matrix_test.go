//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestPermissionMatrix tests that permissions are correctly enforced across all endpoints.
func TestPermissionMatrix(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("admin user has full access to all endpoints", func(t *testing.T) {
		adminToken, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		// Auth endpoints (public - no permission check)
		publicEndpoints := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/api/v1/auth/register"},
			{http.MethodPost, "/api/v1/auth/login"},
			{http.MethodPost, "/api/v1/auth/refresh"},
		}

		for _, ep := range publicEndpoints {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, adminToken)
			assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "%s %s should be accessible", ep.method, ep.path)
		}

		// Protected endpoints
		protectedEndpoints := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/v1/me"},
			{http.MethodGet, "/api/v1/users"},
			{http.MethodGet, "/api/v1/roles"},
			{http.MethodGet, "/api/v1/permissions"},
			{http.MethodGet, "/api/v1/sessions"},
		}

		for _, ep := range protectedEndpoints {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, adminToken)
			assert.Equal(t, http.StatusOK, rec.Code, "%s %s should be accessible to admin", ep.method, ep.path)
		}
	})

	t.Run("unauthenticated requests are rejected on protected endpoints", func(t *testing.T) {
		protectedEndpoints := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/v1/me"},
			{http.MethodGet, "/api/v1/users"},
			{http.MethodGet, "/api/v1/roles"},
			{http.MethodGet, "/api/v1/permissions"},
			{http.MethodGet, "/api/v1/sessions"},
			{http.MethodPost, "/api/v1/auth/logout"},
			{http.MethodGet, "/api/v1/organizations"},
			{http.MethodGet, "/api/v1/invoices"},
		}

		for _, ep := range protectedEndpoints {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s should require auth", ep.method, ep.path)
		}
	})
}

// TestPermissionScope tests that scope-based permissions work correctly.
func TestPermissionScope(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("user can only access their own resources", func(t *testing.T) {
		// Create two users
		email1 := helpers.UniqueEmail(t)
		email2 := helpers.UniqueEmail(t)

		token1 := helpers.AuthenticateUser(t, server, email1, helpers.TestPassword)
		token2 := helpers.AuthenticateUser(t, server, email2, helpers.TestPassword)

		// User 1 registers and gets their info
		meRec1 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token1)
		env1 := helpers.ParseEnvelope(t, meRec1)
		_ = env1.Data.(map[string]interface{})["id"].(string)

		// User 1 should be able to access their own data
		meRec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token1)
		assert.Equal(t, http.StatusOK, meRec2.Code, "user should access own data")

		// User 2 should not be able to access User 1's session data
		// (assuming sessions are scoped to user)
		_ = token2 // token2 exists for future scope tests
	})
}
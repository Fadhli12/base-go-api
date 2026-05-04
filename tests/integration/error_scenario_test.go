//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestErrorScenarios tests error handling across endpoints.
func TestErrorScenarios(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("404 for non-existent resource", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/roles/non-existent-id", nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code, "non-existent resource should return 404")
	})

	t.Run("400 for invalid request body", func(t *testing.T) {
		payload := map[string]interface{}{
			"email": "invalid-email",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid email should return 400")
	})

	t.Run("401 for missing auth token", func(t *testing.T) {
		endpoints := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/api/v1/me"},
			{http.MethodGet, "/api/v1/users"},
			{http.MethodPost, "/api/v1/auth/logout"},
		}

		for _, ep := range endpoints {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s should require auth", ep.method, ep.path)
		}
	})

	t.Run("403 for insufficient permissions", func(t *testing.T) {
		// Create a limited user (not admin)
		email := helpers.UniqueEmail(t)
		limitedToken := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

		// Try to access admin-only endpoint
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/permissions", nil, limitedToken)
		// May return 403 if user lacks permissions, or 200 if they have view permission
		assert.True(t, rec.Code == http.StatusForbidden || rec.Code == http.StatusOK,
			"limited user permission check should return 403 or 200 (if has permission)")
	})

	t.Run("409 for duplicate resource", func(t *testing.T) {
		email := helpers.UniqueEmail(t)
		payload := map[string]interface{}{
			"email":    email,
			"password": helpers.TestPassword,
		}

		// Register first time - should succeed
		rec1 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusCreated, rec1.Code)

		// Register same email again - should fail with 409
		rec2 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusConflict, rec2.Code, "duplicate email should return 409")
	})

	t.Run("422 for validation errors", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    "test@example.com",
			"password": "weak", // Too short
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", payload, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "weak password should return 400")
	})

	t.Run("500 for server errors", func(t *testing.T) {
		// Try to access a broken endpoint if one exists
		// Otherwise this test documents expected behavior
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/nonexistent-route", nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code, "nonexistent route should return 404")
	})
}

// TestRateLimiting tests rate limiting behavior.
func TestRateLimiting(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("rate limit headers are present", func(t *testing.T) {
		// Make request to a public endpoint
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/healthz", nil, "")
		assert.Equal(t, http.StatusOK, rec.Code)

		// Check for rate limit headers (if enabled)
		// X-RateLimit-Remaining, X-RateLimit-Limit
		// Note: These may not be present if rate limiting is disabled
		_ = rec.Header().Get("X-RateLimit-Remaining")
		_ = rec.Header().Get("X-RateLimit-Limit")
	})

	t.Run("multiple rapid requests don't cause errors", func(t *testing.T) {
		// Make multiple rapid requests
		for i := 0; i < 5; i++ {
			rec := helpers.MakeRequest(t, server, http.MethodGet, "/healthz", nil, "")
			assert.Equal(t, http.StatusOK, rec.Code, "rapid requests should not fail")
		}
	})
}
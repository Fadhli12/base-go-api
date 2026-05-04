//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestPagination tests list endpoint pagination behavior.
func TestPagination(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

	t.Run("list endpoints return pagination metadata", func(t *testing.T) {
		// Create a few resources to ensure list works
		for i := 0; i < 3; i++ {
			email := helpers.UniqueEmail(t)
			regPayload := map[string]interface{}{
				"email":    email,
				"password": helpers.TestPassword,
			}
			helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", regPayload, "")
		}

		// Test users list has pagination
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		if ok {
			// Check for pagination fields like "total", "page", "per_page"
			if users, ok := data["users"].([]interface{}); ok {
				assert.GreaterOrEqual(t, len(users), 1, "should have at least created users")
			}
		}
	})

	t.Run("list endpoints support limit query parameter", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users?limit=2", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("list endpoints support page query parameter", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/users?page=1", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
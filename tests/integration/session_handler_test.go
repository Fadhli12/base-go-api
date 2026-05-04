//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestSessionHandler tests the session management endpoints.
func TestSessionHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	email := helpers.UniqueEmail(t)
	token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

	t.Run("GET /api/v1/sessions - list sessions for authenticated user", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/sessions", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list sessions should return 200")
	})

	t.Run("GET /api/v1/sessions - unauthenticated returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/sessions", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated should return 401")
	})

	t.Run("DELETE /api/v1/sessions/:id - revoke session", func(t *testing.T) {
		// First get sessions to find one to delete
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/sessions", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)

		env := helpers.ParseEnvelope(t, listRec)
		if sessions, ok := env.Data.([]interface{}); ok && len(sessions) > 0 {
			if session, ok := sessions[0].(map[string]interface{}); ok {
				sessionID := session["id"].(string)
				rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/sessions/"+sessionID, nil, token)
				assert.Equal(t, http.StatusOK, rec.Code, "delete session should return 200")
			}
		}
	})

	t.Run("DELETE /api/v1/sessions - unauthenticated returns 401", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/sessions/some-session-id", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated should return 401")
	})
}
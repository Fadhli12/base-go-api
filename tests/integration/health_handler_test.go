//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
)

// TestHealthHandler_Healthz tests the liveness endpoint.
func TestHealthHandler_Healthz(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("returns 200 with status", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/healthz", nil, "")
		assert.Equal(t, http.StatusOK, rec.Code, "healthz should return 200")

		// Healthz returns a plain JSON response, not an envelope
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "ok", response["status"], "status should be ok")
	})
}

// TestHealthHandler_Readyz tests the readiness endpoint.
func TestHealthHandler_Readyz(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	t.Run("returns 200 when ready", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/readyz", nil, "")
		assert.Equal(t, http.StatusOK, rec.Code, "readyz should return 200 when database is connected")

		// Readyz returns a plain JSON response with status and checks
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err, "response should be valid JSON")
		assert.Equal(t, "ready", response["status"], "status should be ready")
	})
}
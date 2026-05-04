//go:build integration
// +build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyHandler tests the API key management endpoints.
func TestAPIKeyHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	// Create authenticated user (token used in subtests which re-authenticate)
	_ = helpers.AuthenticateUser(t, server, helpers.UniqueEmail(t), helpers.TestPassword)

	t.Run("POST /api/v1/api-keys - create API key (happy path)", func(t *testing.T) {
		suite.SetupTest(t)

		// Re-authenticate after DB cleanup
		email := helpers.UniqueEmail(t)
		token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

		body := map[string]interface{}{
			"name":   "test-key",
			"scopes": []string{"invoices:read", "invoices:write"},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/api-keys", body, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "create API key should return 201")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")

		// Verify key is returned on creation
		_, hasKey := dataMap["key"]
		assert.True(t, hasKey, "created API key should include the secret key")

		assert.NotEmpty(t, dataMap["id"], "created API key should have an id")
		assert.Equal(t, "test-key", dataMap["name"], "name should match request")
	})

	t.Run("POST /api/v1/api-keys - unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)

		body := map[string]interface{}{
			"name":   "unauth-key",
			"scopes": []string{"invoices:read"},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/api-keys", body, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated create should return 401")
	})

	t.Run("GET /api/v1/api-keys - list API keys (happy path)", func(t *testing.T) {
		suite.SetupTest(t)

		email := helpers.UniqueEmail(t)
		token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

		// Create an API key first
		createBody := map[string]interface{}{
			"name":   "list-test-key",
			"scopes": []string{"invoices:read"},
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/api-keys", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create API key should succeed")

		// List API keys
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/api-keys", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list API keys should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")

		// Response should contain api_keys array and pagination fields
		_, hasKeys := dataMap["api_keys"]
		assert.True(t, hasKeys, "list response should contain api_keys field")
	})

	t.Run("GET /api/v1/api-keys - unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/api-keys", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated list should return 401")
	})

	t.Run("GET /api/v1/api-keys/:id - get API key by ID (happy path)", func(t *testing.T) {
		suite.SetupTest(t)

		email := helpers.UniqueEmail(t)
		token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

		// Create an API key first
		createBody := map[string]interface{}{
			"name":   "get-test-key",
			"scopes": []string{"invoices:read"},
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/api-keys", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create API key should succeed")

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a JSON object")

		keyID, ok := createData["id"].(string)
		require.True(t, ok, "create response should contain id string")
		require.NotEmpty(t, keyID, "created key ID should not be empty")

		// Get the API key by ID
		path := fmt.Sprintf("/api/v1/api-keys/%s", keyID)
		rec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "get API key by ID should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		getData, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.Equal(t, "get-test-key", getData["name"], "retrieved key name should match")
	})

	t.Run("GET /api/v1/api-keys/:id - unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/api-keys/non-existent-id", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated get by ID should return 401")
	})

	t.Run("DELETE /api/v1/api-keys/:id - revoke API key (happy path)", func(t *testing.T) {
		suite.SetupTest(t)

		email := helpers.UniqueEmail(t)
		token := helpers.AuthenticateUser(t, server, email, helpers.TestPassword)

		// Create an API key first
		createBody := map[string]interface{}{
			"name":   "revoke-test-key",
			"scopes": []string{"invoices:read"},
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/api-keys", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create API key should succeed")

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a JSON object")

		keyID, ok := createData["id"].(string)
		require.True(t, ok, "create response should contain id string")
		require.NotEmpty(t, keyID, "created key ID should not be empty")

		// Revoke the API key
		path := fmt.Sprintf("/api/v1/api-keys/%s", keyID)
		rec := helpers.MakeRequest(t, server, http.MethodDelete, path, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "revoke API key should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		deleteData, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.Equal(t, "API key revoked successfully", deleteData["message"], "should include revocation message")
	})

	t.Run("DELETE /api/v1/api-keys/:id - unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/api-keys/some-key-id", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "unauthenticated delete should return 401")
	})
}
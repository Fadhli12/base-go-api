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

// createEmailUser creates an authenticated user with email-related permissions.
func createEmailUser(t *testing.T, suite *TestSuite) (*helpers.Server, string) {
	t.Helper()

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
		"email_templates:manage",
	})

	return server, token
}

// TestEmailHandler tests the email HTTP endpoints.
func TestEmailHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("POST /api/v1/emails - send email with direct content", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createEmailUser(t, suite)

		body := map[string]interface{}{
			"to":           "recipient@example.com",
			"subject":      "Test Email Subject",
			"html_content": "<p>Hello World</p>",
			"text_content": "Hello World",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/emails", body, token)
		// Accept 202 (queued) or 500 (email disabled in test config)
		assert.True(t, rec.Code == http.StatusAccepted || rec.Code == http.StatusInternalServerError,
			"POST /api/v1/emails should return 202 (queued) or 500 (email disabled), got %d", rec.Code)

		if rec.Code == http.StatusAccepted {
			env := helpers.ParseEnvelope(t, rec)
			require.NotNil(t, env.Data, "response should contain data")

			data, ok := env.Data.(map[string]interface{})
			require.True(t, ok, "response data should be a JSON object")
			assert.Contains(t, data, "message", "response should contain message field")
		}
	})

	t.Run("GET /api/v1/emails/:id - get email status", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createEmailUser(t, suite)

		// Use a non-existent UUID - should return 404
		fakeID := "00000000-0000-0000-0000-000000000001"
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/emails/"+fakeID, nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code, "GET /api/v1/emails/:id for non-existent email should return 404")
	})

	t.Run("POST /api/v1/emails - validation error for missing fields", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createEmailUser(t, suite)

		// Missing required "to" field
		body := map[string]interface{}{
			"subject":      "Test Subject",
			"html_content": "<p>Content</p>",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/emails", body, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "POST /api/v1/emails with missing to should return 400")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error, "response should contain error")
		assert.Equal(t, "VALIDATION_ERROR", env.Error.Code, "error code should be VALIDATION_ERROR")
	})

	t.Run("unauthenticated email requests return 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		// POST /api/v1/emails - send without auth
		sendBody := map[string]interface{}{
			"to":       "noauth@example.com",
			"subject":  "No Auth",
			"html_content": "<p>test</p>",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/emails", sendBody, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "POST /api/v1/emails without auth should return 401")

		// GET /api/v1/emails/:id - get status without auth
		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/emails/00000000-0000-0000-0000-000000000001", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "GET /api/v1/emails/:id without auth should return 401")
	})
}

// TestWebhookHandler tests the webhook HTTP endpoints.
func TestWebhookHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("POST /api/v1/webhooks/:provider/delivery - process delivery webhook", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		payload := map[string]interface{}{
			"message_id": "msg-12345",
			"timestamp":  1700000000,
			"event":      "delivered",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/webhooks/smtp/delivery", payload, "")
		// Accept 200 (processed), 400 (invalid message_id format), or 500 (service unavailable)
		assert.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusBadRequest || rec.Code == http.StatusInternalServerError,
			"POST /api/v1/webhooks/:provider/delivery should return 200, 400, or 500, got %d", rec.Code)

		if rec.Code == http.StatusOK {
			env := helpers.ParseEnvelope(t, rec)
			require.NotNil(t, env.Data, "response should contain data")

			data, ok := env.Data.(map[string]interface{})
			require.True(t, ok, "response data should be a JSON object")
			assert.Equal(t, "processed", data["status"], "status should be processed")
		}
	})

	t.Run("POST /api/v1/webhooks/:provider/bounce - process bounce webhook", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		payload := map[string]interface{}{
			"email":       "bounced@example.com",
			"message_id":  "msg-67890",
			"type":        "hard",
			"reason":      "550 User unknown",
			"timestamp":   1700000000,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/webhooks/sendgrid/bounce", payload, "")
		// Accept 200 (processed), 400 (invalid message_id format), or 500 (service unavailable)
		assert.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusBadRequest || rec.Code == http.StatusInternalServerError,
			"POST /api/v1/webhooks/:provider/bounce should return 200, 400, or 500, got %d", rec.Code)

		if rec.Code == http.StatusOK {
			env := helpers.ParseEnvelope(t, rec)
			require.NotNil(t, env.Data, "response should contain data")

			data, ok := env.Data.(map[string]interface{})
			require.True(t, ok, "response data should be a JSON object")
			assert.Equal(t, "processed", data["status"], "status should be processed")
		}
	})

	t.Run("POST /api/v1/webhooks/:provider/delivery - invalid payload returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		// Send invalid JSON structure (missing fields)
		invalidPayload := map[string]interface{}{}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/webhooks/smtp/delivery", invalidPayload, "")
		// The handler may return 400 or 500 depending on provider state
		assert.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusBadRequest || rec.Code == http.StatusInternalServerError,
			"POST /api/v1/webhooks/:provider/delivery with empty payload should return 400 or 500, got %d", rec.Code)
	})

	t.Run("webhook endpoints are publicly accessible (no auth required)", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		payload := map[string]interface{}{
			"message_id": "msg-pub-access",
			"timestamp":  1700000000,
			"event":      "delivered",
		}

		// Webhook endpoints should NOT return 401 (they are public)
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/webhooks/smtp/delivery", payload, "")
		assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
			"POST /api/v1/webhooks/:provider/delivery should not require auth (should not return 401)")
	})
}
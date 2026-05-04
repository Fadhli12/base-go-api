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

// createTemplateUser creates an authenticated user with email_templates:manage permission.
func createTemplateUser(t *testing.T, suite *TestSuite) (*helpers.Server, string) {
	t.Helper()

	server := helpers.NewTestServer(t, suite)
	token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
		"email_templates:manage",
	})

	return server, token
}

// createTestTemplate creates an email template via HTTP and returns its ID.
func createTestTemplate(t *testing.T, server *helpers.Server, token string, name string) string {
	t.Helper()

	body := map[string]interface{}{
		"name":         name,
		"subject":      "Test Subject for " + name,
		"html_content": "<h1>Hello {{.Name}}</h1>",
		"text_content": "Hello {{.Name}}",
		"category":     "transactional",
	}

	rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/email-templates", body, token)
	require.Equal(t, http.StatusCreated, rec.Code, "create template should return 201: %s", rec.Body.String())

	env := helpers.ParseEnvelope(t, rec)
	data, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a JSON object")

	id, ok := data["id"].(string)
	require.True(t, ok, "template id should be a string")
	require.NotEmpty(t, id, "template id should not be empty")

	return id
}

// TestEmailTemplateHandler tests the email template HTTP endpoints.
func TestEmailTemplateHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("POST /api/v1/email-templates - create email template", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createTemplateUser(t, suite)

		body := map[string]interface{}{
			"name":         "welcome-email",
			"subject":      "Welcome to Our Platform",
			"html_content": "<h1>Welcome {{.Name}}</h1><p>Thanks for signing up!</p>",
			"text_content": "Welcome {{.Name}} - Thanks for signing up!",
			"category":     "transactional",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/email-templates", body, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "create template should return 201")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.Equal(t, "welcome-email", data["name"], "template name should match")
		assert.Equal(t, "Welcome to Our Platform", data["subject"], "subject should match")
		assert.Equal(t, "transactional", data["category"], "category should match")
		assert.NotEmpty(t, data["id"], "template id should be present")
	})

	t.Run("GET /api/v1/email-templates - list email templates", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createTemplateUser(t, suite)

		// Create a template first so the list is non-empty
		createTestTemplate(t, server, token, "list-test-template")

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/email-templates", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list templates should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.NotNil(t, data["data"], "response should contain templates array")
		assert.NotNil(t, data["total"], "response should contain total count")
	})

	t.Run("GET /api/v1/email-templates/:id - get email template by ID", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createTemplateUser(t, suite)

		templateID := createTestTemplate(t, server, token, "get-test-template")

		path := fmt.Sprintf("/api/v1/email-templates/%s", templateID)
		rec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "get template by ID should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.Equal(t, templateID, data["id"], "template id should match")
		assert.Equal(t, "get-test-template", data["name"], "template name should match")
	})

	t.Run("PUT /api/v1/email-templates/:id - update email template", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createTemplateUser(t, suite)

		templateID := createTestTemplate(t, server, token, "update-test-template")

		updateBody := map[string]interface{}{
			"name":     "updated-template-name",
			"subject":  "Updated Subject Line",
			"category": "notification",
		}

		path := fmt.Sprintf("/api/v1/email-templates/%s", templateID)
		rec := helpers.MakeRequest(t, server, http.MethodPut, path, updateBody, token)
		assert.Equal(t, http.StatusOK, rec.Code, "update template should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should contain data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "response data should be a JSON object")
		assert.Equal(t, "updated-template-name", data["name"], "updated name should match")
		assert.Equal(t, "Updated Subject Line", data["subject"], "updated subject should match")
		assert.Equal(t, "notification", data["category"], "updated category should match")
	})

	t.Run("DELETE /api/v1/email-templates/:id - delete email template", func(t *testing.T) {
		suite.SetupTest(t)
		server, token := createTemplateUser(t, suite)

		templateID := createTestTemplate(t, server, token, "delete-test-template")

		// Delete the template
		path := fmt.Sprintf("/api/v1/email-templates/%s", templateID)
		rec := helpers.MakeRequest(t, server, http.MethodDelete, path, nil, token)
		assert.Equal(t, http.StatusNoContent, rec.Code, "delete template should return 204")

		// Verify soft delete - getting the deleted template should return 404
		getRec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusNotFound, getRec.Code, "deleted template should return 404")
	})

	// Auth check tests - unauthenticated requests should return 401
	t.Run("unauthenticated requests return 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		fakeID := "00000000-0000-0000-0000-000000000001"

		// POST /api/v1/email-templates - create without auth
		createBody := map[string]interface{}{
			"name":         "noauth-template",
			"subject":      "No Auth Subject",
			"html_content": "<p>test</p>",
			"category":     "transactional",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/email-templates", createBody, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "create template without auth should return 401")

		// GET /api/v1/email-templates - list without auth
		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/email-templates", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "list templates without auth should return 401")

		// GET /api/v1/email-templates/:id - get without auth
		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/email-templates/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "get template without auth should return 401")

		// PUT /api/v1/email-templates/:id - update without auth
		updateBody := map[string]interface{}{
			"name": "hacked-template",
		}
		rec = helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/email-templates/"+fakeID, updateBody, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "update template without auth should return 401")

		// DELETE /api/v1/email-templates/:id - delete without auth
		rec = helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/email-templates/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "delete template without auth should return 401")
	})
}
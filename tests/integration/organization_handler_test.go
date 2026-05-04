//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "Failed to create enforcer")

	t.Run("POST /api/v1/organizations - create organization", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		payload := map[string]interface{}{
			"name": "Test Organization",
			"slug": "test-org-handler",
		}
		rec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", payload, token)

		assert.Equal(t, http.StatusCreated, rec.Code, "create organization should return 201")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, "Test Organization", data["name"], "organization name should match")
		assert.Equal(t, "test-org-handler", data["slug"], "organization slug should match")
		assert.NotEmpty(t, data["id"], "organization id should be present")
	})

	t.Run("GET /api/v1/organizations - list user's organizations", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "List Test Org",
			"slug": "list-test-org",
		}
		makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)

		// List organizations
		rec := makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list organizations should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotNil(t, data["organizations"], "response should contain organizations")
	})

	t.Run("GET /api/v1/organizations/:id - get organization by ID", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "Get Test Org",
			"slug": "get-test-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Get the organization by ID
		rec := makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations/"+orgID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "get organization by id should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, orgID, data["id"], "organization id should match")
		assert.Equal(t, "Get Test Org", data["name"], "organization name should match")
	})

	t.Run("PUT /api/v1/organizations/:id - update organization", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "Update Test Org",
			"slug": "update-test-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Update the organization
		updatePayload := map[string]interface{}{
			"name": "Updated Org Name",
			"slug": "updated-org-slug",
		}
		rec := makeOrgRequest(t, server, http.MethodPut, "/api/v1/organizations/"+orgID, updatePayload, token)
		assert.Equal(t, http.StatusOK, rec.Code, "update organization should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, "Updated Org Name", data["name"], "updated name should match")
		assert.Equal(t, "updated-org-slug", data["slug"], "updated slug should match")
	})

	t.Run("DELETE /api/v1/organizations/:id - delete organization (soft delete)", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "Delete Test Org",
			"slug": "delete-test-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Delete the organization
		rec := makeOrgRequest(t, server, http.MethodDelete, "/api/v1/organizations/"+orgID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "delete organization should return 200")

		// Verify soft delete - getting the deleted org should return 404
		getRec := makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations/"+orgID, nil, token)
		assert.Equal(t, http.StatusNotFound, getRec.Code, "deleted organization should return 404")
	})

	t.Run("POST /api/v1/organizations/:id/members - add member to organization", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "Member Test Org",
			"slug": "member-test-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Create a second user to add as member
		memberEmail := helpers.UniqueEmail(t)
		memberPayload := map[string]interface{}{
			"email":    memberEmail,
			"password": helpers.TestPassword,
		}
		memberRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", memberPayload, "")
		require.Equal(t, http.StatusCreated, memberRec.Code, "member registration should succeed")

		memberEnv := helpers.ParseEnvelope(t, memberRec)
		memberData, ok := memberEnv.Data.(map[string]interface{})
		require.True(t, ok, "member response data should be a map")
		memberUserID, ok := memberData["id"].(string)
		require.True(t, ok, "member user id should be a string")

		// Add member to organization
		addMemberPayload := map[string]interface{}{
			"user_id": memberUserID,
			"role":    "member",
		}
		rec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations/"+orgID+"/members", addMemberPayload, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "add member should return 201")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")
	})

	t.Run("GET /api/v1/organizations/:id/members - list organization members", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization first
		createPayload := map[string]interface{}{
			"name": "Members List Org",
			"slug": "members-list-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Get members - should at least have the owner
		rec := makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations/"+orgID+"/members", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list members should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotNil(t, data["members"], "response should contain members")
	})

	t.Run("DELETE /api/v1/organizations/:id/members/:user_id - remove member", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		// Create an organization
		createPayload := map[string]interface{}{
			"name": "Remove Member Org",
			"slug": "remove-member-org",
		}
		createRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, token)
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		orgID, ok := createData["id"].(string)
		require.True(t, ok, "organization id should be a string")

		// Create a user to add as member
		memberEmail := helpers.UniqueEmail(t)
		memberPayload := map[string]interface{}{
			"email":    memberEmail,
			"password": helpers.TestPassword,
		}
		memberRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", memberPayload, "")
		require.Equal(t, http.StatusCreated, memberRec.Code, "member registration should succeed")

		memberEnv := helpers.ParseEnvelope(t, memberRec)
		memberData, ok := memberEnv.Data.(map[string]interface{})
		require.True(t, ok, "member response data should be a map")
		memberUserID, ok := memberData["id"].(string)
		require.True(t, ok, "member user id should be a string")

		// Add member first
		addMemberPayload := map[string]interface{}{
			"user_id": memberUserID,
			"role":    "member",
		}
		addRec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations/"+orgID+"/members", addMemberPayload, token)
		require.Equal(t, http.StatusCreated, addRec.Code, "add member should succeed")

		// Remove member
		rec := makeOrgRequest(t, server, http.MethodDelete, "/api/v1/organizations/"+orgID+"/members/"+memberUserID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "remove member should return 200")
	})

	t.Run("unauthenticated requests return 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		fakeID := "00000000-0000-0000-0000-000000000001"

		// POST /api/v1/organizations - create without auth
		createPayload := map[string]interface{}{
			"name": "Noauth Org",
			"slug": "noauth-org",
		}
		rec := makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations", createPayload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "create org without auth should return 401")

		// GET /api/v1/organizations - list without auth
		rec = makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "list orgs without auth should return 401")

		// GET /api/v1/organizations/:id - get without auth
		rec = makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "get org without auth should return 401")

		// PUT /api/v1/organizations/:id - update without auth
		updatePayload := map[string]interface{}{
			"name": "Hacked Org",
		}
		rec = makeOrgRequest(t, server, http.MethodPut, "/api/v1/organizations/"+fakeID, updatePayload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "update org without auth should return 401")

		// DELETE /api/v1/organizations/:id - delete without auth
		rec = makeOrgRequest(t, server, http.MethodDelete, "/api/v1/organizations/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "delete org without auth should return 401")

		// POST /api/v1/organizations/:id/members - add member without auth
		addMemberPayload := map[string]interface{}{
			"user_id": fakeID,
			"role":    "member",
		}
		rec = makeOrgRequest(t, server, http.MethodPost, "/api/v1/organizations/"+fakeID+"/members", addMemberPayload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "add member without auth should return 401")

		// GET /api/v1/organizations/:id/members - list members without auth
		rec = makeOrgRequest(t, server, http.MethodGet, "/api/v1/organizations/"+fakeID+"/members", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "list members without auth should return 401")

		// DELETE /api/v1/organizations/:id/members/:user_id - remove member without auth
		rec = makeOrgRequest(t, server, http.MethodDelete, "/api/v1/organizations/"+fakeID+"/members/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "remove member without auth should return 401")
	})
}

func makeOrgRequest(t *testing.T, server *helpers.Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err, "body should marshal to JSON")
		reqBody = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)
	return rec
}

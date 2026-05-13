//go:build integration
// +build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tagPermissions returns the permission strings needed for tag CRUD + manage operations.
func tagPermissions() []string {
	return []string{
		"tag:view",
		"tag:create",
		"tag:update",
		"tag:delete",
		"tag:manage",
	}
}

// tagViewPermissions returns only view permissions (read-only access).
func tagViewPermissions() []string {
	return []string{"tag:view"}
}

// createTagViaAPI is a helper that creates a tag via the HTTP endpoint and returns the response data.
func createTagViaAPI(t *testing.T, server *helpers.Server, token string, body map[string]interface{}, headers map[string]string) map[string]interface{} {
	t.Helper()
	rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, headers)
	require.Equal(t, http.StatusCreated, rec.Code, "create tag should return 201")
	env := helpers.ParseEnvelope(t, rec)
	require.NotNil(t, env.Data, "response should have data")
	data, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map")
	return data
}

// setupTagTest creates a user with the given permissions, an organization, and org-scoped
// Casbin policies. Returns the auth token and org headers for making authenticated,
// org-scoped requests. This is the common setup for all tag tests that need org context.
func setupTagTest(t *testing.T, suite helpers.SuiteProvider, enforcer *permission.Enforcer, server *helpers.Server, perms []string) (string, map[string]string) {
	t.Helper()
	token, userID, roleName, _ := helpers.CreateUserWithPermissionsReturningID(t, suite, enforcer, perms)
	orgID := helpers.CreateTestOrganization(t, suite, userID)
	helpers.AddOrgScopedPolicies(t, enforcer, userID, orgID, roleName, perms)
	orgHeaders := map[string]string{middleware.OrganizationIDHeader: orgID.String()}
	return token, orgHeaders
}

// TestTagHandler_Create tests POST /api/v1/tags.
func TestTagHandler_Create(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("creates a tag with name", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		body := map[string]interface{}{
			"name": "Go",
		}
		data := createTagViaAPI(t, server, token, body, orgHeaders)

		assert.NotEmpty(t, data["id"], "tag should have an ID")
		assert.Equal(t, "Go", data["name"], "tag name should match")
		assert.Equal(t, "go", data["slug"], "slug should be auto-generated from name")
		assert.Equal(t, float64(0), data["usage_count"], "usage_count should start at 0")
	})

	t.Run("creates a tag with name and color", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		body := map[string]interface{}{
			"name":  "Urgent",
			"color": "#FF5733",
		}
		data := createTagViaAPI(t, server, token, body, orgHeaders)

		assert.Equal(t, "Urgent", data["name"])
		assert.Equal(t, "urgent", data["slug"])
		assert.Equal(t, "#FF5733", data["color"])
	})

	t.Run("rejects duplicate tag name", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		body := map[string]interface{}{
			"name": "Duplicate",
		}
		rec1 := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		require.Equal(t, http.StatusCreated, rec1.Code)

		rec2 := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		assert.Equal(t, http.StatusConflict, rec2.Code, "duplicate name should return 409")
		env := helpers.ParseEnvelope(t, rec2)
		require.NotNil(t, env.Error)
		assert.Equal(t, "CONFLICT", env.Error.Code)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		body := map[string]interface{}{
			"name": "",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "empty name should return 400")
	})

	t.Run("rejects invalid color format", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		body := map[string]interface{}{
			"name":  "BadColor",
			"color": "red",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "invalid color format should return 400")
	})

	t.Run("rejects without authentication", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"name": "NoAuth",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/tags", body, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("rejects without tag:create permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagViewPermissions())

		body := map[string]interface{}{
			"name": "Forbidden",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		assert.Equal(t, http.StatusForbidden, rec.Code, "create without permission should return 403")
	})
}

// TestTagHandler_List tests GET /api/v1/tags.
func TestTagHandler_List(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("lists tags with pagination", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		// Create 5 tags
		for i := 0; i < 5; i++ {
			body := map[string]interface{}{
				"name": fmt.Sprintf("Tag %d", i),
			}
			rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
			require.Equal(t, http.StatusCreated, rec.Code)
		}

		// List all
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags?limit=10&offset=0", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		total := data["total"].(float64)
		assert.Equal(t, float64(5), total, "should list 5 tags")

		tags, ok := data["tags"].([]interface{})
		require.True(t, ok)
		assert.Len(t, tags, 5, "should return 5 tag items")
	})

	t.Run("pagination limit and offset", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		// Create 25 tags
		for i := 0; i < 25; i++ {
			body := map[string]interface{}{
				"name": fmt.Sprintf("Alpha-Tag-%02d", i),
			}
			helpers.MakeRequestWithHeaders(t, server, http.MethodPost, "/api/v1/tags", body, token, orgHeaders)
		}

		// First page: limit=10, offset=0
		rec1 := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags?limit=10&offset=0", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec1.Code)
		env1 := helpers.ParseEnvelope(t, rec1)
		data1 := env1.Data.(map[string]interface{})
		assert.Equal(t, float64(25), data1["total"].(float64))
		assert.Len(t, data1["tags"].([]interface{}), 10)

		// Second page: limit=10, offset=10
		rec2 := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags?limit=10&offset=10", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec2.Code)
		env2 := helpers.ParseEnvelope(t, rec2)
		data2 := env2.Data.(map[string]interface{})
		assert.Len(t, data2["tags"].([]interface{}), 10)

		// Last page: limit=10, offset=20
		rec3 := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags?limit=10&offset=20", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec3.Code)
		env3 := helpers.ParseEnvelope(t, rec3)
		data3 := env3.Data.(map[string]interface{})
		assert.Len(t, data3["tags"].([]interface{}), 5)
	})

	t.Run("returns empty list when no tags", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(0), data["total"].(float64), "total should be 0")
	})

	t.Run("rejects without authentication", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/tags", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestTagHandler_GetByID tests GET /api/v1/tags/:id.
func TestTagHandler_GetByID(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("gets a tag by ID", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		createBody := map[string]interface{}{
			"name":  "FindByID",
			"color": "#00FF00",
		}
		data := createTagViaAPI(t, server, token, createBody, orgHeaders)
		tagID := data["id"].(string)

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/"+tagID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		tagData, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "FindByID", tagData["name"])
		assert.Equal(t, "findbyid", tagData["slug"])
		assert.Equal(t, "#00FF00", tagData["color"])
	})

	t.Run("returns 404 for non-existent tag", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagViewPermissions())

		fakeID := "00000000-0000-0000-0000-000000000000"
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/"+fakeID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 400 for invalid ID format", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagViewPermissions())

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/not-a-uuid", nil, token, orgHeaders)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestTagHandler_GetBySlug tests GET /api/v1/tags/slug/:slug.
func TestTagHandler_GetBySlug(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("gets a tag by slug", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		createBody := map[string]interface{}{
			"name": "My Special Tag",
		}
		createTagViaAPI(t, server, token, createBody, orgHeaders)

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/slug/my-special-tag", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		tagData, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "My Special Tag", tagData["name"])
		assert.Equal(t, "my-special-tag", tagData["slug"])
	})

	t.Run("returns 404 for non-existent slug", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagViewPermissions())

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/slug/nonexistent", nil, token, orgHeaders)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestTagHandler_Update tests PUT /api/v1/tags/:id.
func TestTagHandler_Update(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("updates tag name and color", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		createBody := map[string]interface{}{
			"name":  "Original",
			"color": "#000000",
		}
		data := createTagViaAPI(t, server, token, createBody, orgHeaders)
		tagID := data["id"].(string)

		updateBody := map[string]interface{}{
			"name":  "Updated",
			"color": "#FFFFFF",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPut, "/api/v1/tags/"+tagID, updateBody, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		updated, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Updated", updated["name"])
		assert.Equal(t, "updated", updated["slug"], "slug should be regenerated from new name")
		assert.Equal(t, "#FFFFFF", updated["color"])
	})

	t.Run("updates only color", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		createBody := map[string]interface{}{
			"name":  "ColorOnly",
			"color": "#111111",
		}
		data := createTagViaAPI(t, server, token, createBody, orgHeaders)
		tagID := data["id"].(string)

		updateBody := map[string]interface{}{
			"color": "#222222",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPut, "/api/v1/tags/"+tagID, updateBody, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		updated := env.Data.(map[string]interface{})
		assert.Equal(t, "ColorOnly", updated["name"], "name should not change")
		assert.Equal(t, "#222222", updated["color"])
	})

	t.Run("rejects duplicate name on update", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		// Create two tags
		createTagViaAPI(t, server, token, map[string]interface{}{"name": "First"}, orgHeaders)
		secondData := createTagViaAPI(t, server, token, map[string]interface{}{"name": "Second"}, orgHeaders)
		secondID := secondData["id"].(string)

		// Try to rename second tag to first tag's name
		updateBody := map[string]interface{}{
			"name": "First",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPut, "/api/v1/tags/"+secondID, updateBody, token, orgHeaders)
		assert.Equal(t, http.StatusConflict, rec.Code, "duplicate name should return 409")
	})

	t.Run("returns 404 for non-existent tag", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		fakeID := "00000000-0000-0000-0000-000000000000"
		updateBody := map[string]interface{}{
			"name": "Ghost",
		}
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodPut, "/api/v1/tags/"+fakeID, updateBody, token, orgHeaders)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestTagHandler_Delete tests DELETE /api/v1/tags/:id.
func TestTagHandler_Delete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("deletes a tag and verifies 404 on subsequent GET", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		data := createTagViaAPI(t, server, token, map[string]interface{}{"name": "ToDelete"}, orgHeaders)
		tagID := data["id"].(string)

		// Delete
		delRec := helpers.MakeRequestWithHeaders(t, server, http.MethodDelete, "/api/v1/tags/"+tagID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, delRec.Code)

		// Verify 404 on GET by ID
		getRec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/"+tagID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusNotFound, getRec.Code, "soft-deleted tag should return 404")
	})

	t.Run("returns 404 for deleting non-existent tag", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		fakeID := "00000000-0000-0000-0000-000000000000"
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodDelete, "/api/v1/tags/"+fakeID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("rejects delete without tag:delete permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagViewPermissions())

		fakeID := "00000000-0000-0000-0000-000000000000"
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodDelete, "/api/v1/tags/"+fakeID, nil, token, orgHeaders)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestTagHandler_Autocomplete tests GET /api/v1/tags/autocomplete?q=...
func TestTagHandler_Autocomplete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("returns tags matching query prefix", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		// Create tags with different prefixes
		createTagViaAPI(t, server, token, map[string]interface{}{"name": "Golang"}, orgHeaders)
		createTagViaAPI(t, server, token, map[string]interface{}{"name": "Go"}, orgHeaders)
		createTagViaAPI(t, server, token, map[string]interface{}{"name": "JavaScript"}, orgHeaders)

		// Autocomplete with "go" prefix
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/autocomplete?q=go", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		tags, ok := data["tags"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(tags), "should match 'Golang' and 'Go'")
	})

	t.Run("returns empty for non-matching query", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		createTagViaAPI(t, server, token, map[string]interface{}{"name": "Python"}, orgHeaders)

		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/autocomplete?q=zzz", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		tags, ok := data["tags"].([]interface{})
		require.True(t, ok)
		assert.Empty(t, tags, "should return empty for non-matching query")
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, orgHeaders := setupTagTest(t, suite, server.Enforcer(), server, tagPermissions())

		// Create 5 tags starting with "test"
		for i := 0; i < 5; i++ {
			createTagViaAPI(t, server, token, map[string]interface{}{
				"name": fmt.Sprintf("Test-%d", i),
			}, orgHeaders)
		}

		// Limit to 2 results
		rec := helpers.MakeRequestWithHeaders(t, server, http.MethodGet, "/api/v1/tags/autocomplete?q=test&limit=2", nil, token, orgHeaders)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		tags, ok := data["tags"].([]interface{})
		require.True(t, ok)
		assert.LessOrEqual(t, len(tags), 2, "should respect limit parameter")
	})
}

// TestTagHandler_Unauthorized tests that all tag endpoints return 401 without a JWT token.
// This test does NOT need organization headers since it's just checking auth rejection.
func TestTagHandler_Unauthorized(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/tags"},
		{http.MethodGet, "/api/v1/tags"},
		{http.MethodGet, "/api/v1/tags/autocomplete?q=test"},
		{http.MethodGet, "/api/v1/tags/slug/test-slug"},
		{http.MethodGet, "/api/v1/tags/00000000-0000-0000-0000-000000000000"},
		{http.MethodPut, "/api/v1/tags/00000000-0000-0000-0000-000000000000"},
		{http.MethodDelete, "/api/v1/tags/00000000-0000-0000-0000-000000000000"},
	}

	for _, ep := range endpoints {
		ep := ep // capture loop variable
		t.Run(ep.method+" "+ep.path+" returns 401", func(t *testing.T) {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"%s %s should return 401 without auth", ep.method, ep.path)
		})
	}
}
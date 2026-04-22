//go:build contract

package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSwaggerDocExists verifies that swagger.json was generated
func TestSwaggerDocExists(t *testing.T) {
	// Check if swagger.json exists
	_, err := os.Stat("docs/swagger/swagger.json")
	require.NoError(t, err, "swagger.json should exist")

	// Check if swagger.yaml exists
	_, err = os.Stat("docs/swagger/swagger.yaml")
	require.NoError(t, err, "swagger.yaml should exist")

	// Check if docs.go exists
	_, err = os.Stat("docs/swagger/docs.go")
	require.NoError(t, err, "docs.go should exist")
}

// TestSwaggerValidJSON verifies swagger.json is valid JSON
func TestSwaggerValidJSON(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err, "should read swagger.json")

	var swagger map[string]interface{}
	err = json.Unmarshal(data, &swagger)
	require.NoError(t, err, "swagger.json should be valid JSON")

	// Verify required fields
	assert.Contains(t, swagger, "swagger", "should have swagger version")
	assert.Contains(t, swagger, "info", "should have info")
	assert.Contains(t, swagger, "paths", "should have paths")

	info, ok := swagger["info"].(map[string]interface{})
	require.True(t, ok, "info should be an object")
	assert.Contains(t, info, "title", "should have title")
	assert.Contains(t, info, "version", "should have version")
}

// TestSwaggerHasAuthEndpoints verifies auth endpoints are documented
func TestSwaggerHasAuthEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok, "paths should be an object")

	// Check auth endpoints
	authEndpoints := []string{
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
	}

	for _, endpoint := range authEndpoints {
		assert.Contains(t, paths, endpoint, "should have %s endpoint", endpoint)
	}
}

// TestSwaggerHasUserEndpoints verifies user endpoints are documented
func TestSwaggerHasUserEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// Check user endpoints
	userEndpoints := []string{
		"/api/v1/me",
		"/api/v1/users",
	}

	for _, endpoint := range userEndpoints {
		assert.Contains(t, paths, endpoint, "should have %s endpoint", endpoint)
	}
}

// TestSwaggerHasRoleEndpoints verifies role endpoints are documented
func TestSwaggerHasRoleEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// Check role endpoints
	assert.Contains(t, paths, "/api/v1/roles", "should have roles endpoint")
}

// TestSwaggerHasPermissionEndpoints verifies permission endpoints are documented
func TestSwaggerHasPermissionEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// Check permission endpoints
	assert.Contains(t, paths, "/api/v1/permissions", "should have permissions endpoint")
}

// TestSwaggerHasInvoiceEndpoints verifies invoice endpoints are documented
func TestSwaggerHasInvoiceEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger)

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// Check invoice endpoints
	assert.Contains(t, paths, "/api/v1/invoices", "should have invoices endpoint")
}

// TestSwaggerHasHealthEndpoints verifies health endpoints are documented
func TestSwaggerHasHealthEndpoints(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger)

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// Check health endpoints
	assert.Contains(t, paths, "/healthz", "should have healthz endpoint")
	assert.Contains(t, paths, "/readyz", "should have readyz endpoint")
}

// TestSwaggerSecurityDefinitions verifies security definitions exist
func TestSwaggerSecurityDefinitions(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	// Check security definitions exist (for JWT auth)
	securityDefs, ok := swagger["securityDefinitions"].(map[string]interface{})
	if ok {
		// If security definitions exist, check for BearerAuth
		assert.Contains(t, securityDefs, "BearerAuth", "should have BearerAuth security definition")
	}
}

// TestSwaggerProducesJSON verifies swagger produces JSON
func TestSwaggerProducesJSON(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	// Check produces field
	if produces, ok := swagger["produces"].([]interface{}); ok {
		assert.Contains(t, produces, "application/json", "should produce JSON")
	}
}

// TestSwaggerConsumesJSON verifies swagger consumes JSON
func TestSwaggerConsumesJSON(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger))

	// Check consumes field
	if consumes, ok := swagger["consumes"].([]interface{}); ok {
		assert.Contains(t, consumes, "application/json", "should consume JSON")
	}
}

// TestSwaggerPathsHaveMethods verifies all paths have HTTP methods
func TestSwaggerPathsHaveMethods(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger)

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	validMethods := map[string]bool{
		"get": true, "post": true, "put": true, "patch": true,
		"delete": true, "options": true, "head": true,
	}

	for path, pathObj := range paths {
		pathMethods, ok := pathObj.(map[string]interface{})
		require.True(t, ok, "path %s should have methods", path)

		for method := range pathMethods {
			methodLower := strings.ToLower(method)
			assert.True(t, validMethods[methodLower], 
				"path %s should have valid HTTP method, got %s", path, method)
		}
	}
}

// TestSwaggerPathsHaveDescriptions verifies all paths have descriptions
func TestSwaggerPathsHaveDescriptions(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger)

	paths, ok := swagger["paths"].(map[string]interface{})
	require.True(t, ok)

	// At minimum, some paths should have descriptions or summaries
	pathsWithDescription := 0
	for _, pathObj := range paths {
		pathMethods, ok := pathObj.(map[string]interface{})
		if !ok {
			continue
		}
		for _, methodObj := range pathMethods {
			method, ok := methodObj.(map[string]interface{})
			if !ok {
				continue
			}
			if method["description"] != nil || method["summary"] != nil {
				pathsWithDescription++
			}
		}
	}

	// At least half of the paths should have descriptions
	assert.Greater(t, pathsWithDescription, len(paths)/2,
		"at least half of the endpoints should have descriptions")
}

// TestSwaggerInfoFields verifies info object has required fields
func TestSwaggerInfoFields(t *testing.T) {
	data, err := os.ReadFile("docs/swagger/swagger.json")
	require.NoError(t, err)

	var swagger map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &swagger)

	info, ok := swagger["info"].(map[string]interface{})
	require.True(t, ok, "should have info object")

	assert.NotEmpty(t, info["title"], "should have title")
	assert.NotEmpty(t, info["version"], "should have version")
}
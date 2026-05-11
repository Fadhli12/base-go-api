//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createFeatureFlagUser creates a user with feature_flag permissions and returns
// (token, cleanup). The user and policies are written directly to the database
// so that the server's own enforcer will load them during initialization.
func createFeatureFlagUser(t *testing.T, suite *TestSuite, enforcer *permission.Enforcer, perms []string) (string, func()) {
	return helpers.CreateUserWithPermissions(t, suite, enforcer, perms)
}

// =============================================================================
// CRUD TESTS
// =============================================================================

func TestFeatureFlag_Create(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("POST /api/v1/feature-flags - create flag (201)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":         "new_dashboard",
			"name":        "New Dashboard",
			"description": "Enable the redesigned dashboard",
			"enabled":     true,
			"rollout":     50,
			"conditions": map[string]interface{}{
				"envs": []string{"production"},
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "new_dashboard", data["key"])
		assert.Equal(t, "New Dashboard", data["name"])
		assert.Equal(t, true, data["enabled"])
		assert.Equal(t, float64(50), data["rollout"])
		assert.NotEmpty(t, data["id"])
	})

	t.Run("POST /api/v1/feature-flags - duplicate key returns 409", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "duplicate_key",
			"name":    "First Flag",
			"enabled": true,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		body2 := map[string]interface{}{
			"key":     "duplicate_key",
			"name":    "Second Flag",
			"enabled": false,
		}

		rec2 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body2, token)
		assert.Equal(t, http.StatusConflict, rec2.Code)

		env := helpers.ParseEnvelope(t, rec2)
		require.NotNil(t, env.Error)
		assert.Equal(t, "CONFLICT", env.Error.Code)
	})
}

func TestFeatureFlag_ListAndGet(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("GET /api/v1/feature-flags - list flags (200)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "list_test_flag",
			"name":    "List Test Flag",
			"enabled": true,
		}
		helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/feature-flags", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, data["flags"])
	})

	t.Run("GET /api/v1/feature-flags/:id - get by ID (200)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "get_test_flag",
			"name":    "Get Test Flag",
			"enabled": true,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		require.Equal(t, http.StatusCreated, rec.Code)

		createEnv := helpers.ParseEnvelope(t, rec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		flagID := createData["id"].(string)

		rec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/feature-flags/"+flagID, nil, token)
		assert.Equal(t, http.StatusOK, rec2.Code)

		env := helpers.ParseEnvelope(t, rec2)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "get_test_flag", data["key"])
		assert.Equal(t, "Get Test Flag", data["name"])
	})
}

func TestFeatureFlag_Update(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("PUT /api/v1/feature-flags/:id - update flag (200)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "update_test_flag",
			"name":    "Original Name",
			"enabled": false,
			"rollout": 0,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		require.Equal(t, http.StatusCreated, rec.Code)

		createEnv := helpers.ParseEnvelope(t, rec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		flagID := createData["id"].(string)

		updateBody := map[string]interface{}{
			"name":    "Updated Name",
			"enabled": true,
			"rollout": 75,
		}
		rec2 := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/feature-flags/"+flagID, updateBody, token)
		assert.Equal(t, http.StatusOK, rec2.Code)

		env := helpers.ParseEnvelope(t, rec2)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Updated Name", data["name"])
		assert.Equal(t, true, data["enabled"])
		assert.Equal(t, float64(75), data["rollout"])
	})
}

func TestFeatureFlag_Delete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("DELETE /api/v1/feature-flags/:id - delete flag (200)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "delete_test_flag",
			"name":    "Delete Test Flag",
			"enabled": true,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		require.Equal(t, http.StatusCreated, rec.Code)

		createEnv := helpers.ParseEnvelope(t, rec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		flagID := createData["id"].(string)

		rec2 := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/feature-flags/"+flagID, nil, token)
		assert.Equal(t, http.StatusOK, rec2.Code)
	})

	t.Run("DELETE /api/v1/feature-flags/:id - system flag cannot be deleted (403)", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		systemFlagID := uuid.New()
		err = suite.DB.Exec(
			"INSERT INTO feature_flags (id, key, name, enabled, rollout, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
			systemFlagID, "system_flag", "System Flag", true, 100, true,
		).Error
		require.NoError(t, err)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/feature-flags/"+systemFlagID.String(), nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error)
		assert.Equal(t, "FORBIDDEN", env.Error.Code)
	})
}

// =============================================================================
// RBAC TESTS
// =============================================================================

func TestFeatureFlag_RBAC(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("POST /api/v1/feature-flags - without feature_flag:manage returns 403", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"key":     "forbidden_flag",
			"name":    "Forbidden Flag",
			"enabled": true,
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", body, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error)
		assert.Equal(t, "FORBIDDEN", env.Error.Code)
	})

	t.Run("GET /api/v1/feature-flags - without feature_flag:view returns 403", func(t *testing.T) {
		suite.SetupTest(t)

		enforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		token, _ := createFeatureFlagUser(t, suite, enforcer, []string{
			"users:view",
		})

		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/feature-flags", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error)
		assert.Equal(t, "FORBIDDEN", env.Error.Code)
	})
}

// =============================================================================
// EVALUATION TESTS
// =============================================================================

func TestFeatureFlag_Evaluate(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("GET /api/v1/feature-flags/:key/evaluate - evaluate flag (200, any authenticated user)", func(t *testing.T) {
		suite.SetupTest(t)

		adminEnforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		adminToken, _ := createFeatureFlagUser(t, suite, adminEnforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		createBody := map[string]interface{}{
			"key":     "eval_test_flag",
			"name":    "Eval Test Flag",
			"enabled": true,
			"rollout": 100,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", createBody, adminToken)
		require.Equal(t, http.StatusCreated, rec.Code)

		evalEnforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		evalToken, _ := createFeatureFlagUser(t, suite, evalEnforcer, []string{
			"users:view",
		})

		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/feature-flags/eval_test_flag/evaluate", nil, evalToken)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "eval_test_flag", data["key"])
		assert.Equal(t, true, data["enabled"])
	})

	t.Run("GET /api/v1/feature-flags/evaluate - evaluate all flags (200, any authenticated user)", func(t *testing.T) {
		suite.SetupTest(t)

		adminEnforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		adminToken, _ := createFeatureFlagUser(t, suite, adminEnforcer, []string{
			"feature_flag:manage",
			"feature_flag:view",
		})

		server := helpers.NewTestServer(t, suite)

		flag1 := map[string]interface{}{
			"key":     "bulk_eval_flag_1",
			"name":    "Bulk Eval Flag 1",
			"enabled": true,
			"rollout": 100,
		}
		helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", flag1, adminToken)

		flag2 := map[string]interface{}{
			"key":     "bulk_eval_flag_2",
			"name":    "Bulk Eval Flag 2",
			"enabled": false,
			"rollout": 0,
		}
		helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/feature-flags", flag2, adminToken)

		evalEnforcer, err := permission.NewEnforcer(suite.DB)
		require.NoError(t, err)

		evalToken, _ := createFeatureFlagUser(t, suite, evalEnforcer, []string{
			"users:view",
		})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/feature-flags/evaluate", nil, evalToken)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, data["flags"])

		flags, ok := data["flags"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(flags), 2)
	})
}
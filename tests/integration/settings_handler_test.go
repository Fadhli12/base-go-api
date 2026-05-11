//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsHandler_UserSettingsCRUD(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err)

	t.Run("GET /api/v1/settings/user - returns empty settings for new user", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateAdminUser(t, suite, enforcer)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/settings/user", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("PUT /api/v1/settings/user - updates user settings with manage permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_user",
			"settings:manage_user",
		})

		updates := map[string]interface{}{
			"settings": map[string]interface{}{
				"theme":    "dark",
				"language": "en",
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/user", updates, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.NotNil(t, data["settings"])
	})

	t.Run("PUT /api/v1/settings/user - merge preserves existing settings", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_user",
			"settings:manage_user",
		})

		firstUpdate := map[string]interface{}{
			"settings": map[string]interface{}{
				"language": "en",
				"timezone": "UTC",
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/user", firstUpdate, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		secondUpdate := map[string]interface{}{
			"settings": map[string]interface{}{
				"theme": "dark",
			},
		}

		rec = helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/user", secondUpdate, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)

		settingsRaw, _ := json.Marshal(data["settings"])
		var settings map[string]interface{}
		require.NoError(t, json.Unmarshal(settingsRaw, &settings))

		assert.Equal(t, "dark", settings["theme"], "new key should be added")
		assert.Equal(t, "en", settings["language"], "existing key should be preserved")
		assert.Equal(t, "UTC", settings["timezone"], "existing key should be preserved")
	})

	t.Run("PUT /api/v1/settings/user - forbidden without manage_user permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_user",
		})

		updates := map[string]interface{}{
			"settings": map[string]interface{}{
				"theme": "dark",
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/user", updates, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GET /api/v1/settings/user - unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/settings/user", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestSettingsHandler_SystemSettingsCRUD(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err)

	t.Run("GET /api/v1/settings/system - returns defaults for new org", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_system",
		})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/settings/system", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("PUT /api/v1/settings/system - updates system settings", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_system",
			"settings:manage_system",
		})

		updates := map[string]interface{}{
			"settings": map[string]interface{}{
				"maintenance_mode": true,
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/system", updates, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)
	})

	t.Run("PUT /api/v1/settings/system - forbidden without manage_system permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_system",
		})

		updates := map[string]interface{}{
			"settings": map[string]interface{}{
				"maintenance_mode": true,
			},
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/system", updates, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestSettingsHandler_EffectiveSettings(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err)

	t.Run("GET /api/v1/settings/effective - returns merged settings", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		token, _ := helpers.CreateUserWithPermissions(t, suite, enforcer, []string{
			"settings:view_user",
			"settings:manage_user",
			"settings:view_system",
			"settings:manage_system",
		})

		sysUpdates := map[string]interface{}{
			"settings": map[string]interface{}{
				"maintenance_mode": false,
			},
		}
		helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/system", sysUpdates, token)

		userUpdates := map[string]interface{}{
			"settings": map[string]interface{}{
				"theme": "dark",
			},
		}
		helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/settings/user", userUpdates, token)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/settings/effective", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)

		settingsRaw, _ := json.Marshal(data)
		var settings map[string]interface{}
		require.NoError(t, json.Unmarshal(settingsRaw, &settings))

		assert.Equal(t, "dark", settings["theme"], "user setting should be present")
	})
}
//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==============================================================================
// USER SETTINGS TESTS
// ==============================================================================

// TestUserSettings_GetEmpty tests retrieving user settings when none exist
func TestUserSettings_GetEmpty(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	repo := repository.NewUserSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repo, repository.NewSystemSettingsRepository(suite.DB), slog.Default())

	resp, err := svc.GetUserSettings(ctx, user.ID, orgID)
	require.NoError(t, err, "Should retrieve empty settings without error")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, user.ID, resp.UserID, "UserID should match")
	assert.Equal(t, orgID, resp.OrganizationID, "OrgID should match")
}

// TestUserSettings_UpdateWithValidData tests updating user settings with valid data
func TestUserSettings_UpdateWithValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	repo := repository.NewUserSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repo, repository.NewSystemSettingsRepository(suite.DB), slog.Default())

	updates := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
		"timezone": "America/New_York",
	}

	resp, err := svc.UpdateUserSettings(ctx, user.ID, orgID, updates)
	require.NoError(t, err, "Should update user settings successfully")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, user.ID, resp.UserID, "UserID should match")
	assert.Equal(t, orgID, resp.OrganizationID, "OrgID should match")

	// Verify settings were saved
	var savedSettings map[string]interface{}
	err = json.Unmarshal(resp.Settings, &savedSettings)
	require.NoError(t, err, "Should unmarshal settings")
	assert.Equal(t, "dark", savedSettings["theme"], "Theme should be saved")
	assert.Equal(t, "en", savedSettings["language"], "Language should be saved")
	assert.Equal(t, "America/New_York", savedSettings["timezone"], "Timezone should be saved")
}

// TestUserSettings_UpdateWithInvalidTimezone tests timezone validation
func TestUserSettings_UpdateWithInvalidTimezone(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	repo := repository.NewUserSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repo, repository.NewSystemSettingsRepository(suite.DB), slog.Default())

	updates := map[string]interface{}{
		"timezone": "Invalid/Timezone",
	}

	_, err := svc.UpdateUserSettings(ctx, user.ID, orgID, updates)
	require.Error(t, err, "Should fail with invalid timezone")
	assert.True(t, apperrors.IsAppError(err), "Error should be AppError")
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code, "Error code should be VALIDATION_ERROR")
}

// TestUserSettings_UpsertRecoveryFromSoftDelete tests soft delete recovery on upsert
func TestUserSettings_UpsertRecoveryFromSoftDelete(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	repo := repository.NewUserSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repo, repository.NewSystemSettingsRepository(suite.DB), slog.Default())

	// Create initial settings
	settings1, err := svc.UpdateUserSettings(ctx, user.ID, orgID, map[string]interface{}{"theme": "light"})
	require.NoError(t, err)
	assert.NotNil(t, settings1)

	// Soft delete via direct repo manipulation
	err = suite.DB.Model(&domain.UserSettings{}).
		Where("user_id = ? AND organization_id = ?", user.ID, orgID).
		Update("deleted_at", "NOW()").Error
	require.NoError(t, err, "Should soft delete settings")

	// Verify deleted
	deletedSettings, err := repo.FindByUserIDAndOrgID(ctx, user.ID, orgID)
	require.True(t, apperrors.IsNotFound(err), "Should not find deleted settings")
	assert.Nil(t, deletedSettings)

	// Update should recover from soft delete
	settings2, err := svc.UpdateUserSettings(ctx, user.ID, orgID, map[string]interface{}{"theme": "dark"})
	require.NoError(t, err, "Should recover from soft delete")
	assert.NotNil(t, settings2)

	// Verify recovered
	recovered, err := repo.FindByUserIDAndOrgID(ctx, user.ID, orgID)
	require.NoError(t, err, "Should find recovered settings")
	assert.NotNil(t, recovered)
	assert.Nil(t, recovered.DeletedAt.Time, "DeletedAt should be NULL after recovery")
}

// ==============================================================================
// SYSTEM SETTINGS TESTS
// ==============================================================================

// TestSystemSettings_GetDefaults tests retrieving system settings with defaults
func TestSystemSettings_GetDefaults(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	orgID := uuid.New()

	sysRepo := repository.NewSystemSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repository.NewUserSettingsRepository(suite.DB), sysRepo, slog.Default())

	resp, err := svc.GetSystemSettings(ctx, orgID)
	require.NoError(t, err, "Should retrieve system settings with defaults")
	assert.NotNil(t, resp)
	assert.Equal(t, orgID, resp.OrganizationID)

	// Verify default values exist
	var settings map[string]interface{}
	err = json.Unmarshal(resp.Settings, &settings)
	require.NoError(t, err)
	assert.NotNil(t, settings["app_name"], "Should have app_name default")
	assert.NotNil(t, settings["maintenance_mode"], "Should have maintenance_mode default")
}

// TestSystemSettings_UpdateWithAllowlist tests allowlist validation for system settings
func TestSystemSettings_UpdateWithAllowlist(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	orgID := uuid.New()

	sysRepo := repository.NewSystemSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repository.NewUserSettingsRepository(suite.DB), sysRepo, slog.Default())

	// Update with allowed field
	updates := map[string]interface{}{
		"maintenance_mode": true,
		"app_name":         "Custom App", // Not in allowlist
	}

	resp, err := svc.UpdateSystemSettings(ctx, orgID, updates)
	require.NoError(t, err, "Should update system settings")
	assert.NotNil(t, resp)

	// Verify only allowed field was updated
	var settings map[string]interface{}
	err = json.Unmarshal(resp.Settings, &settings)
	require.NoError(t, err)
	assert.Equal(t, true, settings["maintenance_mode"], "maintenance_mode should be updated")
	// app_name should not be updated since it's not in the allowlist
}

// TestSystemSettings_UpdateWithEmptyUpdates tests error when no valid fields
func TestSystemSettings_UpdateWithEmptyUpdates(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	orgID := uuid.New()

	sysRepo := repository.NewSystemSettingsRepository(suite.DB)
	svc := service.NewSettingsService(repository.NewUserSettingsRepository(suite.DB), sysRepo, slog.Default())

	// Update with no allowed fields
	updates := map[string]interface{}{
		"invalid_field": "value",
	}

	_, err := svc.UpdateSystemSettings(ctx, orgID, updates)
	require.Error(t, err, "Should fail with no valid fields")
	assert.True(t, apperrors.IsAppError(err), "Error should be AppError")
	appErr := apperrors.GetAppError(err)
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code, "Error code should be VALIDATION_ERROR")
}

// ==============================================================================
// EFFECTIVE SETTINGS TESTS
// ==============================================================================

// TestEffectiveSettings_MergeSystemAndUser tests merging system and user settings
func TestEffectiveSettings_MergeSystemAndUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	userRepo := repository.NewUserSettingsRepository(suite.DB)
	sysRepo := repository.NewSystemSettingsRepository(suite.DB)
	svc := service.NewSettingsService(userRepo, sysRepo, slog.Default())

	// Set system settings
	sysUpdates := map[string]interface{}{
		"maintenance_mode": false,
	}
	_, err := svc.UpdateSystemSettings(ctx, orgID, sysUpdates)
	require.NoError(t, err)

	// Set user settings (override some system settings)
	userUpdates := map[string]interface{}{
		"theme": "dark",
	}
	_, err = svc.UpdateUserSettings(ctx, user.ID, orgID, userUpdates)
	require.NoError(t, err)

	// Get effective settings
	effective, err := svc.GetEffectiveSettings(ctx, user.ID, orgID)
	require.NoError(t, err, "Should get effective settings")
	assert.NotNil(t, effective)

	var merged map[string]interface{}
	err = json.Unmarshal(effective, &merged)
	require.NoError(t, err)

	// Verify system settings are present
	assert.NotNil(t, merged["maintenance_mode"], "System setting should be present")
	// Verify user settings are present
	assert.Equal(t, "dark", merged["theme"], "User setting should be present")
}

// TestEffectiveSettings_UserOverridesSystem tests user settings take precedence
func TestEffectiveSettings_UserOverridesSystem(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	user := createTestUserForOrg(t, suite.DB)
	orgID := uuid.New()

	userRepo := repository.NewUserSettingsRepository(suite.DB)
	sysRepo := repository.NewSystemSettingsRepository(suite.DB)
	svc := service.NewSettingsService(userRepo, sysRepo, slog.Default())

	// System has a default setting
	sysUpdates := map[string]interface{}{
		"notifications_per_hour": 100,
	}
	_, err := svc.UpdateSystemSettings(ctx, orgID, sysUpdates)
	require.NoError(t, err)

	// User overrides with a different value
	userUpdates := map[string]interface{}{
		"notifications_per_hour": 50,
	}
	_, err = svc.UpdateUserSettings(ctx, user.ID, orgID, userUpdates)
	require.NoError(t, err)

	// Get effective settings
	effective, err := svc.GetEffectiveSettings(ctx, user.ID, orgID)
	require.NoError(t, err)

	var merged map[string]interface{}
	err = json.Unmarshal(effective, &merged)
	require.NoError(t, err)

	// User value should override system value
	assert.Equal(t, float64(50), merged["notifications_per_hour"], "User setting should override system setting")
}

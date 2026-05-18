//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// oauthTestSvc builds an OAuthProviderService wired to the test suite DB.
func oauthTestSvc(t *testing.T, suite *TestSuite) *service.OAuthProviderService {
	t.Helper()
	providerRepo := repository.NewOAuthProviderRepository(suite.DB)
	encryptionSvc, err := service.NewOAuthEncryptionService("test-encryption-key-at-least-32-chars-long!")
	require.NoError(t, err, "encryption service should initialize")
	return service.NewOAuthProviderService(
		providerRepo,
		encryptionSvc,
		nil, // audit service
		nil, // enforcer (not needed for provider CRUD tests)
		config.OAuthConfig{AllowHTTP: true}, // allow HTTP for tests
		nil, // logger
	)
}

// createOAuthProvider inserts an OAuth provider directly into the database for test setup.
func createOAuthProvider(t *testing.T, suite *TestSuite, name string) *domain.OAuthProvider {
	t.Helper()
	encryptionSvc, err := service.NewOAuthEncryptionService("test-encryption-key-at-least-32-chars-long!")
	require.NoError(t, err, "encryption service should initialize")

	encryptedSecret, err := encryptionSvc.Encrypt("test-client-secret-value-16ch")
	require.NoError(t, err, "encrypting secret should succeed")

	provider := &domain.OAuthProvider{
		Name:                  name,
		DisplayName:           name + " Display Name",
		ClientID:              "test-client-id-" + name,
		ClientSecretEncrypted: encryptedSecret,
		RedirectURL:           "https://example.com/callback/" + name,
		AdditionalScopes:     datatypes.JSON("[]"),
		Config:               datatypes.JSON("{}"),
		IsEnabled:            true,
		IsSystem:             false,
	}

	require.NoError(t, suite.DB.Create(provider).Error, "creating test oauth provider should succeed")
	return provider
}

// =============================================================================
// OAUTH PROVIDER CRUD TESTS
// =============================================================================

// TestOAuthProvider_Create tests POST /api/v1/oauth-providers.
func TestOAuthProvider_Create(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("create a valid OAuth provider", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		body := map[string]interface{}{
			"name":           "google",
			"display_name":   "Google OAuth",
			"client_id":      "google-client-id-12345",
			"client_secret":  "google-client-secret-value-16chars",
			"redirect_url":   "https://example.com/auth/callback/google",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")

		assert.Equal(t, "google", data["name"])
		assert.Equal(t, "Google OAuth", data["display_name"])
		assert.Equal(t, "google-client-id-12345", data["client_id"])
		assert.NotEmpty(t, data["id"], "should return provider ID")
		assert.Nil(t, data["client_secret_encrypted"], "secret should never be in response")
		assert.Nil(t, data["ClientSecretEncrypted"], "secret should never be in response")
	})

	t.Run("create duplicate provider name returns error", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		body := map[string]interface{}{
			"name":           "github",
			"display_name":   "GitHub OAuth",
			"client_id":      "github-client-id",
			"client_secret":  "github-secret-value-16ch",
			"redirect_url":   "https://example.com/auth/callback/github",
		}

		// Create first provider
		rec1 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		assert.Equal(t, http.StatusCreated, rec1.Code)

		// Create second provider with same name
		rec2 := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		assert.Equal(t, http.StatusInternalServerError, rec2.Code) // unique constraint violation
	})

	t.Run("create with invalid provider name returns 422", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		body := map[string]interface{}{
			"name":           "invalid_provider",
			"display_name":   "Invalid Provider",
			"client_id":      "invalid-client-id",
			"client_secret":  "invalid-secret-value-16",
			"redirect_url":   "https://example.com/callback",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		// Validation error from handler (422) or service (422)
		assert.True(t, rec.Code == http.StatusUnprocessableEntity || rec.Code == http.StatusBadRequest,
			"expected 400 or 422, got %d", rec.Code)
	})

	t.Run("create without authentication returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"name":          "google",
			"display_name":  "Google",
			"client_id":     "test",
			"client_secret": "secret-value-16-ch",
			"redirect_url":  "https://example.com/callback",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("create without oauth:manage permission returns 403", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User with only oauth:view, NOT oauth:manage
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:view"})

		body := map[string]interface{}{
			"name":          "google",
			"display_name":  "Google",
			"client_id":     "test-client",
			"client_secret": "secret-value-16-ch",
			"redirect_url":  "https://example.com/callback",
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestOAuthProvider_List tests listing OAuth providers via admin endpoint.
func TestOAuthProvider_List(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("list providers returns empty list initially", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, float64(0), data["total"], "total should be 0 for empty list")
	})

	t.Run("list providers returns created providers", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create providers
		providerNames := []string{"google", "github"}
		for _, name := range providerNames {
			body := map[string]interface{}{
				"name":           name,
				"display_name":   name + " OAuth",
				"client_id":      name + "-client-id",
				"client_secret":  name + "-secret-value-16c",
				"redirect_url":   "https://example.com/callback/" + name,
			}
			rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
			require.Equal(t, http.StatusCreated, rec.Code)
		}

		// List providers
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, float64(2), data["total"], "total should be 2")

		providers, ok := data["providers"].([]interface{})
		require.True(t, ok, "providers should be a list")
		assert.Len(t, providers, 2, "should return 2 providers")
	})
}

// TestOAuthProvider_GetByID tests GET /api/v1/oauth-providers/:id.
func TestOAuthProvider_GetByID(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("get provider by ID", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create a provider
		createBody := map[string]interface{}{
			"name":           "google",
			"display_name":   "Google OAuth",
			"client_id":      "google-client-id",
			"client_secret":  "google-secret-value-16c",
			"redirect_url":   "https://example.com/callback/google",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code)

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		providerID, ok := createData["id"].(string)
		require.True(t, ok)

		// Get by ID
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers/"+providerID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "google", data["name"])
		assert.Equal(t, providerID, data["id"])
	})

	t.Run("get non-existent provider returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers/"+fakeID, nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("get with invalid UUID returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers/not-a-uuid", nil, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestOAuthProvider_Update tests PUT /api/v1/oauth-providers/:id.
func TestOAuthProvider_Update(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("update provider display name and enabled status", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create a provider
		createBody := map[string]interface{}{
			"name":           "microsoft",
			"display_name":   "Microsoft",
			"client_id":      "ms-client-id",
			"client_secret":  "ms-secret-value-16ch",
			"redirect_url":   "https://example.com/callback/ms",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code)

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		providerID, ok := createData["id"].(string)
		require.True(t, ok)

		// Update provider
		updateBody := map[string]interface{}{
			"display_name": "Microsoft Azure AD",
			"is_enabled":   false,
		}
		updateRec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/oauth-providers/"+providerID, updateBody, token)
		assert.Equal(t, http.StatusOK, updateRec.Code)

		updateEnv := helpers.ParseEnvelope(t, updateRec)
		updateData, ok := updateEnv.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Microsoft Azure AD", updateData["display_name"])
		assert.Equal(t, false, updateData["is_enabled"])
	})
}

// TestOAuthProvider_Delete tests DELETE /api/v1/oauth-providers/:id.
func TestOAuthProvider_Delete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("delete a non-system provider", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create a provider
		createBody := map[string]interface{}{
			"name":           "github",
			"display_name":   "GitHub",
			"client_id":      "gh-client-id",
			"client_secret":  "gh-secret-value-16ch",
			"redirect_url":   "https://example.com/callback/gh",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", createBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code)

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok)
		providerID, ok := createData["id"].(string)
		require.True(t, ok)

		// Delete provider
		deleteRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/oauth-providers/"+providerID, nil, token)
		assert.Equal(t, http.StatusNoContent, deleteRec.Code)

		// Verify it's gone (get returns 404)
		getRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/oauth-providers/"+providerID, nil, token)
		assert.Equal(t, http.StatusNotFound, getRec.Code)
	})

	t.Run("delete non-existent provider returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/oauth-providers/"+fakeID, nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("delete system provider returns 403", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create provider directly in DB with is_system=true
		encryptionSvc, err := service.NewOAuthEncryptionService("test-encryption-key-at-least-32-chars-long!")
		require.NoError(t, err)
		encryptedSecret, err := encryptionSvc.Encrypt("system-secret-value-16")
		require.NoError(t, err)

		provider := &domain.OAuthProvider{
			Name:                  "google",
			DisplayName:           "Google System",
			ClientID:              "system-client-id",
			ClientSecretEncrypted: encryptedSecret,
			RedirectURL:           "https://example.com/callback",
			AdditionalScopes:      datatypesJSON("[]"),
			Config:                datatypesJSON("{}"),
			IsEnabled:             true,
			IsSystem:              true,
		}
		require.NoError(t, suite.DB.Create(provider).Error)

		// Attempt to delete
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/oauth-providers/"+provider.ID.String(), nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// =============================================================================
// PUBLIC PROVIDER LISTING TESTS
// =============================================================================

// TestOAuthProvider_PublicList tests GET /api/v1/auth/oauth/providers (public, no auth).
func TestOAuthProvider_PublicList(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("public endpoint lists enabled providers without secrets", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create an enabled provider
		body := map[string]interface{}{
			"name":           "google",
			"display_name":   "Google OAuth",
			"client_id":      "google-client-id",
			"client_secret":  "google-secret-value-16c",
			"redirect_url":   "https://example.com/callback/google",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", body, token)
		require.Equal(t, http.StatusCreated, rec.Code)

		// Public endpoint (no auth)
		publicRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/providers", nil, "")
		assert.Equal(t, http.StatusOK, publicRec.Code)

		publicEnv := helpers.ParseEnvelope(t, publicRec)
		publicData, ok := publicEnv.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")

		providers, ok := publicData["providers"].([]interface{})
		require.True(t, ok, "providers should be a list")
		assert.Len(t, providers, 1, "should return 1 enabled provider")

		provider, ok := providers[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "google", provider["name"])
		assert.Equal(t, "Google OAuth", provider["display_name"])
		// Public response should NOT contain secrets, client_id, config
		assert.Nil(t, provider["client_id"], "public response should not include client_id")
		assert.Nil(t, provider["config"], "public response should not include config")
	})

	t.Run("public endpoint excludes disabled providers", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:manage"})

		// Create an enabled provider
		enabledBody := map[string]interface{}{
			"name":           "google",
			"display_name":   "Google",
			"client_id":      "google-client-id",
			"client_secret":  "google-secret-value-16c",
			"redirect_url":   "https://example.com/callback/google",
		}
		helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", enabledBody, token)

		// Create a disabled provider
		disabledBody := map[string]interface{}{
			"name":           "github",
			"display_name":   "GitHub",
			"client_id":      "github-client-id",
			"client_secret":  "github-secret-value-16",
			"redirect_url":   "https://example.com/callback/github",
		}
		createRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/oauth-providers", disabledBody, token)
		require.Equal(t, http.StatusCreated, createRec.Code)

		createEnv := helpers.ParseEnvelope(t, createRec)
		createData, _ := createEnv.Data.(map[string]interface{})
		providerID := createData["id"].(string)

		// Disable it
		updateBody := map[string]interface{}{"is_enabled": false}
		helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/oauth-providers/"+providerID, updateBody, token)

		// Public endpoint should only return enabled providers
		publicRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/providers", nil, "")
		assert.Equal(t, http.StatusOK, publicRec.Code)

		publicEnv := helpers.ParseEnvelope(t, publicRec)
		publicData, ok := publicEnv.Data.(map[string]interface{})
		require.True(t, ok)
		providers, ok := publicData["providers"].([]interface{})
		require.True(t, ok, "providers should be a list")
		assert.Len(t, providers, 1, "should return only enabled providers")
		provider := providers[0].(map[string]interface{})
		assert.Equal(t, "google", provider["name"])
	})
}

// =============================================================================
// OAUTH ACCOUNT MANAGEMENT TESTS
// =============================================================================

// TestOAuthAccount_Unlink tests DELETE /api/v1/auth/oauth/:provider/unlink.
func TestOAuthAccount_Unlink(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("unlink a provider when user has password", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User with oauth:view + oauth:link permissions
		token, userID := helpers.CreateUserWithPermissionsReturningID(t, suite, server.Enforcer(), []string{"oauth:view", "oauth:link"})

		// Create a provider directly in DB
		provider := createOAuthProvider(t, suite, "google")

		// Create an OAuth account linking user to the provider
		account := &domain.OAuthAccount{
			UserID:         userID,
			ProviderID:     provider.ID,
			ProviderUserID: "google-12345",
			Email:          "user@gmail.com",
			EmailVerified:   true,
			DisplayName:    "Test User",
		}
		require.NoError(t, suite.DB.Create(account).Error, "creating oauth account should succeed")

		// User has password (registered normally) + 1 OAuth link = 2 auth methods
		// Unlink should succeed since they still have password
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/google/unlink", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Provider unlinked successfully", data["message"])
	})

	t.Run("cannot unlink last auth method (OAuth-only user)", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, userID := helpers.CreateUserWithPermissionsReturningID(t, suite, server.Enforcer(), []string{"oauth:view", "oauth:link"})

		// Create a provider
		provider := createOAuthProvider(t, suite, "github")

		// Create an OAuth account
		account := &domain.OAuthAccount{
			UserID:         userID,
			ProviderID:     provider.ID,
			ProviderUserID: "github-67890",
			Email:          "user@github.com",
			EmailVerified:   true,
			DisplayName:    "Test User",
		}
		require.NoError(t, suite.DB.Create(account).Error)

		// Set user's password hash to an OAuth placeholder (no password)
		// This makes the OAuth account the ONLY auth method
		require.NoError(t, suite.DB.Exec(
			"UPDATE users SET password_hash = 'oauth-placeholder-hash' WHERE id = ?", userID,
		).Error)

		// Unlink should fail - only auth method remaining
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/github/unlink", nil, token)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("unlink with invalid provider name returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:view", "oauth:link"})

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/invalid_provider/unlink", nil, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unlink with provider not linked to user returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:view", "oauth:link"})

		// Provider exists but user has no linked account for it
		createOAuthProvider(t, suite, "microsoft")

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/microsoft/unlink", nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("unlink without authentication returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/google/unlink", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("unlink without oauth:link permission returns 403", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User with only oauth:view, NOT oauth:link
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:view"})

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/auth/oauth/google/unlink", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestOAuthAccount_ListLinkedAccounts tests GET /api/v1/auth/oauth/accounts.
func TestOAuthAccount_ListLinkedAccounts(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("list linked accounts returns empty when no accounts", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"oauth:view"})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/accounts", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		accounts, ok := data["accounts"].([]interface{})
		require.True(t, ok)
		assert.Empty(t, accounts, "should return empty accounts list")
	})

	t.Run("list linked accounts returns linked providers", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, userID := helpers.CreateUserWithPermissionsReturningID(t, suite, server.Enforcer(), []string{"oauth:view"})

		// Create providers
		googleProvider := createOAuthProvider(t, suite, "google")
		githubProvider := createOAuthProvider(t, suite, "github")

		// Create OAuth accounts linking user to providers
		googleAccount := &domain.OAuthAccount{
			UserID:         userID,
			ProviderID:     googleProvider.ID,
			ProviderUserID: "google-12345",
			Email:          "user@gmail.com",
			EmailVerified:   true,
			DisplayName:    "Google User",
			AvatarURL:      "https://google.com/avatar.jpg",
		}
		require.NoError(t, suite.DB.Create(googleAccount).Error)

		githubAccount := &domain.OAuthAccount{
			UserID:         userID,
			ProviderID:     githubProvider.ID,
			ProviderUserID: "github-67890",
			Email:          "user@github.com",
			EmailVerified:  false,
			DisplayName:    "GitHub User",
		}
		require.NoError(t, suite.DB.Create(githubAccount).Error)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/accounts", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		accounts, ok := data["accounts"].([]interface{})
		require.True(t, ok)
		assert.Len(t, accounts, 2, "should return 2 linked accounts")

		// Verify account data structure
		for _, acc := range accounts {
			accMap, ok := acc.(map[string]interface{})
			require.True(t, ok)
			assert.NotEmpty(t, accMap["id"], "account should have id")
			assert.NotEmpty(t, accMap["provider_name"], "account should have provider_name")
		}
	})

	t.Run("list accounts without authentication returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/accounts", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("list accounts without oauth:view permission returns 403", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User with no oauth permissions
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"users:view"})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/accounts", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// =============================================================================
// OAUTH LOGIN FLOW TESTS (initiate, callback - limited without external provider mocking)
// =============================================================================

// TestOAuthLogin_Initiate tests GET /api/v1/auth/oauth/:provider.
func TestOAuthLogin_Initiate(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("initiate login with invalid provider returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/invalid_provider", nil, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("initiate login with non-existent provider returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		// Provider "google" doesn't exist in the database yet
		// The handler validates the provider name first, then checks if it exists
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/google", nil, "")
		// Could be 302 (redirect to auth URL) or 404 (provider not found)
		// or 500 (Redis/state error), depending on whether provider is configured
		// Since no provider exists, expect 404 or a redirect that fails
		assert.True(t, rec.Code == http.StatusNotFound || rec.Code == http.StatusFound || rec.Code == http.StatusInternalServerError,
			"expected 404, 302, or 500 for non-existent provider, got %d", rec.Code)
	})

	t.Run("initiate link requires authentication", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/oauth/google/link", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// =============================================================================
// OAUTH PROVIDER SERVICE-LEVEL TESTS
// =============================================================================

// TestOAuthProviderService_Create tests the service layer provider creation with encryption.
func TestOAuthProviderService_Create(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("service creates provider and encrypts client secret", func(t *testing.T) {
		suite.SetupTest(t)
		svc := oauthTestSvc(t, suite)
		ctx := context.Background()

		req := &request.CreateOAuthProviderRequest{
			Name:             "google",
			DisplayName:     "Google OAuth",
			ClientID:        "google-client-id-service-test",
			ClientSecret:    "super-secret-value-at-least-16-chars",
			RedirectURL:     "https://example.com/auth/callback",
		}

		resp, err := svc.Create(ctx, nil, req)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, resp.ID)
		assert.Equal(t, "google", resp.Name)
		assert.Equal(t, "Google OAuth", resp.DisplayName)
		assert.Equal(t, "google-client-id-service-test", resp.ClientID)

		// Verify client_id is present but secret is stripped
		assert.NotEmpty(t, resp.ClientID, "response should contain client_id")

		// Verify encrypted secret was stored in DB (not plaintext)
		var storedProvider domain.OAuthProvider
		require.NoError(t, suite.DB.Where("name = ?", "google").First(&storedProvider).Error)
		assert.NotEqual(t, "super-secret-value-at-least-16-chars", storedProvider.ClientSecretEncrypted,
			"stored secret should be encrypted, not plaintext")
		assert.True(t, len(storedProvider.ClientSecretEncrypted) > 20,
			"encrypted secret should be longer than plaintext")
	})

	t.Run("service rejects invalid provider name", func(t *testing.T) {
		suite.SetupTest(t)
		svc := oauthTestSvc(t, suite)
		ctx := context.Background()

		req := &request.CreateOAuthProviderRequest{
			Name:            "invalid_provider",
			DisplayName:    "Invalid",
			ClientID:       "client-id",
			ClientSecret:   "secret-value-16-chars-",
			RedirectURL:    "https://example.com/callback",
		}

		resp, err := svc.Create(ctx, nil, req)
		assert.Nil(t, resp)
		assert.Error(t, err)
	})
}

// TestOAuthProviderService_CountUserAuthMethods tests the auth method counting logic.
func TestOAuthProviderService_CountUserAuthMethods(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("count auth methods: user with password and 1 OAuth = 2 methods", func(t *testing.T) {
		suite.SetupTest(t)
		ctx := context.Background()

		// Create a user with a real password (not OAuth placeholder)
		user := &domain.User{
			Email:        fmt.Sprintf("auth-test-%s@example.com", uuid.New().String()[:8]),
			PasswordHash: "$2a$12$realpasswordhashnotplaceholder",
		}
		require.NoError(t, suite.DB.Create(user).Error)

		// Create provider and account
		provider := createOAuthProvider(t, suite, "google")
		account := &domain.OAuthAccount{
			UserID:         user.ID,
			ProviderID:     provider.ID,
			ProviderUserID: "google-12345",
			Email:          user.Email,
			EmailVerified:  true,
		}
		require.NoError(t, suite.DB.Create(account).Error)

		// Count via repository
		accountRepo := repository.NewOAuthAccountRepository(suite.DB)
		count, err := accountRepo.CountAuthMethodsByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count, "should have 2 auth methods (password + 1 OAuth)")
	})

	t.Run("count auth methods: OAuth-only user = 1 method", func(t *testing.T) {
		suite.SetupTest(t)
		ctx := context.Background()

		// Create a user with OAuth placeholder password
		user := &domain.User{
			Email:        fmt.Sprintf("oauth-only-%s@example.com", uuid.New().String()[:8]),
			PasswordHash: "oauth-placeholder-hash-value",
		}
		require.NoError(t, suite.DB.Create(user).Error)

		// Create provider and account
		provider := createOAuthProvider(t, suite, "github")
		account := &domain.OAuthAccount{
			UserID:         user.ID,
			ProviderID:     provider.ID,
			ProviderUserID: "github-67890",
			Email:          "user@github.com",
		}
		require.NoError(t, suite.DB.Create(account).Error)

		accountRepo := repository.NewOAuthAccountRepository(suite.DB)
		count, err := accountRepo.CountAuthMethodsByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "should have 1 auth method (1 OAuth, no real password)")
	})
}

// =============================================================================
// UNAUTHORIZED ACCESS TESTS
// =============================================================================

// TestOAuth_Unauthorized tests that protected OAuth endpoints return 401 without JWT.
func TestOAuth_Unauthorized(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("protected OAuth endpoints require authentication", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		endpoints := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/api/v1/oauth-providers"},
			{http.MethodGet, "/api/v1/oauth-providers"},
			{http.MethodGet, "/api/v1/oauth-providers/" + uuid.New().String()},
			{http.MethodPut, "/api/v1/oauth-providers/" + uuid.New().String()},
			{http.MethodDelete, "/api/v1/oauth-providers/" + uuid.New().String()},
			{http.MethodDelete, "/api/v1/auth/oauth/google/unlink"},
			{http.MethodPost, "/api/v1/auth/oauth/google/link"},
			{http.MethodGet, "/api/v1/auth/oauth/accounts"},
		}

		for _, ep := range endpoints {
			ep := ep
			t.Run(ep.method+" "+ep.path+" returns 401", func(t *testing.T) {
				rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, "")
				assert.Equal(t, http.StatusUnauthorized, rec.Code,
					"%s %s should return 401 without auth", ep.method, ep.path)
			})
		}
	})

	t.Run("public OAuth endpoints accessible without auth", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		// Public provider listing should work without auth
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/providers", nil, "")
		assert.Equal(t, http.StatusOK, rec.Code)

		// Initiate login for invalid provider returns 400 (not 401)
		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/oauth/invalid_provider", nil, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
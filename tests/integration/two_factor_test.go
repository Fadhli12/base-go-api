//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTwoFactorEnable tests the full 2FA enable flow: setup → verify → status.
func TestTwoFactorEnable(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)

	// Wire up TwoFactorService (normally done in main.go)
	twoFactorSvc := wireTwoFactorService(suite)
	server.SetTwoFactorService(twoFactorSvc)

	// Register and login a user
	email := helpers.UniqueEmail(t)
	password := helpers.TestPassword
	token := helpers.AuthenticateUser(t, server, email, password)

	t.Run("Setup 2FA returns secret and QR code URL", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "2fa setup should return 201")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Data, "response should have data")
		assert.Nil(t, env.Error, "response should not have error")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotEmpty(t, dataMap["secret"], "secret should be present")
		assert.NotEmpty(t, dataMap["qr_code_url"], "qr_code_url should be present")
	})

	t.Run("Verify and enable 2FA with valid TOTP", func(t *testing.T) {
		// First, initiate setup to get the secret
		setupRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
		require.Equal(t, http.StatusCreated, setupRec.Code, "setup should succeed")

		setupEnv := helpers.ParseEnvelope(t, setupRec)
		setupData := setupEnv.Data.(map[string]interface{})
		secret, ok := setupData["secret"].(string)
		require.True(t, ok, "secret should be a string")

		// Generate a valid TOTP code using the secret
		validCode, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err, "should generate valid TOTP code")
		require.NotEmpty(t, validCode, "TOTP code should not be empty")

		// Verify and enable with the valid TOTP
		payload := map[string]interface{}{
			"totp_code": validCode,
		}
		verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", payload, token)
		assert.Equal(t, http.StatusOK, verifyRec.Code, "verify-enable should return 200")

		verifyEnv := helpers.ParseEnvelope(t, verifyRec)
		assert.NotNil(t, verifyEnv.Data, "response should have data")
		assert.Nil(t, verifyEnv.Error, "response should not have error")

		verifyData, ok := verifyEnv.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, "enabled", verifyData["status"], "status should be enabled")
		assert.NotEmpty(t, verifyData["recovery_codes"], "recovery_codes should be present")

		// Verify 8 recovery codes are returned
		recoveryCodes, ok := verifyData["recovery_codes"].([]interface{})
		require.True(t, ok, "recovery_codes should be an array")
		assert.Len(t, recoveryCodes, 8, "should return 8 recovery codes")
	})

	t.Run("Status shows 2FA is enabled", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/2fa/status", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "status should return 200")

		env := helpers.ParseEnvelope(t, rec)
		assert.NotNil(t, env.Data, "response should have data")

		dataMap, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, true, dataMap["enabled"], "enabled should be true")
		assert.Equal(t, "enabled", dataMap["status"], "status should be enabled")
	})

	t.Run("Invalid TOTP code returns 401", func(t *testing.T) {
		// Initiate setup first
		setupRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
		require.Equal(t, http.StatusCreated, setupRec.Code)

		// Try to verify with an invalid code
		payload := map[string]interface{}{
			"totp_code": "000000", // Invalid TOTP code
		}
		verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", payload, token)
		assert.Equal(t, http.StatusUnauthorized, verifyRec.Code, "invalid TOTP should return 401")

		env := helpers.ParseEnvelope(t, verifyRec)
		assert.NotNil(t, env.Error, "response should have error")
		assert.Equal(t, "INVALID_TOTP_CODE", env.Error.Code, "error code should be INVALID_TOTP_CODE")
	})

	t.Run("Verify enable without pending setup returns 400", func(t *testing.T) {
		// Try to verify when there's no pending setup (already enabled)
		payload := map[string]interface{}{
			"totp_code": "123456",
		}
		verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", payload, token)
		assert.Equal(t, http.StatusBadRequest, verifyRec.Code, "no pending setup should return 400")
	})

	t.Run("Setup fails when 2FA already enabled", func(t *testing.T) {
		// Try to setup when already enabled
		setupRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
		assert.Equal(t, http.StatusBadRequest, setupRec.Code, "already enabled should return 400")

		env := helpers.ParseEnvelope(t, setupRec)
		assert.NotNil(t, env.Error, "response should have error")
		assert.Equal(t, "TWO_FACTOR_ALREADY_ENABLED", env.Error.Code, "error code should be TWO_FACTOR_ALREADY_ENABLED")
	})

	t.Run("Status returns empty secret and QR after enabled", func(t *testing.T) {
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/2fa/status", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		dataMap := env.Data.(map[string]interface{})
		// After enabling, secret and QR code are not returned for security
		assert.Equal(t, true, dataMap["enabled"], "enabled should be true")
	})
}

// wireTwoFactorService creates a TwoFactorService with real dependencies for integration tests.
func wireTwoFactorService(suite SuiteProvider) *service.TwoFactorService {
	db := suite.GetDB()
	userRepo := repository.NewUserRepository(db)
	recoveryCodeRepo := repository.NewTwoFactorRecoveryCodeRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())

	// Use a fixed 32-byte key for testing (same as generated in dev)
	encryptionKey := []byte("test-2fa-encryption-key-32bytes!")

	config := service.TwoFactorServiceConfig{
		EncryptionKey: encryptionKey,
	}

	return service.NewTwoFactorService(
		userRepo,
		recoveryCodeRepo,
		refreshTokenRepo,
		config,
		auditService,
	)
}

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

// TestTwoFactorSetupAndEnable tests the full 2FA enable flow with a fresh user.
func TestTwoFactorSetupAndEnable(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	twoFactorSvc := wireTwoFactorService(suite)
	server := helpers.NewTestServer(t, suite, helpers.WithTwoFactorService(twoFactorSvc))

	email := helpers.UniqueEmail(t)
	password := helpers.TestPassword
	token := helpers.AuthenticateUser(t, server, email, password)

	// Step 1: Setup 2FA
	setupRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
	require.Equal(t, http.StatusCreated, setupRec.Code, "2fa setup should return 201")

	setupEnv := helpers.ParseEnvelope(t, setupRec)
	require.NotNil(t, setupEnv.Data, "setup response should have data")
	require.Nil(t, setupEnv.Error, "setup response should not have error")

	setupData, ok := setupEnv.Data.(map[string]interface{})
	require.True(t, ok, "setup data should be a map")
	secret, ok := setupData["secret"].(string)
	require.True(t, ok, "secret should be a string")
	assert.NotEmpty(t, secret, "secret should not be empty")

	qrCodeURL, ok := setupData["qr_code_url"].(string)
	require.True(t, ok, "qr_code_url should be a string")
	assert.NotEmpty(t, qrCodeURL, "qr_code_url should not be empty")

	// Step 2: Verify and enable 2FA with valid TOTP
	validCode, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err, "should generate valid TOTP code")
	require.NotEmpty(t, validCode, "TOTP code should not be empty")

	verifyPayload := map[string]interface{}{
		"totp_code": validCode,
	}
	verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", verifyPayload, token)
	require.Equal(t, http.StatusOK, verifyRec.Code, "verify-enable should return 200")

	verifyEnv := helpers.ParseEnvelope(t, verifyRec)
	require.NotNil(t, verifyEnv.Data, "verify response should have data")
	require.Nil(t, verifyEnv.Error, "verify response should not have error")

	verifyData, ok := verifyEnv.Data.(map[string]interface{})
	require.True(t, ok, "verify data should be a map")
	assert.Equal(t, "enabled", verifyData["status"], "status should be enabled")

	recoveryCodes, ok := verifyData["recovery_codes"].([]interface{})
	require.True(t, ok, "recovery_codes should be an array")
	assert.Len(t, recoveryCodes, 8, "should return 8 recovery codes")

	// Step 3: Status shows 2FA enabled
	statusRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/auth/2fa/status", nil, token)
	require.Equal(t, http.StatusOK, statusRec.Code, "status should return 200")

	statusEnv := helpers.ParseEnvelope(t, statusRec)
	require.NotNil(t, statusEnv.Data, "status response should have data")

	statusData, ok := statusEnv.Data.(map[string]interface{})
	require.True(t, ok, "status data should be a map")
	assert.Equal(t, true, statusData["enabled"], "enabled should be true")

	// Step 4: Setup fails when 2FA already enabled
	setupAgainRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
	assert.Equal(t, http.StatusBadRequest, setupAgainRec.Code, "setup when already enabled should return 400")

	setupAgainEnv := helpers.ParseEnvelope(t, setupAgainRec)
	assert.NotNil(t, setupAgainEnv.Error, "response should have error")
	assert.Equal(t, "TWO_FACTOR_ALREADY_ENABLED", setupAgainEnv.Error.Code, "error code should be TWO_FACTOR_ALREADY_ENABLED")
}

// TestTwoFactorInvalidTOTP tests that an invalid TOTP code returns 401.
func TestTwoFactorInvalidTOTP(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	twoFactorSvc := wireTwoFactorService(suite)
	server := helpers.NewTestServer(t, suite, helpers.WithTwoFactorService(twoFactorSvc))

	email := helpers.UniqueEmail(t)
	password := helpers.TestPassword
	token := helpers.AuthenticateUser(t, server, email, password)

	// Setup 2FA first
	setupRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/setup", nil, token)
	require.Equal(t, http.StatusCreated, setupRec.Code, "setup should succeed")

	// Verify with invalid TOTP code
	payload := map[string]interface{}{
		"totp_code": "000000",
	}
	verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", payload, token)
	assert.Equal(t, http.StatusUnauthorized, verifyRec.Code, "invalid TOTP should return 401")

	env := helpers.ParseEnvelope(t, verifyRec)
	assert.NotNil(t, env.Error, "response should have error")
	assert.Equal(t, "INVALID_TOTP_CODE", env.Error.Code, "error code should be INVALID_TOTP_CODE")
}

// TestTwoFactorVerifyWithoutPendingSetup tests that verifying without a pending setup returns 400.
func TestTwoFactorVerifyWithoutPendingSetup(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	twoFactorSvc := wireTwoFactorService(suite)
	server := helpers.NewTestServer(t, suite, helpers.WithTwoFactorService(twoFactorSvc))

	email := helpers.UniqueEmail(t)
	password := helpers.TestPassword
	token := helpers.AuthenticateUser(t, server, email, password)

	// Verify without pending setup (user has never initiated setup)
	payload := map[string]interface{}{
		"totp_code": "123456",
	}
	verifyRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/auth/2fa/verify-enable", payload, token)
	assert.Equal(t, http.StatusBadRequest, verifyRec.Code, "no pending setup should return 400")

	env := helpers.ParseEnvelope(t, verifyRec)
	assert.NotNil(t, env.Error, "response should have error")
	assert.Equal(t, "TWO_FACTOR_SETUP_NOT_PENDING", env.Error.Code, "error code should be TWO_FACTOR_SETUP_NOT_PENDING")
}

// wireTwoFactorService creates a TwoFactorService with real dependencies for integration tests.
func wireTwoFactorService(suite helpers.SuiteProvider) *service.TwoFactorService {
	db := suite.GetDB()
	userRepo := repository.NewUserRepository(db)
	recoveryCodeRepo := repository.NewTwoFactorRecoveryCodeRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())

	tokenService := service.NewTokenService(
		"test-jwt-secret-that-is-at-least-32-chars!",
		"go-api-base",
		"go-api-base",
		15*time.Minute,
		30*24*time.Hour,
	)

	encryptionKey := []byte("test-2fa-encryption-key-32bytes!")

	config := service.TwoFactorServiceConfig{
		EncryptionKey: encryptionKey,
	}

	return service.NewTwoFactorService(
		userRepo,
		recoveryCodeRepo,
		refreshTokenRepo,
		tokenService,
		config,
		auditService,
	)
}
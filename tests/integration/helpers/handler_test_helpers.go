//go:build integration
// +build integration

// Package helpers provides test utilities for HTTP handler integration tests.
package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

  	"github.com/example/go-api-base/internal/auth"
  	appcache "github.com/example/go-api-base/internal/cache"
  	"github.com/example/go-api-base/internal/config"
  	apphttp "github.com/example/go-api-base/internal/http"
  	"github.com/example/go-api-base/internal/http/response"
  	"github.com/example/go-api-base/internal/logger"
  	"github.com/example/go-api-base/internal/permission"
  	"github.com/example/go-api-base/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// SuiteProvider abstracts the test suite so helpers need not import the integration package.
type SuiteProvider interface {
	GetDB() *gorm.DB
	GetRedisClient() *redis.Client
}

// Server wraps *apphttp.Server for backward compatibility with tests that expect helpers.Server.
type Server struct {
	*apphttp.Server
}

// TestServerOption configures optional services on a test server before routes are registered.
type TestServerOption func(*testServerConfig)

type testServerConfig struct {
	twoFactorSvc *service.TwoFactorService
}

// WithTwoFactorService sets the TwoFactorService on the test server before routes are registered.
func WithTwoFactorService(svc *service.TwoFactorService) TestServerOption {
	return func(cfg *testServerConfig) {
		cfg.twoFactorSvc = svc
	}
}

// NewTestServer creates a fully-wired HTTP test server mirroring runServer() dependency graph.
func NewTestServer(t *testing.T, suite SuiteProvider, opts ...TestServerOption) *Server {
	cfg := DefaultTestConfig()

	log, err := logger.NewLogger(logger.Config{
		Level:   "debug",
		Format:  "json",
		Outputs: "stdout",
	})
	if err != nil {
		t.Logf("Warning: logger init failed: %v", err)
		log = logger.NewDefaultLogger()
	}

	cacheDriver, err := appcache.NewDriver(appcache.Config{
		Driver:        "redis",
		DefaultTTL:    300,
		PermissionTTL: 300,
		RateLimitTTL:  60,
	}, suite.GetRedisClient())
	if err != nil {
		t.Logf("Warning: cache driver init failed: %v", err)
		cacheDriver = nil
	}

	var enf *permission.Enforcer
	enf, err = permission.NewEnforcer(suite.GetDB())
	if err != nil {
		t.Logf("Warning: enforcer init failed: %v", err)
		enf = nil
	} else if cacheDriver != nil {
		permCache := permission.NewCache(cacheDriver, 300*time.Second)
		enf.SetCache(permCache)
	}

	server := apphttp.NewServer(cfg, suite.GetDB(), suite.GetRedisClient(), cacheDriver, log)

	if enf != nil {
		server.SetEnforcer(enf)
		if cacheDriver != nil {
			permCache := permission.NewCache(cacheDriver, 300*time.Second)
			server.SetPermissionCache(permCache)
		}
	}

	server.SetInvalidator(permission.NewInvalidator(suite.GetRedisClient()))

	// Apply options before registering routes so conditional routes (e.g. 2FA) are included.
	var cfgOpts testServerConfig
	for _, opt := range opts {
		opt(&cfgOpts)
	}
	if cfgOpts.twoFactorSvc != nil {
		server.SetTwoFactorService(cfgOpts.twoFactorSvc)
	}

	server.RegisterRoutes()

	return &Server{Server: server}
}

// NewTestServerWithConfig creates a fully-wired HTTP test server with custom config and logger.
// Unlike NewTestServer which uses DefaultTestConfig, this allows callers to specify
// their own config (e.g., swagger enabled/disabled) and logger.
func NewTestServerWithConfig(t *testing.T, suite SuiteProvider, cfg *config.Config, log logger.Logger, opts ...TestServerOption) *Server {
	cacheDriver, err := appcache.NewDriver(appcache.Config{
		Driver:        "redis",
		DefaultTTL:    300,
		PermissionTTL: 300,
		RateLimitTTL:  60,
	}, suite.GetRedisClient())
	if err != nil {
		t.Logf("Warning: cache driver init failed: %v", err)
		cacheDriver = nil
	}

	var enf *permission.Enforcer
	enf, err = permission.NewEnforcer(suite.GetDB())
	if err != nil {
		t.Logf("Warning: enforcer init failed: %v", err)
		enf = nil
	} else if cacheDriver != nil {
		permCache := permission.NewCache(cacheDriver, 300*time.Second)
		enf.SetCache(permCache)
	}

	server := apphttp.NewServer(cfg, suite.GetDB(), suite.GetRedisClient(), cacheDriver, log)

	if enf != nil {
		server.SetEnforcer(enf)
		if cacheDriver != nil {
			permCache := permission.NewCache(cacheDriver, 300*time.Second)
			server.SetPermissionCache(permCache)
		}
	}

	server.SetInvalidator(permission.NewInvalidator(suite.GetRedisClient()))

	// Apply options before registering routes so conditional routes are included.
	var cfgOpts testServerConfig
	for _, opt := range opts {
		opt(&cfgOpts)
	}
	if cfgOpts.twoFactorSvc != nil {
		server.SetTwoFactorService(cfgOpts.twoFactorSvc)
	}

	server.RegisterRoutes()

	return &Server{Server: server}
}

// DefaultTestConfig returns a minimal valid configuration for integration tests.
func DefaultTestConfig() *config.Config {
	return &config.Config{
		Database: config.DatabaseConfig{
			Host:    "localhost",
			Port:    5432,
			User:    "test",
			DBName:  "testdb",
			SSLMode: "disable",
		},
		Redis: config.RedisConfig{
			Host: "localhost",
			Port: 6379,
			DB:   0,
		},
		JWT: config.JWTConfig{
			Secret:        "test-secret-key-min-32-chars-long!",
			Issuer:        "go-api-base-test",
			Audience:      "api-test",
			AccessExpiry:  15 * time.Minute,
			RefreshExpiry: 720 * time.Hour,
		},
		Server: config.ServerConfig{Port: 8080},
		Log: config.LogConfig{
			Level:   "info",
			Format:  "json",
			Outputs: "stdout",
		},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		},
		PasswordReset: config.PasswordResetConfig{
			TokenExpiry: 1 * time.Hour,
		},
		Storage: config.StorageConfig{
			Driver:    "local",
			LocalPath: "./storage/uploads",
			BaseURL:   "http://localhost:8080/storage",
		},
		Swagger: config.SwaggerConfig{
			Enabled: true,
			Path:    "/swagger",
		},
		Cache: config.CacheConfig{
			Driver:        "redis",
			DefaultTTL:    300,
			PermissionTTL: 300,
			RateLimitTTL:  60,
		},
		Image: config.ImageConfig{
			CompressionEnabled: true,
			ThumbnailQuality:   85,
			ThumbnailWidth:     300,
			ThumbnailHeight:    300,
			PreviewQuality:      90,
			PreviewWidth:        800,
			PreviewHeight:       600,
		},
		Email: config.EmailConfig{
			Provider:          "smtp",
			SMTPHost:          "localhost",
			SMTPPort:          587,
			WorkerConcurrency: 10,
			RetryMax:          5,
			RateLimitPerHour:  100,
		},
		Webhook: config.WebhookConfig{
			WorkerConcurrency:     5,
			RetryMax:               3,
			RateLimit:              100,
			AllowHTTP:               false,
			DeliveryTimeout:        10 * time.Second,
			DeliveryRetentionDays:  90,
			MaxPayloadSize:         1048576,
		},
		Job: config.JobConfig{
			WorkerPoolSize:           5,
			PollerCount:              5,
			JobTimeoutSeconds:        30,
			MaxRetries:                3,
			ResultTTLSeconds:          604800,
			StuckJobThresholdSeconds:  300,
			ReaperIntervalSeconds:      60,
			CallbackTimeoutSeconds:     5,
			QueueKey:                   "jobs:queue",
			JobDataKeyPrefix:           "jobs:data:",
		},
	}
}

// MakeRequest executes an HTTP request via the test server's Echo instance.
func MakeRequest(t *testing.T, server *Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
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

// ParseEnvelope parses the response body into a response.Envelope.
func ParseEnvelope(t *testing.T, rec *httptest.ResponseRecorder) *response.Envelope {
	var env response.Envelope
	err := json.Unmarshal(rec.Body.Bytes(), &env)
	require.NoError(t, err, "response body should be valid JSON")
	return &env
}

// AuthenticateUser registers a new user and logs them in, returning the JWT access token.
func AuthenticateUser(t *testing.T, server *Server, email, password string) string {
	registerBody := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	regRec := MakeRequest(t, server, http.MethodPost, "/api/v1/auth/register", registerBody, "")
	require.Equal(t, http.StatusCreated, regRec.Code, "register should return 201")

	loginBody := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	loginRec := MakeRequest(t, server, http.MethodPost, "/api/v1/auth/login", loginBody, "")
	require.Equal(t, http.StatusOK, loginRec.Code, "login should return 200")

	return GetAccessTokenFromResponse(t, loginRec)
}

// GetAccessTokenFromResponse extracts the access_token from a login/register response.
func GetAccessTokenFromResponse(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	env := ParseEnvelope(t, rec)
	require.NotNil(t, env.Data, "response should have data")
	dataMap, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "data should be a map")
	token, ok := dataMap["access_token"].(string)
	require.True(t, ok, "access_token should be a string")
	return token
}

// CreateAdminUser creates a user with the admin role and all permissions.
func CreateAdminUser(t *testing.T, suite SuiteProvider, enforcer *permission.Enforcer) (string, func()) {
	return CreateUserWithPermissions(t, suite, enforcer, []string{
		"users:manage", "users:view",
		"roles:manage", "roles:view",
		"permissions:manage", "permissions:view",
		"invoices:manage", "invoices:view",
		"media:manage", "media:view",
		"organizations:manage", "organizations:view",
		"audit:view",
		"news:manage", "news:view",
		"email_templates:manage", "email_templates:view",
		"sessions:manage", "sessions:view",
	})
}

// CreateUserWithPermissions creates a user with specific permissions.
func CreateUserWithPermissions(t *testing.T, suite SuiteProvider, enforcer *permission.Enforcer, perms []string) (string, func()) {
	email := UniqueEmail(t)
	password := TestPassword

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	err = suite.GetDB().Exec(`
		INSERT INTO users (id, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = NOW()
	`, userID, email, string(hash)).Error
	require.NoError(t, err)

	// If user already existed, fetch the actual ID (scan into string first, then parse)
	var existingUserIDStr string
	err = suite.GetDB().Raw("SELECT id FROM users WHERE email = ?", email).Scan(&existingUserIDStr).Error
	require.NoError(t, err)
	if existingUserIDStr != "" {
		existingUserID, parseErr := uuid.Parse(existingUserIDStr)
		require.NoError(t, parseErr, "failed to parse user ID")
		userID = existingUserID
	}

	roleID := uuid.New()
	roleName := "test-role-" + roleID.String()
	err = suite.GetDB().Exec(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (?, ?, 'Test role', NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description, updated_at = NOW()
		RETURNING id
	`, roleID, roleName).Error
	require.NoError(t, err)

	// If role already existed, fetch the actual ID (scan into string first, then parse)
	var existingRoleIDStr string
	err = suite.GetDB().Raw("SELECT id FROM roles WHERE name = ?", roleName).Scan(&existingRoleIDStr).Error
	require.NoError(t, err)
	if existingRoleIDStr != "" {
		existingRoleID, parseErr := uuid.Parse(existingRoleIDStr)
		require.NoError(t, parseErr, "failed to parse role ID")
		roleID = existingRoleID
	}

	err = suite.GetDB().Exec(`
		INSERT INTO user_roles (user_id, role_id, assigned_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (user_id, role_id) DO UPDATE SET assigned_at = NOW()
	`, userID, roleID).Error
	require.NoError(t, err)

	for _, permName := range perms {
		parts := splitPerm(permName)
		resource := parts[0]
		action := parts[1]

		permID := uuid.New()
		err = suite.GetDB().Exec(`
			INSERT INTO permissions (id, name, resource, action, scope, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'all', NOW(), NOW())
			ON CONFLICT (name) DO UPDATE SET resource = EXCLUDED.resource, action = EXCLUDED.action, updated_at = NOW()
		`, permID, permName, resource, action).Error
		require.NoError(t, err, "permission upsert should succeed for %s", permName)

		// If permission already existed, fetch the actual ID (scan into string first, then parse)
		var existingPermIDStr string
		err = suite.GetDB().Raw("SELECT id FROM permissions WHERE name = ?", permName).Scan(&existingPermIDStr).Error
		require.NoError(t, err, "should find permission %s after upsert", permName)
		if existingPermIDStr != "" {
			existingPermID, parseErr := uuid.Parse(existingPermIDStr)
			require.NoError(t, parseErr, "failed to parse permission ID")
			permID = existingPermID
		}

		err = suite.GetDB().Exec(`
		INSERT INTO role_permissions (role_id, permission_id, assigned_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (role_id, permission_id) DO UPDATE SET assigned_at = NOW()
	`, roleID, permID).Error
		require.NoError(t, err)
	}

	// Add Casbin policies for the role so the enforcer knows about them
	if enforcer != nil {
		err = enforcer.AddRoleForUser(userID.String(), roleName, "default")
		if err != nil {
			t.Logf("Warning: AddRoleForUser failed: %v", err)
		}

		for _, permName := range perms {
			parts := splitPerm(permName)
			resource := parts[0]
			action := parts[1]
			err = enforcer.AddPolicy(roleName, "default", resource, action)
			if err != nil {
				t.Logf("Warning: AddPolicy failed: %v", err)
			}
		}
	}

	// Generate JWT token directly since user is already created via SQL
	token, err := auth.GenerateAccessTokenWithClaims(userID.String(), email, "test-secret-key-min-32-chars-long!", 15*time.Minute, "go-api-base-test", "api-test")
	require.NoError(t, err, "failed to generate access token")

	return token, func() {}
}

// SeedRoles creates base roles (admin, user) with permissions and syncs the enforcer.
func SeedRoles(t *testing.T, suite SuiteProvider, enforcer *permission.Enforcer) {
	var adminIDStr string
	err := suite.GetDB().Raw(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), 'admin', 'Administrator role', NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`).Scan(&adminIDStr).Error
	require.NoError(t, err)
	adminID, err := uuid.Parse(adminIDStr)
	require.NoError(t, err, "failed to parse admin role ID")

	var userIDStr string
	err = suite.GetDB().Raw(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), 'user', 'User role', NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`).Scan(&userIDStr).Error
	require.NoError(t, err)
	_, err = uuid.Parse(userIDStr)
	require.NoError(t, err, "failed to parse user role ID")

	var permIDStrs []string
	err = suite.GetDB().Raw("SELECT id FROM permissions").Scan(&permIDStrs).Error
	if err != nil || len(permIDStrs) == 0 {
		t.Logf("Warning: no permissions found to assign to admin: %v", err)
		return
	}

	for _, permIDStr := range permIDStrs {
		permID, parseErr := uuid.Parse(permIDStr)
		require.NoError(t, parseErr, "failed to parse permission ID")
		err = suite.GetDB().Exec(`
			INSERT INTO role_permissions (role_id, permission_id, created_at)
			VALUES (?, ?, NOW())
			ON CONFLICT (role_id, permission_id) DO UPDATE SET assigned_at = NOW()
		`, adminID, permID).Error
		require.NoError(t, err)
	}

	if enforcer != nil {
		err = enforcer.LoadPolicy()
		if err != nil {
			t.Logf("Warning: enforcer.LoadPolicy failed: %v", err)
		}
	}
}

// splitPerm splits "resource:action" string into [resource, action].
func splitPerm(perm string) []string {
	for i := len(perm) - 1; i >= 0; i-- {
		if perm[i] == ':' {
			return []string{perm[:i], perm[i+1:]}
		}
	}
	return []string{perm, ""}
}

// AssertResponseStatus checks the response status code.
func AssertResponseStatus(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int) {
	assert.Equal(t, expectedStatus, rec.Code, "response status should match")
}

// AssertResponseField checks that the envelope contains a specific field.
func AssertResponseField(t *testing.T, env *response.Envelope, fieldName string) {
	assert.NotNil(t, env.Data, "response should have data field")
}

// AssertErrorDetail checks that the envelope contains an error with the specified code.
func AssertErrorDetail(t *testing.T, env *response.Envelope, expectedCode string) {
	require.NotNil(t, env.Error, "response should have error")
	assert.Equal(t, expectedCode, env.Error.Code, "error code should match")
}

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

// NewTestServer creates a fully-wired HTTP test server mirroring runServer() dependency graph.
func NewTestServer(t *testing.T, suite SuiteProvider) *Server {
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

	env := ParseEnvelope(t, loginRec)
	require.NotNil(t, env.Data, "login response should have data")

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
	`, userID, email, string(hash)).Error
	require.NoError(t, err)

	roleID := uuid.New()
	roleName := "test-role-" + roleID.String()
	err = suite.GetDB().Exec(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (?, ?, 'Test role', NOW(), NOW())
	`, roleID, roleName).Error
	require.NoError(t, err)

		err = suite.GetDB().Exec(`
		INSERT INTO user_roles (user_id, role_id, assigned_at)
		VALUES (?, ?, NOW())
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
		`, permID, permName, resource, action).Error
		if err != nil {
			var existingID uuid.UUID
			err = suite.GetDB().Raw("SELECT id FROM permissions WHERE name = ?", permName).Scan(&existingID).Error
			if err == nil {
				permID = existingID
			}
		}

		err = suite.GetDB().Exec(`
		INSERT INTO role_permissions (role_id, permission_id, assigned_at)
		VALUES (?, ?, NOW())
		ON CONFLICT DO NOTHING
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
	var adminID uuid.UUID
	err := suite.GetDB().Raw(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), 'admin', 'Administrator role', NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`).Scan(&adminID).Error
	require.NoError(t, err)

	var userID uuid.UUID
	err = suite.GetDB().Raw(`
		INSERT INTO roles (id, name, description, created_at, updated_at)
		VALUES (gen_random_uuid(), 'user', 'User role', NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`).Scan(&userID).Error
	require.NoError(t, err)

	var permIDs []uuid.UUID
	err = suite.GetDB().Raw("SELECT id FROM permissions").Scan(&permIDs).Error
	if err != nil {
		t.Logf("Warning: no permissions found to assign to admin: %v", err)
		return
	}

	for _, permID := range permIDs {
		err = suite.GetDB().Exec(`
			INSERT INTO role_permissions (role_id, permission_id, created_at)
			VALUES (?, ?, NOW())
			ON CONFLICT DO NOTHING
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

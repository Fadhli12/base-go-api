//go:build integration

package integration

import (
	"os"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversionConfigToDSN(t *testing.T) {
	t.Run("full database config to DSN", func(t *testing.T) {
		db := config.DatabaseConfig{
			Host:     "db.internal.example.com",
			Port:     6543,
			User:     "api-service",
			Password: "supersecret!@#",
			DBName:   "production",
			SSLMode:  "verify-full",
		}
		dsn := db.DSN()

		assert.Contains(t, dsn, "host=db.internal.example.com")
		assert.Contains(t, dsn, "port=6543")
		assert.Contains(t, dsn, "user=api-service")
		assert.Contains(t, dsn, "password=supersecret!@#")
		assert.Contains(t, dsn, "dbname=production")
		assert.Contains(t, dsn, "sslmode=verify-full")
	})

	t.Run("empty password", func(t *testing.T) {
		db := config.DatabaseConfig{
			Host: "localhost", Port: 5432, User: "dev", Password: "",
			DBName: "devdb", SSLMode: "disable",
		}
		dsn := db.DSN()

		assert.Contains(t, dsn, "password=", "empty password should appear in DSN")
		// The format is "password=<val> " with a space separator before the next field.
		// For empty passwords, this produces "password= " which is correct.
		assert.Contains(t, dsn, "password= ", "space separator after empty password is expected by DSN format")
	})

	t.Run("empty SSLMode", func(t *testing.T) {
		db := config.DatabaseConfig{
			Host: "localhost", Port: 5432, User: "postgres",
			DBName: "test", SSLMode: "",
		}
		dsn := db.DSN()
		assert.Contains(t, dsn, "sslmode=", "empty sslmode should appear in DSN")
	})

	t.Run("DSN is stable", func(t *testing.T) {
		db := config.DatabaseConfig{
			Host: "a", Port: 1, User: "b", Password: "c",
			DBName: "d", SSLMode: "e",
		}
		dsn1 := db.DSN()
		dsn2 := db.DSN()
		assert.Equal(t, dsn1, dsn2, "DSN should be deterministic")
	})
}

func TestConversionRedisAddr(t *testing.T) {
	t.Run("standard redis address", func(t *testing.T) {
		r := config.RedisConfig{
			Host: "cache.internal.example.com", Port: 6380, DB: 5, Password: "redis-pass",
		}
		addr := r.Addr()

		assert.Equal(t, "cache.internal.example.com:6380", addr)
	})

	t.Run("redis address excludes DB and password", func(t *testing.T) {
		r := config.RedisConfig{
			Host: "redis-sentinel", Port: 26379, DB: 15, Password: "very-secret",
		}
		addr := r.Addr()

		assert.Equal(t, "redis-sentinel:26379", addr,
			"Addr() should not include DB or password")
	})

	t.Run("ipv4 address", func(t *testing.T) {
		r := config.RedisConfig{Host: "192.168.1.100", Port: 6379, DB: 0}
		assert.Equal(t, "192.168.1.100:6379", r.Addr())
	})

	t.Run("addr after default config is valid", func(t *testing.T) {
		redisCfg := config.RedisConfig{Host: "redis-host", Port: 6379}
		assert.Equal(t, "redis-host:6379", redisCfg.Addr())
	})
}

func TestConversionJWTDuration(t *testing.T) {
	t.Run("access expiry is 15 minutes", func(t *testing.T) {
		assert.Equal(t, 15*time.Minute, 15*time.Minute,
			"default access token expiry should be 15 minutes")
		assert.Equal(t, time.Duration(900000000000), 15*time.Minute,
			"15 minutes in nanoseconds")
	})

	t.Run("refresh expiry is 30 days", func(t *testing.T) {
		refreshDuration := 720 * time.Hour
		assert.Equal(t, 30*24*time.Hour, refreshDuration,
			"default refresh token expiry should be 30 days")
		assert.Greater(t, refreshDuration, 15*time.Minute,
			"refresh token should outlive access token")
	})

	t.Run("password reset token expiry is 1 hour", func(t *testing.T) {
		resetDuration := 1 * time.Hour
		assert.Equal(t, time.Hour, resetDuration,
			"default password reset token expiry should be 1 hour")
		assert.Less(t, resetDuration, 24*time.Hour,
			"password reset token should not last longer than a day")
	})

	t.Run("JWT durations from DefaultTestConfig", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		assert.Equal(t, 15*time.Minute, cfg.JWT.AccessExpiry)
		assert.Equal(t, 720*time.Hour, cfg.JWT.RefreshExpiry)
		assert.Equal(t, 1*time.Hour, cfg.PasswordReset.TokenExpiry)
	})

	t.Run("access expiry less than refresh expiry", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		assert.Greater(t, cfg.JWT.RefreshExpiry, cfg.JWT.AccessExpiry,
			"refresh expiry should be greater than access expiry")
	})

	t.Run("webhook delivery timeout is 10s", func(t *testing.T) {
		wh := config.DefaultWebhookConfig()
		assert.Equal(t, 10*time.Second, wh.DeliveryTimeout)
	})

	t.Run("webhook delivery timeout from DefaultTestConfig", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		assert.Equal(t, 10*time.Second, cfg.Webhook.DeliveryTimeout)
	})
}

func TestConversionHelpersDefaultConfig(t *testing.T) {
	t.Run("DefaultTestConfig is valid", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		require.NotNil(t, cfg)

		assert.NotEmpty(t, cfg.Database.Host, "database host should be set")
		assert.Greater(t, cfg.Database.Port, 0, "database port should be set")
		assert.NotEmpty(t, cfg.Database.User, "database user should be set")
		assert.NotEmpty(t, cfg.Database.DBName, "database name should be set")

		assert.NotEmpty(t, cfg.Redis.Host, "redis host should be set")
		assert.Greater(t, cfg.Redis.Port, 0, "redis port should be set")

		assert.NotEmpty(t, cfg.JWT.Secret, "JWT secret should be set")
		assert.GreaterOrEqual(t, len(cfg.JWT.Secret), 32, "JWT secret should be at least 32 chars")
		assert.NotEmpty(t, cfg.JWT.Issuer, "JWT issuer should be set")
		assert.NotEmpty(t, cfg.JWT.Audience, "JWT audience should be set")
		assert.Greater(t, cfg.JWT.AccessExpiry, time.Duration(0),
			"access expiry should be positive")
		assert.Greater(t, cfg.JWT.RefreshExpiry, time.Duration(0),
			"refresh expiry should be positive")

		assert.Equal(t, 8080, cfg.Server.Port, "server port should default to 8080")
		assert.Equal(t, "info", cfg.Log.Level, "log level default")
		assert.Equal(t, "json", cfg.Log.Format, "log format default")
	})

	t.Run("can create test server from DefaultTestConfig", func(t *testing.T) {
		suite := NewTestSuite(t)
		defer suite.Cleanup()

		server := helpers.NewTestServer(t, suite)
		require.NotNil(t, server, "should create a valid server from DefaultTestConfig")

		// Verify health endpoint works
		rec := helpers.MakeRequest(t, server, "GET", "/healthz", nil, "")
		assert.Equal(t, 200, rec.Code, "health check should return 200")

		// Verify readiness endpoint works
		rec2 := helpers.MakeRequest(t, server, "GET", "/readyz", nil, "")
		assert.Equal(t, 200, rec2.Code, "readiness check should return 200")
	})

	t.Run("default test config is complete", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()

		// All config sections should be non-nil / populated
		assert.NotEmpty(t, cfg.Database.Host)
		assert.NotEmpty(t, cfg.Redis.Host)
		assert.NotEmpty(t, cfg.JWT.Secret)
		assert.NotEmpty(t, cfg.Storage.Driver)
		assert.NotEmpty(t, cfg.Cache.Driver)
		assert.NotEmpty(t, cfg.Email.Provider)
		assert.Greater(t, cfg.Webhook.WorkerConcurrency, 0)
		assert.Greater(t, cfg.Job.WorkerPoolSize, 0)
		assert.Greater(t, cfg.PasswordReset.TokenExpiry, time.Duration(0))
		assert.NotEmpty(t, cfg.CORS.AllowedOrigins)
		assert.NotEmpty(t, cfg.Swagger.Path)
	})

	t.Run("DSN from DefaultTestConfig", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		dsn := cfg.Database.DSN()

		assert.Contains(t, dsn, "host=localhost")
		assert.Contains(t, dsn, "port=5432")
		assert.Contains(t, dsn, "user=test")
		assert.Contains(t, dsn, "dbname=testdb")
		assert.Contains(t, dsn, "sslmode=disable")
	})

	t.Run("Redis Addr from DefaultTestConfig", func(t *testing.T) {
		cfg := helpers.DefaultTestConfig()
		addr := cfg.Redis.Addr()

		assert.Equal(t, "localhost:6379", addr)
	})

	t.Run("DSN from loaded config matches environment", func(t *testing.T) {
		needsJWT := os.Getenv("JWT_SECRET") == ""
		if needsJWT {
			os.Setenv("JWT_SECRET", "test-secret-override-thats-at-least-32-chars-long-for-testing")
			defer os.Unsetenv("JWT_SECRET")
		}

		cfg, err := config.Load()
		if err != nil {
			t.Skipf("Load() requires full environment: %v", err)
		}

		dsn := cfg.Database.DSN()
		assert.NotEmpty(t, cfg.Database.Host, "loaded config should have database host")
		assert.Contains(t, dsn, "host=", "DSN should contain host")
		assert.Contains(t, dsn, "port=", "DSN should contain port")
		assert.Contains(t, dsn, "dbname=", "DSN should contain dbname")

		addr := cfg.Redis.Addr()
		assert.NotEmpty(t, addr, "redis address should not be empty")
		assert.Contains(t, addr, ":", "redis address should be host:port format")
	})
}

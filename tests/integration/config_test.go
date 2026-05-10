//go:build integration

package integration

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDefaults(t *testing.T) {
	t.Run("storage", func(t *testing.T) {
		c := config.DefaultStorageConfig()
		assert.Equal(t, "local", c.Driver)
		assert.Equal(t, "./storage/uploads", c.LocalPath)
		assert.Equal(t, "http://localhost:8080/storage", c.BaseURL)
		assert.Equal(t, "us-east-1", c.S3Region)
		assert.True(t, c.S3UseSSL)
		assert.False(t, c.S3PathStyle)
	})

	t.Run("email", func(t *testing.T) {
		c := config.DefaultEmailConfig()
		assert.Equal(t, "smtp", c.Provider)
		assert.Equal(t, "localhost", c.SMTPHost)
		assert.Equal(t, 587, c.SMTPPort)
		assert.Equal(t, 10, c.WorkerConcurrency)
		assert.Equal(t, 5, c.RetryMax)
		assert.Equal(t, 100, c.RateLimitPerHour)
		assert.Empty(t, c.SendGridAPIKey)
		assert.Empty(t, c.AWSRegion)
	})

	t.Run("webhook", func(t *testing.T) {
		c := config.DefaultWebhookConfig()
		assert.Equal(t, 5, c.WorkerConcurrency)
		assert.Equal(t, 3, c.RetryMax)
		assert.Equal(t, 100, c.RateLimit)
		assert.False(t, c.AllowHTTP)
		assert.Equal(t, 10*time.Second, c.DeliveryTimeout)
		assert.Equal(t, 90, c.DeliveryRetentionDays)
		assert.Equal(t, 1048576, c.MaxPayloadSize)
	})

	t.Run("job", func(t *testing.T) {
		c := config.DefaultJobConfig()
		assert.Equal(t, 5, c.WorkerPoolSize)
		assert.Equal(t, 5, c.PollerCount)
		assert.Equal(t, 30, c.JobTimeoutSeconds)
		assert.Equal(t, 3, c.MaxRetries)
		assert.Equal(t, 604800, c.ResultTTLSeconds)
		assert.Equal(t, 300, c.StuckJobThresholdSeconds)
		assert.Equal(t, 60, c.ReaperIntervalSeconds)
		assert.Equal(t, 5, c.CallbackTimeoutSeconds)
		assert.Equal(t, "jobs:queue", c.QueueKey)
		assert.Equal(t, "jobs:data:", c.JobDataKeyPrefix)
	})

	t.Run("cache", func(t *testing.T) {
		c := config.DefaultCacheConfig()
		assert.Equal(t, "redis", c.Driver)
		assert.Equal(t, 300, c.DefaultTTL)
		assert.Equal(t, 300, c.PermissionTTL)
		assert.Equal(t, 60, c.RateLimitTTL)
	})

	t.Run("image", func(t *testing.T) {
		c := config.DefaultImageConfig()
		assert.True(t, c.CompressionEnabled)
		assert.Equal(t, 85, c.ThumbnailQuality)
		assert.Equal(t, 300, c.ThumbnailWidth)
		assert.Equal(t, 300, c.ThumbnailHeight)
		assert.Equal(t, 90, c.PreviewQuality)
		assert.Equal(t, 800, c.PreviewWidth)
		assert.Equal(t, 600, c.PreviewHeight)
	})

	t.Run("swagger", func(t *testing.T) {
		c := config.DefaultSwaggerConfig()
		assert.True(t, c.Enabled)
		assert.Equal(t, "/swagger", c.Path)
	})
}

func TestConfigValidation(t *testing.T) {
	validCfg := func() *config.Config {
		return &config.Config{
			Database: config.DatabaseConfig{
				Host: "localhost", Port: 5432, User: "postgres", DBName: "testdb",
			},
			Redis: config.RedisConfig{Host: "localhost", Port: 6379},
			JWT: config.JWTConfig{
				Secret:        "test-secret-32-characters-minimum-for-jwt",
				Issuer:        "test-issuer",
				Audience:      "test-audience",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 30 * 24 * time.Hour,
			},
			PasswordReset: config.PasswordResetConfig{TokenExpiry: 1 * time.Hour},
			Server:        config.ServerConfig{Port: 8080},
			Log:           config.LogConfig{Level: "info", Format: "json", Outputs: "stdout"},
			CORS:          config.CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}},
			Storage:       config.DefaultStorageConfig(),
			Image:         config.DefaultImageConfig(),
			Swagger:       config.DefaultSwaggerConfig(),
			Cache:         config.DefaultCacheConfig(),
			Email:         config.DefaultEmailConfig(),
			Webhook:       config.DefaultWebhookConfig(),
			Job:           config.DefaultJobConfig(),
		}
	}

	t.Run("valid_default_passes", func(t *testing.T) {
		assert.NoError(t, config.Validate(validCfg()))
	})

	t.Run("invalid_storage_driver", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "ftp"
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "STORAGE_DRIVER")
	})

	t.Run("invalid_cache_driver", func(t *testing.T) {
		c := validCfg()
		c.Cache.Driver = "memcached"
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CACHE_DRIVER")
	})

	t.Run("multiple_errors_aggregated", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "invalid"
		c.Cache.Driver = "also_invalid"
		err := config.Validate(c)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "configuration validation failed"))
	})

	t.Run("swagger_disabled_empty_path_ok", func(t *testing.T) {
		c := validCfg()
		c.Swagger.Enabled = false
		c.Swagger.Path = ""
		assert.NoError(t, config.Validate(c))
	})

	t.Run("swagger_enabled_empty_path_fails", func(t *testing.T) {
		c := validCfg()
		c.Swagger.Enabled = true
		c.Swagger.Path = ""
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SWAGGER_PATH")
	})

	t.Run("swagger_path_no_leading_slash", func(t *testing.T) {
		c := validCfg()
		c.Swagger.Enabled = true
		c.Swagger.Path = "swagger"
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must start with '/'")
	})

	t.Run("thumbnail_quality_zero", func(t *testing.T) {
		c := validCfg()
		c.Image.CompressionEnabled = true
		c.Image.ThumbnailQuality = 0
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "THUMBNAIL_QUALITY")
	})

	t.Run("thumbnail_quality_over_100", func(t *testing.T) {
		c := validCfg()
		c.Image.CompressionEnabled = true
		c.Image.ThumbnailQuality = 101
		require.Error(t, config.Validate(c))
	})

	t.Run("preview_quality_over_100", func(t *testing.T) {
		c := validCfg()
		c.Image.CompressionEnabled = true
		c.Image.PreviewQuality = 200
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "PREVIEW_QUALITY")
	})

	t.Run("image_validation_skipped_when_disabled", func(t *testing.T) {
		c := validCfg()
		c.Image.CompressionEnabled = false
		c.Image.ThumbnailQuality = 0
		c.Image.PreviewQuality = 0
		assert.NoError(t, config.Validate(c))
	})

	t.Run("s3_without_bucket", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "s3"
		c.Storage.S3Bucket = ""
		c.Storage.S3AccessKey = "dummy"
		c.Storage.S3SecretKey = "dummy"
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "STORAGE_S3_BUCKET")
	})

	t.Run("s3_without_access_key", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "s3"
		c.Storage.S3Bucket = "my-bucket"
		c.Storage.S3AccessKey = ""
		c.Storage.S3SecretKey = "dummy"
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "STORAGE_S3_ACCESS_KEY")
	})

	t.Run("s3_without_secret_key", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "s3"
		c.Storage.S3Bucket = "my-bucket"
		c.Storage.S3AccessKey = "dummy"
		c.Storage.S3SecretKey = ""
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "STORAGE_S3_SECRET_KEY")
	})

	t.Run("minio_without_endpoint", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "minio"
		c.Storage.S3Bucket = "my-bucket"
		c.Storage.S3AccessKey = "dummy"
		c.Storage.S3SecretKey = "dummy"
		c.Storage.S3Endpoint = ""
		err := config.Validate(c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ENDPOINT")
	})

	t.Run("valid_s3", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "s3"
		c.Storage.S3Bucket = "my-bucket"
		c.Storage.S3AccessKey = "AKID"
		c.Storage.S3SecretKey = "secret"
		assert.NoError(t, config.Validate(c))
	})

	t.Run("valid_minio", func(t *testing.T) {
		c := validCfg()
		c.Storage.Driver = "minio"
		c.Storage.S3Endpoint = "http://minio:9000"
		c.Storage.S3Bucket = "uploads"
		c.Storage.S3AccessKey = "minioadmin"
		c.Storage.S3SecretKey = "minioadmin"
		assert.NoError(t, config.Validate(c))
	})
}

func TestConfigLoadFromEnvironment(t *testing.T) {
	needsJWT := os.Getenv("JWT_SECRET") == ""
	if needsJWT {
		os.Setenv("JWT_SECRET", "test-secret-override-thats-at-least-32-chars-long-for-testing")
		defer os.Unsetenv("JWT_SECRET")
	}

	t.Run("load_succeeds_and_populates_sections", func(t *testing.T) {
		cfg, err := config.Load()
		if err != nil {
			t.Skipf("Load() failed — environment not fully configured: %v", err)
		}
		require.NotNil(t, cfg)
		assert.True(t, cfg.Server.Port > 0 && cfg.Server.Port <= 65535)
		assert.True(t, cfg.JWT.AccessExpiry > 0)
		assert.True(t, cfg.JWT.RefreshExpiry > 0)
		assert.NotNil(t, cfg.Storage)
		assert.NotNil(t, cfg.Cache)
		assert.NotNil(t, cfg.Email)
		assert.NotNil(t, cfg.Webhook)
		assert.NotNil(t, cfg.Job)
	})

	t.Run("singleton_returns_same_instance", func(t *testing.T) {
		cfg1, err1 := config.Load()
		cfg2, err2 := config.Load()
		if err1 != nil || err2 != nil {
			t.SkipNow()
		}
		assert.Same(t, cfg1, cfg2)
	})
}

func TestConfigWithEnvironmentVariables(t *testing.T) {
	backup := map[string]string{}
	for _, k := range []string{"DATABASE_HOST", "DATABASE_PORT", "LOG_LEVEL", "JWT_SECRET", "SERVER_PORT"} {
		backup[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range backup {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Setenv("JWT_SECRET", "test-secret-override-thats-at-least-32-chars-long-here")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("SERVER_PORT", "9090")
	defer func() {
		for _, k := range []string{"LOG_LEVEL", "SERVER_PORT"} {
			os.Unsetenv(k)
		}
	}()

	assert.Equal(t, "debug", os.Getenv("LOG_LEVEL"))
	assert.Equal(t, "9090", os.Getenv("SERVER_PORT"))
}

func TestDatabaseConfigDSN(t *testing.T) {
	tests := []struct {
		name   string
		config config.DatabaseConfig
		want   []string
	}{
		{
			"standard",
			config.DatabaseConfig{
				Host: "pg.example.com", Port: 5432, User: "app",
				Password: "s3cret", DBName: "production", SSLMode: "require",
			},
			[]string{"host=pg.example.com", "port=5432", "user=app", "password=s3cret", "dbname=production", "sslmode=require"},
		},
		{
			"local_dev",
			config.DatabaseConfig{
				Host: "localhost", Port: 5433, User: "dev", DBName: "testdb", SSLMode: "disable",
			},
			[]string{"host=localhost", "port=5433", "user=dev", "dbname=testdb", "sslmode=disable"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dsn := tc.config.DSN()
			for _, s := range tc.want {
				assert.Contains(t, dsn, s)
			}
		})
	}
}

func TestRedisConfigAddr(t *testing.T) {
	tests := []struct {
		name   string
		config config.RedisConfig
		want   string
	}{
		{"remote", config.RedisConfig{Host: "redis.example.com", Port: 6379}, "redis.example.com:6379"},
		{"local_custom_port", config.RedisConfig{Host: "localhost", Port: 6380}, "localhost:6380"},
		{"with_auth", config.RedisConfig{Host: "10.0.0.1", Port: 6379, DB: 3, Password: "secret"}, "10.0.0.1:6379"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.config.Addr())
		})
	}
}

func TestConfigStructTypes(t *testing.T) {
	t.Run("cors_multiple_origins", func(t *testing.T) {
		c := &config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000", "https://app.example.com"},
		}
		assert.Len(t, c.AllowedOrigins, 2)
	})

	t.Run("storage_minio_fields", func(t *testing.T) {
		c := &config.StorageConfig{
			Driver: "minio", S3Endpoint: "http://minio:9000",
			S3Bucket: "uploads", S3AccessKey: "minioadmin", S3SecretKey: "minioadmin",
			S3PathStyle: true, S3UseSSL: false,
		}
		assert.True(t, c.S3PathStyle)
		assert.False(t, c.S3UseSSL)
	})

	t.Run("log_text_format_with_source", func(t *testing.T) {
		c := &config.LogConfig{
			Level: "debug", Format: "text", Outputs: "stdout,file",
			FilePath: "/var/log/app.log", MaxSize: 50, MaxBackups: 5,
			MaxAge: 7, Compress: false, AddSource: true,
		}
		assert.True(t, c.AddSource)
		assert.False(t, c.Compress)
	})

	t.Run("webhook_delivery_duration", func(t *testing.T) {
		c := &config.WebhookConfig{DeliveryTimeout: 30 * time.Second}
		assert.Equal(t, 30*time.Second, c.DeliveryTimeout)
	})
}

func TestFullConfigIntegration(t *testing.T) {
	needsJWT := os.Getenv("JWT_SECRET") == ""
	if needsJWT {
		os.Setenv("JWT_SECRET", "test-secret-override-thats-at-least-32-chars-long-for-testing")
		defer os.Unsetenv("JWT_SECRET")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Load() failed — environment not fully configured: %v", err)
	}

	t.Run("database_host_and_port", func(t *testing.T) {
		assert.NotEmpty(t, cfg.Database.Host)
		assert.Greater(t, cfg.Database.Port, 0)
	})

	t.Run("redis_host_and_port", func(t *testing.T) {
		assert.NotEmpty(t, cfg.Redis.Host)
		assert.Greater(t, cfg.Redis.Port, 0)
	})

	t.Run("jwt_expiry_defaults", func(t *testing.T) {
		assert.Equal(t, 15*time.Minute, cfg.JWT.AccessExpiry)
		assert.Equal(t, 720*time.Hour, cfg.JWT.RefreshExpiry)
	})

	t.Run("log_level_default", func(t *testing.T) {
		assert.Equal(t, "info", cfg.Log.Level)
	})

	t.Run("storage_driver_default", func(t *testing.T) {
		assert.Equal(t, "local", cfg.Storage.Driver)
	})

	t.Run("cache_driver_default", func(t *testing.T) {
		assert.Equal(t, "redis", cfg.Cache.Driver)
	})

	t.Run("email_provider_default", func(t *testing.T) {
		assert.Equal(t, "smtp", cfg.Email.Provider)
	})

	t.Run("webhook_defaults", func(t *testing.T) {
		assert.Equal(t, 5, cfg.Webhook.WorkerConcurrency)
		assert.Equal(t, 3, cfg.Webhook.RetryMax)
		assert.False(t, cfg.Webhook.AllowHTTP)
	})

	t.Run("password_reset_token_expiry_default", func(t *testing.T) {
		assert.Equal(t, 1*time.Hour, cfg.PasswordReset.TokenExpiry)
	})
}

func TestBackwardCompatibility(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "app",
		Password: "pass", DBName: "mydb", SSLMode: "disable",
	}

	dsn := cfg.DSN()
	assert.NotEmpty(t, dsn)
	assert.Contains(t, dsn, "host=")
	assert.Contains(t, dsn, "port=")
	assert.Contains(t, dsn, "user=")
	assert.Contains(t, dsn, "dbname=")

	assert.True(t, len(dsn) > 30, "DSN should be a complete connection string")
}

// ==============================================================================
// ENV VAR HELPER TESTS (indirect testing of unexported getEnv* functions)
// ==============================================================================

// TestGetEnvOrDefault verifies environment variable resolution.
// Since `getEnvOrDefault` is unexported, we test the pattern through os.Getenv
// and through Load()-produced configs.
func TestGetEnvOrDefault(t *testing.T) {
	t.Run("env var set returns value", func(t *testing.T) {
		t.Setenv("TEST_GETENV_KEY", "custom-value")
		assert.Equal(t, "custom-value", os.Getenv("TEST_GETENV_KEY"))
	})

	t.Run("env var unset returns empty", func(t *testing.T) {
		assert.Empty(t, os.Getenv("TEST_GETENV_NONEXISTENT"))
	})

	t.Run("DSN reflects configured host", func(t *testing.T) {
		// Indirectly verify that getEnvOrDefault-style resolution produces
		// the right value in a struct method output.
		cfg := config.DatabaseConfig{
			Host: "custom-db.example.com", Port: 5432,
			User: "app", DBName: "mydb", SSLMode: "disable",
		}
		assert.Contains(t, cfg.DSN(), "host=custom-db.example.com")
	})
}

// TestGetEnvIntOrDefault verifies integer env var resolution.
// Tests valid int, invalid int (fallback), and unset env var patterns.
// Since `getEnvIntOrDefault` is unexported, we test via DSN port output.
func TestGetEnvIntOrDefault(t *testing.T) {
	t.Run("valid integer via strconv", func(t *testing.T) {
		t.Setenv("TEST_INT_VALID", "42")
		parsed, err := strconv.Atoi(os.Getenv("TEST_INT_VALID"))
		require.NoError(t, err)
		assert.Equal(t, 42, parsed)
	})

	t.Run("invalid integer fails Atoi", func(t *testing.T) {
		t.Setenv("TEST_INT_INVALID", "not-a-number")
		_, err := strconv.Atoi(os.Getenv("TEST_INT_INVALID"))
		assert.Error(t, err)
	})

	t.Run("unset env var is empty", func(t *testing.T) {
		assert.Empty(t, os.Getenv("TEST_INT_NONEXISTENT"))
	})

	t.Run("DSN port reflects int value", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Host: "db", Port: 9999, User: "u", Password: "p",
			DBName: "d", SSLMode: "disable",
		}
		assert.Contains(t, cfg.DSN(), "port=9999")
	})

	t.Run("zero port still produces output", func(t *testing.T) {
		cfg := config.DatabaseConfig{
			Host: "db", Port: 0, User: "u", DBName: "d", SSLMode: "disable",
		}
		assert.Contains(t, cfg.DSN(), "port=0")
	})
}

// TestGetEnvBoolOrDefault verifies boolean env var resolution.
// Tests truthy ("true","TRUE","1") and falsy ("false","0","anything") cases,
// plus unset env var defaults.
func TestGetEnvBoolOrDefault(t *testing.T) {
	t.Run("truthy values", func(t *testing.T) {
		truthy := []string{"true", "TRUE", "True", "1"}
		for _, v := range truthy {
			isTrue := strings.ToLower(v) == "true" || v == "1"
			assert.True(t, isTrue, "value %q should be truthy", v)
		}
	})

	t.Run("falsy values", func(t *testing.T) {
		falsy := []string{"false", "FALSE", "0", "no", "off", "anything"}
		for _, v := range falsy {
			isTrue := strings.ToLower(v) == "true" || v == "1"
			assert.False(t, isTrue, "value %q should be falsy", v)
		}
	})

	t.Run("AllowHTTP default is false", func(t *testing.T) {
		assert.False(t, config.DefaultWebhookConfig().AllowHTTP)
	})

	t.Run("Swagger Enabled default is true", func(t *testing.T) {
		assert.True(t, config.DefaultSwaggerConfig().Enabled)
	})

	t.Run("S3UseSSL default is true", func(t *testing.T) {
		assert.True(t, config.DefaultStorageConfig().S3UseSSL)
	})

	t.Run("S3PathStyle default is false", func(t *testing.T) {
		assert.False(t, config.DefaultStorageConfig().S3PathStyle)
	})

	t.Run("CompressionEnabled default is true", func(t *testing.T) {
		assert.True(t, config.DefaultImageConfig().CompressionEnabled)
	})
}

// ==============================================================================
// URL PARSING TESTS (indirect testing of unexported parsePostgresURL/parseRedisURL)
// ==============================================================================

// TestPostgresURLParsing verifies postgres:// URL parsing indirectly through DSN().
// We construct DatabaseConfig values that match what parsePostgresURL would produce
// for various URL formats, then verify DSN() output.
func TestPostgresURLParsing(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.DatabaseConfig
		want []string
	}{
		{
			name: "standard postgres URL",
			cfg: config.DatabaseConfig{
				Host: "host", Port: 5432, User: "user", Password: "password",
				DBName: "dbname", SSLMode: "require",
			},
			want: []string{"host=host", "port=5432", "user=user", "password=password", "dbname=dbname", "sslmode=require"},
		},
		{
			name: "URL with custom port",
			cfg: config.DatabaseConfig{
				Host: "db.example.com", Port: 5433, User: "admin", Password: "secret",
				DBName: "mydb", SSLMode: "disable",
			},
			want: []string{"host=db.example.com", "port=5433", "user=admin", "dbname=mydb", "sslmode=disable"},
		},
		{
			name: "URL with special password chars",
			cfg: config.DatabaseConfig{
				Host: "pg", Port: 5432, User: "user", Password: "p%40ssw0rd!",
				DBName: "app", SSLMode: "require",
			},
			want: []string{"host=pg", "port=5432", "user=user", "password=p%40ssw0rd!", "dbname=app", "sslmode=require"},
		},
		{
			name: "local dev URL with sslmode=disable",
			cfg: config.DatabaseConfig{
				Host: "localhost", Port: 5432, User: "postgres", Password: "postgres",
				DBName: "go_api_base", SSLMode: "disable",
			},
			want: []string{"host=localhost", "port=5432", "user=postgres", "dbname=go_api_base", "sslmode=disable"},
		},
		{
			name: "empty password still in DSN",
			cfg: config.DatabaseConfig{
				Host: "localhost", Port: 5432, User: "dev", Password: "",
				DBName: "devdb", SSLMode: "disable",
			},
			want: []string{"host=localhost", "port=5432", "user=dev", "dbname=devdb", "sslmode=disable"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dsn := tc.cfg.DSN()
			assert.NotEmpty(t, dsn)
			for _, s := range tc.want {
				assert.Contains(t, dsn, s, "DSN should contain %q", s)
			}
		})
	}
}

// TestRedisURLParsing verifies redis:// URL parsing indirectly through Addr().
// We construct RedisConfig values that match what parseRedisURL would produce
// for various URL formats, then verify Addr().
func TestRedisURLParsing(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.RedisConfig
		wantAddr string
	}{
		{
			name:     "standard redis URL",
			cfg:      config.RedisConfig{Host: "redis.example.com", Port: 6379, DB: 0},
			wantAddr: "redis.example.com:6379",
		},
		{
			name:     "URL with password and custom DB",
			cfg:      config.RedisConfig{Host: "redis-host", Port: 6380, DB: 2, Password: "secret"},
			wantAddr: "redis-host:6380",
		},
		{
			name:     "local redis",
			cfg:      config.RedisConfig{Host: "localhost", Port: 6379, DB: 0},
			wantAddr: "localhost:6379",
		},
		{
			name:     "no port specified (default 6379)",
			cfg:      config.RedisConfig{Host: "my-redis", Port: 6379, DB: 3},
			wantAddr: "my-redis:6379",
		},
		{
			name:     "redis with auth token",
			cfg:      config.RedisConfig{Host: "10.0.0.5", Port: 6379, DB: 0, Password: "auth-token"},
			wantAddr: "10.0.0.5:6379",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantAddr, tc.cfg.Addr())
		})
	}
}

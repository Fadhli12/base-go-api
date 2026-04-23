// Package config provides application configuration management.
// It supports loading configuration from environment variables, .env files,
// and YAML configuration files using Viper.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// DatabaseConfig holds PostgreSQL database connection settings.
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		d.Host, d.User, d.Password, d.DBName, d.Port, d.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	DB       int    `mapstructure:"db"`
	Password string `mapstructure:"password"`
}

// Addr returns the Redis server address.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig holds JWT token configuration.
type JWTConfig struct {
	Secret        string        `mapstructure:"secret"`
	AccessExpiry  time.Duration `mapstructure:"access_expiry"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// Config is the top-level application configuration.
// It aggregates all configuration sections for different components.
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Image    ImageConfig    `mapstructure:"image"`
	Swagger  SwaggerConfig  `mapstructure:"swagger"`
	Cache    CacheConfig    `mapstructure:"cache"`
}

var (
	instance *Config
	once     sync.Once
)

// Load returns the singleton Config instance.
// It loads configuration from .env file and environment variables,
// sets sensible defaults, and validates required fields.
// This function is thread-safe and will return the same instance on subsequent calls.
func Load() (*Config, error) {
	var loadErr error

	once.Do(func() {
		instance, loadErr = loadConfig()
	})

	if loadErr != nil {
		return nil, loadErr
	}

	return instance, nil
}

// loadConfig performs the actual configuration loading.
func loadConfig() (*Config, error) {
	v := viper.New()

	// Load .env file if it exists
	loadEnvFile()

	// Set up environment variable binding
	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Try to load YAML config file (optional)
	loadYAMLConfig(v)

	// Parse environment variables into the config struct
	cfg := &Config{}
	if err := parseDatabaseConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}
	if err := parseRedisConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse redis config: %w", err)
	}
	if err := parseJWTConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse jwt config: %w", err)
	}
	if err := parseServerConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}
	if err := parseLogConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse log config: %w", err)
	}
	if err := parseCORSConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse cors config: %w", err)
	}
	if err := parseStorageConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse storage config: %w", err)
	}
	if err := parseImageConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse image config: %w", err)
	}
	if err := parseSwaggerConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse swagger config: %w", err)
	}
	if err := parseCacheConfig(v, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse cache config: %w", err)
	}

	// Validate required fields
	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadEnvFile attempts to load .env file from common locations.
func loadEnvFile() {
	// Try multiple locations for .env file
	locations := []string{
		".env",
		"config/.env",
		"../config/.env",
		"../../config/.env",
	}

	for _, loc := range locations {
		if err := godotenv.Load(loc); err == nil {
			slog.Debug("Loaded .env file", "path", loc)
			return
		}
	}

	// It's okay if no .env file exists - we'll use environment variables
	slog.Debug("No .env file found, using environment variables")
}

// setDefaults configures default values for optional configuration.
func setDefaults(v *viper.Viper) {
	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.dbname", "go_api_base")
	v.SetDefault("database.sslmode", "disable")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)

	// JWT defaults
	v.SetDefault("jwt.access_expiry", "15m")
	v.SetDefault("jwt.refresh_expiry", "720h")

	// Server defaults
	v.SetDefault("server.port", 8080)

	// Log defaults
	v.SetDefault("log.level", "info")

	// CORS defaults
	v.SetDefault("cors.allowed_origins", "http://localhost:3000")

	// Storage defaults
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.local_path", "./storage/uploads")
	v.SetDefault("storage.base_url", "http://localhost:8080/storage")

	// Image defaults
	v.SetDefault("image.compression_enabled", true)
	v.SetDefault("image.thumbnail_quality", 85)
	v.SetDefault("image.thumbnail_width", 300)
	v.SetDefault("image.thumbnail_height", 300)
	v.SetDefault("image.preview_quality", 90)
	v.SetDefault("image.preview_width", 800)
	v.SetDefault("image.preview_height", 600)

	// Swagger defaults
	v.SetDefault("swagger.enabled", true)
	v.SetDefault("swagger.path", "/swagger")

	// Cache defaults
	v.SetDefault("cache.driver", "redis")
	v.SetDefault("cache.default_ttl", 300)
	v.SetDefault("cache.permission_ttl", 300)
	v.SetDefault("cache.rate_limit_ttl", 60)
}

// loadYAMLConfig attempts to load configuration from YAML files.
func loadYAMLConfig(v *viper.Viper) {
	// Try multiple locations for config file
	locations := []string{
		"config.yaml",
		"config/config.yaml",
		"../config/config.yaml",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			v.SetConfigFile(loc)
			v.SetConfigType("yaml")
			if err := v.ReadInConfig(); err == nil {
				slog.Debug("Loaded config file", "path", loc)
				return
			}
		}
	}

	// It's okay if no config file exists - we'll use environment variables and defaults
	slog.Debug("No config.yaml file found, using defaults and environment variables")
}

// parseDatabaseConfig parses database configuration from environment.
func parseDatabaseConfig(v *viper.Viper, cfg *Config) error {
	// Check if DATABASE_URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		// Parse connection string
		parsed, err := parsePostgresURL(databaseURL)
		if err != nil {
			return fmt.Errorf("parsing DATABASE_URL: %w", err)
		}
		cfg.Database = *parsed
		return nil
	}

	// Otherwise, parse individual environment variables
	cfg.Database.Host = getEnvOrDefault("DATABASE_HOST", v.GetString("database.host"))
	cfg.Database.Port = getEnvIntOrDefault("DATABASE_PORT", v.GetInt("database.port"))
	cfg.Database.User = getEnvOrDefault("DATABASE_USER", v.GetString("database.user"))
	cfg.Database.Password = getEnvOrDefault("DATABASE_PASSWORD", v.GetString("database.password"))
	cfg.Database.DBName = getEnvOrDefault("DATABASE_DBNAME", v.GetString("database.dbname"))
	cfg.Database.SSLMode = getEnvOrDefault("DATABASE_SSLMODE", v.GetString("database.sslmode"))

	return nil
}

// parsePostgresURL parses a PostgreSQL connection string.
func parsePostgresURL(url string) (*DatabaseConfig, error) {
	// Simple parser for postgres:// URLs
	// Format: postgres://user:password@host:port/dbname?sslmode=...
	if !strings.HasPrefix(url, "postgres://") {
		return nil, fmt.Errorf("invalid DATABASE_URL format")
	}

	url = strings.TrimPrefix(url, "postgres://")

	// Split user:password and the rest
	parts := strings.SplitN(url, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid DATABASE_URL format: missing @")
	}

	credentials := strings.SplitN(parts[0], ":", 2)
	if len(credentials) != 2 {
		return nil, fmt.Errorf("invalid DATABASE_URL format: missing password")
	}

	user := credentials[0]
	password := credentials[1]

	// Parse host:port/dbname
	remaining := parts[1]
	hostPortDB := remaining
	sslmode := "disable"

	// Check for query parameters
	if idx := strings.Index(remaining, "?"); idx != -1 {
		hostPortDB = remaining[:idx]
		query := remaining[idx+1:]
		if strings.HasPrefix(query, "sslmode=") {
			sslmode = strings.TrimPrefix(query, "sslmode=")
		}
	}

	// Split host:port from dbname
	hostPortSlashDB := strings.SplitN(hostPortDB, "/", 2)
	hostPort := hostPortSlashDB[0]
	dbname := "go_api_base"
	if len(hostPortSlashDB) == 2 {
		dbname = hostPortSlashDB[1]
	}

	// Parse host:port
	host := hostPort
	port := 5432
	if idx := strings.LastIndex(hostPort, ":"); idx != -1 {
		host = hostPort[:idx]
		p, err := strconv.Atoi(hostPort[idx+1:])
		if err == nil {
			port = p
		}
	}

	return &DatabaseConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
		SSLMode:  sslmode,
	}, nil
}

// parseRedisConfig parses Redis configuration from environment.
func parseRedisConfig(v *viper.Viper, cfg *Config) error {
	// Check if REDIS_URL is provided
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		// Parse connection string
		parsed, err := parseRedisURL(redisURL)
		if err != nil {
			return fmt.Errorf("parsing REDIS_URL: %w", err)
		}
		cfg.Redis = *parsed
		return nil
	}

	// Otherwise, parse individual environment variables
	cfg.Redis.Host = getEnvOrDefault("REDIS_HOST", v.GetString("redis.host"))
	cfg.Redis.Port = getEnvIntOrDefault("REDIS_PORT", v.GetInt("redis.port"))
	cfg.Redis.DB = getEnvIntOrDefault("REDIS_DB", v.GetInt("redis.db"))
	cfg.Redis.Password = getEnvOrDefault("REDIS_PASSWORD", v.GetString("redis.password"))

	return nil
}

// parseRedisURL parses a Redis connection string.
func parseRedisURL(url string) (*RedisConfig, error) {
	// Simple parser for redis:// URLs
	// Format: redis://[:password@]host:port/db
	if !strings.HasPrefix(url, "redis://") {
		return nil, fmt.Errorf("invalid REDIS_URL format")
	}

	url = strings.TrimPrefix(url, "redis://")

	host := "localhost"
	port := 6379
	db := 0
	password := ""

	// Check for password
	if idx := strings.Index(url, "@"); idx != -1 {
		passwordPart := url[:idx]
		if strings.HasPrefix(passwordPart, ":") {
			password = passwordPart[1:]
		} else {
			password = passwordPart
		}
		url = url[idx+1:]
	}

	// Parse host:port/db
	parts := strings.Split(url, "/")
	hostPort := parts[0]
	if len(parts) > 1 && parts[1] != "" {
		d, err := strconv.Atoi(parts[1])
		if err == nil {
			db = d
		}
	}

	// Split host:port
	if idx := strings.LastIndex(hostPort, ":"); idx != -1 {
		host = hostPort[:idx]
		p, err := strconv.Atoi(hostPort[idx+1:])
		if err == nil {
			port = p
		}
	} else {
		host = hostPort
	}

	return &RedisConfig{
		Host:     host,
		Port:     port,
		DB:       db,
		Password: password,
	}, nil
}

// parseJWTConfig parses JWT configuration from environment.
func parseJWTConfig(v *viper.Viper, cfg *Config) error {
	cfg.JWT.Secret = getEnvOrDefault("JWT_SECRET", "change-me-in-production")

	accessExpiry := getEnvOrDefault("JWT_ACCESS_EXPIRY", v.GetString("jwt.access_expiry"))
	duration, err := time.ParseDuration(accessExpiry)
	if err != nil {
		return fmt.Errorf("invalid JWT_ACCESS_EXPIRY: %w", err)
	}
	cfg.JWT.AccessExpiry = duration

	refreshExpiry := getEnvOrDefault("JWT_REFRESH_EXPIRY", v.GetString("jwt.refresh_expiry"))
	duration, err = time.ParseDuration(refreshExpiry)
	if err != nil {
		return fmt.Errorf("invalid JWT_REFRESH_EXPIRY: %w", err)
	}
	cfg.JWT.RefreshExpiry = duration

	return nil
}

// parseServerConfig parses server configuration from environment.
func parseServerConfig(v *viper.Viper, cfg *Config) error {
	cfg.Server.Port = getEnvIntOrDefault("SERVER_PORT", v.GetInt("server.port"))
	return nil
}

// parseLogConfig parses logging configuration from environment.
func parseLogConfig(v *viper.Viper, cfg *Config) error {
	cfg.Log.Level = getEnvOrDefault("LOG_LEVEL", v.GetString("log.level"))
	return nil
}

// parseCORSConfig parses CORS configuration from environment.
func parseCORSConfig(v *viper.Viper, cfg *Config) error {
	origins := getEnvOrDefault("CORS_ALLOWED_ORIGINS", v.GetString("cors.allowed_origins"))
	cfg.CORS.AllowedOrigins = strings.Split(origins, ",")
	for i := range cfg.CORS.AllowedOrigins {
		cfg.CORS.AllowedOrigins[i] = strings.TrimSpace(cfg.CORS.AllowedOrigins[i])
	}
	return nil
}

// validate checks that all required configuration fields are set.
func validate(cfg *Config) error {
	var missing []string

	if cfg.Database.Host == "" {
		missing = append(missing, "DATABASE_HOST")
	}
	if cfg.Database.User == "" {
		missing = append(missing, "DATABASE_USER")
	}
	if cfg.Database.DBName == "" {
		missing = append(missing, "DATABASE_DBNAME")
	}
	if cfg.Redis.Host == "" {
		missing = append(missing, "REDIS_HOST")
	}
	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "change-me-in-production" {
		missing = append(missing, "JWT_SECRET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %v", missing)
	}

	return nil
}

// getEnvOrDefault returns the value of an environment variable or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns the integer value of an environment variable or a default.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault returns the boolean value of an environment variable or a default.
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

// parseStorageConfig parses storage configuration from environment.
func parseStorageConfig(v *viper.Viper, cfg *Config) error {
	cfg.Storage.Driver = getEnvOrDefault("STORAGE_DRIVER", v.GetString("storage.driver"))
	cfg.Storage.LocalPath = getEnvOrDefault("STORAGE_LOCAL_PATH", v.GetString("storage.local_path"))
	cfg.Storage.BaseURL = getEnvOrDefault("STORAGE_BASE_URL", v.GetString("storage.base_url"))
	// S3/MinIO fields
	cfg.Storage.S3Endpoint = getEnvOrDefault("STORAGE_S3_ENDPOINT", v.GetString("storage.s3_endpoint"))
	cfg.Storage.S3Region = getEnvOrDefault("STORAGE_S3_REGION", v.GetString("storage.s3_region"))
	cfg.Storage.S3Bucket = getEnvOrDefault("STORAGE_S3_BUCKET", v.GetString("storage.s3_bucket"))
	cfg.Storage.S3AccessKey = getEnvOrDefault("STORAGE_S3_ACCESS_KEY", v.GetString("storage.s3_access_key"))
	cfg.Storage.S3SecretKey = getEnvOrDefault("STORAGE_S3_SECRET_KEY", v.GetString("storage.s3_secret_key"))
	cfg.Storage.S3UseSSL = getEnvBoolOrDefault("STORAGE_S3_USE_SSL", v.GetBool("storage.s3_use_ssl"))
	cfg.Storage.S3PathStyle = getEnvBoolOrDefault("STORAGE_S3_PATH_STYLE", v.GetBool("storage.s3_path_style"))
	return nil
}

// parseImageConfig parses image configuration from environment.
func parseImageConfig(v *viper.Viper, cfg *Config) error {
	cfg.Image.CompressionEnabled = getEnvBoolOrDefault("IMAGE_COMPRESSION_ENABLED", v.GetBool("image.compression_enabled"))
	cfg.Image.ThumbnailQuality = getEnvIntOrDefault("IMAGE_COMPRESSION_THUMBNAIL_QUALITY", v.GetInt("image.thumbnail_quality"))
	cfg.Image.ThumbnailWidth = getEnvIntOrDefault("IMAGE_COMPRESSION_THUMBNAIL_WIDTH", v.GetInt("image.thumbnail_width"))
	cfg.Image.ThumbnailHeight = getEnvIntOrDefault("IMAGE_COMPRESSION_THUMBNAIL_HEIGHT", v.GetInt("image.thumbnail_height"))
	cfg.Image.PreviewQuality = getEnvIntOrDefault("IMAGE_COMPRESSION_PREVIEW_QUALITY", v.GetInt("image.preview_quality"))
	cfg.Image.PreviewWidth = getEnvIntOrDefault("IMAGE_COMPRESSION_PREVIEW_WIDTH", v.GetInt("image.preview_width"))
	cfg.Image.PreviewHeight = getEnvIntOrDefault("IMAGE_COMPRESSION_PREVIEW_HEIGHT", v.GetInt("image.preview_height"))
	return nil
}

// parseSwaggerConfig parses swagger configuration from environment.
func parseSwaggerConfig(v *viper.Viper, cfg *Config) error {
	cfg.Swagger.Enabled = getEnvBoolOrDefault("SWAGGER_ENABLED", v.GetBool("swagger.enabled"))
	cfg.Swagger.Path = getEnvOrDefault("SWAGGER_PATH", v.GetString("swagger.path"))
	return nil
}

// parseCacheConfig parses cache configuration from environment.
func parseCacheConfig(v *viper.Viper, cfg *Config) error {
	cfg.Cache.Driver = getEnvOrDefault("CACHE_DRIVER", v.GetString("cache.driver"))
	cfg.Cache.DefaultTTL = getEnvIntOrDefault("CACHE_DEFAULT_TTL", v.GetInt("cache.default_ttl"))
	cfg.Cache.PermissionTTL = getEnvIntOrDefault("CACHE_PERMISSION_TTL", v.GetInt("cache.permission_ttl"))
	cfg.Cache.RateLimitTTL = getEnvIntOrDefault("CACHE_RATE_LIMIT_TTL", v.GetInt("cache.rate_limit_ttl"))
	return nil
}

// GetProjectRoot returns the project root directory.
func GetProjectRoot() string {
	// Try to find project root by looking for go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

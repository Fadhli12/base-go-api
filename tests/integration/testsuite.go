//go:build integration
// +build integration

// Package integration provides integration test infrastructure.
// It connects to local PostgreSQL and Redis instances using environment variables:
//
//	DATABASE_URL or PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE (defaults to localhost:5432/testdb)
//	REDIS_URL or REDISHOST/REDISPORT (defaults to localhost:6379)
//
// No Docker containers are started — the tests run against your local dev services.
package integration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testSuite is the global shared test suite instance.
// It is initialized once on first call to NewTestSuite and reused across all tests.
var testSuite *TestSuite

// suiteMu protects testSuite initialization from concurrent access.
// Without this, parallel test goroutines can both see testSuite == nil
// and race to DROP+CREATE tables, causing "relation does not exist" errors.
var suiteMu sync.Mutex

// TestSuite holds all dependencies for integration tests.
type TestSuite struct {
	DB             *gorm.DB
	RedisClient    *redis.Client
	Cleanup        func()
	migrationsRun  bool
}

// envLoaded tracks whether .env has been loaded to avoid duplicate loading.
var envLoaded bool

// loadEnvFile reads a .env file and sets environment variables.
// Variables already set in the environment take precedence (are not overridden).
// This allows tests to pick up database credentials from the project's .env file
// without requiring developers to manually export environment variables.
func loadEnvFile() {
	if envLoaded {
		return
	}
	envLoaded = true

	// Walk up from current directory to find .env file
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			file, err := os.Open(path)
			if err != nil {
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// Skip comments and empty lines
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Only set if not already in environment
				if os.Getenv(key) == "" {
					os.Setenv(key, value)
				}
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// getDatabaseURL returns the PostgreSQL connection string from environment or defaults.
// Loads .env file first to pick up project configuration.
// Supports DATABASE_URL (full connection string) or individual DATABASE_HOST/PORT/USER/PASSWORD vars.
// For test isolation, TEST_DATABASE_URL takes precedence, then DATABASE_URL, then individual vars.
// Defaults match the .env.example configuration.
func getDatabaseURL() string {
	loadEnvFile()
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	host := envOrDefault("DATABASE_HOST", "localhost")
	port := envOrDefault("DATABASE_PORT", "5432")
	user := envOrDefault("DATABASE_USER", "postgres")
	password := envOrDefault("DATABASE_PASSWORD", "postgres")
	database := envOrDefault("DATABASE_NAME", "go_api_base")
	sslmode := envOrDefault("DATABASE_SSL_MODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, database, sslmode)
}

// getRedisAddr returns the Redis address from environment or defaults.
// Loads .env file first to pick up project configuration.
func getRedisAddr() string {
	loadEnvFile()
	if url := os.Getenv("REDIS_URL"); url != "" {
		// Strip redis:// prefix if present
		addr := url
		if len(addr) > 8 && addr[:8] == "redis://" {
			addr = addr[8:]
		}
		// Strip trailing /0 db number if present
		if idx := len(addr) - 2; idx > 0 && addr[idx] == '/' {
			addr = addr[:idx]
		}
		return addr
	}
	host := envOrDefault("REDIS_HOST", "localhost")
	port := envOrDefault("REDIS_PORT", "6379")
	return fmt.Sprintf("%s:%s", host, port)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// NewTestSuite creates or returns the shared test suite connected to local PostgreSQL and Redis.
// It is safe for concurrent use — a sync.Mutex ensures only one goroutine initializes the suite.
// Subsequent calls reuse the same connection and just truncate tables for test isolation.
//
// The first call initializes the shared suite (database + migrations). Subsequent calls
// reuse the same connection and just truncate tables for test isolation.
// This ensures all tests share one database connection and don't conflict with each other.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    suite := NewTestSuite(t)
//	    defer suite.Cleanup()
//	    // use suite.DB and suite.RedisClient
//	}
func NewTestSuite(t *testing.T) *TestSuite {
	// Fast path: suite already initialized, no lock needed
	if testSuite != nil {
		testSuite.SetupTest(t)
		return testSuite
	}

	// Slow path: acquire lock to ensure single initialization
	suiteMu.Lock()

	// Double-check after acquiring lock (another goroutine may have initialized)
	if testSuite != nil {
		suiteMu.Unlock()
		testSuite.SetupTest(t)
		return testSuite
	}

	ctx := context.Background()
	connStr := getDatabaseURL()

	// Connect GORM to PostgreSQL with retry
	var db *gorm.DB
	var connErr error
	for i := 0; i < 10; i++ {
		db, connErr = gorm.Open(postgres.Open(connStr), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if connErr == nil {
			sqlDB, pingErr := db.DB()
			if pingErr == nil && sqlDB.Ping() == nil {
				break
			}
		}
		t.Logf("Waiting for PostgreSQL to be ready... (attempt %d/10)", i+1)
		time.Sleep(500 * time.Millisecond)
	}
	if connErr != nil {
		suiteMu.Unlock()
		require.NoError(t, connErr, "Failed to connect GORM to PostgreSQL after 10 retries")
		return nil // unreachable, require.NoError calls t.FailNow
	}

	// Verify database connection
	sqlDB, dbErr := db.DB()
	if dbErr != nil {
		suiteMu.Unlock()
		require.NoError(t, dbErr, "Failed to get underlying SQL DB")
		return nil
	}
	if pingErr := sqlDB.Ping(); pingErr != nil {
		suiteMu.Unlock()
		require.NoError(t, pingErr, "Failed to ping database")
		return nil
	}

	// Connect to Redis
	redisAddr := getRedisAddr()
	redisOpts := &redis.Options{
		Addr:         redisAddr,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	redisClient := redis.NewClient(redisOpts)

	// Verify Redis connection
	ctxRedis, cancelRedis := context.WithTimeout(ctx, 5*time.Second)
	defer cancelRedis()
	if redisErr := redisClient.Ping(ctxRedis).Err(); redisErr != nil {
		suiteMu.Unlock()
		require.NoError(t, redisErr, "Failed to ping Redis")
		return nil
	}

	// Create cleanup function — no-op for shared suite.
	// TestMain handles final cleanup after all tests complete.
	cleanup := func() {
		// Shared suite: connections are closed in TestMain, not per-test.
	}

	ts := &TestSuite{
		DB:          db,
		RedisClient: redisClient,
		Cleanup:     cleanup,
	}

	// Run migrations (returns error instead of using require.NoError,
	// since we hold the mutex and can't call t.FailNow inside the lock)
	migErr := ts.runMigrations()

	// Assign to global BEFORE unlocking so other goroutines see it
	testSuite = ts

	// Release lock — now require.NoError can safely call t.FailNow if needed
	suiteMu.Unlock()

	require.NoError(t, migErr, "Failed to run database migrations")

	// Clean tables before each test for isolation
	testSuite.SetupTest(t)
	return testSuite
}

// SetupTest runs before each test to clean the database and Redis.
// This ensures test isolation between individual test cases.
func (s *TestSuite) SetupTest(t *testing.T) {
	ctx := context.Background()

	// Clean all tables (order matters due to foreign key constraints)
	// Child tables first, then parent tables
	tables := []string{
		"notification_preferences",
		"notifications",
		"email_bounces",
		"email_queue",
		"email_templates",
		"password_reset_tokens",
		"media_downloads",
		"media_conversions",
		"media",
		"api_keys",
		"audit_logs",
		"user_permissions",
		"role_permissions",
		"user_roles",
		"refresh_tokens",
		"invoices",
		"news",
		"organization_members",
		"organizations",
		"webhook_deliveries",
		"webhooks",
		"casbin_rule",
		"permissions",
		"roles",
		"users",
	}

	// Disable audit_logs immutability trigger before truncating (it blocks DELETE/TRUNCATE)
	s.DB.Exec("ALTER TABLE audit_logs DISABLE TRIGGER audit_logs_immutable")

	for _, table := range tables {
		result := s.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if result.Error != nil {
			// Table may not exist if migrations haven't been run or this is a new table
			// Log but don't fail — the test will fail naturally if the table is actually needed
			t.Logf("Warning: truncate skipped for %s: %v", table, result.Error)
		}
	}

	// Re-enable audit_logs immutability trigger
	s.DB.Exec("ALTER TABLE audit_logs ENABLE TRIGGER audit_logs_immutable")

	// Reset sequences only for tables with SERIAL primary keys
	// (UUID-based tables use gen_random_uuid() and don't have sequences)
	serialTables := []string{"casbin_rule"}
	for _, table := range serialTables {
		result := s.DB.Exec(fmt.Sprintf("ALTER SEQUENCE %s_id_seq RESTART WITH 1", table))
		if result.Error != nil {
			t.Logf("Info: sequence reset skipped for %s: %v", table, result.Error)
		}
	}

	// Flush Redis database
	require.NoError(t, s.RedisClient.FlushDB(ctx).Err(), "Failed to flush Redis DB")
}

// RunMigrations executes database migrations on the test database.
// This should be called once after creating the test suite.
// Subsequent calls are no-ops (migrations are idempotent but running them
// repeatedly is wasteful).
func (s *TestSuite) RunMigrations(t *testing.T) {
	if s.migrationsRun {
		t.Logf("Migrations already run, skipping")
		return
	}
	require.NoError(t, s.runMigrations(), "Failed to run database migrations")
}

// runMigrations is the internal migration runner that returns errors instead
// of calling require.NoError. This is necessary because NewTestSuite holds a
// mutex during initialization — calling require.NoError (which may invoke
// t.FailNow via runtime.Goexit) while holding a mutex would deadlock.
func (s *TestSuite) runMigrations() error {
	if s.migrationsRun {
		return nil
	}

	// Collect all migration errors
	var errs []error

	// Drop all tables first to ensure a clean schema.
	if err := s.DB.Exec(`
DROP TABLE IF EXISTS notification_preferences CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS email_bounces CASCADE;
DROP TABLE IF EXISTS email_queue CASCADE;
DROP TABLE IF EXISTS email_templates CASCADE;
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
DROP TABLE IF EXISTS media_downloads CASCADE;
DROP TABLE IF EXISTS media_conversions CASCADE;
DROP TABLE IF EXISTS media CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS user_permissions CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS invoices CASCADE;
DROP TABLE IF EXISTS news CASCADE;
DROP TABLE IF EXISTS organization_members CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS webhook_deliveries CASCADE;
DROP TABLE IF EXISTS webhooks CASCADE;
DROP TABLE IF EXISTS casbin_rule CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
	`).Error; err != nil {
		return fmt.Errorf("drop existing tables: %w", err)
	}

	// Migration 000001: Base schema (users, roles, permissions, pivots, refresh_tokens)
	migration001 := `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Drop all triggers first (idempotent: run migrations on existing databases)
DROP TRIGGER IF EXISTS update_users_updated_at ON users CASCADE;
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles CASCADE;
DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions CASCADE;
DROP TRIGGER IF EXISTS update_invoices_updated_at ON invoices CASCADE;
DROP TRIGGER IF EXISTS audit_logs_immutable ON audit_logs CASCADE;
DROP TRIGGER IF EXISTS update_news_updated_at ON news CASCADE;
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations CASCADE;
DROP TRIGGER IF EXISTS update_media_updated_at ON media CASCADE;
DROP TRIGGER IF EXISTS update_email_templates_updated_at ON email_templates CASCADE;
DROP TRIGGER IF EXISTS update_email_queue_updated_at ON email_queue CASCADE;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys CASCADE;
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications CASCADE;
DROP TRIGGER IF EXISTS update_notification_preferences_updated_at ON notification_preferences CASCADE;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    scope TEXT DEFAULT 'all',
    description TEXT,
    is_system BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create user_roles pivot table
CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    assigned_by UUID,
    PRIMARY KEY (user_id, role_id)
);

-- Create role_permissions pivot table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (role_id, permission_id)
);

-- Create user_permissions table for direct overrides
CREATE TABLE IF NOT EXISTS user_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    effect VARCHAR(10) NOT NULL CHECK (effect IN ('allow', 'deny')),
    assigned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    assigned_by UUID,
    UNIQUE (user_id, permission_id)
);

-- Create refresh_tokens table (includes family_id from 000010 and session metadata from 000011)
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    family_id UUID NOT NULL DEFAULT gen_random_uuid(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    user_agent VARCHAR(500),
    ip_address VARCHAR(45),
    device_name VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_roles_is_system ON roles(is_system);
CREATE INDEX IF NOT EXISTS idx_permissions_name ON permissions(name);
CREATE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);
CREATE INDEX IF NOT EXISTS idx_permissions_deleted_at ON permissions(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_permissions_is_system ON permissions(is_system);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_user_id ON user_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_permission_id ON user_permissions(permission_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_effect ON user_permissions(effect);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_revoked_at ON refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_active ON refresh_tokens(user_id, revoked_at) WHERE revoked_at IS NULL;

-- Add updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add triggers for updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_permissions_updated_at BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration001).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000001: %w", err))
	}

	// Migration 000001_1: Invoices table
	migration001_1 := `
-- Create invoices table
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    number VARCHAR(50) NOT NULL,
    customer VARCHAR(255) NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    description VARCHAR(1000),
    due_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    CONSTRAINT invoices_number_unique UNIQUE (number),
    CONSTRAINT invoices_valid_status CHECK (status IN ('draft', 'pending', 'paid', 'cancelled'))
);

-- Create indexes for invoices table
CREATE INDEX IF NOT EXISTS idx_invoices_user_id ON invoices(user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoices_deleted_at ON invoices(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_invoices_created_at ON invoices(created_at);

-- Add trigger for updated_at (reuse existing function)
CREATE TRIGGER update_invoices_updated_at BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration001_1).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000001_1 (invoices): %w", err))
	}

	// Migration 000002: Audit logs
	migration002 := `
-- Create audit_logs table (resource_id is VARCHAR to match Go model, not UUID)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID,
    action VARCHAR(20) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100),
    before JSONB,
    after JSONB,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create indexes for audit_logs table
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs(resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_created_at ON audit_logs(resource, created_at);

-- Add trigger to prevent UPDATE or DELETE on audit_logs (immutability enforcement)
CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs table is immutable: no UPDATE or DELETE allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_log_modification();
`

	if err := s.DB.Exec(migration002).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000002 (audit_logs): %w", err))
	}

	// Migration 000003: News table
	migration003 := `
-- Create news table
CREATE TABLE IF NOT EXISTS news (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    content TEXT NOT NULL,
    excerpt VARCHAR(500),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    tags JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    published_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create indexes for news table
CREATE INDEX IF NOT EXISTS idx_news_author_id ON news(author_id);
CREATE INDEX IF NOT EXISTS idx_news_slug ON news(slug);
CREATE INDEX IF NOT EXISTS idx_news_status ON news(status);
CREATE INDEX IF NOT EXISTS idx_news_created_at ON news(created_at);
CREATE INDEX IF NOT EXISTS idx_news_published_at ON news(published_at) WHERE published_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_news_deleted_at ON news(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_news_author_status ON news(author_id, status);
CREATE INDEX IF NOT EXISTS idx_news_status_created_at ON news(status, created_at);

-- Add trigger for updated_at
CREATE TRIGGER update_news_updated_at BEFORE UPDATE ON news
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration003).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000003 (news): %w", err))
	}

	// Migration 000006: Organizations and organization_members
	migration006 := `
-- Create organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create organization_members table
CREATE TABLE IF NOT EXISTS organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (organization_id, user_id)
);

-- Create indexes for organizations table
CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);
CREATE INDEX IF NOT EXISTS idx_organizations_owner_id ON organizations(owner_id);
CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at) 
    WHERE deleted_at IS NOT NULL;

-- Create indexes for organization_members table
CREATE INDEX IF NOT EXISTS idx_organization_members_user_id ON organization_members(user_id);

-- Add trigger for organizations updated_at
CREATE TRIGGER update_organizations_updated_at 
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration006).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000006 (organizations): %w", err))
	}

	// Create casbin_rule table for permission testing
	migrationCasbin := `
CREATE TABLE IF NOT EXISTS casbin_rule (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(255) NOT NULL,
    v0 VARCHAR(255),
    v1 VARCHAR(255),
    v2 VARCHAR(255),
    v3 VARCHAR(255),
    v4 VARCHAR(255),
    v5 VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_casbin_rule_ptype ON casbin_rule(ptype);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v0 ON casbin_rule(v0);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v1 ON casbin_rule(v1);
CREATE INDEX IF NOT EXISTS idx_casbin_rule_v2 ON casbin_rule(v2);
`

	if err := s.DB.Exec(migrationCasbin).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration casbin_rule: %w", err))
	}

	// Migration 000008: Email service tables
	migration008 := `
-- Create email_templates table
CREATE TABLE IF NOT EXISTS email_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    subject VARCHAR(255) NOT NULL,
    html_content TEXT,
    text_content TEXT,
    category VARCHAR(50) NOT NULL DEFAULT 'transactional',
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create email_queue table (NO soft delete - permanent audit trail)
CREATE TABLE IF NOT EXISTS email_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    to_address VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL DEFAULT '',
    template VARCHAR(100),
    data JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'sent', 'delivered', 'bounced', 'failed')),
    priority INTEGER DEFAULT 0,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 5,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    bounced_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    bounce_reason TEXT,
    provider VARCHAR(50),
    message_id VARCHAR(255),
    last_error TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create email_bounces table (insert-only for compliance)
CREATE TABLE IF NOT EXISTS email_bounces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    bounce_type VARCHAR(20) NOT NULL CHECK (bounce_type IN ('hard', 'soft', 'spam', 'technical')),
    bounce_reason TEXT,
    message_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- Create indexes for email tables
CREATE INDEX IF NOT EXISTS idx_email_templates_name ON email_templates(name);
CREATE INDEX IF NOT EXISTS idx_email_templates_is_active ON email_templates(is_active);
CREATE INDEX IF NOT EXISTS idx_email_templates_deleted_at ON email_templates(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_email_queue_status ON email_queue(status);
CREATE INDEX IF NOT EXISTS idx_email_queue_to_address ON email_queue(to_address);
CREATE INDEX IF NOT EXISTS idx_email_queue_created_at ON email_queue(created_at);
CREATE INDEX IF NOT EXISTS idx_email_queue_status_created_at ON email_queue(status, created_at);

CREATE INDEX IF NOT EXISTS idx_email_bounces_email ON email_bounces(email);
CREATE INDEX IF NOT EXISTS idx_email_bounces_created_at ON email_bounces(created_at);
CREATE INDEX IF NOT EXISTS idx_email_bounces_type ON email_bounces(bounce_type);

-- Add trigger for email_templates updated_at
CREATE TRIGGER update_email_templates_updated_at BEFORE UPDATE ON email_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add trigger for email_queue updated_at
CREATE TRIGGER update_email_queue_updated_at BEFORE UPDATE ON email_queue
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration008).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000008 (email): %w", err))
	}

	// Migration 000005: API Keys table
	migration005 := `
-- Create api_keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    prefix VARCHAR(12) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMPTZ DEFAULT NULL,
    last_used_at TIMESTAMPTZ DEFAULT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

-- Create indexes for api_keys table
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked_at ON api_keys(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(user_id, deleted_at, revoked_at) WHERE deleted_at IS NULL AND revoked_at IS NULL;

-- Add trigger for updated_at
CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration005).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000005 (api_keys): %w", err))
	}

	// Migration 000004: Media tables
	migration004 := `
-- Create media table
CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type VARCHAR(255) NOT NULL,
    model_id UUID NOT NULL,
    collection_name VARCHAR(255) DEFAULT 'default' NOT NULL,
    disk VARCHAR(50) DEFAULT 'local' NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(500) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    path VARCHAR(2000) NOT NULL,
    metadata JSONB DEFAULT '{}',
    custom_properties JSONB DEFAULT '{}',
    uploaded_by_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    orphaned_at TIMESTAMPTZ DEFAULT NULL,
    CONSTRAINT check_media_size CHECK (size > 0),
    CONSTRAINT check_media_mime_type CHECK (mime_type ~ '^[a-z]+/[a-zA-Z0-9+\-\.]+$')
);

-- Create media_conversions table
CREATE TABLE IF NOT EXISTS media_conversions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    disk VARCHAR(50) DEFAULT 'local',
    path VARCHAR(2000) NOT NULL,
    size BIGINT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT check_conversion_size CHECK (size >= 0),
    CONSTRAINT unique_media_conversion_name UNIQUE (media_id, name)
);

-- Create media_downloads table
CREATE TABLE IF NOT EXISTS media_downloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    downloaded_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    downloaded_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ip_address VARCHAR(45),
    user_agent VARCHAR(2000)
);

-- Create indexes for media table
CREATE INDEX IF NOT EXISTS idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_collection ON media(collection_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_uploaded_by ON media(uploaded_by_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_mime_type ON media(mime_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_orphaned ON media(orphaned_at) WHERE orphaned_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_filename_disk ON media(disk, filename) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_model_collection ON media(model_type, model_id, collection_name) WHERE deleted_at IS NULL;

-- Create indexes for media_conversions table
CREATE INDEX IF NOT EXISTS idx_media_conversions_media ON media_conversions(media_id);
CREATE INDEX IF NOT EXISTS idx_media_conversions_created ON media_conversions(created_at DESC);

-- Create indexes for media_downloads table
CREATE INDEX IF NOT EXISTS idx_downloads_media ON media_downloads(media_id, downloaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_downloads_user ON media_downloads(downloaded_by_id, downloaded_at DESC);

-- Add trigger for updated_at on media table
CREATE TRIGGER update_media_updated_at BEFORE UPDATE ON media
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration004).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000004 (media): %w", err))
	}

	// Migration 000007: Add organization_id to resource tables
	migration007 := `
-- Add organization_id to news table
ALTER TABLE news ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_news_organization_id ON news(organization_id);

-- Add organization_id to media table
ALTER TABLE media ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_media_organization_id ON media(organization_id);

-- Add organization_id to api_keys table
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id);
`

	if err := s.DB.Exec(migration007).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000007 (organization_id): %w", err))
	}

	// Migration 000009: Password reset tokens
	migration009 := `
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_used_at ON password_reset_tokens(used_at) WHERE used_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_unused ON password_reset_tokens(user_id, expires_at) WHERE used_at IS NULL;
`

	if err := s.DB.Exec(migration009).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000009 (password_reset_tokens): %w", err))
	}

	// Migration 000012: Notifications
	migration012 := `
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    action_url VARCHAR(500),
    read_at TIMESTAMPTZ DEFAULT NULL,
    archived_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type VARCHAR(30) NOT NULL,
    email_enabled BOOLEAN DEFAULT true NOT NULL,
    push_enabled BOOLEAN DEFAULT true NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    CONSTRAINT uq_notification_prefs_user_type UNIQUE (user_id, notification_type)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, read_at);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_archived ON notifications(user_id, archived_at) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);

CREATE INDEX IF NOT EXISTS idx_notification_prefs_user_id ON notification_preferences(user_id) WHERE deleted_at IS NULL;

CREATE TRIGGER update_notifications_updated_at
    BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

	if err := s.DB.Exec(migration012).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000012 (notifications): %w", err))
	}

	// Migration 000014: Webhooks
	migration014 := `
-- Create webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events JSONB NOT NULL DEFAULT '[]',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    CONSTRAINT uq_webhooks_url_org UNIQUE (url, organization_id) WHERE deleted_at IS NULL,
    CONSTRAINT chk_webhooks_events_not_empty CHECK (jsonb_array_length(events) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_webhooks_url_org ON webhooks(url, organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_organization_id ON webhooks(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_active ON webhooks(active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_webhooks_events ON webhooks USING gin(events);

CREATE TRIGGER update_webhooks_updated_at
    BEFORE UPDATE ON webhooks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create webhook_deliveries table (no soft delete)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE RESTRICT,
    event VARCHAR(100) NOT NULL,
    payload JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued','processing','delivered','failed','rate_limited')),
    response_code INTEGER,
    response_body TEXT,
    duration_ms INTEGER,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ,
    processing_started_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status IN ('queued','rate_limited');
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status IN ('queued','rate_limited') AND next_retry_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_stuck ON webhook_deliveries(processing_started_at) WHERE status = 'processing' AND processing_started_at IS NOT NULL;
`

	if err := s.DB.Exec(migration014).Error; err != nil {
		errs = append(errs, fmt.Errorf("migration 000014 (webhooks): %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	s.migrationsRun = true
	return nil
}

// GetDB returns the GORM database instance.
func (s *TestSuite) GetDB() *gorm.DB {
	return s.DB
}

// GetRedisClient returns the Redis client instance.
func (s *TestSuite) GetRedisClient() *redis.Client {
	return s.RedisClient
}

// GetRedisAddr returns the Redis address in host:port format.
func (s *TestSuite) GetRedisAddr() string {
	return s.RedisClient.Options().Addr
}

// GetPostgresDSN returns a placeholder string for DSN.
// Note: The actual DSN contains sensitive information and is not stored.
func (s *TestSuite) GetPostgresDSN() string {
	return "<postgres-connection-string>"
}

// WaitForRedis waits for Redis to be ready with a timeout.
func (s *TestSuite) WaitForRedis(t *testing.T, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Timeout waiting for Redis to be ready")
		default:
			if err := s.RedisClient.Ping(ctx).Err(); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
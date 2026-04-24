//go:build integration
// +build integration

// Package integration provides integration test infrastructure using testcontainers.
package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestSuite holds all dependencies for integration tests.
// It provides isolated PostgreSQL and Redis containers with automatic cleanup.
type TestSuite struct {
	DB             *gorm.DB
	RedisClient    *redis.Client
	PGContainer    *postgrescontainer.PostgresContainer
	RedisContainer testcontainers.Container
	Cleanup        func()
}

// NewTestSuite creates a new test suite with PostgreSQL and Redis containers.
// It starts ephemeral containers, runs migrations, and returns a ready-to-use test suite.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    suite := NewTestSuite(t)
//	    defer suite.Cleanup()
//	    // use suite.DB and suite.RedisClient
//	}
func NewTestSuite(t *testing.T) *TestSuite {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgrescontainer.Run(ctx, "postgres:15-alpine",
		postgrescontainer.WithDatabase("testdb"),
		postgrescontainer.WithUsername("test"),
		postgrescontainer.WithPassword("test"),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection string with sslmode=disable for local testing
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get PostgreSQL connection string")

	// Connect GORM to PostgreSQL
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silence logs during tests
	})
	require.NoError(t, err, "Failed to connect GORM to PostgreSQL")

	// Verify database connection
	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get underlying SQL DB")
	require.NoError(t, sqlDB.Ping(), "Failed to ping database")

	// Start Redis container
	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "Failed to start Redis container")

	// Get Redis connection details
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err, "Failed to get Redis host")

	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err, "Failed to get Redis mapped port")

	// Create Redis client
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())
	redisClient := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Verify Redis connection
	ctxRedis, cancelRedis := context.WithTimeout(ctx, 5*time.Second)
	defer cancelRedis()
	require.NoError(t, redisClient.Ping(ctxRedis).Err(), "Failed to ping Redis")

	// Create cleanup function
	cleanup := func() {
		// Close Redis client
		if err := redisClient.Close(); err != nil {
			t.Logf("Warning: failed to close Redis client: %v", err)
		}

		// Terminate containers with timeout
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := pgContainer.Terminate(ctxTimeout); err != nil {
			t.Logf("Warning: failed to terminate PostgreSQL container: %v", err)
		}

		if err := redisContainer.Terminate(ctxTimeout); err != nil {
			t.Logf("Warning: failed to terminate Redis container: %v", err)
		}
	}

	return &TestSuite{
		DB:             db,
		RedisClient:    redisClient,
		PGContainer:    pgContainer,
		RedisContainer: redisContainer,
		Cleanup:        cleanup,
	}
}

// SetupTest runs before each test to clean the database and Redis.
// This ensures test isolation between individual test cases.
func (s *TestSuite) SetupTest(t *testing.T) {
	ctx := context.Background()

	// Clean all tables (order matters due to foreign key constraints)
	tables := []string{
		"user_permissions",
		"role_permissions",
		"user_roles",
		"organization_members",
		"refresh_tokens",
		"audit_logs",
		"news",
		"invoices",
		"organizations",
		"permissions",
		"roles",
		"users",
	}

	for _, table := range tables {
		result := s.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		require.NoError(t, result.Error, "Failed to truncate table %s", table)
	}

	// Reset sequences
	for _, table := range tables {
		result := s.DB.Exec(fmt.Sprintf("ALTER SEQUENCE %s_id_seq RESTART WITH 1", table))
		if result.Error != nil {
			// Sequence might not exist for some tables, ignore error
			t.Logf("Info: sequence reset skipped for %s: %v", table, result.Error)
		}
	}

	// Flush Redis database
	require.NoError(t, s.RedisClient.FlushDB(ctx).Err(), "Failed to flush Redis DB")
}

// RunMigrations executes database migrations on the test database.
// This should be called once after creating the test suite.
func (s *TestSuite) RunMigrations(t *testing.T) {
	// Migration 000001: Base schema (users, roles, permissions, pivots, refresh_tokens)
	migration001 := `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

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
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

-- Create role_permissions pivot table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (role_id, permission_id)
);

-- Create user_permissions table for direct overrides
CREATE TABLE IF NOT EXISTS user_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    effect VARCHAR(10) NOT NULL CHECK (effect IN ('allow', 'deny')),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    UNIQUE (user_id, permission_id)
);

-- Create refresh_tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
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

	result := s.DB.Exec(migration001)
	require.NoError(t, result.Error, "Failed to run migration 000001")

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

	result = s.DB.Exec(migration001_1)
	require.NoError(t, result.Error, "Failed to run invoice migration")

	// Migration 000002: Audit logs
	migration002 := `
-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    resource_id UUID,
    before JSONB,
    after JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
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

	result = s.DB.Exec(migration002)
	require.NoError(t, result.Error, "Failed to run migration 000002")

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

	result = s.DB.Exec(migration003)
	require.NoError(t, result.Error, "Failed to run migration 000003")

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

	result = s.DB.Exec(migration006)
	require.NoError(t, result.Error, "Failed to run migration 000006")

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

	result = s.DB.Exec(migrationCasbin)
	require.NoError(t, result.Error, "Failed to create casbin_rule table")

	t.Logf("Migrations completed successfully")
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
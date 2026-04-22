//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestInfrastructure_Initialization validates that test containers are properly initialized.
// This is the "hello world" test for the integration test infrastructure.
func TestInfrastructure_Initialization(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	// Verify PostgreSQL is accessible
	var result int
	err := suite.DB.Raw("SELECT 1").Scan(&result).Error
	require.NoError(t, err, "PostgreSQL connection should work")
	require.Equal(t, 1, result, "PostgreSQL query should return expected result")

	t.Log("PostgreSQL container is running and accessible")

	// Verify Redis is accessible
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = suite.RedisClient.Ping(ctx).Err()
	require.NoError(t, err, "Redis connection should work")

	t.Log("Redis container is running and accessible")
}

// TestInfrastructure_DatabaseSchema validates that migrations were applied correctly.
func TestInfrastructure_DatabaseSchema(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	// Verify users table exists
	var exists bool
	err := suite.DB.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'users'
		)
	`).Scan(&exists).Error
	require.NoError(t, err, "Should be able to query information_schema")
	require.True(t, exists, "users table should exist")

	t.Log("users table exists")

	// Verify roles table exists
	err = suite.DB.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'roles'
		)
	`).Scan(&exists).Error
	require.NoError(t, err, "Should be able to query information_schema")
	require.True(t, exists, "roles table should exist")

	t.Log("roles table exists")

	// Verify permissions table exists
	err = suite.DB.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'permissions'
		)
	`).Scan(&exists).Error
	require.NoError(t, err, "Should be able to query information_schema")
	require.True(t, exists, "permissions table should exist")

	t.Log("permissions table exists")

	// Verify audit_logs table exists
	err = suite.DB.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'audit_logs'
		)
	`).Scan(&exists).Error
	require.NoError(t, err, "Should be able to query information_schema")
	require.True(t, exists, "audit_logs table should exist")

	t.Log("audit_logs table exists")
}

// TestInfrastructure_TestIsolation validates that tests are properly isolated.
// Each test should start with clean data.
func TestInfrastructure_TestIsolation(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	// Insert a test user
	result := suite.DB.Exec(`
		INSERT INTO users (email, password_hash) 
		VALUES ('test@example.com', 'hash123')
	`)
	require.NoError(t, result.Error, "Should be able to insert user")

	// Verify user exists
	var count int64
	err := suite.DB.Raw("SELECT COUNT(*) FROM users WHERE email = 'test@example.com'").Scan(&count).Error
	require.NoError(t, err, "Should be able to count users")
	require.Equal(t, int64(1), count, "User should exist")

	t.Log("User inserted successfully")

	// Clean up for next test (this is what SetupTest does automatically)
	suite.SetupTest(t)

	// Verify user was cleaned
	err = suite.DB.Raw("SELECT COUNT(*) FROM users WHERE email = 'test@example.com'").Scan(&count).Error
	require.NoError(t, err, "Should be able to count users")
	require.Equal(t, int64(0), count, "User should be cleaned after SetupTest")

	t.Log("Test isolation works correctly")
}

// TestInfrastructure_RedisOperations validates basic Redis operations.
func TestInfrastructure_RedisOperations(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	// Test SET operation
	err := suite.RedisClient.Set(ctx, "test_key", "test_value", 10*time.Second).Err()
	require.NoError(t, err, "Should be able to SET key in Redis")

	t.Log("Redis SET operation successful")

	// Test GET operation
	val, err := suite.RedisClient.Get(ctx, "test_key").Result()
	require.NoError(t, err, "Should be able to GET key from Redis")
	require.Equal(t, "test_value", val, "Redis value should match")

	t.Log("Redis GET operation successful")

	// Test DEL operation
	err = suite.RedisClient.Del(ctx, "test_key").Err()
	require.NoError(t, err, "Should be able to DEL key from Redis")

	t.Log("Redis DEL operation successful")

	// Verify key was deleted
	err = suite.RedisClient.Get(ctx, "test_key").Err()
	require.Error(t, err, "Key should not exist after deletion")

	t.Log("Redis cleanup successful")
}

// TestInfrastructure_ContainerCleanup validates that cleanup function works.
// This test should not leak containers if successful.
func TestInfrastructure_ContainerCleanup(t *testing.T) {
	// Create a fresh test suite to test cleanup
	suite := NewTestSuite(t)

	// Verify it works
	var result int
	err := suite.DB.Raw("SELECT 1").Scan(&result).Error
	require.NoError(t, err, "PostgreSQL connection should work")

	t.Log("Created fresh test suite")

	// Clean up
	suite.Cleanup()

	t.Log("Cleanup executed successfully")
}
//go:build integration
// +build integration

// Package integration provides integration test infrastructure.
// Tests connect to local PostgreSQL and Redis (no Docker required).
package integration

import (
	"fmt"
	"os"
	"testing"
)

// TestMain is the entry point for all integration tests.
// It runs all tests and cleans up the shared suite afterward.
//
// Usage:
//
//	go test ./tests/integration/... -tags=integration
//
// The -tags=integration build tag ensures these tests only run when explicitly requested.
func TestMain(m *testing.M) {
	// Run all tests
	code := m.Run()

	// Cleanup shared suite after all tests complete
	if testSuite != nil {
		if testSuite.RedisClient != nil {
			if err := testSuite.RedisClient.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close Redis client: %v\n", err)
			}
		}
		if testSuite.DB != nil {
			sqlDB, err := testSuite.DB.DB()
			if err == nil {
				sqlDB.Close()
			}
		}
	}

	// Exit with the test result code
	os.Exit(code)
}

// GetTestSuite returns the global test suite instance.
// This allows individual tests to access the shared database and Redis connections.
//
// Usage:
//
//	func TestMyFeature(t *testing.T) {
//	    suite := GetTestSuite(t)
//	    suite.SetupTest(t) // Clean before test
//	    // use suite.DB and suite.RedisClient
//	}
func GetTestSuite(t *testing.T) *TestSuite {
	if testSuite == nil {
		t.Fatal("Test suite not initialized. Ensure TestMain runs first.")
	}
	return testSuite
}

// SetupIntegrationTest prepares the test environment before each test.
// It initializes the test suite on first call and cleans the database and Redis on subsequent calls.
//
// Usage:
//
//	func TestMyFeature(t *testing.T) {
//	    suite := SetupIntegrationTest(t)
//	    defer suite.TeardownTest(t)
//	    // test code here
//	}
func SetupIntegrationTest(t *testing.T) *TestSuite {
	return NewTestSuite(t)
}

// TeardownTest performs cleanup after each test.
// Currently a no-op as SetupTest handles cleaning, but provides extensibility.
func (s *TestSuite) TeardownTest(t *testing.T) {
	// No-op currently - SetupTest handles the cleaning
	// This method exists for future extensibility (e.g., capturing logs, metrics)
}
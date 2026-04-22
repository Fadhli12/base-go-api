//go:build integration
// +build integration

// Package integration provides integration test infrastructure using testcontainers.
package integration

import (
	"os"
	"testing"
)

// Global test suite shared across all integration tests
var testSuite *TestSuite

// TestMain is the entry point for all integration tests.
// It sets up containers once before all tests and tears them down after all tests complete.
//
// Usage:
//
//	go test ./tests/integration/... -tags=integration
//
// The -tags=integration build tag ensures these tests only run when explicitly requested.
func TestMain(m *testing.M) {
	// Initialize test suite with PostgreSQL and Redis containers
	// Note: We cannot create a *testing.T here, so initialization happens lazily
	// in the first test that calls SetupIntegrationTest

	// Run all tests
	code := m.Run()

	// Cleanup containers after all tests complete
	if testSuite != nil && testSuite.Cleanup != nil {
		testSuite.Cleanup()
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
	// Initialize on first call
	if testSuite == nil {
		testSuite = NewTestSuite(t)
		testSuite.RunMigrations(t)
	}

	// Clean before each test
	testSuite.SetupTest(t)
	return testSuite
}

// TeardownTest performs cleanup after each test.
// Currently a no-op as SetupTest handles cleaning, but provides extensibility.
func (s *TestSuite) TeardownTest(t *testing.T) {
	// No-op currently - SetupTest handles the cleaning
	// This method exists for future extensibility (e.g., capturing logs, metrics)
}
//go:build integration

package integration

import (
	"testing"

	"github.com/example/go-api-base/internal/permission"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermission_Enforce_UserHasPermission(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	// Setup enforcer
	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add policy: role "manager" can "create" on "invoice" in domain "default"
	err = enforcer.AddPolicy("manager", "default", "invoice", "create")
	require.NoError(t, err)

	// Assign user to role
	err = enforcer.AddRoleForUser(userID.String(), "manager", "default")
	require.NoError(t, err)

	// Test enforcement
	allowed, err := enforcer.Enforce(userID.String(), "default", "invoice", "create")
	require.NoError(t, err)
	assert.True(t, allowed, "User with role that has permission should be allowed")
}

func TestPermission_Enforce_UserNoPermission(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Test enforcement for user with no permissions
	allowed, err := enforcer.Enforce(userID.String(), "default", "invoice", "create")
	require.NoError(t, err)
	assert.False(t, allowed, "User without permission should be denied")
}

func TestPermission_Enforce_AfterRoleRemoved(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add policy and role via enforcer
	err = enforcer.AddPolicy("tester", "default", "test", "action")
	require.NoError(t, err)
	err = enforcer.AddRoleForUser(userID.String(), "tester", "default")
	require.NoError(t, err)

	// User should have permission
	allowed, err := enforcer.Enforce(userID.String(), "default", "test", "action")
	require.NoError(t, err)
	assert.True(t, allowed, "User should have permission")

	// Remove role from user
	err = enforcer.RemoveRoleForUser(userID.String(), "tester", "default")
	require.NoError(t, err)

	// User should no longer have permission
	allowed, err = enforcer.Enforce(userID.String(), "default", "test", "action")
	require.NoError(t, err)
	assert.False(t, allowed, "User should not have permission after role removal")
}

func TestPermission_Cache_Layer(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	// Create test user with permission via enforcer API
	userID := uuid.New()

	err = enforcer.AddPolicy("cache-role", "default", "cache", "test")
	require.NoError(t, err)
	err = enforcer.AddRoleForUser(userID.String(), "cache-role", "default")
	require.NoError(t, err)

	// First check - should query Casbin
	allowed, err := enforcer.Enforce(userID.String(), "default", "cache", "test")
	require.NoError(t, err)
	assert.True(t, allowed)

	// TODO: Add cache layer tests when cache is integrated
	// The cache layer (internal/permission/cache.go) should:
	// 1. Cache miss → queries Casbin
	// 2. Cache hit → returns cached value
	// 3. Invalidation clears cache
}

func TestPermission_Domain_BasedEnforcement(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userA := uuid.New()
	userB := uuid.New()

	// Add policy: admin role can view invoices in domain "default"
	err = enforcer.AddPolicy("admin", "default", "invoice", "view")
	require.NoError(t, err)

	// Assign userA to admin role
	err = enforcer.AddRoleForUser(userA.String(), "admin", "default")
	require.NoError(t, err)

	// Admin user should be able to view all invoices
	allowed, err := enforcer.Enforce(userA.String(), "default", "invoice", "view")
	require.NoError(t, err)
	assert.True(t, allowed, "Admin with 'all' scope should be able to view any invoice")

	// Regular user should not have permission
	allowed, err = enforcer.Enforce(userB.String(), "default", "invoice", "view")
	require.NoError(t, err)
	assert.False(t, allowed, "Regular user should not have permission")
}

func TestPermission_AddPolicy(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add policy directly
	err = enforcer.AddPolicy(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)

	// Verify policy works
	allowed, err := enforcer.Enforce(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Verify policy doesn't apply to other actions
	allowed, err = enforcer.Enforce(userID.String(), "default", "resource1", "write")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestPermission_RemovePolicy(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add and verify policy
	err = enforcer.AddPolicy(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)

	allowed, err := enforcer.Enforce(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove policy
	err = enforcer.RemovePolicy(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)

	// Verify policy removed
	allowed, err = enforcer.Enforce(userID.String(), "default", "resource1", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "Policy should be removed")
}

func TestPermission_AddRoleForUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add policy: editor role can edit docs
	err = enforcer.AddPolicy("editor", "default", "doc", "edit")
	require.NoError(t, err)

	// Add role for user via enforcer
	err = enforcer.AddRoleForUser(userID.String(), "editor", "default")
	require.NoError(t, err)

	// Verify user has permission through role
	allowed, err := enforcer.Enforce(userID.String(), "default", "doc", "edit")
	require.NoError(t, err)
	assert.True(t, allowed, "User should have permission through role")
}

func TestPermission_RemoveRoleForUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "NewEnforcer should succeed")

	userID := uuid.New()

	// Add policy and role
	err = enforcer.AddPolicy("deleter", "default", "doc", "delete")
	require.NoError(t, err)
	err = enforcer.AddRoleForUser(userID.String(), "deleter", "default")
	require.NoError(t, err)

	// Verify initial permission
	allowed, err := enforcer.Enforce(userID.String(), "default", "doc", "delete")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove role via enforcer
	err = enforcer.RemoveRoleForUser(userID.String(), "deleter", "default")
	require.NoError(t, err)

	// Verify permission removed
	allowed, err = enforcer.Enforce(userID.String(), "default", "doc", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User should not have permission after role removal")
}
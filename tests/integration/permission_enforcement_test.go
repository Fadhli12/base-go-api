//go:build integration

package integration

import (
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
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

	// Create test user and role
	userID := uuid.New()
	roleID := uuid.New()
	permID := uuid.New()

	// Create permission
	perm := &domain.Permission{
		ID:       permID,
		Name:     "invoice:create",
		Resource: "invoice",
		Action:   "create",
		Scope:    "all",
	}
	err = suite.DB.Create(perm).Error
	require.NoError(t, err)

	// Create role
	role := &domain.Role{
		ID:          roleID,
		Name:        "manager",
		Description: "Manager role",
	}
	err = suite.DB.Create(role).Error
	require.NoError(t, err)

	// Assign permission to role
	err = suite.DB.Create(&domain.RolePermission{
		RoleID:       roleID,
		PermissionID: permID,
	}).Error
	require.NoError(t, err)

	// Assign role to user
	err = suite.DB.Create(&domain.UserRole{
		UserID: userID,
		RoleID: roleID,
	}).Error
	require.NoError(t, err)

	// Load policy into enforcer
	err = enforcer.LoadPolicy()
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
	roleID := uuid.New()
	permID := uuid.New()

	// Create permission, role, and assign to user
	perm := &domain.Permission{ID: permID, Name: "test:action", Resource: "test", Action: "action", Scope: "all"}
	require.NoError(t, suite.DB.Create(perm).Error)

	role := &domain.Role{ID: roleID, Name: "tester"}
	require.NoError(t, suite.DB.Create(role).Error)

	require.NoError(t, suite.DB.Create(&domain.RolePermission{RoleID: roleID, PermissionID: permID}).Error)
	require.NoError(t, suite.DB.Create(&domain.UserRole{UserID: userID, RoleID: roleID}).Error)

	require.NoError(t, enforcer.LoadPolicy())

	// User should have permission
	allowed, err := enforcer.Enforce(userID.String(), "default", "test", "action")
	require.NoError(t, err)
	assert.True(t, allowed, "User should have permission")

	// Remove role from user
	require.NoError(t, suite.DB.Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&domain.UserRole{}).Error)

	// Reload policy
	require.NoError(t, enforcer.LoadPolicy())

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

	// Create test user with permission
	userID := uuid.New()
	roleID := uuid.New()
	permID := uuid.New()

	perm := &domain.Permission{ID: permID, Name: "cache:test", Resource: "cache", Action: "test", Scope: "all"}
	require.NoError(t, suite.DB.Create(perm).Error)

	role := &domain.Role{ID: roleID, Name: "cache-role"}
	require.NoError(t, suite.DB.Create(role).Error)

	require.NoError(t, suite.DB.Create(&domain.RolePermission{RoleID: roleID, PermissionID: permID}).Error)
	require.NoError(t, suite.DB.Create(&domain.UserRole{UserID: userID, RoleID: roleID}).Error)

	require.NoError(t, enforcer.LoadPolicy())

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
	adminRoleID := uuid.New()

	// Create admin role with all scope
	permAll := &domain.Permission{ID: uuid.New(), Name: "invoice:view:all", Resource: "invoice", Action: "view", Scope: "all"}
	require.NoError(t, suite.DB.Create(permAll).Error)

	adminRole := &domain.Role{ID: adminRoleID, Name: "admin"}
	require.NoError(t, suite.DB.Create(adminRole).Error)

	require.NoError(t, suite.DB.Create(&domain.RolePermission{RoleID: adminRoleID, PermissionID: permAll.ID}).Error)
	require.NoError(t, suite.DB.Create(&domain.UserRole{UserID: userA, RoleID: adminRoleID}).Error)

	require.NoError(t, enforcer.LoadPolicy())

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
	roleID := uuid.New()
	permID := uuid.New()

	// Create permission and role in DB
	perm := &domain.Permission{ID: permID, Name: "doc:edit", Resource: "doc", Action: "edit", Scope: "all"}
	require.NoError(t, suite.DB.Create(perm).Error)

	role := &domain.Role{ID: roleID, Name: "editor"}
	require.NoError(t, suite.DB.Create(role).Error)

	require.NoError(t, suite.DB.Create(&domain.RolePermission{RoleID: roleID, PermissionID: permID}).Error)

	// Add role for user via enforcer
	err = enforcer.AddRoleForUser(userID.String(), roleID.String(), "default")
	require.NoError(t, err)

	require.NoError(t, enforcer.LoadPolicy())

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
	roleID := uuid.New()
	permID := uuid.New()

	// Setup
	perm := &domain.Permission{ID: permID, Name: "doc:delete", Resource: "doc", Action: "delete", Scope: "all"}
	require.NoError(t, suite.DB.Create(perm).Error)

	role := &domain.Role{ID: roleID, Name: "deleter"}
	require.NoError(t, suite.DB.Create(role).Error)

	require.NoError(t, suite.DB.Create(&domain.RolePermission{RoleID: roleID, PermissionID: permID}).Error)
	require.NoError(t, suite.DB.Create(&domain.UserRole{UserID: userID, RoleID: roleID}).Error)

	require.NoError(t, enforcer.LoadPolicy())

	// Verify initial permission
	allowed, err := enforcer.Enforce(userID.String(), "default", "doc", "delete")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove role via enforcer
	err = enforcer.RemoveRoleForUser(userID.String(), roleID.String(), "default")
	require.NoError(t, err)

	require.NoError(t, enforcer.LoadPolicy())

	// Verify permission removed
	allowed, err = enforcer.Enforce(userID.String(), "default", "doc", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User should not have permission after role removal")
}
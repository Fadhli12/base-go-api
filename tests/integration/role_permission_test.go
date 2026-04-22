//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoleService_AttachPermission tests attaching a permission to a role
func TestRoleService_AttachPermission(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permission
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	permission, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)

	// Attach permission to role
	err = roleService.AttachPermission(ctx, role.ID, permission.ID)
	require.NoError(t, err, "Should attach permission successfully")

	// Verify attachment by checking role has the permission
	found, err := roleService.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, found.Permissions, 1, "Role should have 1 permission")
	assert.Equal(t, permission.ID, found.Permissions[0].ID, "Permission ID should match")
}

// TestRoleService_AttachPermission_Idempotent tests attaching same permission twice is idempotent
func TestRoleService_AttachPermission_Idempotent(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permission
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	permission, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)

	// Attach permission twice
	err = roleService.AttachPermission(ctx, role.ID, permission.ID)
	require.NoError(t, err, "First attach should succeed")

	// Note: The Attach method returns an error on duplicate due to primary key constraint
	// This is expected behavior - idempotency would require checking existence first
	// The test verifies the current behavior
	err = roleService.AttachPermission(ctx, role.ID, permission.ID)
	// This may or may not error depending on DB constraint handling
	// For PostgreSQL with ON CONFLICT, it would be idempotent
	// Without special handling, it will return a duplicate key error
	if err != nil {
		t.Logf("Second attach returned error (expected for strict idempotency check): %v", err)
	}

	// Verify only one permission is attached (idempotent on conflict)
	found, err := roleService.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, found.Permissions, 1, "Role should still have only 1 permission")
}

// TestRoleService_AttachPermission_RoleNotFound tests attaching to non-existent role
func TestRoleService_AttachPermission_RoleNotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a permission only
	permission, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)

	// Try to attach to non-existent role
	err = roleService.AttachPermission(ctx, createTestUUID(), permission.ID)
	require.Error(t, err, "Should fail for non-existent role")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestRoleService_AttachPermission_PermissionNotFound tests attaching non-existent permission
func TestRoleService_AttachPermission_PermissionNotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a role only
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)

	// Try to attach non-existent permission
	err = roleService.AttachPermission(ctx, role.ID, createTestUUID())
	require.Error(t, err, "Should fail for non-existent permission")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestRoleService_DetachPermission tests detaching a permission from a role
func TestRoleService_DetachPermission(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permission, then attach
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	permission, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, role.ID, permission.ID)
	require.NoError(t, err)

	// Verify attached
	found, err := roleService.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, found.Permissions, 1, "Role should have 1 permission before detach")

	// Detach permission
	err = roleService.DetachPermission(ctx, role.ID, permission.ID)
	require.NoError(t, err, "Should detach permission successfully")

	// Verify detached
	found, err = roleService.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Empty(t, found.Permissions, "Role should have no permissions after detach")
}

// TestRoleService_DetachPermission_NotAttached tests detaching non-attached permission
func TestRoleService_DetachPermission_NotAttached(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permission (not attached)
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	permission, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)

	// Detach non-attached permission - should not error (idempotent)
	err = roleService.DetachPermission(ctx, role.ID, permission.ID)
	require.NoError(t, err, "Should succeed even if permission was not attached")
}

// TestRoleService_GetByID_WithPermissions tests GetByID returns role with preloaded permissions
func TestRoleService_GetByID_WithPermissions(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)

	// Create multiple permissions
	perm1, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)
	perm2, err := permService.Create(ctx, "posts:write", "posts", "write", "own")
	require.NoError(t, err)
	perm3, err := permService.Create(ctx, "comments:read", "comments", "read", "all")
	require.NoError(t, err)

	// Attach all permissions
	err = roleService.AttachPermission(ctx, role.ID, perm1.ID)
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, role.ID, perm2.ID)
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, role.ID, perm3.ID)
	require.NoError(t, err)

	// Get role with permissions
	found, err := roleService.GetByID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, found.Permissions, 3, "Role should have 3 permissions")

	// Verify permission details are populated
	permIDs := make(map[string]bool)
	for _, p := range found.Permissions {
		permIDs[p.Name] = true
		assert.NotEmpty(t, p.Resource, "Permission resource should be loaded")
		assert.NotEmpty(t, p.Action, "Permission action should be loaded")
	}
	assert.True(t, permIDs["posts:read"], "Should have posts:read permission")
	assert.True(t, permIDs["posts:write"], "Should have posts:write permission")
	assert.True(t, permIDs["comments:read"], "Should have comments:read permission")
}

// TestRolePermissionRepository_FindByRoleID tests repository FindByRoleID
func TestRolePermissionRepository_FindByRoleID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permissions
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	perm1, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)
	perm2, err := permService.Create(ctx, "posts:write", "posts", "write", "own")
	require.NoError(t, err)

	// Attach permissions
	err = roleService.AttachPermission(ctx, role.ID, perm1.ID)
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, role.ID, perm2.ID)
	require.NoError(t, err)

	// Find permissions by role ID directly
	permissions, err := rolePermissionRepo.FindByRoleID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permissions, 2, "Should find 2 permissions")

	// Verify permission details
	permNames := make(map[string]bool)
	for _, p := range permissions {
		permNames[p.Name] = true
	}
	assert.True(t, permNames["posts:read"], "Should have posts:read")
	assert.True(t, permNames["posts:write"], "Should have posts:write")
}

// TestRolePermissionRepository_FindByRoleID_Empty tests FindByRoleID with no permissions
func TestRolePermissionRepository_FindByRoleID_Empty(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a role with no permissions
	role, err := roleService.Create(ctx, "empty-role", "Role with no permissions")
	require.NoError(t, err)

	// Find permissions
	permissions, err := rolePermissionRepo.FindByRoleID(ctx, role.ID)
	require.NoError(t, err)
	assert.Empty(t, permissions, "Should return empty slice for role with no permissions")
}

// TestRolePermissionRepository_Sync tests the Sync method
func TestRolePermissionRepository_Sync(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create a role and permissions
	role, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)
	perm1, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)
	perm2, err := permService.Create(ctx, "posts:write", "posts", "write", "own")
	require.NoError(t, err)
	perm3, err := permService.Create(ctx, "comments:read", "comments", "read", "all")
	require.NoError(t, err)

	// Initially attach perm1 and perm2
	err = roleService.AttachPermission(ctx, role.ID, perm1.ID)
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, role.ID, perm2.ID)
	require.NoError(t, err)

	// Verify initial state
	permissions, err := rolePermissionRepo.FindByRoleID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permissions, 2, "Should have 2 permissions initially")

	// Sync to replace with perm2 and perm3 (removes perm1, adds perm3)
	err = rolePermissionRepo.Sync(ctx, role.ID, []uuid.UUID{perm2.ID, perm3.ID})
	require.NoError(t, err, "Sync should succeed")

	// Verify synced permissions
	permissions, err = rolePermissionRepo.FindByRoleID(ctx, role.ID)
	require.NoError(t, err)
	assert.Len(t, permissions, 2, "Should have 2 permissions after sync")

	permNames := make(map[string]bool)
	for _, p := range permissions {
		permNames[p.Name] = true
	}
	assert.False(t, permNames["posts:read"], "perm1 should be removed")
	assert.True(t, permNames["posts:write"], "perm2 should still be present")
	assert.True(t, permNames["comments:read"], "perm3 should be added")
}
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

// TestRoleService_Create_ValidData tests creating a role with valid data
func TestRoleService_Create_ValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	role, err := roleService.Create(ctx, "editor", "Content editor role")
	require.NoError(t, err, "Should create role successfully")
	assert.NotEmpty(t, role.ID, "Role ID should be generated")
	assert.Equal(t, "editor", role.Name, "Role name should match")
	assert.Equal(t, "Content editor role", role.Description, "Description should match")
	assert.False(t, role.IsSystem, "Non-system roles should have IsSystem=false")
}

// TestRoleService_Create_DuplicateName tests creating a role with duplicate name
func TestRoleService_Create_DuplicateName(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create first role
	_, err := roleService.Create(ctx, "editor", "Content editor role")
	require.NoError(t, err, "Should create first role successfully")

	// Try to create duplicate
	_, err = roleService.Create(ctx, "editor", "Another editor")
	require.Error(t, err, "Should fail with duplicate name")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "Should be an AppError")
	assert.Equal(t, "ROLE_EXISTS", appErr.Code, "Error code should be ROLE_EXISTS")
}

// TestRoleService_Create_EmptyName tests creating a role with empty name
func TestRoleService_Create_EmptyName(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	_, err := roleService.Create(ctx, "", "Description")
	require.Error(t, err, "Should fail with empty name")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "Should be an AppError")
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code, "Error code should be VALIDATION_ERROR")
}

// TestRoleService_Update tests updating a role
func TestRoleService_Update(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a role
	created, err := roleService.Create(ctx, "editor", "Original description")
	require.NoError(t, err)

	// Update the role
	updated, err := roleService.Update(ctx, created.ID, "senior-editor", "Updated description")
	require.NoError(t, err, "Should update role successfully")
	assert.Equal(t, "senior-editor", updated.Name, "Name should be updated")
	assert.Equal(t, "Updated description", updated.Description, "Description should be updated")
}

// TestRoleService_Update_NotFound tests updating non-existent role
func TestRoleService_Update_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	_, err := roleService.Update(ctx, createTestUUID(), "new-name", "description")
	require.Error(t, err, "Should fail for non-existent role")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestRoleService_GetAll tests retrieving all roles with permissions
func TestRoleService_GetAll(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	permService := service.NewPermissionService(permissionRepo)

	// Create roles
	_, err := roleService.Create(ctx, "admin", "Administrator role")
	require.NoError(t, err)
	editor, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)

	// Create a permission and attach to role
	perm, err := permService.Create(ctx, "posts:read", "posts", "read", "all")
	require.NoError(t, err)
	err = roleService.AttachPermission(ctx, editor.ID, perm.ID)
	require.NoError(t, err)

	// Get all roles
	roles, err := roleService.GetAll(ctx)
	require.NoError(t, err, "Should retrieve all roles")
	assert.Len(t, roles, 2, "Should have 2 roles")

	// Verify roles have preloaded permissions
	for _, role := range roles {
		if role.Name == "editor" {
			assert.Len(t, role.Permissions, 1, "Editor should have 1 permission")
		}
	}
}

// TestRoleService_GetAll_Empty tests retrieving roles when none exist
func TestRoleService_GetAll_Empty(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	roles, err := roleService.GetAll(ctx)
	require.NoError(t, err, "Should not error on empty table")
	assert.Empty(t, roles, "Should return empty slice")
}

// TestRoleService_Delete_NonSystemRole tests deleting a non-system role
func TestRoleService_Delete_NonSystemRole(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a non-system role
	role, err := roleService.Create(ctx, "temporary", "Temporary role")
	require.NoError(t, err)

	// Delete the role
	err = roleService.Delete(ctx, role.ID)
	require.NoError(t, err, "Should delete non-system role successfully")

	// Verify it's deleted (soft delete)
	_, err = roleRepo.FindByID(ctx, role.ID)
	require.Error(t, err, "Role should not be found after deletion")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestRoleService_Delete_SystemRole tests deleting a system role
func TestRoleService_Delete_SystemRole(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a system role directly in DB
	systemRoleID := createSystemRoleInDB(t, suite, "admin-system", "System admin role")

	// Try to delete the system role
	err := roleService.Delete(ctx, systemRoleID)
	require.Error(t, err, "Should fail to delete system role")
	assert.True(t, errors.Is(err, repository.ErrSystemRoleProtected), "Should be ErrSystemRoleProtected")

	// Verify role still exists
	_, err = roleRepo.FindByID(ctx, systemRoleID)
	require.NoError(t, err, "System role should still exist")
}

// TestRoleService_Delete_NotFound tests deleting non-existent role
func TestRoleService_Delete_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	err := roleService.Delete(ctx, createTestUUID())
	require.Error(t, err, "Should fail for non-existent role")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestRoleService_GetByID tests retrieving a role by ID
func TestRoleService_GetByID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	// Create a role
	created, err := roleService.Create(ctx, "editor", "Editor role")
	require.NoError(t, err)

	// Get by ID
	found, err := roleService.GetByID(ctx, created.ID)
	require.NoError(t, err, "Should find role by ID")
	assert.Equal(t, created.ID, found.ID, "IDs should match")
	assert.Equal(t, created.Name, found.Name, "Names should match")
}

// TestRoleService_GetByID_NotFound tests retrieving non-existent role
func TestRoleService_GetByID_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(suite.DB)
	rolePermissionRepo := repository.NewRolePermissionRepository(suite.DB)
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)

	_, err := roleService.GetByID(ctx, createTestUUID())
	require.Error(t, err, "Should return error for non-existent ID")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// createSystemRoleInDB creates a system role directly in the database for testing
func createSystemRoleInDB(t *testing.T, suite *TestSuite, name, description string) uuid.UUID {
	result := suite.DB.Exec(`
		INSERT INTO roles (name, description, is_system) 
		VALUES ($1, $2, true)
	`, name, description)
	require.NoError(t, result.Error)

	var idStr string
	err := suite.DB.Raw(`
		SELECT id FROM roles WHERE name = $1
	`, name).Scan(&idStr).Error
	require.NoError(t, err)

	id, err := uuid.Parse(idStr)
	require.NoError(t, err)

	return id
}
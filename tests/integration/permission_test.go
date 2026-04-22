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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionService_Create_ValidData tests creating a permission with valid data
func TestPermissionService_Create_ValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	permission, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err, "Should create permission successfully")
	assert.NotEmpty(t, permission.ID, "Permission ID should be generated")
	assert.Equal(t, "users:read", permission.Name, "Permission name should match")
	assert.Equal(t, "users", permission.Resource, "Resource should match")
	assert.Equal(t, "read", permission.Action, "Action should match")
	assert.Equal(t, "all", permission.Scope, "Scope should default to 'all'")
}

// TestPermissionService_Create_DuplicateName tests creating a permission with duplicate name
func TestPermissionService_Create_DuplicateName(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	// Create first permission
	_, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err, "Should create first permission successfully")

	// Try to create duplicate
	_, err = permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.Error(t, err, "Should fail with duplicate name")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "Should be an AppError")
	assert.Equal(t, "PERMISSION_EXISTS", appErr.Code, "Error code should be PERMISSION_EXISTS")
}

// TestPermissionService_Create_ValidationError tests validation errors
func TestPermissionService_Create_ValidationError(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	tests := []struct {
		name     string
		permName string
		resource string
		action   string
		scope    string
		wantErr  string
	}{
		{
			name:     "empty name",
			permName: "",
			resource: "users",
			action:   "read",
			scope:    "all",
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name:     "empty resource",
			permName: "test:read",
			resource: "",
			action:   "read",
			scope:    "all",
			wantErr:  "VALIDATION_ERROR",
		},
		{
			name:     "empty action",
			permName: "test:read",
			resource: "test",
			action:   "",
			scope:    "all",
			wantErr:  "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := permissionService.Create(ctx, tt.permName, tt.resource, tt.action, tt.scope)
			require.Error(t, err, "Should fail validation")

			var appErr *apperrors.AppError
			require.True(t, errors.As(err, &appErr), "Should be an AppError")
			assert.Equal(t, tt.wantErr, appErr.Code, "Error code should match")
		})
	}
}

// TestPermissionService_GetAll tests retrieving all permissions
func TestPermissionService_GetAll(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	// Create multiple permissions
	_, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err)
	_, err = permissionService.Create(ctx, "users:write", "users", "write", "own")
	require.NoError(t, err)
	_, err = permissionService.Create(ctx, "invoices:read", "invoices", "read", "all")
	require.NoError(t, err)

	// Get all permissions
	permissions, err := permissionService.GetAll(ctx)
	require.NoError(t, err, "Should retrieve all permissions")
	assert.Len(t, permissions, 3, "Should have 3 permissions")
}

// TestPermissionService_GetAll_Empty tests retrieving permissions when none exist
func TestPermissionService_GetAll_Empty(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	permissions, err := permissionService.GetAll(ctx)
	require.NoError(t, err, "Should not error on empty table")
	assert.Empty(t, permissions, "Should return empty slice")
}

// TestPermissionRepository_FindByID tests finding a permission by ID
func TestPermissionRepository_FindByID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	// Create a permission
	created, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err)

	// Find by ID
	found, err := permissionRepo.FindByID(ctx, created.ID)
	require.NoError(t, err, "Should find permission by ID")
	assert.Equal(t, created.ID, found.ID, "IDs should match")
	assert.Equal(t, created.Name, found.Name, "Names should match")
}

// TestPermissionRepository_FindByID_NotFound tests finding non-existent permission
func TestPermissionRepository_FindByID_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)

	_, err := permissionRepo.FindByID(ctx, createTestUUID())
	require.Error(t, err, "Should return error for non-existent ID")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestPermissionRepository_FindByName tests finding a permission by name
func TestPermissionRepository_FindByName(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	// Create a permission
	_, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err)

	// Find by name
	found, err := permissionRepo.FindByName(ctx, "users:read")
	require.NoError(t, err, "Should find permission by name")
	assert.Equal(t, "users:read", found.Name, "Name should match")
}

// TestPermissionRepository_FindByName_NotFound tests finding non-existent permission by name
func TestPermissionRepository_FindByName_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)

	_, err := permissionRepo.FindByName(ctx, "nonexistent")
	require.Error(t, err, "Should return error for non-existent name")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestPermissionRepository_FindByResource tests finding permissions by resource
func TestPermissionRepository_FindByResource(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)
	permissionService := service.NewPermissionService(permissionRepo)

	// Create permissions for different resources
	_, err := permissionService.Create(ctx, "users:read", "users", "read", "all")
	require.NoError(t, err)
	_, err = permissionService.Create(ctx, "users:write", "users", "write", "own")
	require.NoError(t, err)
	_, err = permissionService.Create(ctx, "invoices:read", "invoices", "read", "all")
	require.NoError(t, err)

	// Find by resource
	permissions, err := permissionRepo.FindByResource(ctx, "users")
	require.NoError(t, err, "Should find permissions by resource")
	assert.Len(t, permissions, 2, "Should find 2 user permissions")

	// Verify all returned permissions are for 'users' resource
	for _, p := range permissions {
		assert.Equal(t, "users", p.Resource, "Resource should be 'users'")
	}
}

// TestPermissionRepository_FindByResource_Empty tests finding permissions for non-existent resource
func TestPermissionRepository_FindByResource_Empty(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	permissionRepo := repository.NewPermissionRepository(suite.DB)

	permissions, err := permissionRepo.FindByResource(ctx, "nonexistent")
	require.NoError(t, err, "Should not error for non-existent resource")
	assert.Empty(t, permissions, "Should return empty slice")
}
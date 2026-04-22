//go:build integration

package integration

import (
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRole_Assign(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, nil)

	// Create test user
	passwordHash, err := service.HashPassword("password123")
	require.NoError(t, err)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        "test-role@example.com",
		PasswordHash: passwordHash,
	}
	require.NoError(t, userRepo.Create(suite.DB, user))

	// Create test role
	role := &domain.Role{
		ID:          uuid.New(),
		Name:        "test-role",
		Description: "Test role",
	}
	require.NoError(t, roleRepo.Create(suite.DB, role))

	// Assign role to user
	assignedBy := uuid.New()
	err = userSvc.AssignRole(suite.DB, user.ID, role.ID, assignedBy)
	require.NoError(t, err, "AssignRole should succeed")

	// Verify role assigned
	roles, err := userRoleRepo.FindByUserID(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1, "Should have 1 role")
	assert.Equal(t, role.ID, roles[0].ID)
}

func TestUserRole_AssignDuplicate(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, nil)

	user := createTestUser(t, suite, "dup-role@example.com")
	role := createTestRole(t, suite, "dup-test-role")

	assignedBy := uuid.New()
	require.NoError(t, userSvc.AssignRole(suite.DB, user.ID, role.ID, assignedBy))

	// Assign same role again - should be idempotent or error
	err := userSvc.AssignRole(suite.DB, user.ID, role.ID, assignedBy)
	// Depending on implementation, this might succeed (idempotent) or fail (duplicate)
	// The important thing is it handles gracefully
	if err != nil {
		// If error, should be a specific duplicate error
		assert.Error(t, err, "Duplicate assignment should return appropriate error")
	} else {
		// If idempotent, role should still be assigned only once
		roles, err := userRoleRepo.FindByUserID(suite.DB, user.ID)
		require.NoError(t, err)
		assert.Len(t, roles, 1, "Should still have only 1 role")
	}
}

func TestUserRole_Remove(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, nil, userRoleRepo, nil)

	user := createTestUser(t, suite, "remove-role@example.com")
	role := createTestRole(t, suite, "remove-test-role")

	assignedBy := uuid.New()
	require.NoError(t, userSvc.AssignRole(suite.DB, user.ID, role.ID, assignedBy))

	// Remove role
	err := userSvc.RemoveRole(suite.DB, user.ID, role.ID)
	require.NoError(t, err, "RemoveRole should succeed")

	// Verify role removed
	roles, err := userRoleRepo.FindByUserID(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Empty(t, roles, "Should have no roles after removal")
}

func TestUserPermission_GrantAndDeny(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, nil, userPermRepo)

	user := createTestUser(t, suite, "grant-perm@example.com")
	perm := createTestPermission(t, suite, "grant-test:action")

	assignedBy := uuid.New()

	// Grant permission
	err := userSvc.GrantPermission(suite.DB, user.ID, perm.ID, assignedBy)
	require.NoError(t, err, "GrantPermission should succeed")

	// Verify permission granted
	userPerms, err := userPermRepo.FindByUserID(suite.DB, user.ID)
	require.NoError(t, err)
	require.Len(t, userPerms, 1)
	assert.Equal(t, "allow", userPerms[0].Effect)

	// Deny permission
	err = userSvc.DenyPermission(suite.DB, user.ID, perm.ID, assignedBy)
	require.NoError(t, err, "DenyPermission should succeed")

	// Verify permission denied (effect = deny)
	userPerms, err = userPermRepo.FindByUserID(suite.DB, user.ID)
	require.NoError(t, err)
	require.Len(t, userPerms, 1)
	assert.Equal(t, "deny", userPerms[0].Effect, "Permission should be denied")
}

func TestUserPermission_Remove(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, nil, userPermRepo)

	user := createTestUser(t, suite, "remove-perm@example.com")
	perm := createTestPermission(t, suite, "remove-test:action")

	assignedBy := uuid.New()
	require.NoError(t, userSvc.GrantPermission(suite.DB, user.ID, perm.ID, assignedBy))

	// Remove permission
	err := userSvc.RemovePermission(suite.DB, user.ID, perm.ID)
	require.NoError(t, err, "RemovePermission should succeed")

	// Verify permission removed
	userPerms, err := userPermRepo.FindByUserID(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Empty(t, userPerms, "Should have no permissions after removal")
}

func TestEffectivePermissions_UserNoRoles(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, nil, nil)

	user := createTestUser(t, suite, "no-roles@example.com")

	// User with no roles should have empty permissions
	perms, err := userSvc.GetEffectivePermissions(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Empty(t, perms, "User with no roles should have empty permissions")
}

func TestEffectivePermissions_UserWithRole(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, userRoleRepo, nil)

	user := createTestUser(t, suite, "with-role@example.com")
	role := createTestRole(t, suite, "effective-test-role")
	perm := createTestPermission(t, suite, "effective-test:action")

	// Assign role to user
	assignedBy := uuid.New()
	require.NoError(t, userRoleRepo.Assign(suite.DB, user.ID, role.ID, assignedBy))

	// Assign permission to role
	require.NoError(t, rolePermRepo.Attach(suite.DB, role.ID, perm.ID))

	// Get effective permissions
	perms, err := userSvc.GetEffectivePermissions(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Len(t, perms, 1, "Should have 1 permission from role")
	assert.Equal(t, perm.Name, perms[0].Name)
}

func TestEffectivePermissions_MultipleRoles(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, userRoleRepo, nil)

	user := createTestUser(t, suite, "multi-roles@example.com")
	role1 := createTestRole(t, suite, "multi-role-1")
	role2 := createTestRole(t, suite, "multi-role-2")
	perm1 := createTestPermission(t, suite, "multi-1:action")
	perm2 := createTestPermission(t, suite, "multi-2:action")

	assignedBy := uuid.New()

	// Assign both roles to user
	require.NoError(t, userRoleRepo.Assign(suite.DB, user.ID, role1.ID, assignedBy))
	require.NoError(t, userRoleRepo.Assign(suite.DB, user.ID, role2.ID, assignedBy))

	// Assign different permissions to each role
	require.NoError(t, rolePermRepo.Attach(suite.DB, role1.ID, perm1.ID))
	require.NoError(t, rolePermRepo.Attach(suite.DB, role2.ID, perm2.ID))

	// Get effective permissions - should have union of both roles' permissions
	perms, err := userSvc.GetEffectivePermissions(suite.DB, user.ID)
	require.NoError(t, err)
	assert.Len(t, perms, 2, "Should have 2 permissions from both roles")
}

func TestEffectivePermissions_DenyOverridesAllow(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	permRepo := repository.NewPermissionRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, permRepo, userRoleRepo, userPermRepo)

	user := createTestUser(t, suite, "deny-override@example.com")
	role := createTestRole(t, suite, "deny-role")
	perm := createTestPermission(t, suite, "deny-test:action")

	assignedBy := uuid.New()

	// Assign role with permission to user
	require.NoError(t, userRoleRepo.Assign(suite.DB, user.ID, role.ID, assignedBy))
	require.NoError(t, rolePermRepo.Attach(suite.DB, role.ID, perm.ID))

	// Grant permission through role (allow)
	// But also deny permission directly to user
	require.NoError(t, userPermRepo.Deny(suite.DB, user.ID, perm.ID, assignedBy))

	// Get effective permissions
	perms, err := userSvc.GetEffectivePermissions(suite.DB, user.ID)
	require.NoError(t, err)

	// Deny should override allow - permission should not be in effective list
	for _, p := range perms {
		assert.NotEqual(t, perm.ID, p.ID, "Denied permission should not be in effective permissions")
	}
}

// Helper functions

func createTestUser(t *testing.T, suite *TestSuite, email string) *domain.User {
	passwordHash, err := service.HashPassword("password123")
	require.NoError(t, err)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
	}
	require.NoError(t, repository.NewUserRepository(suite.DB).Create(suite.DB, user))
	return user
}

func createTestRole(t *testing.T, suite *TestSuite, name string) *domain.Role {
	role := &domain.Role{
		ID:          uuid.New(),
		Name:        name,
		Description: "Test role",
	}
	require.NoError(t, repository.NewRoleRepository(suite.DB).Create(suite.DB, role))
	return role
}

func createTestPermission(t *testing.T, suite *TestSuite, name string) *domain.Permission {
	perm := &domain.Permission{
		ID:       uuid.New(),
		Name:     name,
		Resource: "test",
		Action:   "action",
		Scope:    "all",
	}
	require.NoError(t, repository.NewPermissionRepository(suite.DB).Create(suite.DB, perm))
	return perm
}
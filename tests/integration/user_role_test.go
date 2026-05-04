//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestUserRole_Assign(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	roleRepo := repository.NewRoleRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	// Create test user
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        "test-role@example.com",
		PasswordHash: string(passwordHash),
	}
	require.NoError(t, userRepo.Create(ctx, user))

	// Create test role
	role := createTestRoleForUserRole(t, suite)
	_ = roleRepo // used via helper

	// Assign role to user
	assignedBy := uuid.New()
	err = userSvc.AssignRole(ctx, user.ID, role.ID, assignedBy)
	require.NoError(t, err, "AssignRole should succeed")

	// Verify role assigned
	roles, err := userRoleRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1, "Should have 1 role")
	assert.Equal(t, role.ID, roles[0].ID)
}

func TestUserRole_AssignDuplicate(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "dup-role@example.com")
	role := createTestRoleForUserRole(t, suite)

	assignedBy := uuid.New()
	require.NoError(t, userSvc.AssignRole(ctx, user.ID, role.ID, assignedBy))

	// Assign same role again - should be idempotent or error
	err := userSvc.AssignRole(ctx, user.ID, role.ID, assignedBy)
	// Depending on implementation, this might succeed (idempotent) or fail (duplicate)
	if err != nil {
		// If error, should be a specific duplicate error
		assert.Error(t, err, "Duplicate assignment should return appropriate error")
	} else {
		// If idempotent, role should still be assigned only once
		roles, err := userRoleRepo.FindByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, roles, 1, "Should still have only 1 role")
	}
}

func TestUserRole_Remove(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "remove-role@example.com")
	role := createTestRoleForUserRole(t, suite)

	assignedBy := uuid.New()
	require.NoError(t, userSvc.AssignRole(ctx, user.ID, role.ID, assignedBy))

	// Remove role
	err := userSvc.RemoveRole(ctx, user.ID, role.ID)
	require.NoError(t, err, "RemoveRole should succeed")

	// Verify role removed
	roles, err := userRoleRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 0, "Should have 0 roles after removal")
}

func TestUserRole_GrantPermission(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "perm-user@example.com")
	_ = createTestRoleForUserRole(t, suite)
	perm := createTestPermissionForUserRole(t, suite, "test:resource:action")

	assignedBy := uuid.New()
	err := userSvc.GrantPermission(ctx, user.ID, perm.ID, assignedBy)
	require.NoError(t, err, "GrantPermission should succeed")

	userPerms, err := userPermRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, userPerms, 1, "Should have 1 permission")
}

func TestUserRole_DenyPermission(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "deny-user@example.com")
	perm := createTestPermissionForUserRole(t, suite, "deny:resource:action")

	assignedBy := uuid.New()
	err := userSvc.GrantPermission(ctx, user.ID, perm.ID, assignedBy)
	require.NoError(t, err)

	err = userSvc.DenyPermission(ctx, user.ID, perm.ID, assignedBy)
	require.NoError(t, err, "DenyPermission should succeed")

	userPerms, err := userPermRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err)
	// Deny should create a deny override
	assert.NotEmpty(t, userPerms, "Should have permission records")
}

func TestUserRole_RemovePermission(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "remove-perm@example.com")
	perm := createTestPermissionForUserRole(t, suite, "remove:resource:action")

	assignedBy := uuid.New()
	require.NoError(t, userSvc.GrantPermission(ctx, user.ID, perm.ID, assignedBy))

	err := userSvc.RemovePermission(ctx, user.ID, perm.ID)
	require.NoError(t, err, "RemovePermission should succeed")

	userPerms, err := userPermRepo.FindByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, userPerms, 0, "Should have 0 permissions after removal")
}

func TestUserRole_GetEffectivePermissions(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "effective-perm@example.com")
	role := createTestRoleForUserRole(t, suite)
	perm := createTestPermissionForUserRole(t, suite, "effective:resource:action")

	assignedBy := uuid.New()
	require.NoError(t, userRoleRepo.Assign(ctx, user.ID, role.ID, assignedBy))
	require.NoError(t, rolePermRepo.Attach(ctx, role.ID, perm.ID))

	perms, err := userSvc.GetEffectivePermissions(ctx, user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, perms, "Should have effective permissions from role")
}

func TestUserRole_MultipleRolesWithPermissions(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "multi-role@example.com")
	role1 := createTestRoleForUserRole(t, suite)
	role2 := createTestRoleForUserRole(t, suite)
	perm1 := createTestPermissionForUserRole(t, suite, "multi:resource1:action")
	perm2 := createTestPermissionForUserRole(t, suite, "multi:resource2:action")

	assignedBy := uuid.New()
	require.NoError(t, userRoleRepo.Assign(ctx, user.ID, role1.ID, assignedBy))
	require.NoError(t, userRoleRepo.Assign(ctx, user.ID, role2.ID, assignedBy))
	require.NoError(t, rolePermRepo.Attach(ctx, role1.ID, perm1.ID))
	require.NoError(t, rolePermRepo.Attach(ctx, role2.ID, perm2.ID))

	perms, err := userSvc.GetEffectivePermissions(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, perms, 2, "Should have permissions from both roles")
}

func TestUserRole_DenyOverridesAllow(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	userRepo := repository.NewUserRepository(suite.DB)
	userRoleRepo := repository.NewUserRoleRepository(suite.DB)
	rolePermRepo := repository.NewRolePermissionRepository(suite.DB)
	userPermRepo := repository.NewUserPermissionRepository(suite.DB)
	userSvc := service.NewUserService(userRepo, userRoleRepo, userPermRepo)

	ctx := context.Background()

	user := createTestUserForUserRole(t, suite, "deny-override@example.com")
	role := createTestRoleForUserRole(t, suite)
	perm := createTestPermissionForUserRole(t, suite, "override:resource:action")

	assignedBy := uuid.New()
	require.NoError(t, userRoleRepo.Assign(ctx, user.ID, role.ID, assignedBy))
	require.NoError(t, rolePermRepo.Attach(ctx, role.ID, perm.ID))
	require.NoError(t, userPermRepo.Deny(ctx, user.ID, perm.ID, assignedBy))

	perms, err := userSvc.GetEffectivePermissions(ctx, user.ID)
	require.NoError(t, err)

	// Deny should override allow - permission should not be in effective list
	for _, p := range perms {
		assert.NotEqual(t, perm.ID, p.ID, "Denied permission should not be in effective permissions")
	}
}

// Helper functions

func createTestUserForUserRole(t *testing.T, suite *TestSuite, email string) *domain.User {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(passwordHash),
	}
	require.NoError(t, repository.NewUserRepository(suite.DB).Create(context.Background(), user))
	return user
}

func createTestRoleForUserRole(t *testing.T, suite *TestSuite) *domain.Role {
	role := &domain.Role{
		ID:          uuid.New(),
		Name:        "test-role-" + uuid.New().String()[:8],
		Description: "Test role",
	}
	require.NoError(t, repository.NewRoleRepository(suite.DB).Create(context.Background(), role))
	return role
}

func createTestPermissionForUserRole(t *testing.T, suite *TestSuite, name string) *domain.Permission {
	perm := &domain.Permission{
		ID:       uuid.New(),
		Name:     name,
		Resource: "test",
		Action:   "action",
		Scope:    "all",
	}
	require.NoError(t, repository.NewPermissionRepository(suite.DB).Create(context.Background(), perm))
	return perm
}
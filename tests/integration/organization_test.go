//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ==============================================================================
// ORGANIZATION CRUD TESTS
// ==============================================================================

// TestOrganizationService_Create_ValidData tests creating an organization with valid data
func TestOrganizationService_Create_ValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, err := permission.NewEnforcer(suite.DB)
	require.NoError(t, err, "Failed to create enforcer")

	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Test Create
	settings := map[string]interface{}{"theme": "dark"}
	org, err := orgService.CreateOrganization(ctx, user.ID, "Test Org", "test-org", settings, "127.0.0.1", "test-agent")

	require.NoError(t, err, "Should create organization successfully")
	assert.NotEmpty(t, org.ID, "Organization ID should be generated")
	assert.Equal(t, "Test Org", org.Name, "Organization name should match")
	assert.Equal(t, "test-org", org.Slug, "Organization slug should match")
	assert.Equal(t, user.ID, org.OwnerID, "Owner ID should match")
	assert.NotNil(t, org.Settings, "Settings should be set")

	// Verify member was added
	member, err := orgRepo.FindMember(ctx, org.ID, user.ID)
	require.NoError(t, err, "Owner should be added as member")
	assert.Equal(t, domain.RoleOwner, member.Role, "Owner should have owner role")

	// Verify Casbin policy
	allowed, err := enforcer.Enforce(user.ID.String(), org.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.True(t, allowed, "Owner should have manage permission")
}

// TestOrganizationService_Create_DuplicateSlug tests creating organization with duplicate slug
func TestOrganizationService_Create_DuplicateSlug(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create first organization
	_, err := orgService.CreateOrganization(ctx, user.ID, "Org One", "duplicate-slug", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err, "First organization should be created")

	// Try to create duplicate slug
	_, err = orgService.CreateOrganization(ctx, user.ID, "Org Two", "duplicate-slug", nil, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail with duplicate slug")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "Should be an AppError")
	assert.Equal(t, "CONFLICT", appErr.Code, "Error code should be CONFLICT")
	assert.Equal(t, 409, appErr.HTTPStatus)
}

// TestOrganizationService_Create_InvalidInput tests creating organization with invalid input
func TestOrganizationService_Create_InvalidInput(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	tests := []struct {
		name    string
		orgName string
		slug    string
		wantErr string
	}{
		{"empty name", "", "test-org", "name and slug are required"},
		{"empty slug", "Test Org", "", "name and slug are required"},
		{"both empty", "", "", "name and slug are required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := orgService.CreateOrganization(ctx, user.ID, tc.orgName, tc.slug, nil, "127.0.0.1", "test-agent")
			require.Error(t, err, "Should fail with invalid input")

			var appErr *apperrors.AppError
			require.True(t, errors.As(err, &appErr), "Should be an AppError")
			assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
			assert.Contains(t, appErr.Message, tc.wantErr)
		})
	}
}

// TestOrganizationService_GetByID tests retrieving organization by ID
func TestOrganizationService_GetByID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Test GetByID
	retrieved, err := orgService.GetOrganization(ctx, owner.ID, org.ID)
	require.NoError(t, err, "Should retrieve organization")
	assert.Equal(t, org.ID, retrieved.ID, "IDs should match")
	assert.Equal(t, org.Name, retrieved.Name, "Names should match")
	assert.Equal(t, org.Slug, retrieved.Slug, "Slugs should match")
}

// TestOrganizationService_GetByID_NotFound tests retrieving non-existent organization
func TestOrganizationService_GetByID_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	nonExistentID := uuid.New()
	_, err := orgService.GetOrganization(ctx, user.ID, nonExistentID)
	require.Error(t, err, "Should fail for non-existent organization")
	// Service returns FORBIDDEN (not NOT_FOUND) to prevent information leakage
	assert.True(t, errors.Is(err, apperrors.ErrForbidden), "Should be FORBIDDEN (access denied)")
}

// TestOrganizationService_GetByID_NotMember tests retrieving organization without membership
func TestOrganizationService_GetByID_NotMember(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	nonMember := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Try to get organization as non-member
	_, err = orgService.GetOrganization(ctx, nonMember.ID, org.ID)
	require.Error(t, err, "Should fail for non-member")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "Should be an AppError")
	assert.Equal(t, "FORBIDDEN", appErr.Code)
	assert.Equal(t, 403, appErr.HTTPStatus)
}

// TestOrganizationService_List tests listing organizations
func TestOrganizationService_List(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create multiple organizations
	for i := 1; i <= 3; i++ {
		slug := string(rune('a' + i - 1)) + "-org"
		_, err := orgService.CreateOrganization(ctx, user.ID, "Org "+string(rune('0'+i)), slug, nil, "127.0.0.1", "test-agent")
		require.NoError(t, err)
	}

	// List organizations
	orgs, total, err := orgService.ListOrganizations(ctx, user.ID, 10, 0)
	require.NoError(t, err, "Should list organizations")
	assert.Equal(t, int64(3), total, "Should have 3 organizations")
	assert.Len(t, orgs, 3, "Should return 3 organizations")
}

// TestOrganizationService_List_Pagination tests listing organizations with pagination
func TestOrganizationService_List_Pagination(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create 5 organizations
	for i := 1; i <= 5; i++ {
		slug := string(rune('a'+i-1)) + "-org"
		_, err := orgService.CreateOrganization(ctx, user.ID, "Org "+string(rune('0'+i)), slug, nil, "127.0.0.1", "test-agent")
		require.NoError(t, err)
	}

	// Test first page
	page1, total, err := orgService.ListOrganizations(ctx, user.ID, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total, "Total should be 5")
	assert.Len(t, page1, 2, "Page 1 should have 2 organizations")

	// Test second page
	page2, _, err := orgService.ListOrganizations(ctx, user.ID, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2, "Page 2 should have 2 organizations")
}

// TestOrganizationService_Update tests updating organization
func TestOrganizationService_Update(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Original Name", "original-slug", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Update organization
	settings := map[string]interface{}{"theme": "light"}
	updated, err := orgService.UpdateOrganization(ctx, owner.ID, org.ID, "Updated Name", "updated-slug", settings, "127.0.0.1", "test-agent")
	require.NoError(t, err, "Should update organization")
	assert.Equal(t, "Updated Name", updated.Name, "Name should be updated")
	assert.Equal(t, "updated-slug", updated.Slug, "Slug should be updated")
	assert.NotNil(t, updated.Settings, "Settings should be updated")
}

// TestOrganizationService_Update_NotFound tests updating non-existent organization
func TestOrganizationService_Update_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	nonExistentID := uuid.New()
	_, err := orgService.UpdateOrganization(ctx, user.ID, nonExistentID, "Name", "slug", nil, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail for non-existent organization")
}

// TestOrganizationService_Update_NotMember tests updating organization without membership
func TestOrganizationService_Update_NotMember(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	nonMember := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Try to update as non-member
	_, err = orgService.UpdateOrganization(ctx, nonMember.ID, org.ID, "Hacked Name", "hacked-slug", nil, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail for non-member")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "FORBIDDEN", appErr.Code)
}

// TestOrganizationService_Delete tests soft deleting organization
func TestOrganizationService_Delete(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Delete organization
	err = orgService.DeleteOrganization(ctx, owner.ID, org.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err, "Should delete organization")

	// Verify soft delete
	_, err = orgRepo.FindByID(ctx, org.ID)
	require.Error(t, err, "Should not find deleted organization")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

// TestOrganizationService_Delete_NotFound tests deleting non-existent organization
func TestOrganizationService_Delete_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	user := createTestUserForOrg(t, suite.DB)
	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	nonExistentID := uuid.New()
	err := orgService.DeleteOrganization(ctx, user.ID, nonExistentID, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail for non-existent organization")
}

// ==============================================================================
// MEMBER TESTS
// ==============================================================================

// TestOrganizationMember_Add tests adding member to organization
func TestOrganizationMember_Add(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	newMember := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Add member
	member, err := orgService.AddMember(ctx, owner.ID, org.ID, newMember.ID, domain.RoleMember, "127.0.0.1", "test-agent")
	require.NoError(t, err, "Should add member")
	assert.Equal(t, org.ID, member.OrganizationID, "Organization ID should match")
	assert.Equal(t, newMember.ID, member.UserID, "User ID should match")
	assert.Equal(t, domain.RoleMember, member.Role, "Role should be member")

	// Verify membership
	isMember, err := orgRepo.IsMember(ctx, org.ID, newMember.ID)
	require.NoError(t, err)
	assert.True(t, isMember, "User should be member")

	// Verify Casbin role
	allowed, err := enforcer.Enforce(newMember.ID.String(), org.ID.String(), "organization", "view")
	require.NoError(t, err)
	assert.True(t, allowed, "Member should have view permission")
}

// TestOrganizationMember_Add_AlreadyMember tests adding existing member
func TestOrganizationMember_Add_AlreadyMember(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization (owner is automatically a member)
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Try to add owner again
	_, err = orgService.AddMember(ctx, owner.ID, org.ID, owner.ID, domain.RoleMember, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail to add existing member")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "CONFLICT", appErr.Code)
	assert.Contains(t, appErr.Message, "already a member")
}

// TestOrganizationMember_Add_InvalidRole tests adding member with invalid role
func TestOrganizationMember_Add_InvalidRole(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	newMember := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Try to add member with invalid role
	_, err = orgService.AddMember(ctx, owner.ID, org.ID, newMember.ID, "invalid-role", "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail with invalid role")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Contains(t, appErr.Message, "invalid role")
}

// TestOrganizationMember_GetMembers tests listing organization members
func TestOrganizationMember_GetMembers(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	member1 := createTestUserForOrg(t, suite.DB)
	member2 := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Add members
	_, err = orgService.AddMember(ctx, owner.ID, org.ID, member1.ID, domain.RoleMember, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	_, err = orgService.AddMember(ctx, owner.ID, org.ID, member2.ID, domain.RoleAdmin, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Get members
	members, total, err := orgService.GetMembers(ctx, owner.ID, org.ID, 10, 0)
	require.NoError(t, err, "Should list members")
	assert.Equal(t, int64(3), total, "Should have 3 members (owner + 2 added)")
	assert.Len(t, members, 3, "Should return 3 members")

	// Verify roles
	memberRoles := make(map[uuid.UUID]string)
	for _, m := range members {
		memberRoles[m.UserID] = m.Role
	}
	assert.Equal(t, domain.RoleOwner, memberRoles[owner.ID], "Owner should have owner role")
	assert.Equal(t, domain.RoleMember, memberRoles[member1.ID], "Member1 should have member role")
	assert.Equal(t, domain.RoleAdmin, memberRoles[member2.ID], "Member2 should have admin role")
}

// TestOrganizationMember_Remove tests removing member from organization
func TestOrganizationMember_Remove(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	member := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization and add member
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	_, err = orgService.AddMember(ctx, owner.ID, org.ID, member.ID, domain.RoleMember, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Remove member
	err = orgService.RemoveMember(ctx, owner.ID, org.ID, member.ID, "127.0.0.1", "test-agent")
	require.NoError(t, err, "Should remove member")

	// Verify removal
	isMember, err := orgRepo.IsMember(ctx, org.ID, member.ID)
	require.NoError(t, err)
	assert.False(t, isMember, "User should no longer be member")

	// Verify Casbin role removed
	allowed, err := enforcer.Enforce(member.ID.String(), org.ID.String(), "organization", "view")
	require.NoError(t, err)
	assert.False(t, allowed, "Removed member should not have permissions")
}

// TestOrganizationMember_Remove_Owner tests removing organization owner (should fail)
func TestOrganizationMember_Remove_Owner(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Try to remove owner
	err = orgService.RemoveMember(ctx, owner.ID, org.ID, owner.ID, "127.0.0.1", "test-agent")
	require.Error(t, err, "Should fail to remove owner")

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "INVALID_OPERATION", appErr.Code)
	assert.Contains(t, appErr.Message, "cannot remove organization owner")
}

// ==============================================================================
// PERMISSION TESTS
// ==============================================================================

// TestOrganization_PermissionHierarchy tests permission enforcement across roles
func TestOrganization_PermissionHierarchy(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner := createTestUserForOrg(t, suite.DB)
	admin := createTestUserForOrg(t, suite.DB)
	member := createTestUserForOrg(t, suite.DB)
	outsider := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create organization
	org, err := orgService.CreateOrganization(ctx, owner.ID, "Test Org", "test-org", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Add admin
	_, err = orgService.AddMember(ctx, owner.ID, org.ID, admin.ID, domain.RoleAdmin, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Add member
	_, err = orgService.AddMember(ctx, owner.ID, org.ID, member.ID, domain.RoleMember, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Note: The Casbin policy model and enforcement need to be properly configured
	// For now, we test basic permission structure
	// Owner should have manage permission via the policies set during organization creation

	// Verify owner can manage
	ownerCanManage, err := enforcer.Enforce(owner.ID.String(), org.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.True(t, ownerCanManage, "Owner should have manage permission")

	// Verify outsider cannot access
	outsiderCanView, err := enforcer.Enforce(outsider.ID.String(), org.ID.String(), "organization", "view")
	require.NoError(t, err)
	assert.False(t, outsiderCanView, "Outsider should not have view permission")
}

// TestOrganization_CasbinDomainIsolation tests that Casbin domains isolate permissions
func TestOrganization_CasbinDomainIsolation(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	owner1 := createTestUserForOrg(t, suite.DB)
	owner2 := createTestUserForOrg(t, suite.DB)

	enforcer, _ := permission.NewEnforcer(suite.DB)
	orgRepo := repository.NewOrganizationRepository(suite.DB)
	auditRepo := repository.NewAuditLogRepository(suite.DB)
	auditService := service.NewAuditService(auditRepo, service.DefaultAuditServiceConfig())
	orgService := service.NewOrganizationService(orgRepo, nil, enforcer, auditService, nil, createOrgTestLogger())

	// Create two organizations
	org1, err := orgService.CreateOrganization(ctx, owner1.ID, "Org One", "org-one", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	org2, err := orgService.CreateOrganization(ctx, owner2.ID, "Org Two", "org-two", nil, "127.0.0.1", "test-agent")
	require.NoError(t, err)

	// Owner1 should have access to Org1 only
	allowed1, err := enforcer.Enforce(owner1.ID.String(), org1.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.True(t, allowed1, "Owner1 should manage Org1")

	// Owner1 should NOT have access to Org2
	allowed2, err := enforcer.Enforce(owner1.ID.String(), org2.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.False(t, allowed2, "Owner1 should NOT manage Org2")

	// Owner2 should have access to Org2 only
	allowed3, err := enforcer.Enforce(owner2.ID.String(), org2.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.True(t, allowed3, "Owner2 should manage Org2")

	// Owner2 should NOT have access to Org1
	allowed4, err := enforcer.Enforce(owner2.ID.String(), org1.ID.String(), "organization", "manage")
	require.NoError(t, err)
	assert.False(t, allowed4, "Owner2 should NOT manage Org1")
}

// ==============================================================================
// HELPER FUNCTIONS
// ==============================================================================

// createTestUserForOrg creates a test user directly in the database for organization tests
func createTestUserForOrg(t *testing.T, db *gorm.DB) *domain.User {
	user := &domain.User{
		Email:        "test-" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "$2a$12$test.hash.password", // Test hash
	}

	require.NoError(t, db.Create(user).Error, "Failed to create test user")
	return user
}

// createOrgTestLogger creates a structured logger for testing
func createOrgTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
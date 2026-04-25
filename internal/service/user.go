package service

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// UserService handles user-related business logic
type UserService struct {
	userRepo           repository.UserRepository
	userRoleRepo       repository.UserRoleRepository
	userPermissionRepo repository.UserPermissionRepository
}

// Performance Notes:
// - GetEffectivePermissions uses a map-based merge algorithm O(n+m) where n = role perms, m = user perms
// - UserRepository methods use GORM Preload to avoid N+1 queries when loading roles with permissions
// - RoleRepository.FindByID/FindAll preloads Permissions association
// - UserRoleRepository.FindByUserIDWithPermissions uses Preload for efficient role+permission loading

// NewUserService creates a new UserService instance
func NewUserService(
	userRepo repository.UserRepository,
	userRoleRepo repository.UserRoleRepository,
	userPermissionRepo repository.UserPermissionRepository,
) *UserService {
	return &UserService{
		userRepo:           userRepo,
		userRoleRepo:       userRoleRepo,
		userPermissionRepo: userPermissionRepo,
	}
}

// FindByID retrieves a user by their ID
func (s *UserService) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

// FindByEmail retrieves a user by their email address
func (s *UserService) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.userRepo.FindByEmail(ctx, email)
}

// FindAll retrieves all users
func (s *UserService) FindAll(ctx context.Context) ([]domain.User, error) {
	return s.userRepo.FindAll(ctx)
}

// SoftDelete performs a soft delete on a user
func (s *UserService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return s.userRepo.SoftDelete(ctx, id)
}

// AssignRole assigns a role to a user
func (s *UserService) AssignRole(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
	return s.userRoleRepo.Assign(ctx, userID, roleID, assignedBy)
}

// RemoveRole removes a role from a user
func (s *UserService) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.userRoleRepo.Remove(ctx, userID, roleID)
}

// GetUserRoles retrieves all roles for a user
func (s *UserService) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	return s.userRoleRepo.FindByUserID(ctx, userID)
}

// GrantPermission grants a permission to a user (allow effect)
func (s *UserService) GrantPermission(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	userPerm := &domain.UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
		Effect:       domain.EffectAllow,
		AssignedBy:   assignedBy,
	}
	if err := userPerm.Validate(); err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", err.Error(), 400)
	}
	return s.userPermissionRepo.Grant(ctx, userID, permissionID, assignedBy)
}

// DenyPermission denies a permission for a user (deny effect)
func (s *UserService) DenyPermission(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	userPerm := &domain.UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
		Effect:       domain.EffectDeny,
		AssignedBy:   assignedBy,
	}
	if err := userPerm.Validate(); err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", err.Error(), 400)
	}
	return s.userPermissionRepo.Deny(ctx, userID, permissionID, assignedBy)
}

// RemovePermission removes a direct permission assignment from a user
func (s *UserService) RemovePermission(ctx context.Context, userID, permissionID uuid.UUID) error {
	return s.userPermissionRepo.Remove(ctx, userID, permissionID)
}

// GetEffectivePermissions calculates the effective permissions for a user.
// It combines permissions from roles with direct user permissions,
// applying deny overrides where deny takes precedence.
func (s *UserService) GetEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]domain.Permission, error) {
	// Map to track permissions with their effects
	// Key: permission ID string, Value: permission with effect
	permissionMap := make(map[uuid.UUID]permissionWithEffect)

	// Get permissions from roles (all are "allow")
	roles, err := s.userRoleRepo.FindByUserIDWithPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		for _, perm := range role.Permissions {
			permissionMap[perm.ID] = permissionWithEffect{
				Permission: perm,
				Effect:     "allow",
				Source:     "role",
			}
		}
	}

	// Get direct user permissions (can be allow or deny)
	userPerms, err := s.userPermissionRepo.FindByUserIDWithDetails(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply user permissions (they override role permissions)
	for _, up := range userPerms {
		permissionMap[up.Permission.ID] = permissionWithEffect{
			Permission: up.Permission,
			Effect:     up.UserPermission.Effect,
			Source:     "direct",
		}
	}

	// Filter out denied permissions and return only allowed ones
	var effectivePermissions []domain.Permission
	for _, pe := range permissionMap {
		if pe.Effect == "allow" {
			effectivePermissions = append(effectivePermissions, pe.Permission)
		}
	}

	return effectivePermissions, nil
}

// permissionWithEffect tracks a permission with its effect and source
type permissionWithEffect struct {
	Permission domain.Permission
	Effect     string // "allow" or "deny"
	Source     string // "role" or "direct"
}
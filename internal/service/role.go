package service

import (
	"context"
	"errors"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// RoleService handles role-related business logic
type RoleService struct {
	roleRepo             repository.RoleRepository
	rolePermissionRepo   repository.RolePermissionRepository
	permissionRepo       repository.PermissionRepository
}

// NewRoleService creates a new RoleService instance
func NewRoleService(
	roleRepo repository.RoleRepository,
	rolePermissionRepo repository.RolePermissionRepository,
	permissionRepo repository.PermissionRepository,
) *RoleService {
	return &RoleService{
		roleRepo:           roleRepo,
		rolePermissionRepo: rolePermissionRepo,
		permissionRepo:     permissionRepo,
	}
}

// Create creates a new role with the given name and description
func (s *RoleService) Create(ctx context.Context, name, description string) (*domain.Role, error) {
	// Validate inputs
	if name == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Role name is required", 422)
	}

	// Check if role name already exists
	_, err := s.roleRepo.FindByName(ctx, name)
	if err == nil {
		return nil, apperrors.NewAppError("ROLE_EXISTS", "Role name already exists", 409)
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	// Create the role
	role := &domain.Role{
		Name:        name,
		Description: description,
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

// Update updates an existing role's name and description
func (s *RoleService) Update(ctx context.Context, id uuid.UUID, name, description string) (*domain.Role, error) {
	// Find role by ID
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	// Update fields
	role.Name = name
	role.Description = description

	// Save changes
	if err := s.roleRepo.Update(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

// AttachPermission attaches a permission to a role
func (s *RoleService) AttachPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	// Find role by ID to ensure it exists
	_, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}

	// Find permission by ID to ensure it exists
	_, err = s.permissionRepo.FindByID(ctx, permissionID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return apperrors.ErrNotFound
		}
		return err
	}

	// Attach permission to role
	return s.rolePermissionRepo.Attach(ctx, roleID, permissionID)
}

// DetachPermission detaches a permission from a role
func (s *RoleService) DetachPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return s.rolePermissionRepo.Detach(ctx, roleID, permissionID)
}

// GetByID retrieves a role by its ID with preloaded permissions
func (s *RoleService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return role, nil
}

// GetAll retrieves all roles with their permissions
func (s *RoleService) GetAll(ctx context.Context) ([]domain.Role, error) {
	roles, err := s.roleRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return roles, nil
}

// Delete soft deletes a role by its ID
func (s *RoleService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.roleRepo.SoftDelete(ctx, id)
}
package service

import (
	"context"
	"errors"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

// PermissionService handles permission-related business logic
type PermissionService struct {
	permissionRepo repository.PermissionRepository
}

// NewPermissionService creates a new PermissionService instance
func NewPermissionService(permissionRepo repository.PermissionRepository) *PermissionService {
	return &PermissionService{
		permissionRepo: permissionRepo,
	}
}

// Create creates a new permission with the given attributes
func (s *PermissionService) Create(ctx context.Context, name, resource, action, scope string) (*domain.Permission, error) {
	// Validate inputs
	if name == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Permission name is required", 422)
	}
	if resource == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Resource is required", 422)
	}
	if action == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Action is required", 422)
	}
	if scope == "" {
		scope = "all"
	}

	// Check if permission name already exists
	_, err := s.permissionRepo.FindByName(ctx, name)
	if err == nil {
		return nil, apperrors.NewAppError("PERMISSION_EXISTS", "Permission name already exists", 409)
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	// Create the permission
	permission := &domain.Permission{
		Name:     name,
		Resource: resource,
		Action:   action,
		Scope:    scope,
	}

	if err := s.permissionRepo.Create(ctx, permission); err != nil {
		return nil, err
	}

	return permission, nil
}

// GetAll retrieves all permissions
func (s *PermissionService) GetAll(ctx context.Context) ([]domain.Permission, error) {
	permissions, err := s.permissionRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return permissions, nil
}
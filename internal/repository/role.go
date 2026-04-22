package repository

import (
	"context"
	"errors"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrSystemRoleProtected is returned when attempting to delete a system role
var ErrSystemRoleProtected = errors.New("cannot delete system role")

// RoleRepository defines the interface for role data access operations
type RoleRepository interface {
	Create(ctx context.Context, role *domain.Role) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	FindAll(ctx context.Context) ([]domain.Role, error)
	Update(ctx context.Context, role *domain.Role) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	FindByName(ctx context.Context, name string) (*domain.Role, error)
}

// Optimization Note:
// All Find* methods use GORM Preload to eagerly load associated Permissions.
// This prevents N+1 query problems when iterating over roles and accessing their permissions.
// Role -> Permissions is a many-to-many relationship via role_permissions table.

// roleRepository implements RoleRepository interface
type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository creates a new RoleRepository instance
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{
		db: db,
	}
}

// Create inserts a new role into the database
func (r *roleRepository) Create(ctx context.Context, role *domain.Role) error {
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a role by its UUID with permissions preloaded
func (r *roleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	if err := r.db.WithContext(ctx).Preload("Permissions").First(&role, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &role, nil
}

// FindAll retrieves all roles with permissions preloaded
func (r *roleRepository) FindAll(ctx context.Context) ([]domain.Role, error) {
	var roles []domain.Role
	if err := r.db.WithContext(ctx).Preload("Permissions").Find(&roles).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return roles, nil
}

// Update updates an existing role in the database
func (r *roleRepository) Update(ctx context.Context, role *domain.Role) error {
	if err := r.db.WithContext(ctx).Save(role).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// SoftDelete performs a soft delete on a role by its ID
// Returns ErrSystemRoleProtected if the role is a system role
func (r *roleRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	// First check if the role is a system role
	var role domain.Role
	if err := r.db.WithContext(ctx).First(&role, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.ErrNotFound
		}
		return apperrors.WrapInternal(err)
	}

	if role.IsSystem {
		return ErrSystemRoleProtected
	}

	result := r.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// FindByName finds a role by its unique name with permissions preloaded
func (r *roleRepository) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	if err := r.db.WithContext(ctx).Preload("Permissions").Where("name = ?", name).First(&role).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &role, nil
}
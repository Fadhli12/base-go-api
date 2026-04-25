package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RolePermissionRepository defines the interface for role-permission association operations
type RolePermissionRepository interface {
	Attach(ctx context.Context, roleID, permissionID uuid.UUID) error
	Detach(ctx context.Context, roleID, permissionID uuid.UUID) error
	FindByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
	Sync(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
}

// rolePermissionRepository implements RolePermissionRepository interface
type rolePermissionRepository struct {
	db *gorm.DB
}

// NewRolePermissionRepository creates a new RolePermissionRepository instance
func NewRolePermissionRepository(db *gorm.DB) RolePermissionRepository {
	return &rolePermissionRepository{
		db: db,
	}
}

// Attach adds a permission to a role
func (r *rolePermissionRepository) Attach(ctx context.Context, roleID, permissionID uuid.UUID) error {
	rolePermission := domain.RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}
	if err := r.db.WithContext(ctx).Create(&rolePermission).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// Detach removes a permission from a role
func (r *rolePermissionRepository) Detach(ctx context.Context, roleID, permissionID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&domain.RolePermission{})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	return nil
}

// FindByRoleID retrieves all permissions assigned to a role
func (r *rolePermissionRepository) FindByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	var permissions []domain.Permission
	if err := r.db.WithContext(ctx).
		Table("permissions").
		Joins("JOIN role_permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Where("permissions.deleted_at IS NULL"). // Filter soft-deleted permissions
		Find(&permissions).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return permissions, nil
}

// Sync replaces all permissions for a role with the provided list
func (r *rolePermissionRepository) Sync(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove all existing permissions for this role
		if err := tx.Where("role_id = ?", roleID).Delete(&domain.RolePermission{}).Error; err != nil {
			return errors.WrapInternal(err)
		}

		// Insert new permissions
		for _, permissionID := range permissionIDs {
			rolePermission := domain.RolePermission{
				RoleID:       roleID,
				PermissionID: permissionID,
			}
			if err := tx.Create(&rolePermission).Error; err != nil {
				return errors.WrapInternal(err)
			}
		}

		return nil
	})
}
package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRoleRepository defines the interface for user-role association operations
type UserRoleRepository interface {
	// Assign assigns a role to a user
	Assign(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error
	// Remove removes a role from a user
	Remove(ctx context.Context, userID, roleID uuid.UUID) error
	// FindByUserID retrieves all roles assigned to a user
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
	// FindByUserIDWithPermissions retrieves all roles with permissions assigned to a user
	FindByUserIDWithPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
}

// userRoleRepository implements UserRoleRepository interface
type userRoleRepository struct {
	db *gorm.DB
}

// NewUserRoleRepository creates a new UserRoleRepository instance
func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepository{
		db: db,
	}
}

// Assign creates a new user-role association
func (r *userRoleRepository) Assign(ctx context.Context, userID, roleID, assignedBy uuid.UUID) error {
	userRole := &domain.UserRole{
		UserID:     userID,
		RoleID:     roleID,
		AssignedBy: assignedBy,
	}

	if err := r.db.WithContext(ctx).Create(userRole).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// Remove deletes a user-role association
func (r *userRoleRepository) Remove(ctx context.Context, userID, roleID uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.UserRole{}, "user_id = ? AND role_id = ?", userID, roleID)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// FindByUserID retrieves all roles assigned to a user (without permissions)
func (r *userRoleRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	if err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Where("roles.deleted_at IS NULL").
		Find(&roles).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return roles, nil
}

// FindByUserIDWithPermissions retrieves all roles with permissions assigned to a user
func (r *userRoleRepository) FindByUserIDWithPermissions(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	if err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ?", userID).
		Where("roles.deleted_at IS NULL").
		Preload("Permissions").
		Find(&roles).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return roles, nil
}
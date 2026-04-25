package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserPermissionRepository defines the interface for direct user permission operations
type UserPermissionRepository interface {
	// Grant creates an "allow" permission override for a user
	Grant(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error
	// Deny creates a "deny" permission override for a user
	Deny(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error
	// FindByUserID retrieves all direct permission assignments for a user
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserPermission, error)
	// FindByUserIDWithDetails retrieves user permissions with permission details
	FindByUserIDWithDetails(ctx context.Context, userID uuid.UUID) ([]UserPermissionWithDetails, error)
	// Remove removes a direct permission assignment from a user
	Remove(ctx context.Context, userID, permissionID uuid.UUID) error
}

// Optimization Note:
// FindByUserIDWithDetails uses a JOIN query instead of GORM Preload to fetch
// both user_permissions and permission details in a single query.
// This pattern eliminates N+1 query problems when iterating over user permissions
// and accessing their associated Permission details.

// UserPermissionWithDetails contains a user permission with its associated permission details
type UserPermissionWithDetails struct {
	UserPermission domain.UserPermission
	Permission      domain.Permission
}

// userPermissionRepository implements UserPermissionRepository interface
type userPermissionRepository struct {
	db *gorm.DB
}

// NewUserPermissionRepository creates a new UserPermissionRepository instance
func NewUserPermissionRepository(db *gorm.DB) UserPermissionRepository {
	return &userPermissionRepository{
		db: db,
	}
}

// Grant creates an "allow" permission override for a user
func (r *userPermissionRepository) Grant(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	userPerm := &domain.UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
		Effect:       domain.EffectAllow,
		AssignedBy:   assignedBy,
	}

	// Use upsert to handle existing record
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND permission_id = ?", userID, permissionID).
		Assign(map[string]interface{}{
			"effect":      "allow",
			"assigned_by": assignedBy,
		}).
		FirstOrCreate(userPerm).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// Deny creates a "deny" permission override for a user
func (r *userPermissionRepository) Deny(ctx context.Context, userID, permissionID, assignedBy uuid.UUID) error {
	userPerm := &domain.UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
		Effect:       domain.EffectDeny,
		AssignedBy:   assignedBy,
	}

	// Use upsert to handle existing record
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND permission_id = ?", userID, permissionID).
		Assign(map[string]interface{}{
			"effect":      domain.EffectDeny,
			"assigned_by": assignedBy,
		}).
		FirstOrCreate(userPerm).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByUserID retrieves all direct permission assignments for a user
func (r *userPermissionRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.UserPermission, error) {
	var userPerms []domain.UserPermission
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&userPerms).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return userPerms, nil
}

// FindByUserIDWithDetails retrieves user permissions with permission details
func (r *userPermissionRepository) FindByUserIDWithDetails(ctx context.Context, userID uuid.UUID) ([]UserPermissionWithDetails, error) {
	var results []UserPermissionWithDetails

	query := `
		SELECT up.user_id, up.permission_id, up.effect, up.assigned_at, up.assigned_by,
			   p.id as "permission_id", p.name, p.resource, p.action, p.scope, p.is_system, p.created_at, p.updated_at
		FROM user_permissions up
		JOIN permissions p ON up.permission_id = p.id
		WHERE up.user_id = ? AND p.deleted_at IS NULL
	`

	rows, err := r.db.WithContext(ctx).Raw(query, userID).Rows()
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var up domain.UserPermission
		var p domain.Permission
		if err := rows.Scan(
			&up.UserID, &up.PermissionID, &up.Effect, &up.AssignedAt, &up.AssignedBy,
			&p.ID, &p.Name, &p.Resource, &p.Action, &p.Scope, &p.IsSystem, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, apperrors.WrapInternal(err)
		}
		results = append(results, UserPermissionWithDetails{
			UserPermission: up,
			Permission:     p,
		})
	}

	// Check for errors that occurred during iteration
	if err := rows.Err(); err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	return results, nil
}

// Remove removes a direct permission assignment from a user
func (r *userPermissionRepository) Remove(ctx context.Context, userID, permissionID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Delete(&domain.UserPermission{}, "user_id = ? AND permission_id = ?", userID, permissionID)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
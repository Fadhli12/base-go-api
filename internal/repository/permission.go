package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PermissionRepository defines the interface for permission data access operations
type PermissionRepository interface {
	Create(ctx context.Context, permission *domain.Permission) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)
	FindAll(ctx context.Context) ([]domain.Permission, error)
	FindByResource(ctx context.Context, resource string) ([]domain.Permission, error)
	FindByName(ctx context.Context, name string) (*domain.Permission, error)
}

// permissionRepository implements PermissionRepository interface
type permissionRepository struct {
	db *gorm.DB
}

// NewPermissionRepository creates a new PermissionRepository instance
func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepository{
		db: db,
	}
}

// Create inserts a new permission into the database
func (r *permissionRepository) Create(ctx context.Context, permission *domain.Permission) error {
	if err := r.db.WithContext(ctx).Create(permission).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a permission by its UUID
func (r *permissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	var permission domain.Permission
	if err := r.db.WithContext(ctx).First(&permission, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &permission, nil
}

// FindAll retrieves all permissions
func (r *permissionRepository) FindAll(ctx context.Context) ([]domain.Permission, error) {
	var permissions []domain.Permission
	if err := r.db.WithContext(ctx).Find(&permissions).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return permissions, nil
}

// FindByResource finds all permissions for a given resource
func (r *permissionRepository) FindByResource(ctx context.Context, resource string) ([]domain.Permission, error) {
	var permissions []domain.Permission
	if err := r.db.WithContext(ctx).Where("resource = ?", resource).Find(&permissions).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return permissions, nil
}

// FindByName finds a permission by its unique name
func (r *permissionRepository) FindByName(ctx context.Context, name string) (*domain.Permission, error) {
	var permission domain.Permission
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&permission).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &permission, nil
}
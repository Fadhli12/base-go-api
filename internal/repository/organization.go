package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrganizationRepository defines the interface for organization data access operations
type OrganizationRepository interface {
	// Organization operations
	Create(ctx context.Context, org *domain.Organization) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	FindAll(ctx context.Context, limit, offset int) ([]*domain.Organization, int64, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Organization, int64, error)
	Update(ctx context.Context, org *domain.Organization) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Member operations
	AddMember(ctx context.Context, member *domain.OrganizationMember) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	FindMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.OrganizationMember, error)
	FindMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.OrganizationMember, int64, error)
	GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error)
	IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

// organizationRepository implements OrganizationRepository interface
type organizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new OrganizationRepository instance
func NewOrganizationRepository(db *gorm.DB) OrganizationRepository {
	return &organizationRepository{db: db}
}

// Create inserts a new organization into the database
func (r *organizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	if err := r.db.WithContext(ctx).Create(org).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds an organization by its UUID
func (r *organizationRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).
		Preload("Owner", "deleted_at IS NULL").
		Preload("Members").
		First(&org, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &org, nil
}

// FindBySlug finds an organization by its unique slug
func (r *organizationRepository) FindBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).
		Preload("Owner", "deleted_at IS NULL").
		Preload("Members").
		Where("slug = ?", slug).
		First(&org).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &org, nil
}

// FindAll retrieves all organizations with pagination
func (r *organizationRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.Organization, int64, error) {
	var orgs []*domain.Organization
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&domain.Organization{}).Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	// Note: Preload with soft-delete filter to exclude deleted owners
	err := r.db.WithContext(ctx).
		Preload("Owner", "deleted_at IS NULL").
		Limit(limit).
		Offset(offset).
		Find(&orgs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return orgs, total, nil
}

// FindByUserID retrieves organizations where the user is a member
func (r *organizationRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Organization, int64, error) {
	var orgs []*domain.Organization
	var total int64

	// Count total organizations where user is a member
	if err := r.db.WithContext(ctx).
		Model(&domain.Organization{}).
		Joins("JOIN organization_members ON organizations.id = organization_members.organization_id").
		Where("organization_members.user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch organizations where user is a member
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Joins("JOIN organization_members ON organizations.id = organization_members.organization_id").
		Where("organization_members.user_id = ?", userID).
		Limit(limit).
		Offset(offset).
		Find(&orgs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return orgs, total, nil
}

// Update updates an existing organization in the database
func (r *organizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	result := r.db.WithContext(ctx).Save(org)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on an organization by its ID
func (r *organizationRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Organization{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// AddMember adds a new member to an organization
func (r *organizationRepository) AddMember(ctx context.Context, member *domain.OrganizationMember) error {
	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// RemoveMember removes a member from an organization
func (r *organizationRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Delete(&domain.OrganizationMember{})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// FindMember finds a specific organization membership
func (r *organizationRepository) FindMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Preload("User", "deleted_at IS NULL").
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &member, nil
}

// FindMembers retrieves all members of an organization with pagination
func (r *organizationRepository) FindMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.OrganizationMember, int64, error) {
	var members []*domain.OrganizationMember
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ?", orgID).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	// Note: Preload with soft-delete filter to exclude deleted users
	err := r.db.WithContext(ctx).
		Preload("User", "deleted_at IS NULL").
		Where("organization_id = ?", orgID).
		Limit(limit).
		Offset(offset).
		Find(&members).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return members, total, nil
}

// GetMemberRole retrieves the role of a member in an organization
func (r *organizationRepository) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Select("role").
		Scan(&role).Error
	if err == gorm.ErrRecordNotFound {
		return "", errors.ErrNotFound
	}
	if err != nil {
		return "", errors.WrapInternal(err)
	}
	return role, nil
}

// IsMember checks if a user is a member of an organization
func (r *organizationRepository) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Count(&count).Error
	if err != nil {
		return false, errors.WrapInternal(err)
	}
	return count > 0, nil
}
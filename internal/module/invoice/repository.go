package invoice

import (
	"context"

	"github.com/google/uuid"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"gorm.io/gorm"
)

// Repository defines the interface for invoice data access operations
type Repository interface {
	// Create inserts a new invoice into the database
	Create(ctx context.Context, invoice *Invoice) error
	// FindByID finds an invoice by its UUID
	FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	// FindByUserID finds all invoices for a user with pagination
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Invoice, error)
	// Update updates an existing invoice in the database
	Update(ctx context.Context, invoice *Invoice) error
	// SoftDelete performs a soft delete on an invoice by its ID
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// FindAll retrieves all invoices with pagination (for admin)
	FindAll(ctx context.Context, limit, offset int) ([]Invoice, error)
	// CountByUserID counts invoices for a user
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	// CountAll counts all invoices
	CountAll(ctx context.Context) (int64, error)
}

// repository implements Repository interface
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Create inserts a new invoice into the database
func (r *repository) Create(ctx context.Context, invoice *Invoice) error {
	if err := r.db.WithContext(ctx).Create(invoice).Error; err != nil {
		return apperrors.WrapInternal(err)
	}
	return nil
}

// FindByID finds an invoice by its UUID
func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error) {
	var invoice Invoice
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&invoice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.WrapInternal(err)
	}
	return &invoice, nil
}

// FindByUserID finds all invoices for a user with pagination
func (r *repository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Invoice, error) {
	var invoices []Invoice
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&invoices).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return invoices, nil
}

// Update updates an existing invoice in the database
func (r *repository) Update(ctx context.Context, invoice *Invoice) error {
	result := r.db.WithContext(ctx).Save(invoice)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on an invoice by its ID
func (r *repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&Invoice{}, "id = ?", id)
	if result.Error != nil {
		return apperrors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// FindAll retrieves all invoices with pagination (for admin)
func (r *repository) FindAll(ctx context.Context, limit, offset int) ([]Invoice, error) {
	var invoices []Invoice
	query := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&invoices).Error; err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return invoices, nil
}

// CountByUserID counts invoices for a user
func (r *repository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Invoice{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}
	return count, nil
}

// CountAll counts all invoices
func (r *repository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Invoice{}).
		Where("deleted_at IS NULL").
		Count(&count).Error; err != nil {
		return 0, apperrors.WrapInternal(err)
	}
	return count, nil
}
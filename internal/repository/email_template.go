package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailTemplateRepository defines the interface for email template data access operations
type EmailTemplateRepository interface {
	Create(ctx context.Context, template *domain.EmailTemplate) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailTemplate, error)
	FindByName(ctx context.Context, name string) (*domain.EmailTemplate, error)
	FindByCategory(ctx context.Context, category string, limit, offset int) ([]*domain.EmailTemplate, int64, error)
	FindActive(ctx context.Context, category string) ([]*domain.EmailTemplate, error)
	FindAll(ctx context.Context, limit, offset int) ([]*domain.EmailTemplate, int64, error)
	Update(ctx context.Context, template *domain.EmailTemplate) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// emailTemplateRepository implements EmailTemplateRepository interface
type emailTemplateRepository struct {
	db *gorm.DB
}

// NewEmailTemplateRepository creates a new EmailTemplateRepository instance
func NewEmailTemplateRepository(db *gorm.DB) EmailTemplateRepository {
	return &emailTemplateRepository{db: db}
}

// Create inserts a new email template into the database
func (r *emailTemplateRepository) Create(ctx context.Context, template *domain.EmailTemplate) error {
	if err := r.db.WithContext(ctx).Create(template).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds an email template by its UUID
func (r *emailTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.EmailTemplate, error) {
	var template domain.EmailTemplate
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&template).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &template, nil
}

// FindByName finds an email template by its unique name
func (r *emailTemplateRepository) FindByName(ctx context.Context, name string) (*domain.EmailTemplate, error) {
	var template domain.EmailTemplate
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		First(&template).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &template, nil
}

// FindByCategory retrieves all email templates in a category with pagination
func (r *emailTemplateRepository) FindByCategory(ctx context.Context, category string, limit, offset int) ([]*domain.EmailTemplate, int64, error) {
	var templates []*domain.EmailTemplate
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.EmailTemplate{}).
		Where("category = ?", category).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Limit(limit).
		Offset(offset).
		Find(&templates).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return templates, total, nil
}

// FindActive retrieves all active email templates, optionally filtered by category
func (r *emailTemplateRepository) FindActive(ctx context.Context, category string) ([]*domain.EmailTemplate, error) {
	var templates []*domain.EmailTemplate
	query := r.db.WithContext(ctx).Where("is_active = ?", true)

	if category != "" {
		query = query.Where("category = ?", category)
	}

	err := query.Find(&templates).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}

	return templates, nil
}

// FindAll retrieves all email templates with pagination
func (r *emailTemplateRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.EmailTemplate, int64, error) {
	var templates []*domain.EmailTemplate
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&domain.EmailTemplate{}).Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch with pagination
	err := r.db.WithContext(ctx).
		Limit(limit).
		Offset(offset).
		Find(&templates).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return templates, total, nil
}

// Update updates an existing email template in the database
func (r *emailTemplateRepository) Update(ctx context.Context, template *domain.EmailTemplate) error {
	result := r.db.WithContext(ctx).Save(template)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on an email template by its ID
func (r *emailTemplateRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.EmailTemplate{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}
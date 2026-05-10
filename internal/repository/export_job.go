package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExportJobRepository interface {
	Create(ctx context.Context, job *domain.ExportJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ExportJob, error)
	FindByStatus(ctx context.Context, status string, page, pageSize int) ([]*domain.ExportJob, int64, error)
	FindByOrgID(ctx context.Context, orgID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error)
	FindByCreatedBy(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error
	UpdateFilePath(ctx context.Context, id uuid.UUID, filePath string, recordCount int, hmacSignature string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error)
	FindExpired(ctx context.Context) ([]*domain.ExportJob, error)
	ClearFileRefs(ctx context.Context, id uuid.UUID) error
}

type exportJobRepository struct {
	db *gorm.DB
}

func NewExportJobRepository(db *gorm.DB) ExportJobRepository {
	return &exportJobRepository{db: db}
}

func (r *exportJobRepository) Create(ctx context.Context, job *domain.ExportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

func (r *exportJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ExportJob, error) {
	var job domain.ExportJob
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &job, nil
}

func (r *exportJobRepository) FindByStatus(ctx context.Context, status string, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	var jobs []*domain.ExportJob
	var total int64

	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("status = ?", status)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&jobs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return jobs, total, nil
}

func (r *exportJobRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	var jobs []*domain.ExportJob
	var total int64

	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("org_id = ?", orgID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&jobs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return jobs, total, nil
}

func (r *exportJobRepository) FindByCreatedBy(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	var jobs []*domain.ExportJob
	var total int64

	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("created_by = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&jobs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return jobs, total, nil
}

func (r *exportJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}

	result := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *exportJobRepository) UpdateFilePath(ctx context.Context, id uuid.UUID, filePath string, recordCount int, hmacSignature string) error {
	result := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"file_path":      filePath,
		"record_count":   recordCount,
		"hmac_signature": hmacSignature,
	})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *exportJobRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.ExportJob{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *exportJobRepository) List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ExportJob, int64, error) {
	var jobs []*domain.ExportJob
	var total int64

	offset := (page - 1) * pageSize

	db := r.db.WithContext(ctx).Model(&domain.ExportJob{})
	if orgID != nil {
		db = db.Where("org_id = ?", orgID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&jobs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return jobs, total, nil
}

func (r *exportJobRepository) FindExpired(ctx context.Context) ([]*domain.ExportJob, error) {
	var jobs []*domain.ExportJob
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("file_expires_at IS NOT NULL AND file_expires_at < ? AND file_path IS NOT NULL AND deleted_at IS NULL", now).
		Find(&jobs).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return jobs, nil
}

func (r *exportJobRepository) ClearFileRefs(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.ExportJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"file_path":      nil,
		"hmac_signature": nil,
	})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}
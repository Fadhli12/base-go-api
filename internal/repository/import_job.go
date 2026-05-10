package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImportJobRepository interface {
	Create(ctx context.Context, job *domain.ImportJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)
	FindByIdempotencyKey(ctx context.Context, key string) (*domain.ImportJob, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error
	UpdateResult(ctx context.Context, id uuid.UUID, result domain.ImportResult) error
	UpdateSourceFilePath(ctx context.Context, id uuid.UUID, path string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error)
	FindQueued(ctx context.Context, limit int) ([]*domain.ImportJob, error)
	FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.ImportJob, error)
	UpdateProcessingStartedAt(ctx context.Context, id uuid.UUID, startedAt time.Time) error
}

type importJobRepository struct {
	db *gorm.DB
}

func NewImportJobRepository(db *gorm.DB) ImportJobRepository {
	return &importJobRepository{db: db}
}

func (r *importJobRepository) Create(ctx context.Context, job *domain.ImportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

func (r *importJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	var job domain.ImportJob
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

func (r *importJobRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.ImportJob, error) {
	var job domain.ImportJob
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return &job, nil
}

func (r *importJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}

	result := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *importJobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result domain.ImportResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return errors.WrapInternal(err)
	}

	result2 := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Update("result", resultJSON)
	if result2.Error != nil {
		return errors.WrapInternal(result2.Error)
	}
	if result2.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *importJobRepository) UpdateSourceFilePath(ctx context.Context, id uuid.UUID, path string) error {
	result := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Update("source_file_path", path)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *importJobRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.ImportJob{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (r *importJobRepository) List(ctx context.Context, orgID *uuid.UUID, page, pageSize int) ([]*domain.ImportJob, int64, error) {
	var jobs []*domain.ImportJob
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.ImportJob{})
	if orgID == nil {
		db = db.Where("org_id IS NULL")
	} else {
		db = db.Where("org_id = ?", orgID)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	offset := (page - 1) * pageSize
	err := db.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&jobs).Error
	if err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return jobs, total, nil
}

func (r *importJobRepository) FindQueued(ctx context.Context, limit int) ([]*domain.ImportJob, error) {
	var jobs []*domain.ImportJob
	err := r.db.WithContext(ctx).
		Where("status = ?", domain.ImportQueued).
		Order("created_at ASC").
		Limit(limit).
		Find(&jobs).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return jobs, nil
}

func (r *importJobRepository) FindStuckProcessing(ctx context.Context, timeout time.Duration) ([]*domain.ImportJob, error) {
	var jobs []*domain.ImportJob
	cutoff := time.Now().Add(-timeout)
	err := r.db.WithContext(ctx).
		Where("status = ? AND processing_started_at < ?", domain.ImportProcessing, cutoff).
		Find(&jobs).Error
	if err != nil {
		return nil, errors.WrapInternal(err)
	}
	return jobs, nil
}

func (r *importJobRepository) UpdateProcessingStartedAt(ctx context.Context, id uuid.UUID, startedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Update("processing_started_at", startedAt)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}
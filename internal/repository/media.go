package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MediaRepository defines the interface for media data access operations
type MediaRepository interface {
	// Create inserts a new media record into the database
	Create(ctx context.Context, media *domain.Media) error
	// FindByID finds a media record by its UUID
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Media, error)
	// FindByModelTypeAndID finds all media for a specific model
	FindByModelTypeAndID(ctx context.Context, modelType string, modelID uuid.UUID, collection string) ([]*domain.Media, error)
	// FindByFilename finds a media record by filename
	FindByFilename(ctx context.Context, filename string) (*domain.Media, error)
	// Update updates an existing media record
	Update(ctx context.Context, media *domain.Media) error
	// SoftDelete performs a soft delete on a media record
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// MarkOrphaned marks media as orphaned for cleanup
	MarkOrphaned(ctx context.Context, id uuid.UUID) error
	// FindOrphaned finds media records marked as orphaned before a cutoff time
	FindOrphaned(ctx context.Context, cutoff time.Time) ([]*domain.Media, error)
	// HardDelete permanently deletes a media record (for cleanup jobs)
	HardDelete(ctx context.Context, id uuid.UUID) error

	// Conversion methods
	// CreateConversion inserts a new media conversion record
	CreateConversion(ctx context.Context, conversion *domain.MediaConversion) error
	// FindConversionsByMediaID finds all conversions for a media record
	FindConversionsByMediaID(ctx context.Context, mediaID uuid.UUID) ([]*domain.MediaConversion, error)
	// DeleteConversion deletes a specific conversion
	DeleteConversion(ctx context.Context, mediaID uuid.UUID, name string) error

	// Download audit methods
	// CreateDownload records a media download
	CreateDownload(ctx context.Context, download *domain.MediaDownload) error
	// CountDownloads counts downloads for a media record
	CountDownloads(ctx context.Context, mediaID uuid.UUID) (int64, error)
}

// mediaRepository implements MediaRepository interface
type mediaRepository struct {
	db *gorm.DB
}

// NewMediaRepository creates a new MediaRepository instance
func NewMediaRepository(db *gorm.DB) MediaRepository {
	return &mediaRepository{
		db: db,
	}
}

// Create inserts a new media record into the database
func (r *mediaRepository) Create(ctx context.Context, media *domain.Media) error {
	if err := r.db.WithContext(ctx).Create(media).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a media record by its UUID
func (r *mediaRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Media, error) {
	var media domain.Media
	if err := r.db.WithContext(ctx).Preload("Conversions").First(&media, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &media, nil
}

// FindByModelTypeAndID finds all media for a specific model
func (r *mediaRepository) FindByModelTypeAndID(ctx context.Context, modelType string, modelID uuid.UUID, collection string) ([]*domain.Media, error) {
	var media []*domain.Media
	query := r.db.WithContext(ctx).
		Preload("Conversions").
		Where("model_type = ? AND model_id = ?", modelType, modelID)

	if collection != "" {
		query = query.Where("collection_name = ?", collection)
	}

	if err := query.Order("created_at DESC").Find(&media).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return media, nil
}

// FindByFilename finds a media record by filename
func (r *mediaRepository) FindByFilename(ctx context.Context, filename string) (*domain.Media, error) {
	var media domain.Media
	if err := r.db.WithContext(ctx).Preload("Conversions").Where("filename = ?", filename).First(&media).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &media, nil
}

// Update updates an existing media record
func (r *mediaRepository) Update(ctx context.Context, media *domain.Media) error {
	result := r.db.WithContext(ctx).Save(media)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDelete performs a soft delete on a media record
func (r *mediaRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.Media{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkOrphaned marks media as orphaned for cleanup
func (r *mediaRepository) MarkOrphaned(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.Media{}).
		Where("id = ?", id).
		Update("orphaned_at", now)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// FindOrphaned finds media records marked as orphaned before a cutoff time
func (r *mediaRepository) FindOrphaned(ctx context.Context, cutoff time.Time) ([]*domain.Media, error) {
	var media []*domain.Media
	if err := r.db.WithContext(ctx).
		Preload("Conversions").
		Where("orphaned_at IS NOT NULL AND orphaned_at < ?", cutoff).
		Find(&media).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return media, nil
}

// HardDelete permanently deletes a media record (for cleanup jobs)
func (r *mediaRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Unscoped().Delete(&domain.Media{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// CreateConversion inserts a new media conversion record
func (r *mediaRepository) CreateConversion(ctx context.Context, conversion *domain.MediaConversion) error {
	if err := r.db.WithContext(ctx).Create(conversion).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindConversionsByMediaID finds all conversions for a media record
func (r *mediaRepository) FindConversionsByMediaID(ctx context.Context, mediaID uuid.UUID) ([]*domain.MediaConversion, error) {
	var conversions []*domain.MediaConversion
	if err := r.db.WithContext(ctx).
		Where("media_id = ?", mediaID).
		Order("created_at DESC").
		Find(&conversions).Error; err != nil {
		return nil, errors.WrapInternal(err)
	}
	return conversions, nil
}

// DeleteConversion deletes a specific conversion
func (r *mediaRepository) DeleteConversion(ctx context.Context, mediaID uuid.UUID, name string) error {
	result := r.db.WithContext(ctx).
		Where("media_id = ? AND name = ?", mediaID, name).
		Delete(&domain.MediaConversion{})
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// CreateDownload records a media download
func (r *mediaRepository) CreateDownload(ctx context.Context, download *domain.MediaDownload) error {
	if err := r.db.WithContext(ctx).Create(download).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// CountDownloads counts downloads for a media record
func (r *mediaRepository) CountDownloads(ctx context.Context, mediaID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.MediaDownload{}).
		Where("media_id = ?", mediaID).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MediaVersionRepository defines the interface for media version data access operations
type MediaVersionRepository interface {
	// Create inserts a new media version record
	Create(ctx context.Context, version *domain.MediaVersion) error
	// FindByID finds a media version by its UUID
	FindByID(ctx context.Context, id uuid.UUID) (*domain.MediaVersion, error)
	// FindByMediaIDAndVersion finds a specific version of a media by media ID and version number
	FindByMediaIDAndVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersion, error)
	// FindByMediaID finds all versions for a media, paginated, excluding soft-deleted
	FindByMediaID(ctx context.Context, mediaID uuid.UUID, limit, offset int) ([]*domain.MediaVersion, int64, error)
	// SoftDelete performs a soft delete on a media version
	SoftDelete(ctx context.Context, id uuid.UUID) error
	// CountByMediaID counts non-deleted versions for a media
	CountByMediaID(ctx context.Context, mediaID uuid.UUID) (int64, error)
	// FindCurrentVersion finds the version matching the media's current_version field
	FindCurrentVersion(ctx context.Context, mediaID uuid.UUID, currentVersion int) (*domain.MediaVersion, error)
	// FindByChecksum finds a version by media ID and checksum (for duplicate detection)
	FindByChecksum(ctx context.Context, mediaID uuid.UUID, checksum string) (*domain.MediaVersion, error)
	// UpdateCurrentVersion atomically updates the media's current_version with optimistic locking
	UpdateCurrentVersion(ctx context.Context, mediaID uuid.UUID, oldVersion, newVersion int) error
}

// mediaVersionRepository implements MediaVersionRepository interface
type mediaVersionRepository struct {
	db *gorm.DB
}

// NewMediaVersionRepository creates a new MediaVersionRepository instance
func NewMediaVersionRepository(db *gorm.DB) MediaVersionRepository {
	return &mediaVersionRepository{
		db: db,
	}
}

// Create inserts a new media version record
func (r *mediaVersionRepository) Create(ctx context.Context, version *domain.MediaVersion) error {
	if err := r.db.WithContext(ctx).Create(version).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID finds a media version by its UUID
func (r *mediaVersionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.MediaVersion, error) {
	var version domain.MediaVersion
	if err := r.db.WithContext(ctx).First(&version, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &version, nil
}

// FindByMediaIDAndVersion finds a specific version of a media by media ID and version number
func (r *mediaVersionRepository) FindByMediaIDAndVersion(ctx context.Context, mediaID uuid.UUID, version int) (*domain.MediaVersion, error) {
	var mv domain.MediaVersion
	if err := r.db.WithContext(ctx).
		Where("media_id = ? AND version = ? AND deleted_at IS NULL", mediaID, version).
		First(&mv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &mv, nil
}

// FindByMediaID finds all versions for a media, paginated, excluding soft-deleted
func (r *mediaVersionRepository) FindByMediaID(ctx context.Context, mediaID uuid.UUID, limit, offset int) ([]*domain.MediaVersion, int64, error) {
	var versions []*domain.MediaVersion
	var total int64

	query := r.db.WithContext(ctx).
		Where("media_id = ? AND deleted_at IS NULL", mediaID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	if err := query.
		Order("version DESC").
		Limit(limit).
		Offset(offset).
		Find(&versions).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return versions, total, nil
}

// SoftDelete performs a soft delete on a media version
func (r *mediaVersionRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.MediaVersion{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// CountByMediaID counts non-deleted versions for a media
func (r *mediaVersionRepository) CountByMediaID(ctx context.Context, mediaID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&domain.MediaVersion{}).
		Where("media_id = ? AND deleted_at IS NULL", mediaID).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}
	return count, nil
}

// FindCurrentVersion finds the version matching the media's current_version field
func (r *mediaVersionRepository) FindCurrentVersion(ctx context.Context, mediaID uuid.UUID, currentVersion int) (*domain.MediaVersion, error) {
	var mv domain.MediaVersion
	if err := r.db.WithContext(ctx).
		Where("media_id = ? AND version = ? AND deleted_at IS NULL", mediaID, currentVersion).
		First(&mv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &mv, nil
}

// UpdateCurrentVersion atomically updates the media's current_version with optimistic locking
func (r *mediaVersionRepository) UpdateCurrentVersion(ctx context.Context, mediaID uuid.UUID, oldVersion, newVersion int) error {
	result := r.db.WithContext(ctx).Model(&domain.Media{}).
		Where("id = ? AND current_version = ?", mediaID, oldVersion).
		Update("current_version", newVersion)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewAppError("CONFLICT", "Concurrent modification detected", 409)
	}
	return nil
}

// FindByChecksum finds a version by media ID and checksum (for duplicate detection)
func (r *mediaVersionRepository) FindByChecksum(ctx context.Context, mediaID uuid.UUID, checksum string) (*domain.MediaVersion, error) {
	var mv domain.MediaVersion
	if err := r.db.WithContext(ctx).
		Where("media_id = ? AND checksum = ? AND deleted_at IS NULL", mediaID, checksum).
		First(&mv).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &mv, nil
}

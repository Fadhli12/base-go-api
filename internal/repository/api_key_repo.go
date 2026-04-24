package repository

import (
	"context"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// apiKeyRepository implements APIKeyRepository interface
type apiKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository creates a new APIKeyRepository instance
func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepository{
		db: db,
	}
}

// Create inserts a new API key into the database
func (r *apiKeyRepository) Create(ctx context.Context, apiKey *domain.APIKey) error {
	if err := r.db.WithContext(ctx).Create(apiKey).Error; err != nil {
		// Check for unique constraint violation on key_hash
		if isUniqueConstraintError(err) {
			return errors.ErrConflict
		}
		return errors.WrapInternal(err)
	}
	return nil
}

// FindByID retrieves an API key by its ID
func (r *apiKeyRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	if err := r.db.WithContext(ctx).First(&apiKey, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &apiKey, nil
}

// FindByKeyHash retrieves an API key by its key hash (for authentication)
func (r *apiKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	if err := r.db.WithContext(ctx).
		Where("key_hash = ?", keyHash).
		First(&apiKey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &apiKey, nil
}

// FindByIDWithUser retrieves an API key by ID with user preloaded
func (r *apiKeyRepository) FindByIDWithUser(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	if err := r.db.WithContext(ctx).
		Preload("User").
		First(&apiKey, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &apiKey, nil
}

// FindByKeyHashWithUser retrieves an API key by hash with user preloaded
func (r *apiKeyRepository) FindByKeyHashWithUser(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	var apiKey domain.APIKey
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("key_hash = ?", keyHash).
		First(&apiKey).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapInternal(err)
	}
	return &apiKey, nil
}

// FindByUserID retrieves all API keys for a user with pagination
func (r *apiKeyRepository) FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error) {
	var apiKeys []domain.APIKey
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).
		Model(&domain.APIKey{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	// Fetch paginated results (soft delete filtered automatically by GORM)
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&apiKeys).Error; err != nil {
		return nil, 0, errors.WrapInternal(err)
	}

	return apiKeys, total, nil
}

// Update updates an existing API key
func (r *apiKeyRepository) Update(ctx context.Context, apiKey *domain.APIKey) error {
	result := r.db.WithContext(ctx).Save(apiKey)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// Revoke marks an API key as revoked (sets revoked_at timestamp)
func (r *apiKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&domain.APIKey{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now)

	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		// Check if key exists at all
		var count int64
		if err := r.db.WithContext(ctx).Model(&domain.APIKey{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return errors.WrapInternal(err)
		}
		if count == 0 {
			return errors.ErrNotFound
		}
		// Key exists but is already revoked
		return errors.ErrConflict
	}
	return nil
}

// SoftDelete performs a soft delete on an API key
func (r *apiKeyRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&domain.APIKey{}, "id = ?", id)
	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// SoftDeleteByUserID soft deletes all API keys for a user
func (r *apiKeyRepository) SoftDeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&domain.APIKey{}).Error; err != nil {
		return errors.WrapInternal(err)
	}
	return nil
}

// UpdateLastUsedAt atomically updates the last_used_at timestamp
// Uses atomic UPDATE to prevent race conditions
func (r *apiKeyRepository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&domain.APIKey{}).
		Where("id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", id, now).
		Update("last_used_at", now)

	if result.Error != nil {
		return errors.WrapInternal(result.Error)
	}
	// Don't return error if no rows affected - key may be revoked/expired
	// Calling code should have already validated the key is active
	return nil
}

// CountActiveByUserID counts active (non-revoked, non-expired, non-deleted) keys for a user
func (r *apiKeyRepository) CountActiveByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	now := time.Now()

	if err := r.db.WithContext(ctx).
		Model(&domain.APIKey{}).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&count).Error; err != nil {
		return 0, errors.WrapInternal(err)
	}

	return count, nil
}

// isUniqueConstraintError checks if an error is a unique constraint violation
func isUniqueConstraintError(err error) bool {
	// PostgreSQL unique constraint error
	// Error code 23505 = unique_violation
	return err != nil && (containsString(err.Error(), "duplicate key") ||
		containsString(err.Error(), "unique constraint") ||
		containsString(err.Error(), "23505"))
}

// containsString checks if s contains substr (case-insensitive helper)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && containsSubstring(s, substr)))
}

// containsSubstring is a simple substring check
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
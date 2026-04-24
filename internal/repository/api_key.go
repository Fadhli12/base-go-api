package repository

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
)

// APIKeyRepository defines the interface for API key data access operations.
// All methods use context for cancellation and trace propagation.
type APIKeyRepository interface {
	// Create inserts a new API key into the database.
	// Returns error if key_hash already exists (unique constraint).
	Create(ctx context.Context, apiKey *domain.APIKey) error

	// FindByID retrieves an API key by its ID.
	// Returns ErrNotFound if the key does not exist.
	FindByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)

	// FindByKeyHash retrieves an API key by its key hash (for authentication).
	// Returns ErrNotFound if the key does not exist.
	// Does NOT load user association - use FindByIDWithUser if needed.
	FindByKeyHash(ctx context.Context, keyHash string) (*domain.APIKey, error)

	// FindByIDWithUser retrieves an API key by ID with user preloaded.
	// Returns ErrNotFound if the key does not exist.
	FindByIDWithUser(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)

	// FindByKeyHashWithUser retrieves an API key by hash with user preloaded.
	// Returns ErrNotFound if the key does not exist.
	// Use for authentication where user info is needed.
	FindByKeyHashWithUser(ctx context.Context, keyHash string) (*domain.APIKey, error)

	// FindByUserID retrieves all API keys for a user with pagination.
	// Returns keys sorted by created_at DESC.
	// Excludes soft-deleted keys automatically.
	FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error)

	// Update updates an existing API key.
	// Returns ErrNotFound if the key does not exist.
	Update(ctx context.Context, apiKey *domain.APIKey) error

	// Revoke marks an API key as revoked (sets revoked_at timestamp).
	// Returns ErrNotFound if the key does not exist.
	// Returns error if key is already revoked.
	Revoke(ctx context.Context, id uuid.UUID) error

	// SoftDelete performs a soft delete on an API key.
	// Returns ErrNotFound if the key does not exist.
	// Note: This is typically triggered by user cascade delete.
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// SoftDeleteByUserID soft deletes all API keys for a user.
	// Used when user is deleted.
	SoftDeleteByUserID(ctx context.Context, userID uuid.UUID) error

	// UpdateLastUsedAt atomically updates the last_used_at timestamp.
	// Uses atomic UPDATE to prevent race conditions.
	// Returns ErrNotFound if the key does not exist.
	UpdateLastUsedAt(ctx context.Context, id uuid.UUID) error

	// CountActiveByUserID counts active (non-revoked, non-expired, non-deleted) keys for a user.
	// Used for rate limiting and quota enforcement.
	CountActiveByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
}
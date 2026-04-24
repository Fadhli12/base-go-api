package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

// KeyPrefix constants for API key format
const (
	KeyPrefixLive = "ak_live_"
	KeyPrefixTest = "ak_test_"
	KeyRandomLen  = 32 // Number of random characters in key
	KeyPrefixLen  = 12 // Characters to store as prefix (ak_env_XXXX)
)

// APIKeyService handles API key business logic
type APIKeyService struct {
	apiKeyRepo repository.APIKeyRepository
	userRepo   repository.UserRepository
	auditSvc   *AuditService
}

// NewAPIKeyService creates a new APIKeyService instance
func NewAPIKeyService(
	apiKeyRepo repository.APIKeyRepository,
	userRepo repository.UserRepository,
	auditSvc *AuditService,
) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
		auditSvc:   auditSvc,
	}
}

// APIKeyWithSecret contains an API key response with the raw secret (only on creation)
type APIKeyWithSecret struct {
	APIKey  *domain.APIKeyResponse
	Secret  string // Full key, only returned once on creation
	KeyHash string // For testing/debugging
}

// Create creates a new API key for a user
func (s *APIKeyService) Create(ctx context.Context, userID uuid.UUID, req request.CreateAPIKeyRequest) (*APIKeyWithSecret, error) {
	// Validate user exists
	_, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewAppError("USER_NOT_FOUND", "User not found", 404)
		}
		return nil, apperrors.WrapInternal(err)
	}

	// Validate expiration if set
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "Expiration must be in the future", 422)
	}

	// Validate scopes format
	for _, scope := range req.Scopes {
		if !request.IsValidScope(scope) {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("Invalid scope format: %s", scope), 422)
		}
	}

	// Generate key and hash
	secret, keyHash, prefix, err := s.generateKey()
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Marshal scopes to JSON
	scopesJSON, err := json.Marshal(req.Scopes)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Create API key entity
	apiKey := &domain.APIKey{
		UserID:    userID,
		Name:      req.Name,
		Prefix:    prefix,
		KeyHash:   keyHash,
		Scopes:    datatypes.JSON(scopesJSON),
		ExpiresAt: req.ExpiresAt,
	}

	// Persist
	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			return nil, apperrors.NewAppError("CONFLICT", "API key name already exists for this user", 409)
		}
		return nil, apperrors.WrapInternal(err)
	}

	// Audit log (async)
	auditData := map[string]interface{}{
		"id":      apiKey.ID.String(),
		"name":    apiKey.Name,
		"prefix":  apiKey.Prefix,
		"scopes":  req.Scopes,
		"user_id": userID.String(),
	}
	_ = s.auditSvc.LogAction(ctx, userID, "api_key:create", "api_key", apiKey.ID.String(), nil, auditData, "", "")

	response := apiKey.ToResponse()
	return &APIKeyWithSecret{
		APIKey:  &response,
		Secret:  secret,
		KeyHash: keyHash,
	}, nil
}

// List retrieves all API keys for a user with pagination
func (s *APIKeyService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.APIKey, int64, error) {
	// Set defaults
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	apiKeys, total, err := s.apiKeyRepo.FindByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	return apiKeys, total, nil
}

// GetByID retrieves a specific API key by ID with ownership check
func (s *APIKeyService) GetByID(ctx context.Context, userID, keyID uuid.UUID) (*domain.APIKey, error) {
	apiKey, err := s.apiKeyRepo.FindByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewAppError("NOT_FOUND", "API key not found", 404)
		}
		return nil, apperrors.WrapInternal(err)
	}

	// Ownership check
	if !apiKey.IsOwnedBy(userID) {
		return nil, apperrors.NewAppError("FORBIDDEN", "API key belongs to another user", 403)
	}

	return apiKey, nil
}

// Revoke revokes an API key (soft deletion) with ownership check
func (s *APIKeyService) Revoke(ctx context.Context, userID, keyID uuid.UUID) error {
	// Get and check ownership
	apiKey, err := s.GetByID(ctx, userID, keyID)
	if err != nil {
		return err
	}

	// Check if already revoked
	if apiKey.IsRevoked() {
		return apperrors.NewAppError("CONFLICT", "API key already revoked", 409)
	}

	// Revoke
	if err := s.apiKeyRepo.Revoke(ctx, keyID); err != nil {
		if errors.Is(err, apperrors.ErrConflict) {
			return apperrors.NewAppError("CONFLICT", "API key already revoked", 409)
		}
		return apperrors.WrapInternal(err)
	}

	// Audit log (async)
	auditData := map[string]interface{}{
		"id":      keyID.String(),
		"name":    apiKey.Name,
		"user_id": userID.String(),
	}
	_ = s.auditSvc.LogAction(ctx, userID, "api_key:revoke", "api_key", keyID.String(), auditData, nil, "", "")

	return nil
}

// Validate validates an API key for authentication
// Returns the API key and associated user on success
// Updates last_used_at if more than 1 minute has passed since last update
func (s *APIKeyService) Validate(ctx context.Context, key string) (*domain.APIKey, *domain.User, error) {
	// Hash the key for lookup
	keyHash, err := s.hashKey(key)
	if err != nil {
		return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "Invalid API key", 401)
	}

	// Find key with user
	apiKey, err := s.apiKeyRepo.FindByKeyHashWithUser(ctx, keyHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "Invalid API key", 401)
		}
		return nil, nil, apperrors.WrapInternal(err)
	}

	// Check active state
	if !apiKey.IsActive() {
		if apiKey.IsRevoked() {
			return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "API key has been revoked", 401)
		}
		if apiKey.IsExpired() {
			return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "API key has expired", 401)
		}
		return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "API key is inactive", 401)
	}

	// Check user exists and not deleted
	if apiKey.User.ID == uuid.Nil {
		return nil, nil, apperrors.NewAppError("UNAUTHORIZED", "User not found", 401)
	}

	// Update last_used_at (atomic, with 1-minute throttle)
	if s.shouldUpdateLastUsedAt(apiKey) {
		// Fire-and-forget update (don't block authentication)
		go func() {
			// Use background context for async update
			bgCtx := context.Background()
			_ = s.apiKeyRepo.UpdateLastUsedAt(bgCtx, apiKey.ID)
		}()
	}

	return apiKey, &apiKey.User, nil
}

// ValidateScopes validates that the requested scopes are valid and a subset of user's permissions
// This is a placeholder - actual permission check would query the permission system
func (s *APIKeyService) ValidateScopes(ctx context.Context, userID uuid.UUID, scopes []string) error {
	if len(scopes) == 0 {
		return apperrors.NewAppError("VALIDATION_ERROR", "At least one scope is required", 422)
	}

	// Validate format
	for _, scope := range scopes {
		if !request.IsValidScope(scope) {
			return apperrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("Invalid scope format: %s", scope), 422)
		}
	}

	// TODO: Actual permission validation against user's effective permissions
	// This would require integrating with the permission enforcer
	// For now, we accept all valid-format scopes

	return nil
}

// generateKey generates a new API key with prefix, hash, and full secret
func (s *APIKeyService) generateKey() (secret, keyHash, prefix string, err error) {
	// Generate random portion (32 bytes = 64 hex chars, but we'll use as-is)
	bytes := make([]byte, KeyRandomLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create full key: ak_live_<random>
	randomPart := hex.EncodeToString(bytes)
	secret = KeyPrefixLive + randomPart

	// Extract prefix for identification (first 12 characters)
	prefix = secret[:KeyPrefixLen]

	// Hash for storage (bcrypt cost 12)
	keyHash, err = s.hashKey(secret)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to hash key: %w", err)
	}

	return secret, keyHash, prefix, nil
}

// hashKey hashes an API key using bcrypt (cost 12)
func (s *APIKeyService) hashKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}
	return string(hash), nil
}

// verifyKey verifies an API key against a hash (for testing)
func (s *APIKeyService) verifyKey(hashedKey, key string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedKey), []byte(key))
}

// shouldUpdateLastUsedAt checks if last_used_at should be updated (throttle to 1 minute)
func (s *APIKeyService) shouldUpdateLastUsedAt(apiKey *domain.APIKey) bool {
	if apiKey.LastUsedAt == nil {
		return true
	}
	// Only update if more than 1 minute has passed
	return time.Since(*apiKey.LastUsedAt) > time.Minute
}

// CountActiveByUserID counts active API keys for a user
func (s *APIKeyService) CountActiveByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.apiKeyRepo.CountActiveByUserID(ctx, userID)
}
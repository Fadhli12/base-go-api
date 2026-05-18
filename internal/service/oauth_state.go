package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// OAuthStateKeyPrefix is the Redis key prefix for OAuth state entries.
	OAuthStateKeyPrefix = "oauth:state:"

	// PKCEVerifierLength is the length of the PKCE code_verifier (43-128 chars per RFC 7636).
	// 43 bytes of random data encoded as base64url produces 43+ characters.
	PKCEVerifierLength = 43

	// NonceBytes is the number of random bytes used to generate the OAuth state nonce.
	NonceBytes = 32
)

// pkceUnreservedChars contains the RFC 7636 unreserved character set for code_verifier generation.
const pkceUnreservedChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

// OAuthStateManager defines the interface for managing OAuth state parameters.
// State is stored in Redis with a TTL for CSRF protection and PKCE verification.
type OAuthStateManager interface {
	// CreateState generates a random nonce and PKCE code_verifier, stores the state
	// in Redis with TTL, and returns both the nonce and code_verifier.
	// The code_challenge is computed internally using S256 (SHA256 + base64url).
	CreateState(ctx context.Context, callbackURL, provider, intent string, userID, orgID uuid.UUID) (nonce string, codeVerifier string, err error)

	// ValidateState retrieves and validates the state from Redis by nonce.
	// IMPORTANT: This method deletes the state from Redis after reading to prevent
	// replay attacks. Each state can only be consumed once.
	ValidateState(ctx context.Context, nonce string) (*domain.OAuthState, error)

	// DeleteState removes a state from Redis explicitly (for cleanup).
	DeleteState(ctx context.Context, nonce string) error
}

// redisOAuthStateManager implements OAuthStateManager using Redis.
type redisOAuthStateManager struct {
	client *redis.Client
	cfg    config.OAuthConfig
	logger logger.Logger
}

// NewRedisOAuthStateManager creates a new Redis-backed OAuth state manager.
func NewRedisOAuthStateManager(client *redis.Client, cfg config.OAuthConfig, log logger.Logger) OAuthStateManager {
	return &redisOAuthStateManager{
		client: client,
		cfg:    cfg,
		logger: log,
	}
}

// CreateState generates a random nonce and PKCE code_verifier, stores the state
// in Redis with TTL, and returns both the nonce and code_verifier.
func (m *redisOAuthStateManager) CreateState(
	ctx context.Context,
	callbackURL, provider, intent string,
	userID, orgID uuid.UUID,
) (string, string, error) {
	// 1. Generate random nonce (32 bytes, hex-encoded)
	nonce, err := generateRandomHex(NonceBytes)
	if err != nil {
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to generate nonce: %w", err))
	}

	// 2. Generate PKCE code_verifier (RFC 7636 unreserved chars, 43-128 length)
	codeVerifier, err := generatePKCEVerifier()
	if err != nil {
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to generate code_verifier: %w", err))
	}

	// 3. Compute code_challenge using S256 method: base64url(sha256(code_verifier))
	// Note: code_challenge is NOT stored in Redis; it's computed by the handler for the auth URL.

	// 4. Create OAuthState struct
	state := &domain.OAuthState{
		CallbackURL:  callbackURL,
		Provider:     provider,
		Intent:       intent,
		UserID:       userID,
		CodeVerifier: codeVerifier,
		OrgID:        orgID,
		CreatedAt:    time.Now(),
	}

	// 5. Marshal to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to marshal oauth state: %w", err))
	}

	// 6. Store in Redis with TTL from config
	key := fmt.Sprintf("%s%s", OAuthStateKeyPrefix, nonce)
	ttl := m.cfg.StateTTL
	if ttl == 0 {
		ttl = 600 * time.Second // Default: 10 minutes
	}

	if err := m.client.Set(ctx, key, data, ttl).Err(); err != nil {
		m.logger.Error(ctx, "failed to store oauth state in redis",
			logger.String("nonce", nonce),
			logger.Err(err),
		)
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to store oauth state: %w", err))
	}

	m.logger.Debug(ctx, "oauth state created",
		logger.String("nonce", nonce),
		logger.String("provider", provider),
		logger.String("intent", intent),
	)

	// 7. Return nonce and code_verifier (NOT code_challenge)
	return nonce, codeVerifier, nil
}

// ValidateState retrieves and validates the state from Redis by nonce.
// The state is deleted after reading to prevent replay attacks.
func (m *redisOAuthStateManager) ValidateState(ctx context.Context, nonce string) (*domain.OAuthState, error) {
	if nonce == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "nonce is required", 422)
	}

	key := fmt.Sprintf("%s%s", OAuthStateKeyPrefix, nonce)

	// 1. Get from Redis
	data, err := m.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			m.logger.Warn(ctx, "oauth state not found or expired",
				logger.String("nonce", nonce),
			)
			return nil, apperrors.NewAppError("NOT_FOUND", "oauth state not found or expired", 404)
		}
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to get oauth state: %w", err))
	}

	// 2-4. Delete the state immediately to prevent replay attacks
	if err := m.client.Del(ctx, key).Err(); err != nil {
		m.logger.Warn(ctx, "failed to delete consumed oauth state",
			logger.String("nonce", nonce),
			logger.Err(err),
		)
		// Don't fail the request — the state was already retrieved
	}

	// 5. Unmarshal and return
	var state domain.OAuthState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to unmarshal oauth state: %w", err))
	}

	m.logger.Debug(ctx, "oauth state validated and consumed",
		logger.String("nonce", nonce),
		logger.String("provider", state.Provider),
	)

	return &state, nil
}

// DeleteState removes a state from Redis explicitly (for cleanup).
func (m *redisOAuthStateManager) DeleteState(ctx context.Context, nonce string) error {
	if nonce == "" {
		return nil
	}

	key := fmt.Sprintf("%s%s", OAuthStateKeyPrefix, nonce)
	if err := m.client.Del(ctx, key).Err(); err != nil {
		m.logger.Warn(ctx, "failed to delete oauth state",
			logger.String("nonce", nonce),
			logger.Err(err),
		)
		return apperrors.WrapInternal(fmt.Errorf("failed to delete oauth state: %w", err))
	}

	m.logger.Debug(ctx, "oauth state deleted",
		logger.String("nonce", nonce),
	)
	return nil
}

// ComputePKCECodeChallenge computes the S256 code_challenge from a code_verifier
// per RFC 7636: base64url(sha256(code_verifier)) with padding removed.
func ComputePKCECodeChallenge(codeVerifier string) string {
	h := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateRandomHex generates n random bytes and returns their hex-encoded string.
func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generatePKCEVerifier generates a cryptographically random code_verifier
// using the RFC 7636 unreserved character set [A-Z][a-z][0-9]-._~ with length 43.
func generatePKCEVerifier() (string, error) {
	result := make([]byte, PKCEVerifierLength)
	for i := range result {
		randomBytes := make([]byte, 1)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random byte for code_verifier: %w", err)
		}
		result[i] = pkceUnreservedChars[int(randomBytes[0])%len(pkceUnreservedChars)]
	}
	return string(result), nil
}
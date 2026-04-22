package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/example/go-api-base/internal/domain"
	apperrors "github.com/example/go-api-base/pkg/errors"
)

// TokenService handles token generation, validation, and rotation
type TokenService struct {
	db                 *gorm.DB
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
	jwtSecret          string
}

// NewTokenService creates a new TokenService instance
func NewTokenService(db *gorm.DB, jwtSecret string, accessExpiry, refreshExpiry time.Duration) *TokenService {
	return &TokenService{
		db:                 db,
		jwtSecret:          jwtSecret,
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

// GenerateRefreshToken generates a new refresh token for a user
// Returns the RefreshToken entity (with hash stored) and the raw token string
func (s *TokenService) GenerateRefreshToken(userID uuid.UUID) (*domain.RefreshToken, string, error) {
	// Generate random token using crypto/rand
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return nil, "", apperrors.WrapInternal(fmt.Errorf("failed to generate random token: %w", err))
	}
	rawTokenString := hex.EncodeToString(rawToken)

	// Hash token using SHA256
	hash := sha256.Sum256([]byte(rawTokenString))
	tokenHash := hex.EncodeToString(hash[:])

	// Create refresh token entity
	refreshToken := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.refreshTokenExpiry),
	}

	// Store hash in DB
	if err := s.db.Create(refreshToken).Error; err != nil {
		return nil, "", apperrors.WrapInternal(fmt.Errorf("failed to store refresh token: %w", err))
	}

	return refreshToken, rawTokenString, nil
}

// ValidateRefreshToken validates a refresh token
// Looks up by hash and checks that it's not revoked and not expired
func (s *TokenService) ValidateRefreshToken(token string) (*domain.RefreshToken, error) {
	// Hash the provided token
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	// Look up by hash
	var refreshToken domain.RefreshToken
	if err := s.db.Where("token_hash = ?", tokenHash).First(&refreshToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.WrapUnauthorized(fmt.Errorf("invalid refresh token"))
		}
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to lookup refresh token: %w", err))
	}

	// Check if revoked
	if refreshToken.IsRevoked() {
		return nil, apperrors.WrapUnauthorized(fmt.Errorf("refresh token has been revoked"))
	}

	// Check if expired
	if !refreshToken.IsValid() {
		return nil, apperrors.WrapUnauthorized(fmt.Errorf("refresh token has expired"))
	}

	return &refreshToken, nil
}

// RotateRefreshToken rotates a refresh token (refresh token rotation for security)
// Validates the old token, revokes it, and generates a new one
func (s *TokenService) RotateRefreshToken(oldToken string) (*domain.RefreshToken, string, error) {
	// Validate old token
	oldRefreshToken, err := s.ValidateRefreshToken(oldToken)
	if err != nil {
		return nil, "", err
	}

	// Revoke old token
	now := time.Now()
	oldRefreshToken.RevokedAt = &now
	if err := s.db.Save(oldRefreshToken).Error; err != nil {
		return nil, "", apperrors.WrapInternal(fmt.Errorf("failed to revoke old refresh token: %w", err))
	}

	// Generate new token
	newRefreshToken, newRawToken, err := s.GenerateRefreshToken(oldRefreshToken.UserID)
	if err != nil {
		return nil, "", err
	}

	return newRefreshToken, newRawToken, nil
}

// RevokeRefreshToken revokes a refresh token by its hash
func (s *TokenService) RevokeRefreshToken(tokenHash string) error {
	now := time.Now()
	result := s.db.Model(&domain.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", now)

	if result.Error != nil {
		return apperrors.WrapInternal(fmt.Errorf("failed to revoke refresh token: %w", result.Error))
	}

	if result.RowsAffected == 0 {
		return apperrors.WrapNotFound(fmt.Errorf("refresh token not found or already revoked"), "refresh token")
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (s *TokenService) RevokeAllUserTokens(userID uuid.UUID) error {
	now := time.Now()
	result := s.db.Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now)

	if result.Error != nil {
		return apperrors.WrapInternal(fmt.Errorf("failed to revoke user tokens: %w", result.Error))
	}

	return nil
}

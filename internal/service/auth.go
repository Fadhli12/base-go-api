package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

// TokenService handles JWT token operations
type TokenService struct {
	secret        string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewTokenService creates a new TokenService instance
func NewTokenService(secret string, accessExpiry, refreshExpiry time.Duration) *TokenService {
	return &TokenService{
		secret:        secret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateAccessToken generates a new access token for a user
func (s *TokenService) GenerateAccessToken(userID, email string) (string, error) {
	return auth.GenerateAccessToken(userID, email, s.secret, s.accessExpiry)
}

// GenerateRefreshToken generates a new refresh token and returns the token and its hash
func (s *TokenService) GenerateRefreshToken() (token string, hash string, err error) {
	// Generate a random 32-byte token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	token = hex.EncodeToString(bytes)

	// Hash the token for storage (using simple SHA-256 would be better but we'll use bcrypt for consistency)
	hash, err = auth.Hash(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash refresh token: %w", err)
	}

	return token, hash, nil
}

// GetRefreshExpiry returns the refresh token expiry duration
func (s *TokenService) GetRefreshExpiry() time.Duration {
	return s.refreshExpiry
}

// PasswordHasher defines the interface for password hashing operations
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hashedPassword, password string) error
}

// bcryptHasher implements PasswordHasher using bcrypt
type bcryptHasher struct{}

// Hash generates a bcrypt hash of the provided password
func (b *bcryptHasher) Hash(password string) (string, error) {
	return auth.Hash(password)
}

// Verify compares a bcrypt hashed password with a plain text password
func (b *bcryptHasher) Verify(hashedPassword, password string) error {
	return auth.Verify(hashedPassword, password)
}

// NewPasswordHasher creates a new PasswordHasher instance
func NewPasswordHasher() PasswordHasher {
	return &bcryptHasher{}
}

// AuthService handles authentication business logic
type AuthService struct {
	userRepo       repository.UserRepository
	tokenRepo      repository.RefreshTokenRepository
	tokenService   *TokenService
	passwordHasher PasswordHasher
}

// NewAuthService creates a new AuthService instance
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.RefreshTokenRepository,
	tokenService *TokenService,
	passwordHasher PasswordHasher,
) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		tokenRepo:      tokenRepo,
		tokenService:   tokenService,
		passwordHasher: passwordHasher,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *request.RegisterRequest) (*domain.User, error) {
	// Check if email is already taken
	_, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, apperrors.NewAppError("EMAIL_EXISTS", "Email already registered", 409)
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	// Hash the password
	hashedPassword, err := s.passwordHasher.Hash(req.Password)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Create the user
	user := &domain.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest) (*domain.User, string, string, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
		}
		return nil, "", "", err
	}

	// Verify password
	if err := s.passwordHasher.Verify(user.PasswordHash, req.Password); err != nil {
		return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
	}

	// Generate access token
	accessToken, err := s.tokenService.GenerateAccessToken(user.ID.String(), user.Email)
	if err != nil {
		return nil, "", "", err
	}

	// Generate refresh token
	refreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, "", "", err
	}

	// Store refresh token in database
	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	token := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: expiresAt,
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// Logout revokes all refresh tokens for a user
func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	if err := s.tokenRepo.RevokeAllByUser(ctx, userID); err != nil {
		return err
	}
	return nil
}

// Refresh validates a refresh token and issues new tokens
// It validates the refresh token, revokes the old one, and generates new access and refresh tokens
func (s *AuthService) Refresh(ctx context.Context, refreshTokenString string) (*domain.User, string, string, error) {
	// Hash the provided token to look it up
	tokenHash, err := auth.Hash(refreshTokenString)
	if err != nil {
		return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Invalid refresh token", 401)
	}

	// Find the token in the database
	storedToken, err := s.tokenRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Invalid refresh token", 401)
		}
		return nil, "", "", err
	}

	// Check if token is valid (not expired and not revoked)
	if !storedToken.IsValid() {
		return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Refresh token has expired or been revoked", 401)
	}

	// Get the user
	user, err := s.userRepo.FindByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, "", "", err
	}

	// Revoke the old refresh token
	if err := s.tokenRepo.MarkRevoked(ctx, tokenHash); err != nil {
		return nil, "", "", err
	}

	// Generate new access token
	accessToken, err := s.tokenService.GenerateAccessToken(user.ID.String(), user.Email)
	if err != nil {
		return nil, "", "", err
	}

	// Generate new refresh token
	newRefreshToken, newRefreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, "", "", err
	}

	// Store the new refresh token
	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	newToken := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: newRefreshHash,
		ExpiresAt: expiresAt,
	}

	if err := s.tokenRepo.Create(ctx, newToken); err != nil {
		return nil, "", "", err
	}

	return user, accessToken, newRefreshToken, nil
}

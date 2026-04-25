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
	issuer        string
	audience     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewTokenService creates a new TokenService instance
func NewTokenService(secret, issuer, audience string, accessExpiry, refreshExpiry time.Duration) *TokenService {
	return &TokenService{
		secret:        secret,
		issuer:        issuer,
		audience:      audience,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateAccessToken generates a new access token for a user
func (s *TokenService) GenerateAccessToken(userID, email string) (string, error) {
	return auth.GenerateAccessTokenWithClaims(userID, email, s.secret, s.accessExpiry, s.issuer, s.audience)
}

// GenerateRefreshToken generates a new refresh token and returns the token and its hash
// HIGH-004: Uses SHA256 for token hashing (appropriate for random tokens, not passwords)
func (s *TokenService) GenerateRefreshToken() (token string, hash string, err error) {
	// Generate a random 32-byte token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	token = hex.EncodeToString(bytes)

	// Hash the token using SHA256 for storage
	// SHA256 is appropriate for randomly-generated tokens (unlike passwords which need bcrypt)
	hash = auth.HashToken(token)

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
	userRepo            repository.UserRepository
	tokenRepo           repository.RefreshTokenRepository
	resetTokenRepo      repository.PasswordResetTokenRepository
	tokenService        *TokenService
	passwordHasher      PasswordHasher
	emailService        *EmailService
	auditService        *AuditService
	passwordResetExpiry time.Duration
	roleRepo            repository.RoleRepository
	userRoleRepo        repository.UserRoleRepository
}

// NewAuthService creates a new AuthService instance
func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.RefreshTokenRepository,
	resetTokenRepo repository.PasswordResetTokenRepository,
	tokenService *TokenService,
	passwordHasher PasswordHasher,
	emailService *EmailService,
	auditService *AuditService,
	passwordResetExpiry time.Duration,
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		tokenRepo:           tokenRepo,
		resetTokenRepo:      resetTokenRepo,
		tokenService:        tokenService,
		passwordHasher:      passwordHasher,
		emailService:        emailService,
		auditService:        auditService,
		passwordResetExpiry: passwordResetExpiry,
		roleRepo:            roleRepo,
		userRoleRepo:        userRoleRepo,
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

	// Assign default "viewer" role to new users (non-blocking)
	// This ensures new users can access protected endpoints
	if s.roleRepo != nil && s.userRoleRepo != nil {
		go func() {
			bgCtx := context.Background()
			
			// Find the "viewer" role
			viewerRole, err := s.roleRepo.FindByName(bgCtx, "viewer")
			if err == nil && viewerRole != nil {
				// Assign the role to the user (non-blocking, fire-and-forget)
				_ = s.userRoleRepo.Assign(bgCtx, user.ID, viewerRole.ID, user.ID)
			}
		}()
	}

	// Queue welcome email (non-blocking, fire-and-forget)
	// Email failures should NOT block registration
	if s.emailService != nil {
		go func() {
			bgCtx := context.Background()
			err := s.emailService.QueueEmail(bgCtx, &EmailRequest{
				To:       user.Email,
				Template: "welcome",
				Data: map[string]any{
					"UserEmail": user.Email,
					"UserID":    user.ID.String(),
				},
			})
			if err != nil {
				// Log error but don't fail registration
				// Email delivery is best-effort
			}
		}()
	}

	return user, nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest) (*domain.User, string, string, error) {
	// MED-001: Fix timing attack - always run bcrypt to prevent user enumeration
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// User not found, but still run bcrypt to prevent timing attack
			// Use a dummy hash to make timing consistent
			s.passwordHasher.Verify("$2a$12$dummyhashdummyhashdummyhashdummyhashdu", req.Password)
			// MED-006: Track failed login
			GetAuthMetrics().IncrementLoginFailed()
			return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
		}
		return nil, "", "", err
	}

	// Verify password
	if err := s.passwordHasher.Verify(user.PasswordHash, req.Password); err != nil {
		// MED-006: Track failed login
		GetAuthMetrics().IncrementLoginFailed()
		return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
	}

	// MED-006: Track successful login
	GetAuthMetrics().IncrementLoginSuccess()

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

	// MED-004: Create new token family for this login session
	// Each login gets a new family ID to track token rotation
	familyID := uuid.New()

	// Store refresh token in database
	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	token := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		FamilyID:  familyID,
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
	// CRIT-002: Also revoke all password reset tokens for security
	if s.resetTokenRepo != nil {
		if err := s.resetTokenRepo.RevokeAllByUser(ctx, userID); err != nil {
			// Log but don't fail logout
		}
	}
	return nil
}

// Refresh validates a refresh token and issues new tokens
// MED-004: Now includes token family tracking for rotation attack detection
// When a revoked token is used, we revoke the entire family to prevent attacks
func (s *AuthService) Refresh(ctx context.Context, refreshTokenString string) (*domain.User, string, string, error) {
	// Hash the provided token to look it up
	tokenHash := auth.HashToken(refreshTokenString)

	// Find the token in the database
	storedToken, err := s.tokenRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// MED-006: Track failed token refresh
			GetAuthMetrics().IncrementTokenRefreshFailed()
			return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Invalid refresh token", 401)
		}
		return nil, "", "", err
	}

	// MED-004: Check if token is revoked (potential reuse attack)
	if storedToken.IsRevoked() {
		// Token reuse detected - revoke entire family for security
		// This prevents attackers from using stolen tokens
		if err := s.tokenRepo.RevokeFamily(ctx, storedToken.FamilyID); err != nil {
			// Log the error but don't reveal it to attacker
		}
		
		// MED-006: Track token reuse attack
		GetAuthMetrics().IncrementTokenReuseDetected()
		
		// MED-002: Audit the potential attack
		s.AuditPotentialTokenReuse(ctx, storedToken.UserID, storedToken.FamilyID)
		
		return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Refresh token has been revoked", 401)
	}

	// Check if token is expired
	if time.Now().After(storedToken.ExpiresAt) {
		// MED-006: Track failed token refresh
		GetAuthMetrics().IncrementTokenRefreshFailed()
		return nil, "", "", apperrors.NewAppError("INVALID_REFRESH_TOKEN", "Refresh token has expired", 401)
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

	// MED-004: Store the new refresh token with the SAME family ID
	// This tracks the token chain and enables detection of reused tokens
	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	newToken := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: newRefreshHash,
		FamilyID:  storedToken.FamilyID, // Keep same family for rotation tracking
		ExpiresAt: expiresAt,
	}

	if err := s.tokenRepo.Create(ctx, newToken); err != nil {
		return nil, "", "", err
	}

	// MED-006: Track successful token refresh
	GetAuthMetrics().IncrementTokenRefreshSuccess()

	return user, accessToken, newRefreshToken, nil
}

// RequestPasswordReset generates a password reset token and queues a reset email
// CRIT-002: Token is now persisted to database for validation
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// Don't reveal if email exists or not - security best practice
			// Return empty string without error (silent failure)
			return "", nil
		}
		return "", err
	}

	// Generate a secure reset token (32 bytes = 64 hex characters)
	resetTokenBytes := make([]byte, 32)
	if _, err := rand.Read(resetTokenBytes); err != nil {
		return "", apperrors.WrapInternal(err)
	}
	resetToken := hex.EncodeToString(resetTokenBytes)

	// Hash the token using SHA256 for storage
	// Revoke any existing password reset tokens for this user first
	if s.resetTokenRepo != nil {
		if err := s.resetTokenRepo.RevokeAllByUser(ctx, user.ID); err != nil {
			// Log but continue - we'll still create a new token
		}
	}

	// Create token record in database
	tokenHash := auth.HashToken(resetToken)
	expiresAt := time.Now().Add(s.passwordResetExpiry)

	resetTokenRecord := &domain.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if s.resetTokenRepo != nil {
		if err := s.resetTokenRepo.Create(ctx, resetTokenRecord); err != nil {
			return "", apperrors.WrapInternal(err)
		}
	}

	// Queue password reset email (non-blocking, fire-and-forget)
	// Email failures should NOT block password reset - token is in database
	if s.emailService != nil {
		go func() {
			bgCtx := context.Background()
			err := s.emailService.QueueEmail(bgCtx, &EmailRequest{
				To:       user.Email,
				Template: "password-reset",
				Data: map[string]any{
					"UserEmail":  user.Email,
					"ResetToken": resetToken,
					"UserID":     user.ID.String(),
					"ExpiresIn":  s.passwordResetExpiry.String(),
				},
			})
			if err != nil {
				// Log error but don't fail password reset
				// Email delivery is best-effort - token is in database
			}
		}()
	}

	// MED-006: Track password reset request
	GetAuthMetrics().IncrementPasswordResetRequested()

	return resetToken, nil
}

// ResetPassword validates a password reset token and updates the user's password
// CRIT-002: Token is validated from database storage
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Hash the provided token to look it up
	tokenHash := auth.HashToken(token)

	// Find the token in the database
	resetToken, err := s.resetTokenRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// MED-006: Track failed password reset
			GetAuthMetrics().IncrementPasswordResetFailed()
			return apperrors.NewAppError("INVALID_RESET_TOKEN", "Invalid or expired password reset token", 400)
		}
		return apperrors.WrapInternal(err)
	}

	// Check if token is valid (not expired and not used)
	if !resetToken.IsValid() {
		// MED-006: Track failed password reset
		GetAuthMetrics().IncrementPasswordResetFailed()
		return apperrors.NewAppError("INVALID_RESET_TOKEN", "Password reset token has expired or been used", 400)
	}

	// Get the user
	user, err := s.userRepo.FindByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	// Hash the new password
	hashedPassword, err := s.passwordHasher.Hash(newPassword)
	if err != nil {
		return apperrors.WrapInternal(err)
	}

	// Update the user's password
	user.PasswordHash = hashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Mark the reset token as used
	if err := s.resetTokenRepo.MarkUsed(ctx, tokenHash); err != nil {
		// Log but don't fail - password was already updated
	}

	// Revoke all refresh tokens for this user (security measure - force re-login)
	if err := s.tokenRepo.RevokeAllByUser(ctx, user.ID); err != nil {
		// Log but don't fail - password was already updated
	}

	// MED-006: Track password reset completion
	GetAuthMetrics().IncrementPasswordResetCompleted()

	return nil
}

// MED-005: Session Management Methods

// GetActiveSessions returns all active (non-revoked, non-expired) sessions for a user
func (s *AuthService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*domain.RefreshToken, error) {
	sessions, err := s.tokenRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter out expired and revoked sessions
	active := make([]*domain.RefreshToken, 0)
	for _, session := range sessions {
		if session.IsValid() {
			active = append(active, session)
		}
	}

	// MED-006: Update active sessions count
	GetAuthMetrics().SetActiveSessions(int64(len(active)))

	return active, nil
}

// RevokeSession revokes a specific session by ID
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	// Get the session to verify ownership
	session, err := s.tokenRepo.FindByID(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify session belongs to the user
	if session.UserID != userID {
		return apperrors.NewAppError("FORBIDDEN", "Session does not belong to user", 403)
	}

	// Revoke the session
	// MED-006: Track session revocation
	GetAuthMetrics().IncrementSessionRevoked()
	
	return s.tokenRepo.MarkRevoked(ctx, session.TokenHash)
}

// RevokeAllOtherSessions revokes all sessions except the specified one
func (s *AuthService) RevokeAllOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) (int, error) {
	// Get all sessions for the user
	sessions, err := s.tokenRepo.FindByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}

	// Get current session if provided
	var currentTokenHash string
	if currentSessionID != uuid.Nil {
		currentSession, err := s.tokenRepo.FindByID(ctx, currentSessionID)
		if err == nil {
			currentTokenHash = currentSession.TokenHash
		}
	}

	// Revoke all sessions except current
	count := 0
	for _, session := range sessions {
		if session.TokenHash == currentTokenHash {
			continue // Skip current session
		}
		if session.IsRevoked() {
			continue // Already revoked
		}

		if err := s.tokenRepo.MarkRevoked(ctx, session.TokenHash); err != nil {
			continue // Log and continue
		}
		count++
	}

	return count, nil
}
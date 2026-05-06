# Two-Factor Authentication (2FA) Implementation Plan

**Feature:** Two-Factor Authentication with TOTP (Time-based One-Time Password)  
**Complexity:** Medium-High  
**Effort:** 3-4 days  
**Branch:** `007-two-factor-auth`  
**Status:** Planning  
**Depends on:** JWT auth system (existing)

---

## Executive Summary

This feature adds Two-Factor Authentication (2FA) to the Go API Base using TOTP (Time-based One-Time Password) algorithm compatible with Google Authenticator, Authy, and other authenticator apps. Users can enable 2FA for enhanced account security.

**Key Principles:**
- TOTP standard (RFC 6238) - compatible with authenticator apps
- Enable/verify flow: User-initiated 2FA setup with QR code secret
- Soft deletes on all new entities
- UUID primary keys
- SQL migrations only (no AutoMigrate)
- Full audit trail via AuditService
- Backward compatible - 2FA optional, doesn't break existing auth

---

## Architecture Overview

### Multi-Factor Authentication Flow

```
LOGIN WITHOUT 2FA:                    LOGIN WITH 2FA ENABLED:
1. Email + Password                  1. Email + Password
2. Access Token + Refresh Token      2. Access Token + Refresh Token
                                   3. → 2FA_VERIFY_REQUIRED response
                                   4. TOTP Code from Authenticator App
                                   5. Final Access Token (with 2FA claim)
```

### 2FA State Machine

```
┌─────────┐  enable   ┌────────────┐ verify   ┌──────────────┐
│  NONE   │ ────────→ │ PENDING    │ ───────→ │    ENABLED   │
└─────────┘            │  (waiting  │           └──────────────┘
                       │  for first │              │
                       │  verify)   │              │ disable
                       └────────────┘              │
                                                    ↓
                                              ┌──────────────┐
                                              │   DISABLED   │
                                              └──────────────┘
```

### 2FA Verification Flow

```
Login Request
     │
     ▼
┌────────────┐
│ Validate   │
│ Password   │
└────────────┘
     │
     ▼
┌─────────────────┐
│ Has 2FA?       │──No──→ Return tokens normally
└─────────────────┘
     │Yes
     ▼
┌─────────────────┐
│ Return special  │─── Return 2FA_VERIFY_REQUIRED
│ response with   │     with temp refresh token
│ session_id      │
└─────────────────┘
     │
     ▼
Verify TOTP Request
     │
     ▼
┌─────────────────┐
│ Validate TOTP   │
│ Code            │
└─────────────────┘
     │
     │ Valid
     ▼
Final Access + Refresh Tokens
(with 2fa_verified:true claim)
```

---

## Phase 1: Database Schema

### Migration: `000011_two_factor_auth.up.sql`

```sql
-- Two-factor authentication recovery codes
CREATE TABLE two_factor_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL,  -- bcrypt hash of recovery code
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_two_factor_recovery_codes_user_id (user_id),
    INDEX idx_two_factor_recovery_codes_used_at (used_at)
);

-- Add 2FA fields to users table
ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN two_factor_secret VARCHAR(255);  -- Encrypted TOTP secret
ALTER TABLE users ADD COLUMN two_factor_verified_at TIMESTAMP;
ALTER TABLE users ADD COLUMN two_factor_status VARCHAR(20) NOT NULL DEFAULT 'none';
-- Status: none, pending, enabled, disabled

-- Create indexes
CREATE INDEX idx_users_two_factor_enabled ON users(two_factor_enabled);
CREATE INDEX idx_users_two_factor_status ON users(two_factor_status);
```

### Migration: `000011_two_factor_auth.down.sql`

```sql
-- Remove 2FA fields
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_secret;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_status;

-- Drop recovery codes table
DROP TABLE IF EXISTS two_factor_recovery_codes;
```

---

## Phase 2: Domain Models

### File: `internal/domain/two_factor.go`

```go
package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TwoFactorStatus represents the 2FA setup status
type TwoFactorStatus string

const (
	TwoFactorStatusNone     TwoFactorStatus = "none"
	TwoFactorStatusPending TwoFactorStatus = "pending"
	TwoFactorStatusEnabled TwoFactorStatus = "enabled"
	TwoFactorStatusDisabled TwoFactorStatus = "disabled"
)

// TwoFactorRecoveryCode represents a backup recovery code
type TwoFactorRecoveryCode struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	CodeHash  string        `gorm:"size:255;not null" json:"-"`
	UsedAt    *time.Time     `gorm:"default:null" json:"used_at,omitempty"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

func (TwoFactorRecoveryCode) TableName() string {
	return "two_factor_recovery_codes"
}

// IsUsed checks if the recovery code has been used
func (r *TwoFactorRecoveryCode) IsUsed() bool {
	return r.UsedAt != nil
}

// TwoFactorSetupResponse for initial setup (contains secret for QR code)
type TwoFactorSetupResponse struct {
	Secret       string `json:"secret"`
	QRCodeURL    string `json:"qr_code_url"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// TwoFactorVerifyRequest for verifying TOTP code
type TwoFactorVerifyRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// TwoFactorEnableRequest for initial setup verification
type TwoFactorEnableRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// TwoFactorRecoveryRequest for using recovery code
type TwoFactorRecoveryRequest struct {
	RecoveryCode string `json:"recovery_code" validate:"required"`
}
```

---

## Phase 3: TOTP Service

### File: `internal/service/totp.go`

```go
package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TOTPService handles TOTP generation and verification
type TOTPService struct {
	issuer string
	secretEncryptionKey []byte
}

// NewTOTPService creates a new TOTP service
func NewTOTPService(issuer string, encryptionKey []byte) *TOTPService {
	return &TOTPService{
		issuer:              issuer,
		secretEncryptionKey: encryptionKey,
	}
}

// GenerateSecret generates a new TOTP secret
func (s *TOTPService) GenerateSecret(userID uuid.UUID) (secret string, err error) {
	// Generate 20-byte random secret
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	
	// Encode to base32 (no padding for Google Authenticator compatibility)
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	
	return secret, nil
}

// GenerateQRCodeURL generates the otpauth:// URL for QR code
func (s *TOTPService) GenerateQRCodeURL(email, secret string) string {
	// otpauth://totp/Issuer:Email?secret=SECRET&issuer=Issuer&digits=6&period=30
	label := fmt.Sprintf("%s:%s", s.issuer, email)
	otpauth := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&digits=6&period=30",
		url.PathEscape(label),
		secret,
		url.PathEscape(s.issuer),
	)
	return otpauth
}

// GenerateRecoveryCodes generates backup recovery codes
func (s *TOTPService) GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// Generate 10-digit numeric codes
		bytes := make([]byte, 5)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		code := fmt.Sprintf("%08d", binary.BigEndian.Uint32(bytes)%100000000)
		codes[i] = code
	}
	return codes, nil
}

// EncryptSecret encrypts the TOTP secret for storage
func (s *TOTPService) EncryptSecret(secret string) (string, error) {
	// Using AES-GCM for symmetric encryption
	block, err := aes.NewCipher(s.secretEncryptionKey[:16])
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts the stored TOTP secret
func (s *TOTPService) DecryptSecret(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(s.secretEncryptionKey[:16])
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// ValidateCode validates a TOTP code
// Implements RFC 6238 with 30-second window and 1 step tolerance
func (s *TOTPService) ValidateCode(secret, code string) bool {
	if len(code) != 6 {
		return false
	}
	
	// Parse the code
	var c int64
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
		c = c*10 + int64(r-'0')
	}
	
	// Decode the secret
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	secret = strings.TrimRight(secret, "=")
	
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false
	}
	
	// Check current and adjacent time steps (for clock drift tolerance)
	now := time.Now().Unix() / 30
	
	for _, offset := range []int64{-1, 0, 1} {
		expected := s.computeTOTP(key, now+offset)
		if c == expected {
			return true
		}
	}
	
	return false
}

// computeTOTP computes the TOTP value for a given time step
func (s *TOTPService) computeTOTP(key []byte, counter int64) int64 {
	// Convert counter to 8 bytes (big-endian)
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(counter))
	
	// Compute HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(msg)
	hash := h.Sum(nil)
	
	// Dynamic truncation
	offset := hash[19] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	
	// 6 digits
	return truncated % int64(math.Pow10(6))
}
```

---

## Phase 4: Service Layer

### File: `internal/service/two_factor.go`

```go
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go-api/internal/domain"
	"go-api/internal/repository"
	apperrors "go-api/pkg/errors"
)

// TwoFactorService handles 2FA operations
type TwoFactorService struct {
	userRepo         repository.UserRepository
	recoveryCodeRepo repository.TwoFactorRecoveryCodeRepository
	totpService      *TOTPService
	auditService     *AuditService
	log              *zap.Logger
}

func NewTwoFactorService(
	userRepo repository.UserRepository,
	recoveryCodeRepo repository.TwoFactorRecoveryCodeRepository,
	totpService *TOTPService,
	auditService *AuditService,
	log *zap.Logger,
) *TwoFactorService {
	return &TwoFactorService{
		userRepo:         userRepo,
		recoveryCodeRepo: recoveryCodeRepo,
		totpService:      totpService,
		auditService:     auditService,
		log:              log,
	}
}

// GenerateSetupSecret initiates 2FA setup for a user
// Returns the secret and QR code URL for authenticator app setup
func (s *TwoFactorService) GenerateSetupSecret(ctx context.Context, userID uuid.UUID) (*domain.TwoFactorSetupResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.TwoFactorStatus == domain.TwoFactorStatusEnabled {
		return nil, apperrors.NewAppError("2FA_ALREADY_ENABLED", "2FA is already enabled", 400)
	}

	// Generate new TOTP secret
	secret, err := s.totpService.GenerateSecret(userID)
	if err != nil {
		s.log.Error("failed to generate TOTP secret", zap.Error(err))
		return nil, apperrors.WrapInternal(err)
	}

	// Generate recovery codes
	recoveryCodes, err := s.totpService.GenerateRecoveryCodes(8)
	if err != nil {
		s.log.Error("failed to generate recovery codes", zap.Error(err))
		return nil, apperrors.WrapInternal(err)
	}

	// Encrypt the secret for storage
	encryptedSecret, err := s.totpService.EncryptSecret(secret)
	if err != nil {
		s.log.Error("failed to encrypt TOTP secret", zap.Error(err))
		return nil, apperrors.WrapInternal(err)
	}

	// Update user with pending 2FA status
	user.TwoFactorStatus = domain.TwoFactorStatusPending
	user.TwoFactorSecret = encryptedSecret

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// Store recovery code hashes
	for _, code := range recoveryCodes {
		codeHash, _ := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		rc := &domain.TwoFactorRecoveryCode{
			UserID:   userID,
			CodeHash: string(codeHash),
		}
		if err := s.recoveryCodeRepo.Create(ctx, rc); err != nil {
			s.log.Error("failed to store recovery code", zap.Error(err))
		}
	}

	// Generate QR code URL
	qrURL := s.totpService.GenerateQRCodeURL(user.Email, secret)

	s.log.Info("2FA setup initiated", zap.String("user_id", userID.String()))

	return &domain.TwoFactorSetupResponse{
		Secret:        secret,
		QRCodeURL:     qrURL,
		RecoveryCodes: recoveryCodes,
	}, nil
}

// VerifySetup verifies the initial TOTP code to enable 2FA
func (s *TwoFactorService) VerifySetup(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.TwoFactorStatus != domain.TwoFactorStatusPending {
		return apperrors.NewAppError("2FA_NOT_PENDING", "2FA setup not initiated", 400)
	}

	if user.TwoFactorSecret == "" {
		return apperrors.NewAppError("2FA_NO_SECRET", "No 2FA secret found", 400)
	}

	// Decrypt the stored secret
	secret, err := s.totpService.DecryptSecret(user.TwoFactorSecret)
	if err != nil {
		s.log.Error("failed to decrypt TOTP secret", zap.Error(err))
		return apperrors.WrapInternal(err)
	}

	// Validate the TOTP code
	if !s.totpService.ValidateCode(secret, code) {
		s.log.Warn("invalid TOTP code during 2FA setup", zap.String("user_id", userID.String()))
		return apperrors.NewAppError("INVALID_2FA_CODE", "Invalid verification code", 400)
	}

	// Enable 2FA
	user.TwoFactorStatus = domain.TwoFactorStatusEnabled
	user.TwoFactorEnabled = true
	user.TwoFactorVerifiedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Audit log
	s.auditService.LogMutation(ctx, userID, "enable_2fa", "user", userID.String(), nil, user)

	s.log.Info("2FA enabled", zap.String("user_id", userID.String()))

	return nil
}

// ValidateCode validates a TOTP code for login verification
func (s *TwoFactorService) ValidateCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if !user.TwoFactorEnabled || user.TwoFactorStatus != domain.TwoFactorStatusEnabled {
		return false, apperrors.NewAppError("2FA_NOT_ENABLED", "2FA is not enabled", 400)
	}

	// Decrypt the stored secret
	secret, err := s.totpService.DecryptSecret(user.TwoFactorSecret)
	if err != nil {
		s.log.Error("failed to decrypt TOTP secret", zap.Error(err))
		return false, apperrors.WrapInternal(err)
	}

	valid := s.totpService.ValidateCode(secret, code)
	
	if !valid {
		s.log.Warn("invalid TOTP code during login", zap.String("user_id", userID.String()))
	}

	return valid, nil
}

// VerifyRecoveryCode verifies and consumes a recovery code
func (s *TwoFactorService) VerifyRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	// Find the recovery code
	codes, err := s.recoveryCodeRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, rc := range codes {
		if rc.IsUsed() {
			continue
		}
		
		// Use bcrypt to verify - recovery codes are bcrypt hashed
		if err := bcrypt.Check([]byte(code), []byte(rc.CodeHash)); err == nil {
			// Mark as used
			now := time.Now()
			rc.UsedAt = &now
			if err := s.recoveryCodeRepo.Update(ctx, rc); err != nil {
				s.log.Error("failed to mark recovery code as used", zap.Error(err))
			}
			
			s.log.Info("recovery code used", zap.String("user_id", userID.String()))
			return true, nil
		}
	}

	s.log.Warn("invalid recovery code", zap.String("user_id", userID.String()))
	return false, nil
}

// Disable disables 2FA for a user (requires valid TOTP code)
func (s *TwoFactorService) Disable(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.TwoFactorStatus != domain.TwoFactorStatusEnabled {
		return apperrors.NewAppError("2FA_NOT_ENABLED", "2FA is not enabled", 400)
	}

	// Validate the code first
	if !s.ValidateCode(ctx, userID, code) {
		return apperrors.NewAppError("INVALID_2FA_CODE", "Invalid verification code", 400)
	}

	// Disable 2FA
	user.TwoFactorEnabled = false
	user.TwoFactorStatus = domain.TwoFactorStatusDisabled
	user.TwoFactorSecret = ""

	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Delete recovery codes
	if err := s.recoveryCodeRepo.DeleteAllByUser(ctx, userID); err != nil {
		s.log.Error("failed to delete recovery codes", zap.Error(err))
	}

	// Audit log
	s.auditService.LogMutation(ctx, userID, "disable_2fa", "user", userID.String(), nil, user)

	s.log.Info("2FA disabled", zap.String("user_id", userID.String()))

	return nil
}

// GetStatus returns the 2FA status for a user
func (s *TwoFactorService) GetStatus(ctx context.Context, userID uuid.UUID) (*TwoFactorStatusResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &TwoFactorStatusResponse{
		Enabled:    user.TwoFactorEnabled,
		Status:     user.TwoFactorStatus,
		VerifiedAt: user.TwoFactorVerifiedAt,
	}, nil
}

type TwoFactorStatusResponse struct {
	Enabled    bool       `json:"enabled"`
	Status     string     `json:"status"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}
```

---

## Phase 5: HTTP Handlers

### File: `internal/http/handler/two_factor.go`

```go
package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"go-api/internal/http/middleware"
	"go-api/internal/http/request"
	"go-api/internal/http/response"
	"go-api/internal/service"
	apperrors "go-api/pkg/errors"
)

// TwoFactorHandler handles 2FA endpoints
type TwoFactorHandler struct {
	svc             *service.TwoFactorService
	authService     *service.AuthService
}

func NewTwoFactorHandler(svc *service.TwoFactorService, authService *service.AuthService) *TwoFactorHandler {
	return &TwoFactorHandler{
		svc:         svc,
		authService: authService,
	}
}

// SetupResponse for initial 2FA setup
type SetupResponse struct {
	Secret        string   `json:"secret"`
	QRCodeURL     string   `json:"qr_code_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// GenerateSetup POST /api/v1/2fa/setup
// @Summary Initiate 2FA setup
// @Description Generate TOTP secret and QR code for authenticator app
// @Tags 2fa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=handler.SetupResponse}
// @Failure 401 {object} response.Envelope "Unauthorized"
// @Failure 400 {object} response.Envelope "2FA already enabled"
// @Router /api/v1/2fa/setup [post]
func (h *TwoFactorHandler) GenerateSetup(c echo.Context) error {
	userID := middleware.GetUserID(c) // Already validated by JWT middleware

	setup, err := h.svc.GenerateSetupSecret(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to setup 2FA"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, SetupResponse{
		Secret:        setup.Secret,
		QRCodeURL:     setup.QRCodeURL,
		RecoveryCodes: setup.RecoveryCodes,
	}))
}

// VerifySetup POST /api/v1/2fa/verify-setup
// @Summary Verify TOTP code to enable 2FA
// @Description Verify the initial TOTP code to complete 2FA setup
// @Tags 2fa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.TwoFactorEnableRequest true "TOTP code"
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope "Invalid code"
// @Router /api/v1/2fa/verify-setup [post]
func (h *TwoFactorHandler) VerifySetup(c echo.Context) error {
	userID := middleware.GetUserID(c)

	var req request.TwoFactorEnableRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request"))
	}

	if err := h.svc.VerifySetup(c.Request().Context(), userID, req.Code); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify 2FA"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"message": "2FA has been enabled successfully",
	}))
}

// VerifyCode POST /api/v1/2fa/verify
// @Summary Verify TOTP code during login
// @Description Verify TOTP code to complete authentication (after password verification)
// @Tags 2fa
// @Accept json
// @Produce json
// @Param request body request.TwoFactorVerifyRequest true "TOTP code"
// @Success 200 {object} response.Envelope{data=handler.RefreshResponse}
// @Failure 400 {object} response.Envelope "Invalid code"
// @Router /api/v1/2fa/verify [post]
func (h *TwoFactorHandler) VerifyCode(c echo.Context) error {
	var req request.TwoFactorVerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request"))
	}

	// Get session info from context (set during login with 2FA)
	sessionIDVal := c.Get("2fa_session_id")
	if sessionIDVal == nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "No pending 2FA session"))
	}
	sessionID := sessionIDVal.(uuid.UUID)

	userIDVal := c.Get("2fa_user_id")
	if userIDVal == nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "No user ID for 2FA session"))
	}
	userID := userIDVal.(uuid.UUID)

	valid, err := h.svc.ValidateCode(c.Request().Context(), userID, req.Code)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify code"))
	}

	if !valid {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_2FA_CODE", "Invalid verification code"))
	}

	// Generate final access and refresh tokens
	accessToken, err := h.authService.GenerateAccessToken(userID.String())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to generate token"))
	}

	refreshToken, _, err := h.authService.GenerateRefreshToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to generate refresh token"))
	}

	// Revoke the temp session
	h.authService.RevokeSession(c.Request().Context(), userID, sessionID)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}))
}

// UseRecoveryCode POST /api/v1/2fa/recover
// @Summary Use recovery code instead of TOTP
// @Description Use a backup recovery code to complete authentication
// @Tags 2fa
// @Accept json
// @Produce json
// @Param request body request.TwoFactorRecoveryRequest true "Recovery code"
// @Success 200 {object} response.Envelope{data=handler.RefreshResponse}
// @Failure 400 {object} response.Envelope "Invalid recovery code"
// @Router /api/v1/2fa/recover [post]
func (h *TwoFactorHandler) UseRecoveryCode(c echo.Context) error {
	var req request.TwoFactorRecoveryRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request"))
	}

	sessionIDVal := c.Get("2fa_session_id")
	if sessionIDVal == nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "No pending 2FA session"))
	}
	sessionID := sessionIDVal.(uuid.UUID)

	userIDVal := c.Get("2fa_user_id")
	if userIDVal == nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "No user ID for 2FA session"))
	}
	userID := userIDVal.(uuid.UUID)

	valid, err := h.svc.VerifyRecoveryCode(c.Request().Context(), userID, req.RecoveryCode)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify recovery code"))
	}

	if !valid {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_RECOVERY_CODE", "Invalid recovery code"))
	}

	// Generate final access and refresh tokens
	accessToken, err := h.authService.GenerateAccessToken(userID.String())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to generate token"))
	}

	refreshToken, _, err := h.authService.GenerateRefreshToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to generate refresh token"))
	}

	// Revoke the temp session
	h.authService.RevokeSession(c.Request().Context(), userID, sessionID)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}))
}

// Disable POST /api/v1/2fa/disable
// @Summary Disable 2FA
// @Description Disable 2FA for the authenticated user
// @Tags 2fa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.TwoFactorVerifyRequest true "TOTP code to confirm disable"
// @Success 200 {object} response.Envelope
// @Failure 400 {object} response.Envelope "Invalid code"
// @Router /api/v1/2fa/disable [post]
func (h *TwoFactorHandler) Disable(c echo.Context) error {
	userID := middleware.GetUserID(c)

	var req request.TwoFactorVerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request"))
	}

	if err := h.svc.Disable(c.Request().Context(), userID, req.Code); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to disable 2FA"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"message": "2FA has been disabled",
	}))
}

// GetStatus GET /api/v1/2fa/status
// @Summary Get 2FA status
// @Description Get the current 2FA status for the authenticated user
// @Tags 2fa
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope{data=handler.TwoFactorStatusResponse}
// @Router /api/v1/2fa/status [get]
func (h *TwoFactorHandler) GetStatus(c echo.Context) error {
	userID := middleware.GetUserID(c)

	status, err := h.svc.GetStatus(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get 2FA status"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, status))
}
```

---

## Phase 6: Repository Layer

### File: `internal/repository/two_factor_recovery_code.go`

```go
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"go-api/internal/domain"
	apperrors "go-api/pkg/errors"
)

// TwoFactorRecoveryCodeRepository defines recovery code operations
type TwoFactorRecoveryCodeRepository interface {
	Create(ctx context.Context, code *domain.TwoFactorRecoveryCode) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.TwoFactorRecoveryCode, error)
	Update(ctx context.Context, code *domain.TwoFactorRecoveryCode) error
	DeleteAllByUser(ctx context.Context, userID uuid.UUID) error
}

// twoFactorRecoveryCodeRepository implements TwoFactorRecoveryCodeRepository
type twoFactorRecoveryCodeRepository struct {
	db *gorm.DB
}

func NewTwoFactorRecoveryCodeRepository(db *gorm.DB) TwoFactorRecoveryCodeRepository {
	return &twoFactorRecoveryCodeRepository{db: db}
}

func (r *twoFactorRecoveryCodeRepository) Create(ctx context.Context, code *domain.TwoFactorRecoveryCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

func (r *twoFactorRecoveryCodeRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.TwoFactorRecoveryCode, error) {
	var codes []*domain.TwoFactorRecoveryCode
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&codes).Error
	
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *twoFactorRecoveryCodeRepository) Update(ctx context.Context, code *domain.TwoFactorRecoveryCode) error {
	return r.db.WithContext(ctx).Save(code).Error
}

func (r *twoFactorRecoveryCodeRepository) DeleteAllByUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&domain.TwoFactorRecoveryCode{}).Error
}
```

---

## Phase 7: Auth Service Modifications

### Update Login Flow

Modify `internal/service/auth.go` to handle 2FA during login:

```go
// Login authenticates a user and returns tokens
// If 2FA is enabled, returns special response requiring 2FA verification
func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest) (*domain.User, string, string, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			s.passwordHasher.Verify("$2a$12$dummyhashdummyhashdummyhashdummyhashdu", req.Password)
			GetAuthMetrics().IncrementLoginFailed()
			return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
		}
		return nil, "", "", err
	}

	if err := s.passwordHasher.Verify(user.PasswordHash, req.Password); err != nil {
		GetAuthMetrics().IncrementLoginFailed()
		return nil, "", "", apperrors.NewAppError("INVALID_CREDENTIALS", "Invalid email or password", 401)
	}

	GetAuthMetrics().IncrementLoginSuccess()

	// Check if 2FA is enabled
	if user.TwoFactorEnabled && user.TwoFactorStatus == domain.TwoFactorStatusEnabled {
		// Generate temp tokens for 2FA session
		tempAccessToken, err := s.tokenService.GenerateAccessToken(user.ID.String(), user.Email)
		if err != nil {
			return nil, "", "", err
		}

		// Generate temp refresh token tied to 2FA session
		tempRefreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
		if err != nil {
			return nil, "", "", err
		}

		// Store temp refresh token with 2FA flag
		familyID := uuid.New()
		expiresAt := time.Now().Add(5 * time.Minute) // Short expiry for 2FA session
		token := &domain.RefreshToken{
			UserID:    user.ID,
			TokenHash: refreshHash,
			FamilyID:  familyID,
			ExpiresAt: expiresAt,
			Metadata:  map[string]interface{}{"2fa_pending": true},
		}

		if err := s.tokenRepo.Create(ctx, token); err != nil {
			return nil, "", "", err
		}

		// Return special response indicating 2FA required
		// The handler should return HTTP 403 with code "2FA_VERIFY_REQUIRED"
		return user, tempAccessToken, tempRefreshToken, nil // Indicates 2FA pending
	}

	// Standard login without 2FA
	accessToken, err := s.tokenService.GenerateAccessToken(user.ID.String(), user.Email)
	if err != nil {
		return nil, "", "", err
	}

	refreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, "", "", err
	}

	familyID := uuid.New()
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
```

---

## Phase 8: Request Types

### File: `internal/http/request/two_factor.go`

```go
package request

type TwoFactorVerifyRequest struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

type TwoFactorEnableRequest struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

type TwoFactorRecoveryRequest struct {
	RecoveryCode string `json:"recovery_code" validate:"required"`
}
```

---

## Phase 9: User Model Update

### Update: `internal/domain/user.go`

```go
// User represents a user account in the system
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// 2FA fields
	TwoFactorEnabled   bool             `gorm:"default:false" json:"-"`
	TwoFactorSecret    string           `gorm:"size:255" json:"-"`
	TwoFactorVerifiedAt *time.Time     `json:"-"`
	TwoFactorStatus    TwoFactorStatus  `gorm:"type:varchar(20);default:'none'" json:"-"`
}
```

---

## Phase 10: Server Registration

### Update: `internal/http/server.go`

```go
// Two factor routes
twoFactor := v1.Group("/2fa")
twoFactor.Use(middleware.JWT(...))
twoFactor.POST("/setup", twoFactorHandler.GenerateSetup)
twoFactor.POST("/verify-setup", twoFactorHandler.VerifySetup)
twoFactor.POST("/verify", twoFactorHandler.VerifyCode)
twoFactor.POST("/recover", twoFactorHandler.UseRecoveryCode)
twoFactor.POST("/disable", twoFactorHandler.Disable)
twoFactor.GET("/status", twoFactorHandler.GetStatus)
```

---

## API Endpoints

```
PUBLIC:
POST   /api/v1/auth/login              Login (may return 2FA_REQUIRED)
POST   /api/v1/auth/refresh            Refresh token
POST   /api/v1/2fa/verify             Verify TOTP after login
POST   /api/v1/2fa/recover            Use recovery code

PROTECTED (JWT required):
POST   /api/v1/2fa/setup              Initiate 2FA setup
POST   /api/v1/2fa/verify-setup       Complete 2FA setup
POST   /api/v1/2fa/disable           Disable 2FA
GET    /api/v1/2fa/status            Get 2FA status
```

---

## Success Criteria

- ✅ TOTP RFC 6238 compliant (compatible with Google Authenticator)
- ✅ Recovery codes for account recovery
- ✅ 2FA verification during login flow
- ✅ User-initiated 2FA enable/disable
- ✅ Audit logging for 2FA operations
- ✅ Soft deletes on recovery codes table
- ✅ Encrypted secret storage
- ✅ Integration tests passing
- ✅ No breaking changes to existing auth

---

## Timeline

**Day 1:**
- Database migrations
- Domain models
- TOTP service implementation
- Recovery code repository

**Day 2:**
- TwoFactorService business logic
- Auth service modifications (2FA check in login)
- Request/response types

**Day 3:**
- HTTP handlers
- Server registration
- Integration tests

**Day 4:**
- Full integration testing
- Security review
- Documentation

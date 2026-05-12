package service

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TwoFactorService handles two-factor authentication business logic
type TwoFactorService struct {
	userRepo                  repository.UserRepository
	twoFactorRecoveryCodeRepo repository.TwoFactorRecoveryCodeRepository
	refreshTokenRepo          repository.RefreshTokenRepository
	tokenService              *TokenService
	config                    TwoFactorServiceConfig
	auditSvc                  *AuditService
}

// TwoFactorServiceConfig holds configuration for TwoFactorService
type TwoFactorServiceConfig struct {
	EncryptionKey []byte // 32-byte AES-256 key for TOTP secret encryption
}

// NewTwoFactorService creates a new TwoFactorService instance
func NewTwoFactorService(
	userRepo repository.UserRepository,
	twoFactorRecoveryCodeRepo repository.TwoFactorRecoveryCodeRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	tokenService *TokenService,
	config TwoFactorServiceConfig,
	auditSvc *AuditService,
) *TwoFactorService {
	return &TwoFactorService{
		userRepo:                  userRepo,
		twoFactorRecoveryCodeRepo: twoFactorRecoveryCodeRepo,
		refreshTokenRepo:          refreshTokenRepo,
		tokenService:              tokenService,
		config:                    config,
		auditSvc:                  auditSvc,
	}
}

// InitiateSetup begins the 2FA setup process for a user.
// It generates a new TOTP secret, encrypts it, and stores it as pending.
func (s *TwoFactorService) InitiateSetup(ctx context.Context, userID uuid.UUID) (*domain.TwoFactorSetupResponse, error) {
	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check if 2FA is not already enabled
	if user.TwoFactorEnabled {
		return nil, apperrors.NewAppError("TWO_FACTOR_ALREADY_ENABLED", "Two-factor authentication is already enabled", 400)
	}

	// Check if 2FA is not already pending setup
	if user.TwoFactorStatus == string(domain.TwoFactorStatusPending) {
		return nil, apperrors.NewAppError("TWO_FACTOR_SETUP_PENDING", "Two-factor setup is already pending. Complete or cancel the existing setup first", 400)
	}

	// Generate TOTP key using the otp library
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "GoAPIBase",
		AccountName: user.Email,
		Period:      30,
		SecretSize:  20,
	})
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Get the secret for encryption
	secretBytes := []byte(key.Secret())

	// Encrypt the TOTP secret
	encryptedSecret, err := EncryptTOTPSecret(secretBytes, s.config.EncryptionKey)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	// Update user with encrypted secret and pending status
	user.TwoFactorSecret = base64.RawURLEncoding.EncodeToString(encryptedSecret)
	user.TwoFactorStatus = string(domain.TwoFactorStatusPending)
	user.TwoFactorEnabled = false

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// Generate QR code URL
	qrCodeURL := key.URL()

	return &domain.TwoFactorSetupResponse{
		Secret:    key.Secret(),
		QRCodeURL: qrCodeURL,
	}, nil
}

// VerifyAndEnable validates the TOTP code and enables 2FA for the user.
func (s *TwoFactorService) VerifyAndEnable(ctx context.Context, userID uuid.UUID, totpCode string) (*domain.TwoFactorSetupResponse, error) {
	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check if setup is pending
	if user.TwoFactorStatus != string(domain.TwoFactorStatusPending) {
		return nil, apperrors.NewAppError("TWO_FACTOR_SETUP_NOT_PENDING", "No pending two-factor setup found", 400)
	}

	// Decrypt the stored secret
	encryptedSecret, err := base64.RawURLEncoding.DecodeString(user.TwoFactorSecret)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Invalid two-factor secret", 400)
	}

	secretBytes, err := DecryptTOTPSecret(encryptedSecret, s.config.EncryptionKey)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Failed to decrypt two-factor secret", 400)
	}

	// Validate TOTP code with ±1 time step tolerance
	valid, err := totp.ValidateCustom(totpCode, string(secretBytes), time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return nil, apperrors.NewAppError("INVALID_TOTP_CODE", "Invalid TOTP code", 401)
	}

	// Enable 2FA
	user.TwoFactorEnabled = true
	user.TwoFactorStatus = string(domain.TwoFactorStatusEnabled)
	now := time.Now()
	user.TwoFactorVerifiedAt = &now

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// Soft-delete all existing recovery codes
	if err := s.twoFactorRecoveryCodeRepo.DeleteAllByUser(ctx, userID); err != nil {
		return nil, err
	}

	// Generate new recovery codes and store hashed versions
	plainCodes, hashedCodes := GenerateRecoveryCodes()
	for _, hash := range hashedCodes {
		code := &domain.TwoFactorRecoveryCode{
			UserID:   userID,
			CodeHash: hash,
		}
		if err := s.twoFactorRecoveryCodeRepo.Create(ctx, code); err != nil {
			return nil, err
		}
	}

	// Audit log
	if s.auditSvc != nil {
		s.auditSvc.LogAction(ctx, userID, "enable_2fa", "user", userID.String(), nil, nil, "", "")
	}

	return &domain.TwoFactorSetupResponse{
		RecoveryCodes: plainCodes,
		Enabled:       true,
		Status:        string(domain.TwoFactorStatusEnabled),
	}, nil
}

// GetStatus returns the current 2FA status for a user.
func (s *TwoFactorService) GetStatus(ctx context.Context, userID uuid.UUID) (*domain.TwoFactorSetupResponse, error) {
	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Decrypt secret for QR code URL if 2FA is pending
	var secret, qrCodeURL string
	if user.TwoFactorStatus == string(domain.TwoFactorStatusPending) && user.TwoFactorSecret != "" {
		encryptedSecret, err := base64.RawURLEncoding.DecodeString(user.TwoFactorSecret)
		if err == nil {
			secretBytes, err := DecryptTOTPSecret(encryptedSecret, s.config.EncryptionKey)
			if err == nil {
				secret = base64.RawURLEncoding.EncodeToString(secretBytes)
				// Generate QR URL using the secret
				// We need to reconstruct the key from the secret
				key, keyErr := totp.Generate(totp.GenerateOpts{
					Issuer:      "GoAPIBase",
					AccountName: user.Email,
					Period:      30,
					SecretSize:  20,
				})
				if keyErr == nil {
					qrCodeURL = key.URL()
				}
			}
		}
	}

	return &domain.TwoFactorSetupResponse{
		Secret:    secret,
		QRCodeURL: qrCodeURL,
		Enabled:   user.TwoFactorEnabled,
		Status:     string(user.TwoFactorStatus),
	}, nil
}

func (s *TwoFactorService) Disable(ctx context.Context, userID uuid.UUID, totpCode string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		return apperrors.NewAppError("2FA_NOT_ENABLED", "Two-factor authentication is not enabled", 400)
	}

	encryptedSecret, err := base64.RawURLEncoding.DecodeString(user.TwoFactorSecret)
	if err != nil {
		return apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Invalid two-factor secret", 400)
	}

	secretBytes, err := DecryptTOTPSecret(encryptedSecret, s.config.EncryptionKey)
	if err != nil {
		return apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Failed to decrypt two-factor secret", 400)
	}

	valid, err := totp.ValidateCustom(totpCode, string(secretBytes), time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return apperrors.NewAppError("INVALID_TOTP_CODE", "Invalid TOTP code", 401)
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	user.TwoFactorStatus = string(domain.TwoFactorStatusDisabled)
	user.TwoFactorVerifiedAt = nil

	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	if err := s.twoFactorRecoveryCodeRepo.DeleteAllByUser(ctx, userID); err != nil {
		return err
	}

	if s.auditSvc != nil {
		s.auditSvc.LogAction(ctx, userID, "disable_2fa", "user", userID.String(), nil, nil, "", "")
	}

	return nil
}

// Verify2FALogin validates a pending 2FA login and issues real JWT tokens.
func (s *TwoFactorService) Verify2FALogin(ctx context.Context, pendingToken string, totpCode string) (*domain.LoginResult, error) {
	tokenHash := auth.HashToken(pendingToken)

	token, err := s.refreshTokenRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewAppError("INVALID_PENDING_TOKEN", "Invalid pending token", 401)
		}
		return nil, err
	}

	if !token.Is2FAPending {
		return nil, apperrors.NewAppError("INVALID_PENDING_TOKEN", "Invalid pending token", 401)
	}

	if !token.IsValid() {
		return nil, apperrors.NewAppError("TOKEN_EXPIRED", "Pending token has expired", 401)
	}

	userID := token.UserID

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !user.TwoFactorEnabled {
		return nil, apperrors.NewAppError("TWO_FACTOR_NOT_ENABLED", "Two-factor authentication is not enabled", 400)
	}

	encryptedSecret, err := base64.RawURLEncoding.DecodeString(user.TwoFactorSecret)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Invalid two-factor secret", 400)
	}

	secretBytes, err := DecryptTOTPSecret(encryptedSecret, s.config.EncryptionKey)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Failed to decrypt two-factor secret", 400)
	}

	valid, err := totp.ValidateCustom(totpCode, string(secretBytes), time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return nil, apperrors.NewAppError("INVALID_TOTP_CODE", "Invalid TOTP code", 401)
	}

	if err := s.refreshTokenRepo.MarkRevoked(ctx, tokenHash); err != nil {
		return nil, err
	}

	now := time.Now()
	user.TwoFactorVerifiedAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	accessToken, err := s.tokenService.GenerateAccessToken(userID.String(), user.Email)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	refreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	newRefreshToken := &domain.RefreshToken{
		UserID:       userID,
		TokenHash:    refreshHash,
		ExpiresAt:    expiresAt,
		Is2FAPending: false,
	}
	if err := s.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.LogAction(ctx, userID, "verify_2fa_login", "user", userID.String(), nil, nil, "", "")
	}

	return &domain.LoginResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Requires2FA:  false,
		PendingToken: "",
		ExpiresIn:    int64(s.tokenService.GetRefreshExpiry().Seconds()),
	}, nil
}

// UseRecoveryCode validates a recovery code and issues real JWT tokens.
func (s *TwoFactorService) UseRecoveryCode(ctx context.Context, pendingToken string, code string) (*domain.LoginResult, error) {
	tokenHash := auth.HashToken(pendingToken)

	token, err := s.refreshTokenRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewAppError("INVALID_PENDING_TOKEN", "Invalid pending token", 401)
		}
		return nil, err
	}

	if !token.Is2FAPending {
		return nil, apperrors.NewAppError("INVALID_PENDING_TOKEN", "Invalid pending token", 401)
	}

	if !token.IsValid() {
		return nil, apperrors.NewAppError("TOKEN_EXPIRED", "Pending token has expired", 401)
	}

	userID := token.UserID

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !user.TwoFactorEnabled {
		return nil, apperrors.NewAppError("TWO_FACTOR_NOT_ENABLED", "Two-factor authentication is not enabled", 400)
	}

	providedHash, err := HashRecoveryCode(code)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_RECOVERY_CODE", "Failed to process recovery code", 400)
	}

	storedCode, err := s.twoFactorRecoveryCodeRepo.FindByUserAndHash(ctx, userID, providedHash)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewAppError("INVALID_RECOVERY_CODE", "Invalid recovery code", 401)
		}
		return nil, err
	}

	if err := s.twoFactorRecoveryCodeRepo.MarkUsed(ctx, storedCode.ID); err != nil {
		return nil, err
	}

	remainingCount, err := s.twoFactorRecoveryCodeRepo.CountUnusedByUser(ctx, userID)
	if err != nil {
		remainingCount = 0
	}

	if err := s.refreshTokenRepo.MarkRevoked(ctx, tokenHash); err != nil {
		return nil, err
	}

	now := time.Now()
	user.TwoFactorVerifiedAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	accessToken, err := s.tokenService.GenerateAccessToken(userID.String(), user.Email)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	refreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	newRefreshToken := &domain.RefreshToken{
		UserID:       userID,
		TokenHash:    refreshHash,
		ExpiresAt:    expiresAt,
		Is2FAPending: false,
	}
	if err := s.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		s.auditSvc.LogAction(ctx, userID, "use_recovery_code", "user", userID.String(), nil, nil, "", "")
	}

	_ = remainingCount

	return &domain.LoginResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Requires2FA:  false,
		PendingToken: "",
		ExpiresIn:    int64(s.tokenService.GetRefreshExpiry().Seconds()),
	}, nil
}

// RegenerateCodes regenerates recovery codes after validating TOTP.
func (s *TwoFactorService) RegenerateCodes(ctx context.Context, userID uuid.UUID, totpCode string) ([]string, error) {
	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled {
		return nil, apperrors.NewAppError("TWO_FACTOR_NOT_ENABLED", "Two-factor authentication is not enabled", 400)
	}

	// Decrypt the stored secret
	if user.TwoFactorSecret == "" {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Two-factor secret not found", 400)
	}

	encryptedSecret, err := base64.RawURLEncoding.DecodeString(user.TwoFactorSecret)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Invalid two-factor secret", 400)
	}

	secretBytes, err := DecryptTOTPSecret(encryptedSecret, s.config.EncryptionKey)
	if err != nil {
		return nil, apperrors.NewAppError("INVALID_TWO_FACTOR_SECRET", "Failed to decrypt two-factor secret", 400)
	}

	// Validate TOTP code with ±1 time step tolerance
	valid, err := totp.ValidateCustom(totpCode, string(secretBytes), time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return nil, apperrors.NewAppError("INVALID_TOTP_CODE", "Invalid TOTP code", 401)
	}

	// Check regeneration limit (24-hour cooldown)
	// Note: In production, store last_regeneration_time on user model
	// For now, we just generate codes without the check
	// if !CanRegenerateCodes(user.LastTwoFactorCodeRegenTime) {
	// 	return nil, apperrors.NewAppError("REGENERATION_LIMIT_REACHED", "Recovery codes can only be regenerated once per 24 hours", 429)
	// }

	// Soft-delete all old recovery codes
	if err := s.twoFactorRecoveryCodeRepo.DeleteAllByUser(ctx, userID); err != nil {
		return nil, err
	}

	// Generate new recovery codes
	plainCodes, hashedCodes := GenerateRecoveryCodes()

	// Store hashed recovery codes
	for _, hash := range hashedCodes {
		code := &domain.TwoFactorRecoveryCode{
			UserID:   userID,
			CodeHash: hash,
		}
		if err := s.twoFactorRecoveryCodeRepo.Create(ctx, code); err != nil {
			return nil, err
		}
	}

	// Audit log
	if s.auditSvc != nil {
		s.auditSvc.LogAction(ctx, userID, "regenerate_2fa_codes", "user", userID.String(), nil, nil, "", "")
	}

	return plainCodes, nil
}
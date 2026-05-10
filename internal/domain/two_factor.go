package domain

import (
	"github.com/google/uuid"
)

// TwoFactorStatus represents the status of two-factor authentication for a user
type TwoFactorStatus string

const (
	TwoFactorStatusDisabled TwoFactorStatus = "disabled"
	TwoFactorStatusPending  TwoFactorStatus = "pending"
	TwoFactorStatusEnabled  TwoFactorStatus = "enabled"
)

// TwoFactorSetupResponse contains the secret and QR code for setting up 2FA
type TwoFactorSetupResponse struct {
	Secret        string   `json:"secret,omitempty"`
	QRCodeURL     string   `json:"qr_code_url,omitempty"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
	Enabled       bool     `json:"enabled"`
	Status        string   `json:"status,omitempty"`
}

// LoginResult represents the result of a login attempt
// If Requires2FA is true, the caller must complete 2FA verification
// using the PendingToken before access tokens are issued
type LoginResult struct {
	User         *User   `json:"user,omitempty"`
	AccessToken  string  `json:"access_token,omitempty"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	Requires2FA  bool    `json:"requires_2fa"`
	PendingToken string  `json:"pending_token,omitempty"`
	ExpiresIn    int64   `json:"expires_in"` // Seconds until pending_token expires; 0 if not pending
}

// TwoFactorVerifyRequest represents a TOTP verification request
type TwoFactorVerifyRequest struct {
	UserID   uuid.UUID `json:"user_id"`
	TOTPCode string    `json:"totp_code"`
	Secret   string    `json:"secret,omitempty"` // Optional: for re-verification of setup
}

// TwoFactorRecoveryRequest represents a recovery code verification request
type TwoFactorRecoveryRequest struct {
	UserID uuid.UUID `json:"user_id"`
	Code   string    `json:"code"`
}
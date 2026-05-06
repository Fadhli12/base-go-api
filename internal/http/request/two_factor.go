package request

// InitiateSetupRequest represents initiating 2FA setup (empty - uses JWT user ID)
type InitiateSetupRequest struct {
}

// VerifyEnableRequest represents verifying TOTP code to enable 2FA
type VerifyEnableRequest struct {
	TOTPCode string `json:"totp_code" validate:"required,len=6,number"`
}

// DisableRequest represents disabling 2FA (requires TOTP code)
type DisableRequest struct {
	TOTPCode string `json:"totp_code" validate:"required,len=6,number"`
}

// Verify2FALoginRequest represents verifying 2FA code during login
type Verify2FALoginRequest struct {
	PendingToken string `json:"pending_token" validate:"required"`
	TOTPCode    string `json:"totp_code" validate:"required,len=6,number"`
}

// RecoveryLoginRequest represents using a recovery code to bypass 2FA
type RecoveryLoginRequest struct {
	PendingToken string `json:"pending_token" validate:"required"`
	Code         string `json:"code" validate:"required,len=8,number"`
}

// RegenerateCodesRequest represents regenerating 2FA recovery codes
type RegenerateCodesRequest struct {
	TOTPCode string `json:"totp_code" validate:"required,len=6,number"`
}

// Validate validates the InitiateSetupRequest
func (r *InitiateSetupRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the VerifyEnableRequest
func (r *VerifyEnableRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the DisableRequest
func (r *DisableRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the Verify2FALoginRequest
func (r *Verify2FALoginRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the RecoveryLoginRequest
func (r *RecoveryLoginRequest) Validate() error {
	return validate.Struct(r)
}

// Validate validates the RegenerateCodesRequest
func (r *RegenerateCodesRequest) Validate() error {
	return validate.Struct(r)
}

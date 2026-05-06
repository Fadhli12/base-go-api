package response

// TwoFactorSetupResponse is returned when 2FA setup is initiated
type TwoFactorSetupResponse struct {
	Status        string   `json:"status"`
	Secret        string   `json:"secret"`
	QRCodeURL     string   `json:"qr_code_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TwoFactorEnabledResponse is returned when 2FA is successfully enabled
type TwoFactorEnabledResponse struct {
	Status        string   `json:"status"`
	RecoveryCodes []string `json:"recovery_codes"`
	VerifiedAt    string   `json:"verified_at"`
}

// TwoFactorStatusResponse is returned when checking 2FA status
type TwoFactorStatusResponse struct {
	Status     string `json:"status"`
	VerifiedAt string `json:"verified_at"`
}

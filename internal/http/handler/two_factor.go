package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/labstack/echo/v4"
)

// TwoFactorHandler handles two-factor authentication endpoints
type TwoFactorHandler struct {
	twoFactorSvc *service.TwoFactorService
}

// NewTwoFactorHandler creates a new TwoFactorHandler instance
func NewTwoFactorHandler(twoFactorSvc *service.TwoFactorService) *TwoFactorHandler {
	return &TwoFactorHandler{
		twoFactorSvc: twoFactorSvc,
	}
}

// SetupResponse represents the response for 2FA setup initiation
type SetupResponse struct {
	Secret    string `json:"secret"`
	QRCodeURL string `json:"qr_code_url"`
}

// VerifyEnableResponse represents the response for 2FA verification and enable
type VerifyEnableResponse struct {
	Status        string   `json:"status"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// StatusResponse represents the response for 2FA status check
type StatusResponse struct {
	Enabled   bool   `json:"enabled"`
	Secret    string `json:"secret,omitempty"`
	QRCodeURL string `json:"qr_code_url,omitempty"`
	Status    string `json:"status"`
}

// DisableResponse represents the response for 2FA disable
type DisableResponse struct {
	Message string `json:"message"`
}

// RegenerateCodesResponse represents the response for recovery code regeneration
type RegenerateCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// Verify2FALoginRequest represents the request for 2FA login verification
type Verify2FALoginRequest struct {
	PendingToken string `json:"pending_token"`
	TOTPCode     string `json:"totp_code"`
}

// UseRecoveryCodeRequest represents the request for recovery code login
type UseRecoveryCodeRequest struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

// TwoFALoginResponse represents the response after successful 2FA login
type TwoFALoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *UserResponse `json:"user"`
}

// InitiateSetup handles POST /auth/2fa/setup
// Begins the 2FA setup process for the authenticated user
//
//	@Summary	Initiate 2FA setup
//	@Description	Begin two-factor authentication setup, returns QR code URL and secret
//	@Tags		2fa
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	201	{object}	response.Envelope{data=handler.SetupResponse}
//	@Failure	400	{object}	response.Envelope	"2FA already enabled or pending"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/auth/2fa/setup [post]
func (h *TwoFactorHandler) InitiateSetup(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "2fa setup failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "initiating 2FA setup", log.String("user_id", userID.String()))

	result, err := h.twoFactorSvc.InitiateSetup(ctx, userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "2FA setup initiation failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "2FA setup initiation failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to initiate 2FA setup"))
	}

	resp := SetupResponse{
		Secret:    result.Secret,
		QRCodeURL: result.QRCodeURL,
	}

	log.Info(ctx, "2FA setup initiated successfully",
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// VerifyAndEnable handles POST /auth/2fa/verify-enable
// Verifies TOTP code and enables 2FA for the authenticated user
//
//	@Summary	Verify and enable 2FA
//	@Description	Verify TOTP code and enable two-factor authentication
//	@Tags		2fa
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.Verify2FARequest	true	"TOTP code"
//	@Success	200	{object}	response.Envelope{data=handler.VerifyEnableResponse}
//	@Failure	400	{object}	response.Envelope	"No pending setup"
//	@Failure	401	{object}	response.Envelope	"Invalid TOTP code"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/auth/2fa/verify-enable [post]
func (h *TwoFactorHandler) VerifyAndEnable(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "2FA verify failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req struct {
		TOTPCode string `json:"totp_code"`
	}
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if req.TOTPCode == "" {
		log.Warn(ctx, "2FA verify failed - missing TOTP code",
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "TOTP code is required"))
	}

	log.Info(ctx, "verifying and enabling 2FA", log.String("user_id", userID.String()))

	result, err := h.twoFactorSvc.VerifyAndEnable(ctx, userID, req.TOTPCode)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "2FA verification failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "2FA verification failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify and enable 2FA"))
	}

	resp := VerifyEnableResponse{
		Status:        "enabled",
		RecoveryCodes: result.RecoveryCodes,
	}

	log.Info(ctx, "2FA enabled successfully",
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetStatus handles GET /auth/2fa/status
// Returns the current 2FA status for the authenticated user
//
//	@Summary	Get 2FA status
//	@Description	Get the current two-factor authentication status
//	@Tags		2fa
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Envelope{data=handler.StatusResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/auth/2fa/status [get]
func (h *TwoFactorHandler) GetStatus(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "get 2FA status failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "getting 2FA status", log.String("user_id", userID.String()))

	result, err := h.twoFactorSvc.GetStatus(ctx, userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to get 2FA status",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to get 2FA status",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get 2FA status"))
	}

	resp := StatusResponse{
		Enabled:   result.Enabled,
		Secret:    result.Secret,
		QRCodeURL: result.QRCodeURL,
		Status:    result.Status,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// Disable handles DELETE /auth/2fa
// Disables 2FA for the authenticated user after validating TOTP code
//
//	@Summary	Disable 2FA
//	@Description	Disable two-factor authentication with TOTP verification
//	@Tags		2fa
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.Disable2FARequest	true	"TOTP code"
//	@Success	200	{object}	response.Envelope{data=handler.DisableResponse}
//	@Failure	400	{object}	response.Envelope	"2FA not enabled"
//	@Failure	401	{object}	response.Envelope	"Invalid TOTP code"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/auth/2fa [delete]
func (h *TwoFactorHandler) Disable(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "2FA disable failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req struct {
		TOTPCode string `json:"totp_code"`
	}
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if req.TOTPCode == "" {
		log.Warn(ctx, "2FA disable failed - missing TOTP code",
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "TOTP code is required"))
	}

	log.Info(ctx, "disabling 2FA", log.String("user_id", userID.String()))

	if err := h.twoFactorSvc.Disable(ctx, userID, req.TOTPCode); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "2FA disable failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "2FA disable failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to disable 2FA"))
	}

	resp := DisableResponse{
		Message: "Two-factor authentication has been disabled",
	}

	log.Info(ctx, "2FA disabled successfully",
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// RegenerateCodes handles POST /auth/2fa/regenerate-codes
// Regenerates recovery codes after validating TOTP code
//
//	@Summary	Regenerate recovery codes
//	@Description	Regenerate two-factor authentication recovery codes
//	@Tags		2fa
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.RegenerateCodesRequest	true	"TOTP code"
//	@Success	200	{object}	response.Envelope{data=handler.RegenerateCodesResponse}
//	@Failure	400	{object}	response.Envelope	"2FA not enabled"
//	@Failure	401	{object}	response.Envelope	"Invalid TOTP code"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/auth/2fa/regenerate-codes [post]
func (h *TwoFactorHandler) RegenerateCodes(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "regenerate codes failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req struct {
		TOTPCode string `json:"totp_code"`
	}
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if req.TOTPCode == "" {
		log.Warn(ctx, "regenerate codes failed - missing TOTP code",
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "TOTP code is required"))
	}

	log.Info(ctx, "regenerating 2FA recovery codes", log.String("user_id", userID.String()))

	codes, err := h.twoFactorSvc.RegenerateCodes(ctx, userID, req.TOTPCode)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "regenerate codes failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "regenerate codes failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to regenerate recovery codes"))
	}

	resp := RegenerateCodesResponse{
		RecoveryCodes: codes,
	}

	log.Info(ctx, "recovery codes regenerated successfully",
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// Verify2FALogin handles POST /auth/2fa/verify
// Verifies TOTP code to complete 2FA login flow (public endpoint - no JWT required)
func (h *TwoFactorHandler) Verify2FALogin(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req Verify2FALoginRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if req.PendingToken == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "pending_token is required"))
	}
	if req.TOTPCode == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "totp_code is required"))
	}

	log.Info(ctx, "verifying 2FA login")

	result, err := h.twoFactorSvc.Verify2FALogin(ctx, req.PendingToken, req.TOTPCode)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "2FA login verification failed",
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "2FA login verification failed", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify 2FA login"))
	}

	resp := TwoFALoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		User: &UserResponse{
			ID:    result.User.ID.String(),
			Email: result.User.Email,
		},
	}

	log.Info(ctx, "2FA login verified successfully")
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// UseRecoveryCode handles POST /auth/2fa/recovery-login
// Uses a recovery code to complete 2FA login flow (public endpoint - no JWT required)
func (h *TwoFactorHandler) UseRecoveryCode(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req UseRecoveryCodeRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if req.PendingToken == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "pending_token is required"))
	}
	if req.Code == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "code is required"))
	}

	log.Info(ctx, "verifying recovery code login")

	result, err := h.twoFactorSvc.UseRecoveryCode(ctx, req.PendingToken, req.Code)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "recovery code login failed",
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "recovery code login failed", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to use recovery code"))
	}

	resp := TwoFALoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		User: &UserResponse{
			ID:    result.User.ID.String(),
			Email: result.User.Email,
		},
	}

	log.Info(ctx, "recovery code login successful")
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

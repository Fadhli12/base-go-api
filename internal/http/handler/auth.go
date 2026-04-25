package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/labstack/echo/v4"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler instance
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// RegisterRequestResponse represents the response for a successful registration
type RegisterRequestResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// LoginResponse represents the response for a successful login
type LoginResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

// UserResponse represents a user in the login response
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Register handles user registration
//
//	@Summary	Register a new user
//	@Description	Create a new user account with email and password
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.RegisterRequest	true	"Registration details"
//	@Success	201	{object}	response.Envelope{data=handler.RegisterRequestResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	409	{object}	response.Envelope	"Email already exists"
//	@Router		/api/v1/auth/register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req request.RegisterRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "registering user", log.String("email", req.Email))

	user, err := h.authService.Register(ctx, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "user registration failed",
				log.String("email", req.Email),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "user registration failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to register user"))
	}

	log.Info(ctx, "user registered successfully",
		log.String("user_id", user.ID.String()),
		log.String("email", user.Email),
	)

	resp := RegisterRequestResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// Login handles user authentication
//
//	@Summary	Login a user
//	@Description	Authenticate user and return access/refresh tokens
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.LoginRequest	true	"Login credentials"
//	@Success	200	{object}	response.Envelope{data=handler.LoginResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Invalid credentials"
//	@Router		/api/v1/auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req request.LoginRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "user login attempt", log.String("email", req.Email))

	user, accessToken, refreshToken, err := h.authService.Login(ctx, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "authentication failed",
				log.String("email", req.Email),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Warn(ctx, "authentication failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "INVALID_CREDENTIALS", "Invalid email or password"))
	}

	log.Info(ctx, "user logged in successfully",
		log.String("user_id", user.ID.String()),
		log.String("email", user.Email),
	)

	resp := LoginResponse{
		User: UserResponse{
			ID:    user.ID.String(),
			Email: user.Email,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// RefreshResponse represents the response for a successful token refresh
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Refresh handles token refresh
//
//	@Summary	Refresh access token
//	@Description	Exchange a valid refresh token for new access and refresh tokens
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.RefreshRequest	true	"Refresh token"
//	@Success	200	{object}	response.Envelope{data=handler.RefreshResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Invalid refresh token"
//	@Router		/api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req request.RefreshRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "token refresh attempt")

	user, accessToken, refreshToken, err := h.authService.Refresh(ctx, req.RefreshToken)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "token refresh failed",
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Warn(ctx, "token refresh failed", logger.Err(err))
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "INVALID_REFRESH_TOKEN", "Invalid refresh token"))
	}

	_ = user // User is available but not included in refresh response for security

	log.Info(ctx, "token refreshed successfully",
		log.String("user_id", user.ID.String()),
	)

	resp := RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// LogoutResponse represents the response for a successful logout
type LogoutResponse struct {
	Message string `json:"message"`
}

// Logout handles user logout
//
//	@Summary	Logout a user
//	@Description	Revoke all refresh tokens for the authenticated user
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Envelope{data=handler.LogoutResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "logout attempt without authentication")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	log.Info(ctx, "user logout attempt", log.String("user_id", userID.String()))

	if err := h.authService.Logout(ctx, userID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "logout failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "logout failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to logout"))
	}

	log.Info(ctx, "user logged out successfully", log.String("user_id", userID.String()))

	resp := LogoutResponse{
		Message: "Successfully logged out",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// PasswordResetRequestResponse represents the response for a password reset request
type PasswordResetRequestResponse struct {
	Message string `json:"message"`
}

// RequestPasswordReset handles password reset requests
//
//	@Summary	Request password reset
//	@Description	Send a password reset email to the user
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.PasswordResetRequest	true	"Email address"
//	@Success	200	{object}	response.Envelope{data=handler.PasswordResetRequestResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Router		/api/v1/auth/password-reset [post]
func (h *AuthHandler) RequestPasswordReset(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req request.PasswordResetRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Always return success to prevent email enumeration
	log.Info(ctx, "password reset requested", log.String("email", req.Email))

	_, err := h.authService.RequestPasswordReset(ctx, req.Email)
	if err != nil {
		// Log the error but don't reveal it to the user
		log.Error(ctx, "password reset request failed",
			log.String("email", req.Email),
			logger.Err(err),
		)
		// Note: We still return success to prevent email enumeration
	}

	resp := PasswordResetRequestResponse{
		Message: "If the email exists in our system, a password reset link has been sent.",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// PasswordResetConfirmResponse represents the response for password reset confirmation
type PasswordResetConfirmResponse struct {
	Message string `json:"message"`
}

// ResetPassword handles password reset confirmation
//
//	@Summary	Reset password
//	@Description	Reset user password using the token from email
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.PasswordResetConfirmRequest	true	"Reset token and new password"
//	@Success	200	{object}	response.Envelope{data=handler.PasswordResetConfirmResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request or expired token"
//	@Router		/api/v1/auth/password-reset/confirm [post]
func (h *AuthHandler) ResetPassword(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req request.PasswordResetConfirmRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "password reset attempt")

	if err := h.authService.ResetPassword(ctx, req.Token, req.Password); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Warn(ctx, "password reset failed",
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Warn(ctx, "password reset failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_RESET_TOKEN", "Invalid or expired password reset token"))
	}

	log.Info(ctx, "password reset successful")

	resp := PasswordResetConfirmResponse{
		Message: "Password has been reset successfully. Please log in with your new password.",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

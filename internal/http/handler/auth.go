package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
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
	var req request.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	user, err := h.authService.Register(c.Request().Context(), &req)
	if err != nil {
		// Error is already an AppError from the service layer
		return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "EMAIL_EXISTS", "Email already registered"))
	}

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
	var req request.LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	user, accessToken, refreshToken, err := h.authService.Login(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "INVALID_CREDENTIALS", "Invalid email or password"))
	}

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
	var req request.RefreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	user, accessToken, refreshToken, err := h.authService.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "INVALID_REFRESH_TOKEN", "Invalid refresh token"))
	}

	_ = user // User is available but not included in refresh response for security

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
//	@Param		X-User-ID	header	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=handler.LogoutResponse}
//	@Failure	400	{object}	response.Envelope	"Missing user ID"
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	// Get user ID from header (will be set by auth middleware in future)
	userIDStr := c.Request().Header.Get("X-User-ID")
	if userIDStr == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Missing user ID"))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	if err := h.authService.Logout(c.Request().Context(), userID); err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to logout"))
	}

	resp := LogoutResponse{
		Message: "Successfully logged out",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

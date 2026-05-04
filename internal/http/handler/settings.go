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

// SettingsHandler handles settings endpoints
type SettingsHandler struct {
	settingsService *service.SettingsService
}

// NewSettingsHandler creates a new SettingsHandler instance
func NewSettingsHandler(settingsService *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
	}
}

// GetUserSettings retrieves user settings for the current user
//
//	@Summary	Get user settings
//	@Description	Retrieves user-specific settings for the current user in the organization
//	@Tags		settings
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=domain.UserSettingsResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Router		/api/v1/settings/user [get]
//	@Security	BearerAuth
func (h *SettingsHandler) GetUserSettings(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, ok := middleware.GetOrganizationID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing organization context"))
	}

	settings, err := h.settingsService.GetUserSettings(ctx, userID, orgID)
	if err != nil {
		log.Error(ctx, "failed to get user settings", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve settings"))
	}

	return c.JSON(http.StatusOK, response.Success(settings))
}

// UpdateUserSettings updates user settings for the current user
//
//	@Summary	Update user settings
//	@Description	Updates user-specific settings for the current user in the organization
//	@Tags		settings
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.UpdateUserSettingsRequest	true	"Settings to update"
//	@Success	200	{object}	response.Envelope{data=domain.UserSettingsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	422	{object}	response.Envelope	"Validation failed"
//	@Router		/api/v1/settings/user [put]
//	@Security	BearerAuth
func (h *SettingsHandler) UpdateUserSettings(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, ok := middleware.GetOrganizationID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing organization context"))
	}

	var req request.UpdateUserSettingsRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	settings, err := h.settingsService.UpdateUserSettings(ctx, userID, orgID, req.Settings)
	if err != nil {
		log.Error(ctx, "failed to update user settings", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update settings"))
	}

	return c.JSON(http.StatusOK, response.Success(settings))
}

// GetEffectiveSettings retrieves merged user and system settings for the current user
//
//	@Summary	Get effective settings
//	@Description	Retrieves merged settings combining system defaults and user overrides
//	@Tags		settings
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=object}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Router		/api/v1/settings/effective [get]
//	@Security	BearerAuth
func (h *SettingsHandler) GetEffectiveSettings(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, ok := middleware.GetOrganizationID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing organization context"))
	}

	settings, err := h.settingsService.GetEffectiveSettings(ctx, userID, orgID)
	if err != nil {
		log.Error(ctx, "failed to get effective settings", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve settings"))
	}

	return c.JSON(http.StatusOK, response.Success(settings))
}

// GetSystemSettings retrieves system settings for the organization
//
//	@Summary	Get system settings
//	@Description	Retrieves organization-wide system settings
//	@Tags		settings
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=domain.SystemSettingsResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Router		/api/v1/settings/system [get]
//	@Security	BearerAuth
func (h *SettingsHandler) GetSystemSettings(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	orgID, ok := middleware.GetOrganizationID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing organization context"))
	}

	settings, err := h.settingsService.GetSystemSettings(ctx, orgID)
	if err != nil {
		log.Error(ctx, "failed to get system settings", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve settings"))
	}

	return c.JSON(http.StatusOK, response.Success(settings))
}

// UpdateSystemSettings updates system settings for the organization
//
//	@Summary	Update system settings
//	@Description	Updates organization-wide system settings (requires manage_system_settings permission)
//	@Tags		settings
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.UpdateSystemSettingsRequest	true	"Settings to update"
//	@Success	200	{object}	response.Envelope{data=domain.SystemSettingsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	422	{object}	response.Envelope	"Validation failed"
//	@Router		/api/v1/settings/system [put]
//	@Security	BearerAuth
func (h *SettingsHandler) UpdateSystemSettings(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	orgID, ok := middleware.GetOrganizationID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing organization context"))
	}

	var req request.UpdateSystemSettingsRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	settings, err := h.settingsService.UpdateSystemSettings(ctx, orgID, req.Settings)
	if err != nil {
		log.Error(ctx, "failed to update system settings", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update settings"))
	}

	return c.JSON(http.StatusOK, response.Success(settings))
}

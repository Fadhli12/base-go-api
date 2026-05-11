package handler

import (
	"encoding/json"
	"net/http"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type FeatureFlagHandler struct {
	service  *service.FeatureFlagService
	enforcer *permission.Enforcer
}

func NewFeatureFlagHandler(service *service.FeatureFlagService, enforcer *permission.Enforcer) *FeatureFlagHandler {
	return &FeatureFlagHandler{
		service:  service,
		enforcer:  enforcer,
	}
}

func (h *FeatureFlagHandler) Create(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveFeatureFlagOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	var req request.CreateFeatureFlagRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	flag, err := h.service.Create(ctx, userID, hasOrgID, orgID, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to create feature flag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create feature flag"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, flag))
}

func (h *FeatureFlagHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveFeatureFlagOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "view")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid feature flag ID"))
	}

	flag, err := h.service.GetByID(ctx, userID, hasOrgID, orgID, id)
	if err != nil {
		log.Error(ctx, "failed to get feature flag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get feature flag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, flag))
}

func (h *FeatureFlagHandler) List(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveFeatureFlagOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "view")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	pagination := ParsePagination(c)
	flags, total, err := h.service.List(ctx, userID, hasOrgID, orgID, pagination.Limit, pagination.Offset)
	if err != nil {
		log.Error(ctx, "failed to list feature flags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list feature flags"))
	}

	items := make([]json.RawMessage, 0, len(flags))
	for _, f := range flags {
		data, _ := json.Marshal(f)
		items = append(items, data)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"flags":  items,
		"total":  total,
		"limit":  pagination.Limit,
		"offset": pagination.Offset,
	}))
}

func (h *FeatureFlagHandler) Update(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveFeatureFlagOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid feature flag ID"))
	}

	var req request.UpdateFeatureFlagRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	flag, err := h.service.Update(ctx, userID, hasOrgID, orgID, id, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to update feature flag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update feature flag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, flag))
}

func (h *FeatureFlagHandler) Delete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveFeatureFlagOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "feature_flag", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid feature flag ID"))
	}

	if err := h.service.Delete(ctx, userID, hasOrgID, orgID, id, c.RealIP(), c.Request().UserAgent()); err != nil {
		log.Error(ctx, "failed to delete feature flag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete feature flag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "deleted"}))
}

func (h *FeatureFlagHandler) Evaluate(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Feature flag key is required"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	result, err := h.service.IsEnabledWithReason(ctx, userID, hasOrgID, orgID, key)
	if err != nil {
		log.Error(ctx, "failed to evaluate feature flag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to evaluate feature flag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

func (h *FeatureFlagHandler) EvaluateAll(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	result, err := h.service.EvaluateAll(ctx, userID, hasOrgID, orgID)
	if err != nil {
		log.Error(ctx, "failed to evaluate feature flags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to evaluate feature flags"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

func resolveFeatureFlagOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}
	return orgDomain
}
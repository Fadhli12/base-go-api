package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/domain"
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

type CommentHandler struct {
	service  *service.CommentService
	enforcer *permission.Enforcer
}

func NewCommentHandler(service *service.CommentService, enforcer *permission.Enforcer) *CommentHandler {
	return &CommentHandler{
		service:  service,
		enforcer: enforcer,
	}
}

func resolveCommentHandlerOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	if hasOrgID && orgID != uuid.Nil {
		return orgID.String()
	}
	return "default"
}

func (h *CommentHandler) Create(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	commentableType := c.Param("type")
	commentableIDStr := c.Param("id")

	if !domain.IsValidCommentableType(commentableType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid commentable type: "+commentableType))
	}

	commentableID, err := uuid.Parse(commentableIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid commentable ID"))
	}

	var req request.CreateCommentRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", err.Error()))
	}

	commentResp, err := h.service.Create(ctx, userID, hasOrgID, orgID, commentableType, commentableID, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to create comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create comment"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, commentResp))
}

func (h *CommentHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	commentResp, err := h.service.GetByID(ctx, userID, hasOrgID, orgID, id)
	if err != nil {
		log.Error(ctx, "failed to get comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get comment"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, commentResp))
}

func (h *CommentHandler) ListByCommentable(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	commentableType := c.Param("type")
	commentableIDStr := c.Param("id")

	if !domain.IsValidCommentableType(commentableType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid commentable type: "+commentableType))
	}

	commentableID, err := uuid.Parse(commentableIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid commentable ID"))
	}

	limit, offset := parseCommentPagination(c)

	comments, total, err := h.service.ListByCommentable(ctx, userID, hasOrgID, orgID, commentableType, commentableID, limit, offset)
	if err != nil {
		log.Error(ctx, "failed to list comments", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list comments"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.CommentListResponse{
		Comments: marshalComments(comments),
		Total:    total,
	}))
}

func (h *CommentHandler) ListReplies(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	parentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	limit, offset := parseCommentPagination(c)

	replies, total, err := h.service.ListReplies(ctx, userID, hasOrgID, orgID, parentID, limit, offset)
	if err != nil {
		log.Error(ctx, "failed to list replies", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list replies"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.CommentListResponse{
		Comments: marshalComments(replies),
		Total:    total,
	}))
}

func (h *CommentHandler) Update(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	var req request.UpdateCommentRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", err.Error()))
	}

	commentResp, err := h.service.Update(ctx, userID, hasOrgID, orgID, id, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to update comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update comment"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, commentResp))
}

func (h *CommentHandler) Delete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	if err := h.service.Delete(ctx, userID, hasOrgID, orgID, id, c.RealIP(), c.Request().UserAgent()); err != nil {
		log.Error(ctx, "failed to delete comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete comment"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Comment deleted successfully"}))
}

func (h *CommentHandler) Pin(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveCommentHandlerOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "comment", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	commentResp, err := h.service.Pin(ctx, userID, hasOrgID, orgID, id, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to pin comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to pin comment"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, commentResp))
}

func (h *CommentHandler) Unpin(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveCommentHandlerOrgDomain(hasOrgID, orgID)
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "comment", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid comment ID"))
	}

	commentResp, err := h.service.Unpin(ctx, userID, hasOrgID, orgID, id, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to unpin comment", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to unpin comment"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, commentResp))
}

func parseCommentPagination(c echo.Context) (limit, offset int) {
	limit = 20
	offset = 0
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}

func marshalComments(comments []*domain.CommentResponse) []json.RawMessage {
	data := make([]json.RawMessage, len(comments))
	for i, c := range comments {
		raw, _ := json.Marshal(c)
		data[i] = raw
	}
	return data
}
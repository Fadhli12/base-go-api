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
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type TagHandler struct {
	service *service.TagService
}

func NewTagHandler(service *service.TagService) *TagHandler {
	return &TagHandler{service: service}
}

func (h *TagHandler) Create(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var req request.CreateTagRequest
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

	tagResp, err := h.service.Create(ctx, hasOrgID, orgID, userID, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to create tag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create tag"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, tagResp))
}

func (h *TagHandler) List(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	sort := c.QueryParam("sort")
	order := c.QueryParam("order")

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if sort == "" {
		sort = "name"
	}

	tagList, err := h.service.List(ctx, hasOrgID, orgID, userID, limit, offset, sort, order)
	if err != nil {
		log.Error(ctx, "failed to list tags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list tags"))
	}

	tagsJSON := make([]json.RawMessage, len(tagList.Tags))
	for i, t := range tagList.Tags {
		b, _ := json.Marshal(t)
		tagsJSON[i] = b
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.TagListResponse{
		Tags:  tagsJSON,
		Total: tagList.Total,
	}))
}

func (h *TagHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid tag ID"))
	}

	tagResp, err := h.service.GetByID(ctx, hasOrgID, orgID, userID, id)
	if err != nil {
		log.Error(ctx, "failed to get tag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get tag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, tagResp))
}

func (h *TagHandler) GetBySlug(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	slug := c.Param("slug")
	if slug == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Slug parameter is required"))
	}

	tagResp, err := h.service.GetBySlug(ctx, hasOrgID, orgID, userID, slug)
	if err != nil {
		log.Error(ctx, "failed to get tag by slug", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get tag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, tagResp))
}

func (h *TagHandler) Update(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid tag ID"))
	}

	var req request.UpdateTagRequest
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

	tagResp, err := h.service.Update(ctx, hasOrgID, orgID, userID, id, req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to update tag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update tag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, tagResp))
}

func (h *TagHandler) Delete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid tag ID"))
	}

	if err := h.service.Delete(ctx, hasOrgID, orgID, userID, id, c.RealIP(), c.Request().UserAgent()); err != nil {
		log.Error(ctx, "failed to delete tag", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete tag"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "deleted"}))
}

func (h *TagHandler) Autocomplete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	query := c.QueryParam("q")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	tagResp, err := h.service.Autocomplete(ctx, hasOrgID, orgID, userID, query, limit)
	if err != nil {
		log.Error(ctx, "failed to autocomplete tags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to autocomplete tags"))
	}

	tagsJSON := make([]json.RawMessage, len(tagResp.Tags))
	for i, t := range tagResp.Tags {
		b, _ := json.Marshal(t)
		tagsJSON[i] = b
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.AutocompleteResponse{
		Tags: tagsJSON,
	}))
}

func (h *TagHandler) AttachTags(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	entityType := c.Param("type")
	entityIDStr := c.Param("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid entity ID"))
	}

	if !domain.IsValidTaggableType(entityType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid entity type: "+entityType))
	}

	var req request.AttachTagRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	req.EntityType = entityType
	req.EntityID = entityIDStr

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", err.Error()))
	}

	result, err := h.service.AttachTags(ctx, hasOrgID, orgID, userID, entityType, entityID, req.TagIDs, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to attach tags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to attach tags"))
	}

	toArray := func(tags []domain.TagResponse) []json.RawMessage {
		arr := make([]json.RawMessage, len(tags))
		for i, t := range tags {
			b, _ := json.Marshal(t)
			arr[i] = b
		}
		return arr
	}

	errsToJSON := func(errs []domain.BulkError) []json.RawMessage {
		arr := make([]json.RawMessage, len(errs))
		for i, e := range errs {
			b, _ := json.Marshal(e)
			arr[i] = b
		}
		return arr
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.BulkAttachResult{
		Attached: toArray(result.Attached),
		Skipped:  toArray(result.Skipped),
		Errors:   errsToJSON(result.Errors),
	}))
}

func (h *TagHandler) DetachTags(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	entityType := c.Param("type")
	entityIDStr := c.Param("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid entity ID"))
	}

	if !domain.IsValidTaggableType(entityType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid entity type: "+entityType))
	}

	var req request.DetachTagRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	req.EntityType = entityType
	req.EntityID = entityIDStr

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", err.Error()))
	}

	result, err := h.service.DetachTags(ctx, hasOrgID, orgID, userID, entityType, entityID, req.TagIDs, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		log.Error(ctx, "failed to detach tags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to detach tags"))
	}

	toArray := func(tags []domain.TagResponse) []json.RawMessage {
		arr := make([]json.RawMessage, len(tags))
		for i, t := range tags {
			b, _ := json.Marshal(t)
			arr[i] = b
		}
		return arr
	}

	errsToJSON := func(errs []domain.BulkError) []json.RawMessage {
		arr := make([]json.RawMessage, len(errs))
		for i, e := range errs {
			b, _ := json.Marshal(e)
			arr[i] = b
		}
		return arr
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.BulkDetachResult{
		Detached: toArray(result.Detached),
		Skipped:  toArray(result.Skipped),
		Errors:   errsToJSON(result.Errors),
	}))
}

func (h *TagHandler) ListEntityTags(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	entityType := c.Param("type")
	entityIDStr := c.Param("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid entity ID"))
	}

	if !domain.IsValidTaggableType(entityType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid entity type: "+entityType))
	}

	result, err := h.service.ListEntityTags(ctx, hasOrgID, orgID, userID, entityType, entityID)
	if err != nil {
		log.Error(ctx, "failed to list entity tags", logger.Err(err))
		if apperrors.IsAppError(err) {
			appErr := apperrors.GetAppError(err)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list entity tags"))
	}

	tagsJSON := make([]json.RawMessage, len(result.Tags))
	for i, t := range result.Tags {
		b, _ := json.Marshal(t)
		tagsJSON[i] = b
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.EntityTagsResponse{
		EntityID:   entityID.String(),
		EntityType: entityType,
		Tags:       tagsJSON,
	}))
}
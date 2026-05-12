package handler

import (
	"net/http"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ActivityHandler handles activity feed HTTP endpoints.
type ActivityHandler struct {
	activityService *service.ActivityService
	followService   *service.ActivityFollowService
	enforcer        *permission.Enforcer
}

// NewActivityHandler creates a new ActivityHandler instance.
func NewActivityHandler(
	activityService *service.ActivityService,
	followService *service.ActivityFollowService,
	enforcer *permission.Enforcer,
) *ActivityHandler {
	return &ActivityHandler{
		activityService: activityService,
		followService:   followService,
		enforcer:        enforcer,
	}
}

// List retrieves activities for the authenticated user's organization context.
func (h *ActivityHandler) List(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	// Parse query parameters
	limit := DefaultLimit
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > MaxLimit {
				limit = MaxLimit
			}
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := parseIntParam(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Build filters
	filters := repository.ActivityFilters{
		ActionType:   c.QueryParam("action_type"),
		ResourceType: c.QueryParam("resource_type"),
		ResourceID:  c.QueryParam("resource_id"),
	}

	if unread := c.QueryParam("unread"); unread != "" {
		val := unread == "true"
		filters.UnreadOnly = val
	}

	if since := c.QueryParam("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err == nil {
			filters.Since = &t
		}
	}

	result, err := h.activityService.ListByOrganization(
		c.Request().Context(), userID, hasOrgID, orgID, filters, limit, offset,
	)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list activities"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// CountUnread returns the number of unread activities for the user.
func (h *ActivityHandler) CountUnread(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	filters := repository.ActivityFilters{
		ActionType:   c.QueryParam("action_type"),
		ResourceType: c.QueryParam("resource_type"),
		ResourceID:  c.QueryParam("resource_id"),
	}

	count, err := h.activityService.CountUnread(
		c.Request().Context(), userID, hasOrgID, orgID, filters,
	)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to count unread activities"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.UnreadCountResponse{UnreadCount: count}))
}

// MarkAllRead marks all visible activities as read for the user.
func (h *ActivityHandler) MarkAllRead(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	count, err := h.activityService.MarkAllRead(
		c.Request().Context(), userID, hasOrgID, orgID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to mark all as read"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.MarkAllReadResponse{MarkedCount: count}))
}

// MarkAsRead marks a single activity as read for the user.
func (h *ActivityHandler) MarkAsRead(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	activityIDStr := c.Param("id")
	activityID, err := uuid.Parse(activityIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid activity ID"))
	}

	if err := h.activityService.MarkAsRead(
		c.Request().Context(), userID, activityID,
	); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to mark activity as read"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.MarkReadResponse{
		ActivityID: activityID.String(),
		ReadAt:     time.Now().UTC().Format(time.RFC3339),
	}))
}

// Follow creates a follow relationship for a resource.
func (h *ActivityHandler) Follow(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var req struct {
		ResourceType string `json:"resource_type" validate:"required"`
		ResourceID   string `json:"resource_id" validate:"required"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	// Validate resource type
	if !domain.IsValidResourceType(req.ResourceType) {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "invalid resource_type: "+req.ResourceType))
	}
	// Validate resource ID is a UUID
	if _, err := uuid.Parse(req.ResourceID); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", "resource_id must be a valid UUID"))
	}

	follow, err := h.followService.Follow(
		c.Request().Context(), userID, hasOrgID, orgID,
		req.ResourceType, req.ResourceID,
		c.RealIP(), c.Request().UserAgent(),
	)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to follow resource"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, follow.ToResponse()))
}

// Unfollow removes a follow relationship.
func (h *ActivityHandler) Unfollow(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	followIDStr := c.Param("id")
	followID, err := uuid.Parse(followIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid follow ID"))
	}

	if err := h.followService.Unfollow(
		c.Request().Context(), userID, followID,
		c.RealIP(), c.Request().UserAgent(),
	); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to unfollow resource"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "unfollowed"}))
}

// ListFollows lists all follows for the authenticated user.
func (h *ActivityHandler) ListFollows(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	pagination := ParsePagination(c)

	follows, total, err := h.followService.ListFollows(
		c.Request().Context(), userID, pagination.Limit, pagination.Offset,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list follows"))
	}

	responses := make([]domain.ActivityFollowResponse, len(follows))
	for i, f := range follows {
		responses[i] = f.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, response.ActivityFollowListResponse{
		Follows: responses,
		Total:   total,
	}))
}

// SoftDelete soft-deletes an activity. Requires activity:manage permission.
func (h *ActivityHandler) SoftDelete(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	activityIDStr := c.Param("id")
	activityID, err := uuid.Parse(activityIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid activity ID"))
	}

	// Permission check: activity:manage required for delete
	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "activity", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	if err := h.activityService.SoftDelete(
		c.Request().Context(), userID, hasOrgID, orgID, activityID,
	); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete activity"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "deleted"}))
}

// RegisterRoutes registers all activity feed routes on the given Echo group.
func (h *ActivityHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
	activities := v1.Group("/activities")

	// JWT authentication required for all activity endpoints
	activities.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))

	// Organization context extraction
	activities.Use(middleware.ExtractOrganizationID())

	// View permission required for all activity endpoints
	activities.Use(middleware.RequirePermission(h.enforcer, "activity", "view"))

	activities.GET("", h.List)
	activities.GET("/count-unread", h.CountUnread)
	activities.PUT("/read-all", h.MarkAllRead)
	activities.PUT("/:id/read", h.MarkAsRead)
	activities.POST("/follow", h.Follow)
	activities.DELETE("/follow/:id", h.Unfollow)
	activities.GET("/follows", h.ListFollows)

	// Manage permission required for delete - separate group with higher permission
	activities.DELETE("/:id", h.SoftDelete)
}
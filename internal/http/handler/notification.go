package handler

import (
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// NotificationHandler handles notification-related HTTP endpoints
type NotificationHandler struct {
	service *service.NotificationService
}

// NewNotificationHandler creates a new NotificationHandler instance
func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		service: service,
	}
}

// NotificationListResponse represents the response for notification list endpoints
type NotificationListResponse struct {
	Data   []domain.NotificationResponse `json:"data"`
	Total  int64                         `json:"total"`
	Limit  int                           `json:"limit"`
	Offset int                           `json:"offset"`
}

// List handles GET /api/v1/notifications
// Returns a paginated list of notifications for the authenticated user
//
//	@Summary		List notifications
//	@Description	List notifications for the authenticated user
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit		query	int		false	"Limit"		default(20)
//	@Param			offset		query	int		false	"Offset"	default(0)
//	@Param			unread_only	query	bool	false	"Filter unread only"
//	@Success		200	{object}	response.Envelope{data=NotificationListResponse}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/notifications [get]
func (h *NotificationHandler) List(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	limit := 20
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			if v > 100 {
				limit = 100
			} else {
				limit = v
			}
		}
	}

	offset := 0
	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	unreadOnly := false
	if unreadStr := c.QueryParam("unread_only"); unreadStr == "true" || unreadStr == "1" {
		unreadOnly = true
	}

	var notifications []*domain.Notification
	var total int64

	if unreadOnly {
		notifications, total, err = h.service.ListUnreadByUser(c.Request().Context(), userID, limit, offset)
	} else {
		notifications, total, err = h.service.ListByUser(c.Request().Context(), userID, limit, offset)
	}

	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list notifications"))
	}

	data := make([]domain.NotificationResponse, len(notifications))
	for i, n := range notifications {
		data[i] = n.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, NotificationListResponse{
		Data:   data,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}))
}

// CountUnread handles GET /api/v1/notifications/unread-count
// Returns the count of unread notifications for the authenticated user
//
//	@Summary		Count unread notifications
//	@Description	Get the count of unread notifications for the authenticated user
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Envelope{data=map[string]int64}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/notifications/unread-count [get]
func (h *NotificationHandler) CountUnread(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	count, err := h.service.CountUnread(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to count unread notifications"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]int64{
		"unread_count": count,
	}))
}

// MarkAsRead handles PATCH /api/v1/notifications/:id/read
// Marks a single notification as read
//
//	@Summary		Mark notification as read
//	@Description	Mark a specific notification as read
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Notification ID"
//	@Success		200	{object}	response.Envelope{data=map[string]string}
//	@Failure		400	{object}	response.Envelope	"Invalid notification ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		404	{object}	response.Envelope	"Notification not found"
//	@Router			/api/v1/notifications/{id}/read [patch]
func (h *NotificationHandler) MarkAsRead(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	notifID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid notification ID"))
	}

	if err := h.service.MarkAsRead(c.Request().Context(), notifID, userID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to mark notification as read"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"message": "Notification marked as read",
	}))
}

// MarkAllAsRead handles POST /api/v1/notifications/read-all
// Marks all unread notifications as read for the authenticated user
//
//	@Summary		Mark all notifications as read
//	@Description	Mark all unread notifications as read, optionally filtered by type
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			type	query	string	false	"Notification type filter"
//	@Success		200	{object}	response.Envelope{data=map[string]int64}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/notifications/read-all [post]
func (h *NotificationHandler) MarkAllAsRead(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var typeFilter *string
	if t := c.QueryParam("type"); t != "" {
		typeFilter = &t
	}

	count, err := h.service.MarkAllAsRead(c.Request().Context(), userID, typeFilter)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to mark notifications as read"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]int64{
		"marked_count": count,
	}))
}

// Archive handles DELETE /api/v1/notifications/:id
// Archives a notification (sets archived_at)
//
//	@Summary		Archive notification
//	@Description	Archive a specific notification
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Notification ID"
//	@Success		200	{object}	response.Envelope{data=map[string]string}
//	@Failure		400	{object}	response.Envelope	"Invalid notification ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		404	{object}	response.Envelope	"Notification not found"
//	@Router			/api/v1/notifications/{id} [delete]
func (h *NotificationHandler) Archive(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	notifID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid notification ID"))
	}

	if err := h.service.ArchiveNotification(c.Request().Context(), notifID, userID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to archive notification"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"message": "Notification archived",
	}))
}

// GetPreferences handles GET /api/v1/notifications/preferences
// Returns the notification preferences for the authenticated user
//
//	@Summary		Get notification preferences
//	@Description	Get all notification preferences for the authenticated user
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Envelope{data=[]domain.NotificationPreferenceResponse}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/notifications/preferences [get]
func (h *NotificationHandler) GetPreferences(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	prefs, err := h.service.GetPreferences(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch notification preferences"))
	}

	data := make([]domain.NotificationPreferenceResponse, len(prefs))
	for i, p := range prefs {
		data[i] = p.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, data))
}

// UpdatePreference handles PUT /api/v1/notifications/preferences
// Creates or updates a notification preference for the authenticated user
//
//	@Summary		Update notification preference
//	@Description	Create or update a notification preference for the authenticated user
//	@Tags			notifications
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body	request.UpdateNotificationPreferenceRequest	true	"Preference data"
//	@Success		200	{object}	response.Envelope{data=map[string]string}
//	@Failure		400	{object}	response.Envelope	"Invalid request"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/notifications/preferences [put]
func (h *NotificationHandler) UpdatePreference(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req request.UpdateNotificationPreferenceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	if err := h.service.UpdatePreference(c.Request().Context(), userID, req.NotificationType, *req.EmailEnabled, *req.PushEnabled); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update notification preference"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"message": "Notification preference updated",
	}))
}

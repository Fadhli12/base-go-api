package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/datatypes"
)

// NewsHandler handles news-related HTTP endpoints
type NewsHandler struct {
	service  *service.NewsService
	enforcer *permission.Enforcer
}

// NewNewsHandler creates a new NewsHandler instance
func NewNewsHandler(service *service.NewsService, enforcer *permission.Enforcer) *NewsHandler {
	return &NewsHandler{
		service:  service,
		enforcer: enforcer,
	}
}

// NewsListResponse represents the response for list endpoints
type NewsListResponse struct {
	Data   []domain.NewsResponse `json:"data"`
	Total  int64                 `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// Create handles POST /api/v1/news
// Creates a new news article
//
//	@Summary	Create news article
//	@Description	Create a new news article
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.CreateNewsRequest	true	"News article data"
//	@Success	201	{object}	response.Envelope{data=domain.NewsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/news [post]
func (h *NewsHandler) Create(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req request.CreateNewsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	news, err := h.service.Create(c.Request().Context(), userID, req.Title, req.Content, req.Excerpt, datatypes.JSON(req.Tags), datatypes.JSON(req.Metadata))
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create news article"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, news.ToResponse()))
}

// GetByID handles GET /api/v1/news/:id
// Returns a news article by ID
//
//	@Summary	Get news by ID
//	@Description	Get a news article by ID. Regular users can only see their own articles.
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"News ID"
//	@Success	200	{object}	response.Envelope{data=domain.NewsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid news ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"News not found"
//	@Router		/api/v1/news/{id} [get]
func (h *NewsHandler) GetByID(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse news ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid news ID"))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	news, err := h.service.GetByID(c.Request().Context(), userID, id, isAdmin)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "News not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch news"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, news.ToResponse()))
}

// GetBySlug handles GET /api/v1/news/slug/:slug
// Returns a news article by slug (public endpoint for published articles)
//
//	@Summary	Get news by slug
//	@Description	Get a published news article by its slug
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Param		slug	path	string	true	"News slug"
//	@Success	200	{object}	response.Envelope{data=domain.NewsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid slug"
//	@Failure	404	{object}	response.Envelope	"News not found"
//	@Router		/api/v1/news/slug/{slug} [get]
func (h *NewsHandler) GetBySlug(c echo.Context) error {
	slug := c.Param("slug")
	if slug == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Slug is required"))
	}

	// Try to get user ID if authenticated (for accessing non-published articles)
	userID, authErr := middleware.GetUserID(c)
	isAdmin := false
	if authErr == nil {
		isAdmin = h.isAdmin(c)
	}

	var news *domain.News
	var err error

	if authErr == nil {
		// User is authenticated, check ownership for non-published articles
		news, err = h.service.GetBySlugWithAuth(c.Request().Context(), userID, slug, isAdmin)
	} else {
		// User is not authenticated, only allow published articles
		news, err = h.service.GetBySlug(c.Request().Context(), slug)
	}

	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "News not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch news"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, news.ToResponse()))
}

// List handles GET /api/v1/news
// Lists news articles - regular users see their own, admins see all
//
//	@Summary	List news articles
//	@Description	List news articles. Regular users see their own articles, admins see all.
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		limit	query	int		false	"Limit"    	default(20)
//	@Param		offset	query	int		false	"Offset"   	default(0)
//	@Param		status	query	string	false	"Status filter" 	enum(draft,published,archived)
//	@Success	200	{object}	response.Envelope{data=NewsListResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/news [get]
func (h *NewsHandler) List(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse query parameters
	var query request.NewsListQuery
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	limit := query.GetLimit()
	offset := query.GetOffset()

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	var news []domain.News
	var total int64

	if isAdmin {
		// Admin can filter by status or see all
		if query.Status != "" {
			status := domain.NewsStatus(query.Status)
			news, total, err = h.service.ListByStatus(c.Request().Context(), status, limit, offset)
		} else {
			news, total, err = h.service.ListAll(c.Request().Context(), limit, offset)
		}
	} else {
		// Regular users only see their own articles
		if query.Status != "" {
			// Filter by status within user's articles
			// This requires a custom query - for now, fetch all and filter
			news, total, err = h.service.ListByAuthor(c.Request().Context(), userID, limit, offset)
			// Filter by status in memory for simplicity
			var filtered []domain.News
			for _, n := range news {
				if string(n.Status) == query.Status {
					filtered = append(filtered, n)
				}
			}
			news = filtered
		} else {
			news, total, err = h.service.ListByAuthor(c.Request().Context(), userID, limit, offset)
		}
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list news"))
	}

	// Convert to response
	responses := make([]domain.NewsResponse, len(news))
	for i, n := range news {
		responses[i] = n.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, NewsListResponse{
		Data:   responses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}))
}

// Update handles PUT /api/v1/news/:id
// Updates a news article with ownership check
//
//	@Summary	Update news article
//	@Description	Update a news article. Users can only update their own articles.
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string					true	"News ID"
//	@Param		request	body	request.UpdateNewsRequest	true	"News data"
//	@Success	200	{object}	response.Envelope{data=domain.NewsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"News not found"
//	@Router		/api/v1/news/{id} [put]
func (h *NewsHandler) Update(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse news ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid news ID"))
	}

	var req request.UpdateNewsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	// Convert status string to NewsStatus
	var status domain.NewsStatus
	if req.Status != "" {
		status = domain.NewsStatus(req.Status)
	}

	news, err := h.service.Update(c.Request().Context(), userID, id, req.Title, req.Content, req.Excerpt, datatypes.JSON(req.Tags), datatypes.JSON(req.Metadata), status, isAdmin)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "News not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update news"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, news.ToResponse()))
}

// Delete handles DELETE /api/v1/news/:id
// Soft-deletes a news article with ownership check
//
//	@Summary	Delete news article
//	@Description	Soft delete a news article. Users can only delete their own articles.
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"News ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid news ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"News not found"
//	@Router		/api/v1/news/{id} [delete]
func (h *NewsHandler) Delete(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse news ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid news ID"))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	if err := h.service.Delete(c.Request().Context(), userID, id, isAdmin); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "News not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete news"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "News deleted successfully"}))
}

// UpdateStatus handles PATCH /api/v1/news/:id/status
// Updates news article status
//
//	@Summary	Update news status
//	@Description	Update the status of a news article
//	@Tags		news
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string						true	"News ID"
//	@Param		request	body	request.UpdateNewsStatusRequest	true	"Status update"
//	@Success	200	{object}	response.Envelope{data=domain.NewsResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"News not found"
//	@Router		/api/v1/news/{id}/status [patch]
func (h *NewsHandler) UpdateStatus(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse news ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid news ID"))
	}

	var req request.UpdateNewsStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	status := domain.NewsStatus(req.Status)
	if err := h.service.UpdateStatus(c.Request().Context(), userID, id, status, isAdmin); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "News not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update news status"))
	}

	// Fetch updated news
	news, err := h.service.GetByID(c.Request().Context(), userID, id, isAdmin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch updated news"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, news.ToResponse()))
}

// isAdmin checks if the current user has admin privileges for news
func (h *NewsHandler) isAdmin(c echo.Context) bool {
	if h.enforcer == nil {
		return false
	}

	claims, err := middleware.GetUserClaims(c)
	if err != nil {
		return false
	}

	// Check if user has permission to manage all news (not just own)
	allowed, err := h.enforcer.Enforce(claims.UserID, "default", "news", "manage")
	if err != nil {
		return false
	}

	return allowed
}

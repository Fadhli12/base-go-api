package handler

import (
	"context"
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

// SearchHandler handles search and saved search endpoints
type SearchHandler struct {
	searchSvc    *service.SearchService
	auditService *service.AuditService
}

// NewSearchHandler creates a new SearchHandler instance
func NewSearchHandler(searchSvc *service.SearchService, auditService *service.AuditService) *SearchHandler {
	return &SearchHandler{
		searchSvc:    searchSvc,
		auditService: auditService,
	}
}

// Search handles GET /api/v1/search
// Performs full-text search on news articles with faceted filtering
//
//	@Summary	Search news articles
//	@Description	Perform full-text search on news articles with optional filters
//	@Tags		search
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		q		query	string	false	"Search query"
//	@Param		status	query	string	false	"Filter by status"
//	@Param		author_id	query	string	false	"Filter by author"
//	@Param		date_from	query	string	false	"Filter from date (RFC3339)"
//	@Param		date_to		query	string	false	"Filter to date (RFC3339)"
//	@Param		page		query	int		false	"Page number"	default(1)
//	@Param		page_size	query	int		false	"Items per page"	default(20)
//	@Param		sort_by		query	string	false	"Sort field (relevance, title, created_at, updated_at)"
//	@Param		sort_dir	query	string	false	"Sort direction (asc, desc)"	default(desc)
//	@Success	200	{object}	response.Envelope{data=response.SearchResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/search [get]
func (h *SearchHandler) Search(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "search failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse query parameters
	query := c.QueryParam("q")
	if query == "" {
		query = c.QueryParam("query")
	}

	// Build filters from query params
	filters := make(map[string]interface{})
	if status := c.QueryParam("status"); status != "" {
		filters["status"] = status
	}
	if authorID := c.QueryParam("author_id"); authorID != "" {
		filters["author_id"] = authorID
	}
	if dateFrom := c.QueryParam("date_from"); dateFrom != "" {
		filters["date_from"] = dateFrom
	}
	if dateTo := c.QueryParam("date_to"); dateTo != "" {
		filters["date_to"] = dateTo
	}

	// Parse pagination
	page := 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := parsePositiveInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	pageSize := 20
	if pageSizeStr := c.QueryParam("page_size"); pageSizeStr != "" {
		if ps, err := parsePositiveInt(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	sortBy := c.QueryParam("sort_by")
	sortDir := c.QueryParam("sort_dir")
	if sortDir == "" {
		sortDir = "desc"
	}

	log.Info(ctx, "searching",
		log.String("user_id", userID.String()),
		log.String("query", query),
		log.Int("page", page),
		log.Int("page_size", pageSize),
	)

	result, err := h.searchSvc.Search(ctx, userID, query, filters, page, pageSize, sortBy, sortDir)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "search failed",
				log.String("user_id", userID.String()),
				log.String("query", query),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "search failed",
			log.String("user_id", userID.String()),
			log.String("query", query),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Search failed"))
	}

	// Convert results to response items
	items := make([]response.SearchItem, 0, len(result.Items))
for _, item := range result.Items {
		searchItem := response.SearchItem{
			ID:       getStringValue(item, "id"),
			Title:    getStringValue(item, "title"),
			Excerpt:  getStringValue(item, "excerpt"),
			Status:   getStringValue(item, "status"),
			AuthorID: getStringValue(item, "author_id"),
			Metadata: extractMetadata(item),
		}
		if rank, ok := item["rank"].(float64); ok {
			searchItem.Rank = rank
		}
		items = append(items, searchItem)
	}

	resp := response.SearchResponse{
		Items:      items,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		Highlights: result.Highlights,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// CreateSavedSearch handles POST /api/v1/saved-searches
// Creates a new saved search for the authenticated user
//
//	@Summary	Create saved search
//	@Description	Save a search query with filters for later use
//	@Tags		saved-searches
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.SavedSearchCreateRequest	true	"Saved search details"
//	@Success	201	{object}	response.Envelope{data=domain.SavedSearchResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	422	{object}	response.Envelope	"Validation error"
//	@Router		/api/v1/saved-searches [post]
func (h *SearchHandler) CreateSavedSearch(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "create saved search failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req request.SavedSearchCreateRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "creating saved search",
		log.String("user_id", userID.String()),
		log.String("name", req.Name),
	)

	// Convert filters from map[string]string to map[string]interface{}
	var filters map[string]interface{}
	if req.Filters != nil {
		filters = make(map[string]interface{})
		for k, v := range req.Filters {
			filters[k] = v
		}
	}

	savedSearch, err := h.searchSvc.CreateSavedSearch(ctx, userID, req.Name, req.QueryText, filters)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "create saved search failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "create saved search failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create saved search"))
	}

	log.Info(ctx, "saved search created successfully",
		log.String("id", savedSearch.ID.String()),
		log.String("user_id", userID.String()),
	)

	if h.auditService != nil {
		go func() {
			bgCtx := context.Background()
			h.auditService.LogAction(bgCtx, &domain.AuditLog{
				ActorID:    userID,
				Action:     domain.AuditActionCreate,
				Resource:   "saved_search",
				ResourceID: savedSearch.ID.String(),
				After:      savedSearch.ToResponse(),
				IPAddress:  c.RealIP(),
				UserAgent:  c.Request().Header.Get("User-Agent"),
			})
		}()
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, savedSearch.ToResponse()))
}

// ListSavedSearches handles GET /api/v1/saved-searches
// Lists all saved searches for the authenticated user
//
//	@Summary	List saved searches
//	@Description	Retrieve all saved searches for the current user
//	@Tags		saved-searches
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Envelope{data=[]domain.SavedSearchResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/saved-searches [get]
func (h *SearchHandler) ListSavedSearches(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "list saved searches failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "listing saved searches",
		log.String("user_id", userID.String()),
	)

	searches, err := h.searchSvc.ListSavedSearches(ctx, userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "list saved searches failed",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "list saved searches failed",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list saved searches"))
	}

	resp := make([]domain.SavedSearchResponse, len(searches))
	for i, search := range searches {
		resp[i] = search.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetSavedSearch handles GET /api/v1/saved-searches/:id
// Retrieves a single saved search by ID (ownership validated by service)
//
//	@Summary	Get saved search by ID
//	@Description	Retrieve a specific saved search by its ID
//	@Tags		saved-searches
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Saved search ID"
//	@Success	200	{object}	response.Envelope{data=domain.SavedSearchResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid saved search ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Saved search not found"
//	@Router		/api/v1/saved-searches/{id} [get]
func (h *SearchHandler) GetSavedSearch(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid saved search ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid saved search ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "get saved search failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "fetching saved search",
		log.String("id", id.String()),
		log.String("user_id", userID.String()),
	)

	search, err := h.searchSvc.GetSavedSearch(ctx, userID, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve saved search",
				log.String("id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve saved search",
			log.String("id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve saved search"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, search.ToResponse()))
}

// UpdateSavedSearch handles PUT /api/v1/saved-searches/:id
// Updates an existing saved search (ownership validated by service)
//
//	@Summary	Update saved search
//	@Description	Update an existing saved search's name, query, or filters
//	@Tags		saved-searches
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Saved search ID"
//	@Param		request	body	request.SavedSearchUpdateRequest	true	"Update details"
//	@Success	200	{object}	response.Envelope{data=domain.SavedSearchResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Saved search not found"
//	@Failure	422	{object}	response.Envelope	"Validation error"
//	@Router		/api/v1/saved-searches/{id} [put]
func (h *SearchHandler) UpdateSavedSearch(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid saved search ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid saved search ID"))
	}

	var req request.SavedSearchUpdateRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "update saved search failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "updating saved search",
		log.String("id", id.String()),
		log.String("user_id", userID.String()),
	)

	// Convert filters from map[string]string to map[string]interface{}
	var filters map[string]interface{}
	if req.Filters != nil {
		filters = make(map[string]interface{})
		for k, v := range req.Filters {
			filters[k] = v
		}
	}

	search, err := h.searchSvc.UpdateSavedSearch(ctx, userID, id, req.Name, req.QueryText, filters)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "update saved search failed",
				log.String("id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "update saved search failed",
			log.String("id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update saved search"))
	}

	log.Info(ctx, "saved search updated successfully",
		log.String("id", id.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, search.ToResponse()))
}

// DeleteSavedSearch handles DELETE /api/v1/saved-searches/:id
// Deletes a saved search (ownership validated by service)
//
//	@Summary	Delete saved search
//	@Description	Soft delete a saved search
//	@Tags		saved-searches
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Saved search ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid saved search ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Saved search not found"
//	@Router		/api/v1/saved-searches/{id} [delete]
func (h *SearchHandler) DeleteSavedSearch(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid saved search ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid saved search ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "delete saved search failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	log.Info(ctx, "deleting saved search",
		log.String("id", id.String()),
		log.String("user_id", userID.String()),
	)

	if err := h.searchSvc.DeleteSavedSearch(ctx, userID, id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "delete saved search failed",
				log.String("id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "delete saved search failed",
			log.String("id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete saved search"))
	}

	log.Info(ctx, "saved search deleted successfully",
		log.String("id", id.String()),
	)

	if h.auditService != nil {
		go func() {
			bgCtx := context.Background()
			h.auditService.LogAction(bgCtx, &domain.AuditLog{
				ActorID:    userID,
				Action:     domain.AuditActionDelete,
				Resource:   "saved_search",
				ResourceID: id.String(),
				IPAddress:  c.RealIP(),
				UserAgent:  c.Request().Header.Get("User-Agent"),
			})
		}()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Saved search deleted successfully"}))
}

// parsePositiveInt parses a string to a positive integer
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "must be positive")
	}
	return n, nil
}

// getStringValue safely extracts a string value from a map
func getStringValue(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getTimePtrValue safely extracts a time pointer value from a map
func getTimePtrValue(m map[string]interface{}, key string) *string {
	if v, ok := m[key]; ok && v != nil {
		if ts, ok := v.(string); ok {
			return &ts
		}
	}
	return nil
}

// extractMetadata extracts non-standard fields as metadata
func extractMetadata(m map[string]interface{}) map[string]interface{} {
	knownFields := map[string]bool{
		"id": true, "title": true, "excerpt": true, "status": true,
		"author_id": true, "rank": true, "created_at": true,
		"updated_at": true, "published_at": true, "tags": true,
	}
	metadata := make(map[string]interface{})
	for k, v := range m {
		if !knownFields[k] && v != nil {
			metadata[k] = v
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}
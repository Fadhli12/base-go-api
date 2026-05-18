package handler

import (
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// IdempotencyHandler handles idempotency key management HTTP endpoints.
type IdempotencyHandler struct {
	service      *service.IdempotencyService
	auditService *service.AuditService
	enforcer     *permission.Enforcer
}

// NewIdempotencyHandler creates a new IdempotencyHandler instance.
func NewIdempotencyHandler(service *service.IdempotencyService, auditService *service.AuditService, enforcer *permission.Enforcer) *IdempotencyHandler {
	return &IdempotencyHandler{
		service:      service,
		auditService: auditService,
		enforcer:     enforcer,
	}
}

// RegisterRoutes registers all idempotency routes on the given Echo group.
func (h *IdempotencyHandler) RegisterRoutes(api *echo.Group, jwtSecret string) {
	keys := api.Group("/idempotency/keys")
	keys.Use(middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))

	// View permission required for listing and getting
	keys.Use(middleware.RequirePermission(h.enforcer, "idempotency", "view"))

	keys.GET("", h.List)
	keys.GET("/:id", h.GetByID)

	// Delete requires manage permission
	keysManage := api.Group("/idempotency/keys")
	keysManage.Use(middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))
	keysManage.Use(middleware.RequirePermission(h.enforcer, "idempotency", "manage"))
	keysManage.DELETE("/:id", h.Delete)

	// Manage permission for cleanup trigger
	cleanup := api.Group("/idempotency/cleanup")
	cleanup.Use(middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))
	cleanup.Use(middleware.RequirePermission(h.enforcer, "idempotency", "manage"))
	cleanup.POST("", h.TriggerCleanup)
}

// List returns paginated idempotency records for the current user.
func (h *IdempotencyHandler) List(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	// Parse pagination parameters
	cfg := h.service.Config()
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if perPage < 1 {
		perPage = cfg.DefaultPageSize
	}
	if perPage > cfg.MaxPageSize {
		perPage = cfg.MaxPageSize
	}

	records, total, err := h.service.ListByUser(c.Request().Context(), userID, page, perPage)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list idempotency records"))
	}

	// Convert to response DTOs
	items := make([]*domain.IdempotencyRecordResponse, 0, len(records))
	for _, r := range records {
		items = append(items, r.ToResponse())
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, &domain.IdempotencyListResponse{
		Records: items,
		Meta: &domain.ListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
		},
	}))
}

// GetByID retrieves a single idempotency record by ID.
func (h *IdempotencyHandler) GetByID(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_ID", "Invalid record ID format"))
	}

	record, err := h.service.FindRecordByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Idempotency record not found"))
	}

	// Ownership check: user can only see their own records (or same org)
	if record.UserID != userID {
		// Check org match if applicable
		orgID, hasOrgID := middleware.GetOrganizationID(c)
		if !hasOrgID || orgID == uuid.Nil || record.OrganizationID == nil || *record.OrganizationID != orgID {
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "You can only access your own idempotency records"))
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, record.ToResponse()))
}

// TriggerCleanup triggers manual cleanup of expired idempotency records.
// Returns 202 Accepted since cleanup runs asynchronously.
func (h *IdempotencyHandler) TriggerCleanup(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	retentionDays := h.service.Config().RetentionDays

	// Allow override via query parameter
	if daysStr := c.QueryParam("retention_days"); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
			retentionDays = days
		}
	}

	count, err := h.service.CleanupExpired(retentionDays)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to trigger cleanup"))
	}

	_ = h.auditService.LogAction(c.Request().Context(), userID, "cleanup", "idempotency_key", "", nil, nil, c.RealIP(), c.Request().UserAgent())

	return c.JSON(http.StatusAccepted, response.SuccessWithContext(c, map[string]interface{}{
		"status":         "accepted",
		"cleaned_count":  count,
		"retention_days": retentionDays,
	}))
}

// Delete soft-deletes an idempotency record by ID.
func (h *IdempotencyHandler) Delete(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_ID", "Invalid record ID format"))
	}

	// Ownership check: get record first, verify it belongs to user
	record, err := h.service.FindRecordByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Idempotency record not found"))
	}
	if record.UserID != userID {
		orgID, hasOrgID := middleware.GetOrganizationID(c)
		if !hasOrgID || orgID == uuid.Nil || record.OrganizationID == nil || *record.OrganizationID != orgID {
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "You can only delete your own idempotency records"))
		}
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete idempotency record"))
	}

	_ = h.auditService.LogAction(c.Request().Context(), userID, "delete", "idempotency_key", id.String(), nil, nil, c.RealIP(), c.Request().UserAgent())

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{"status": "deleted"}))
}
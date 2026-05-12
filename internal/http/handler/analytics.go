package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AggregationTrigger defines the interface for triggering aggregation.
type AggregationTrigger interface {
	Trigger(ctx context.Context)
}

// AnalyticsHandler handles analytics HTTP endpoints.
type AnalyticsHandler struct {
	analyticsService  *service.AnalyticsService
	enforcer         *permission.Enforcer
	aggregationWorker AggregationTrigger
}

// NewAnalyticsHandler creates a new AnalyticsHandler instance.
func NewAnalyticsHandler(analyticsService *service.AnalyticsService, enforcer *permission.Enforcer, aggregationWorker AggregationTrigger) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService:  analyticsService,
		enforcer:         enforcer,
		aggregationWorker: aggregationWorker,
	}
}

// GetDashboard retrieves aggregated dashboard data for the authenticated user's organization context.
func (h *AnalyticsHandler) GetDashboard(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var req request.DashboardQuery
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request parameters"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid request parameters"))
	}

	result, err := h.analyticsService.GetDashboard(c.Request().Context(), userID, hasOrgID, orgID, req.Period)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get dashboard data"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// GetMetrics retrieves time-series metric data for the given filters.
func (h *AnalyticsHandler) GetMetrics(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var req request.MetricsQuery
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request parameters"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid request parameters"))
	}

	// Parse date range, defaulting to last 7 days
	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	if req.From != "" {
		parsedFrom, err := time.Parse("2006-01-02", req.From)
		if err != nil {
			return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid 'from' date format, expected YYYY-MM-DD"))
		}
		from = parsedFrom
	}

	if req.To != "" {
		parsedTo, err := time.Parse("2006-01-02", req.To)
		if err != nil {
			return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid 'to' date format, expected YYYY-MM-DD"))
		}
		to = parsedTo
	}

	// Validate date range: from must be before or equal to to
	if from.After(to) {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "'from' date must be before or equal to 'to' date"))
	}

	// Split event types by comma and validate each
	eventTypes := strings.Split(req.Type, ",")
	validEventTypes := make([]string, 0, len(eventTypes))
	for _, et := range eventTypes {
		et = strings.TrimSpace(et)
		if et == "" {
			continue
		}
		if !domain.ValidateEventType(et) {
			return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid event type: "+et))
		}
		validEventTypes = append(validEventTypes, et)
	}

	if len(validEventTypes) == 0 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "At least one valid event type is required"))
	}

	result, err := h.analyticsService.GetMetrics(c.Request().Context(), userID, hasOrgID, orgID, validEventTypes, req.Period, from, to)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get metrics data"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// GetPreferences retrieves dashboard preferences for the current organization context.
func (h *AnalyticsHandler) GetPreferences(c echo.Context) error {
	orgID, hasOrgID := middleware.GetOrganizationID(c)

	categories, err := h.analyticsService.GetPreferences(c.Request().Context(), hasOrgID, orgID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get preferences"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"metric_categories": categories,
	}))
}

// UpdatePreferences updates dashboard preferences for the current organization context.
func (h *AnalyticsHandler) UpdatePreferences(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	// Permission check: analytics:manage required for updating preferences
	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "analytics", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	var req request.UpdatePreferencesRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid request parameters"))
	}

	result, err := h.analyticsService.UpdatePreferences(c.Request().Context(), orgID, userID, req.MetricCategories)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update preferences"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// TriggerAggregation triggers the analytics aggregation worker.
// Returns 202 Accepted since actual aggregation is performed asynchronously.
func (h *AnalyticsHandler) TriggerAggregation(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Missing user context"))
	}

	// Permission check: analytics:manage required for triggering aggregation
	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}
	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "analytics", "manage")
	if err != nil || !allowed {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	// Trigger aggregation asynchronously in a goroutine so we can return 202 immediately
	go h.aggregationWorker.Trigger(c.Request().Context())

	return c.JSON(http.StatusAccepted, response.SuccessWithContext(c, map[string]string{
		"status":  "accepted",
		"message": "Aggregation triggered successfully",
	}))
}

// RegisterRoutes registers all analytics routes on the given Echo group.
func (h *AnalyticsHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
	analytics := v1.Group("/analytics")

	// JWT authentication required for all analytics endpoints
	analytics.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))

	// Organization context extraction
	analytics.Use(middleware.ExtractOrganizationID())

	// View permission required for all analytics endpoints
	analytics.Use(middleware.RequirePermission(h.enforcer, "analytics", "view"))

	// View routes (analytics:view permission required)
	analytics.GET("/dashboard", h.GetDashboard)
	analytics.GET("/metrics", h.GetMetrics)
	analytics.GET("/dashboard/preferences", h.GetPreferences)

	// Manage routes (analytics:manage permission required)
	manage := analytics.Group("")
	manage.Use(middleware.RequirePermission(h.enforcer, "analytics", "manage"))
	manage.PUT("/dashboard/preferences", h.UpdatePreferences)
	manage.POST("/aggregate", h.TriggerAggregation)
}
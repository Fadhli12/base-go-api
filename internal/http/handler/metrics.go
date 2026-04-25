package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/service"
	"github.com/labstack/echo/v4"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	metrics *service.AuthMetrics
}

// NewMetricsHandler creates a new MetricsHandler instance
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		metrics: service.GetAuthMetrics(),
	}
}

// GetAuthMetrics returns current authentication metrics
func (h *MetricsHandler) GetAuthMetrics(c echo.Context) error {
	snapshot := h.metrics.Snapshot()
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": snapshot,
		"meta": map[string]interface{}{
			"type": "auth_metrics",
		},
	})
}
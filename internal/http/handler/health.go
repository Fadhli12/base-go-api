package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthStatusResponse represents the response for health check endpoints
type HealthStatusResponse struct {
	Status string `json:"status" example:"ok"`
}

// ReadyzSuccessResponse represents the successful response for readiness probe
type ReadyzSuccessResponse struct {
	Status string            `json:"status" example:"ready"`
	Checks map[string]string `json:"checks"`
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *gorm.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redisClient,
	}
}

// Healthz handles liveness probe - always returns ok if server is running
//
//	@Summary	Liveness probe
//	@Description	Check if the server is alive
//	@Tags		health
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	HealthStatusResponse
//	@Router		/healthz [get]
func (h *HealthHandler) Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthStatusResponse{
		Status: "ok",
	})
}

// Readyz handles readiness probe - checks if server is ready to accept requests
//
//	@Summary	Readiness probe
//	@Description	Check if the server is ready by verifying database and Redis connections
//	@Tags		health
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	ReadyzSuccessResponse
//	@Failure	503	{object}	response.Envelope	"Service unavailable"
//	@Router		/readyz [get]
func (h *HealthHandler) Readyz(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	status := make(map[string]string)
	allHealthy := true

	// Check database
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			status["database"] = "error: " + err.Error()
			allHealthy = false
		} else if err := sqlDB.PingContext(ctx); err != nil {
			status["database"] = "error: " + err.Error()
			allHealthy = false
		} else {
			status["database"] = "ok"
		}
	} else {
		status["database"] = "not configured"
	}

	// Check Redis
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			status["redis"] = "error: " + err.Error()
			allHealthy = false
		} else {
			status["redis"] = "ok"
		}
	} else {
		status["redis"] = "not configured"
	}

	if allHealthy {
		return c.JSON(http.StatusOK, ReadyzSuccessResponse{
			Status:   "ready",
			Checks:   status,
		})
	}

	status["status"] = "not ready"
	requestID := middleware.GetRequestID(c)

	return c.JSON(http.StatusServiceUnavailable, response.Envelope{
		Error: &response.ErrorDetail{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "Service is not ready",
			Details: status,
		},
		Meta: &response.Meta{
			RequestID: requestID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

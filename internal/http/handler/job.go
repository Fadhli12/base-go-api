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
	"log/slog"
)

// JobHandler handles job-related endpoints
type JobHandler struct {
	svc      *service.JobService
	enforcer *permission.Enforcer
	logger   *slog.Logger
}

// NewJobHandler creates a new JobHandler instance
func NewJobHandler(svc *service.JobService, enforcer *permission.Enforcer, log *slog.Logger) *JobHandler {
	return &JobHandler{
		svc:      svc,
		enforcer: enforcer,
		logger:   log,
	}
}

// Submit handles POST /api/v1/jobs
// Submits a new background job for processing
//
//	@Summary	Submit job
//	@Description	Submit a new background job for asynchronous processing
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.SubmitJobRequest	true	"Job submission details"
//	@Success	201	{object}	response.Envelope{data=domain.JobResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	422	{object}	response.Envelope	"Validation error"
//	@Router		/api/v1/jobs [post]
func (h *JobHandler) Submit(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "submit job failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req request.SubmitJobRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed",
			log.String("type", req.Type),
			logger.Err(err),
		)
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "submitting job",
		log.String("user_id", userID.String()),
		log.String("type", req.Type),
	)

	submitReq := &service.SubmitRequest{
		Type:       req.Type,
		Payload:    mustMarshalJSON(req.Payload),
		MaxRetries: req.MaxRetries,
		WebhookURL: req.WebhookURL,
	}

	job, err := h.svc.Submit(ctx, userID, submitReq)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "job submission failed",
				log.String("user_id", userID.String()),
				log.String("type", req.Type),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "job submission failed",
			log.String("user_id", userID.String()),
			log.String("type", req.Type),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to submit job"))
	}

	log.Info(ctx, "job submitted successfully",
		log.String("job_id", job.ID.String()),
		log.String("user_id", userID.String()),
		log.String("type", req.Type),
	)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, job.ToResponse()))
}

// List handles GET /api/v1/jobs
// Returns a list of jobs for the authenticated user
//
//	@Summary	List jobs
//	@Description	Retrieve a list of jobs for the authenticated user
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		status	query	string	false	"Filter by job status (pending, processing, completed, failed, dead)"
//	@Param		limit	query	int		false	"Maximum number of results (default 20, max 100)"
//	@Param		offset	query	int		false	"Number of results to skip (default 0)"
//	@Success	200	{object}	response.Envelope{data=[]domain.JobResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Router		/api/v1/jobs [get]
func (h *JobHandler) List(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "list jobs failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse status filter
	var status domain.JobStatus
	statusStr := c.QueryParam("status")
	if statusStr != "" {
		status = domain.JobStatus(statusStr)
	}

	// Parse pagination
	limit := 20
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	offset := 0
	if offsetStr := c.QueryParam("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	log.Info(ctx, "listing jobs",
		log.String("user_id", userID.String()),
		log.String("status", string(status)),
		log.Int("limit", limit),
		log.Int("offset", offset),
	)

	jobs, total, err := h.svc.ListByUser(ctx, userID, status, limit, offset)
	if err != nil {
		log.Error(ctx, "failed to list jobs",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list jobs"))
	}

	resp := make([]*domain.JobResponse, len(jobs))
	for i, job := range jobs {
		resp[i] = job.ToResponse()
	}

	log.Info(ctx, "jobs listed successfully",
		log.String("user_id", userID.String()),
		log.Int("count", len(resp)),
		log.Int("total", total),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetByID handles GET /api/v1/jobs/:id
// Returns a job by ID
//
//	@Summary	Get job by ID
//	@Description	Retrieve a job by its ID
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Job ID"
//	@Success	200	{object}	response.Envelope{data=domain.JobResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid job ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Job not found"
//	@Router		/api/v1/jobs/{id} [get]
func (h *JobHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "get job failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid job ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid job ID"))
	}

	log.Info(ctx, "fetching job by ID",
		log.String("user_id", userID.String()),
		log.String("job_id", jobID.String()),
	)

	job, err := h.svc.GetByID(ctx, jobID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to fetch job",
				log.String("job_id", jobID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			log.Warn(ctx, "job not found", log.String("job_id", jobID.String()))
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Job not found"))
		}
		log.Error(ctx, "failed to fetch job",
			log.String("job_id", jobID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch job"))
	}

	// Check ownership
	if job.UserID != userID {
		log.Warn(ctx, "job ownership check failed",
			log.String("job_id", jobID.String()),
			log.String("owner_id", job.UserID.String()),
			log.String("requester_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Access denied"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, job.ToResponse()))
}

// Cancel handles DELETE /api/v1/jobs/:id
// Cancels a pending or failed job
//
//	@Summary	Cancel job
//	@Description	Cancel a pending or failed job
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Job ID"
//	@Success	200	{object}	response.Envelope{data=domain.JobResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid job ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Job not found"
//	@Failure	409	{object}	response.Envelope	"Cannot cancel job in current state"
//	@Router		/api/v1/jobs/{id} [delete]
func (h *JobHandler) Cancel(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "cancel job failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid job ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid job ID"))
	}

	log.Info(ctx, "cancelling job",
		log.String("user_id", userID.String()),
		log.String("job_id", jobID.String()),
	)

	if err := h.svc.Cancel(ctx, jobID, userID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "job cancellation failed",
				log.String("job_id", jobID.String()),
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			log.Warn(ctx, "job not found", log.String("job_id", jobID.String()))
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Job not found"))
		}
		if err == apperrors.ErrForbidden {
			log.Warn(ctx, "unauthorized cancellation attempt",
				log.String("job_id", jobID.String()),
				log.String("user_id", userID.String()),
			)
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Access denied"))
		}
		log.Error(ctx, "job cancellation failed",
			log.String("job_id", jobID.String()),
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to cancel job"))
	}

	// Fetch updated job for response
	job, err := h.svc.GetByID(ctx, jobID)
	if err != nil {
		log.Error(ctx, "failed to fetch job after cancellation",
			log.String("job_id", jobID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch cancelled job"))
	}

	log.Info(ctx, "job cancelled successfully",
		log.String("job_id", jobID.String()),
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, job.ToResponse()))
}

// ResubmitJob handles POST /api/v1/jobs/:id/resubmit
// Resubmits a dead job for processing
//
//	@Summary	Resubmit job
//	@Description	Resubmit a dead job for reprocessing
//	@Tags		jobs
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Job ID"
//	@Success	200	{object}	response.Envelope{data=domain.JobResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid job ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Job not found"
//	@Failure	409	{object}	response.Envelope	"Only dead jobs can be resubmitted"
//	@Router		/api/v1/jobs/{id}/resubmit [post]
func (h *JobHandler) ResubmitJob(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "resubmit job failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	idStr := c.Param("id")
	jobID, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid job ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid job ID"))
	}

	log.Info(ctx, "resubmitting job",
		log.String("user_id", userID.String()),
		log.String("job_id", jobID.String()),
	)

	job, err := h.svc.Resubmit(ctx, jobID, userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "job resubmit failed",
				log.String("job_id", jobID.String()),
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			log.Warn(ctx, "job not found", log.String("job_id", jobID.String()))
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Job not found"))
		}
		if err == apperrors.ErrForbidden {
			log.Warn(ctx, "unauthorized resubmit attempt",
				log.String("job_id", jobID.String()),
				log.String("user_id", userID.String()),
			)
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Access denied"))
		}
		if err == apperrors.ErrConflict {
			log.Warn(ctx, "job not in dead state", log.String("job_id", jobID.String()))
			return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "CONFLICT", "Only dead jobs can be resubmitted"))
		}
		log.Error(ctx, "job resubmit failed",
			log.String("job_id", jobID.String()),
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to resubmit job"))
	}

	log.Info(ctx, "job resubmitted successfully",
		log.String("job_id", jobID.String()),
		log.String("user_id", userID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, job.ToResponse()))
}

// mustMarshalJSON marshals a map to JSON bytes, panics on error
func mustMarshalJSON(v map[string]interface{}) []byte {
	if v == nil {
		return []byte("{}")
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic("failed to marshal JSON: " + err.Error())
	}
	return data
}
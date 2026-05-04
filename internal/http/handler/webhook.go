package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// WebhookHandler handles webhook CRUD endpoints.
type WebhookHandler struct {
	webhookService *service.WebhookService
	enforcer       *permission.Enforcer
	logger         logger.Logger
}

// NewWebhookHandler creates a new WebhookHandler instance.
func NewWebhookHandler(webhookService *service.WebhookService, enforcer *permission.Enforcer, logger logger.Logger) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		enforcer:       enforcer,
		logger:         logger,
	}
}

// isOwner verifies that the given organization context is authorized to access the webhook.
// Global webhooks are accessible from any org context.
func isOwner(webhook *domain.Webhook, orgID *uuid.UUID, hasOrgID bool) bool {
	if webhook.IsGlobal() {
		return true
	}
	if !hasOrgID || orgID == nil {
		return false
	}
	return webhook.OrganizationID != nil && *webhook.OrganizationID == *orgID
}

// resolveOrgDomain converts the optional organization ID to a Casbin domain string.
func resolveOrgDomain(hasOrgID bool, orgID uuid.UUID) string {
	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}
	return orgDomain
}

// Create handles webhook creation.
//
//	@Summary		Create a new webhook
//	@Description		Create a new webhook endpoint with events and optional organization scoping
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body	request.CreateWebhookRequest	true	"Webhook details"
//	@Success		201	{object}	response.Envelope{data=domain.WebhookCreateResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid request"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks [post]
func (h *WebhookHandler) Create(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "create webhook failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "manage")
	if err != nil || !allowed {
		log.Warn(ctx, "create webhook failed - permission denied",
			log.String("user_id", userID.String()),
			log.String("org_domain", orgDomain),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	var req request.CreateWebhookRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "creating webhook",
		log.String("name", req.Name),
		log.String("url", req.URL),
		log.String("user_id", userID.String()),
		log.String("org_domain", orgDomain),
	)

	webhook, err := h.webhookService.Create(ctx, orgIDPtr, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "webhook creation failed",
				log.String("name", req.Name),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "webhook creation failed",
			log.String("name", req.Name),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create webhook"))
	}

	log.Info(ctx, "webhook created successfully",
		log.String("webhook_id", webhook.ID.String()),
		log.String("name", webhook.Name),
	)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, webhook.ToCreateResponse()))
}

// GetByID retrieves a webhook by ID.
//
//	@Summary		Get webhook by ID
//	@Description		Retrieve a specific webhook by its ID. Does not include the secret.
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Webhook ID"
//	@Success		200	{object}	response.Envelope{data=domain.WebhookResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid webhook ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Webhook not found"
//	@Router			/api/v1/webhooks/{id} [get]
func (h *WebhookHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "get webhook failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "view")
	if err != nil || !allowed {
		log.Warn(ctx, "get webhook failed - permission denied",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	log.Info(ctx, "fetching webhook",
		log.String("webhook_id", id.String()),
		log.String("user_id", userID.String()),
	)

	webhook, err := h.webhookService.GetByID(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook",
				log.String("webhook_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook",
			log.String("webhook_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "get webhook failed - ownership mismatch",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, webhook.ToResponse()))
}

// List retrieves all webhooks for the current organization context.
//
//	@Summary		List webhooks
//	@Description		Retrieve webhooks scoped to the current organization context with pagination
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query	int	false	"Limit"	default(20)
//	@Param			offset	query	int	false	"Offset"	default(0)
//	@Success		200	{object}	response.Envelope{data=map[string]interface{}}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks [get]
func (h *WebhookHandler) List(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "list webhooks failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "view")
	if err != nil || !allowed {
		log.Warn(ctx, "list webhooks failed - permission denied",
			log.String("user_id", userID.String()),
			log.String("org_domain", orgDomain),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	pagination := ParsePagination(c)

	log.Info(ctx, "listing webhooks",
		log.String("user_id", userID.String()),
		log.String("org_domain", orgDomain),
		log.Int("limit", pagination.Limit),
		log.Int("offset", pagination.Offset),
	)

	webhooks, total, err := h.webhookService.ListByOrg(ctx, orgIDPtr, pagination.Limit, pagination.Offset)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhooks",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhooks",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhooks"))
	}

	resp := make([]domain.WebhookResponse, len(webhooks))
	for i, webhook := range webhooks {
		resp[i] = webhook.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"webhooks": resp,
		"total":    total,
		"limit":    pagination.Limit,
		"offset":   pagination.Offset,
	}))
}

// Update handles webhook updates.
//
//	@Summary		Update a webhook
//	@Description		Update a webhook's name, URL, events, active state, or rate limit
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Webhook ID"
//	@Param			request	body	request.UpdateWebhookRequest	true	"Webhook update details"
//	@Success		200	{object}	response.Envelope{data=domain.WebhookResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid request"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Webhook not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks/{id} [put]
func (h *WebhookHandler) Update(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "update webhook failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "manage")
	if err != nil || !allowed {
		log.Warn(ctx, "update webhook failed - permission denied",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	var req request.UpdateWebhookRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "updating webhook",
		log.String("webhook_id", id.String()),
		log.String("user_id", userID.String()),
	)

	webhook, err := h.webhookService.GetByID(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook for update",
				log.String("webhook_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook for update",
			log.String("webhook_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "update webhook failed - ownership mismatch",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	webhook, err = h.webhookService.Update(ctx, id, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "webhook update failed",
				log.String("webhook_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "webhook update failed",
			log.String("webhook_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update webhook"))
	}

	log.Info(ctx, "webhook updated successfully",
		log.String("webhook_id", webhook.ID.String()),
		log.String("name", webhook.Name),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, webhook.ToResponse()))
}

// SoftDelete handles webhook soft deletion.
//
//	@Summary		Delete a webhook
//	@Description		Soft delete a webhook (requires manage permission)
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Webhook ID"
//	@Success		204	"Webhook deleted successfully"
//	@Failure		400	{object}	response.Envelope	"Invalid webhook ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Webhook not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks/{id} [delete]
func (h *WebhookHandler) SoftDelete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "delete webhook failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "manage")
	if err != nil || !allowed {
		log.Warn(ctx, "delete webhook failed - permission denied",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	log.Info(ctx, "deleting webhook",
		log.String("webhook_id", id.String()),
		log.String("user_id", userID.String()),
	)

	webhook, err := h.webhookService.GetByID(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook for deletion",
				log.String("webhook_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook for deletion",
			log.String("webhook_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "delete webhook failed - ownership mismatch",
			log.String("webhook_id", id.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	if err := h.webhookService.SoftDelete(ctx, id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "webhook deletion failed",
				log.String("webhook_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "webhook deletion failed",
			log.String("webhook_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete webhook"))
	}

	log.Info(ctx, "webhook deleted successfully",
		log.String("webhook_id", id.String()),
	)

	return c.NoContent(http.StatusNoContent)
}

// ListDeliveries retrieves deliveries for a webhook with optional filters.
//
//	@Summary		List webhook deliveries
//	@Description		Retrieve delivery records for a specific webhook with optional status and event filters
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string	true	"Webhook ID"
//	@Param			limit	query	int	false	"Limit"	default(20)
//	@Param			offset	query	int	false	"Offset"	default(0)
//	@Param			status	query	string	false	"Filter by status (queued, processing, delivered, failed, rate_limited)"
//	@Param			event	query	string	false	"Filter by event type"
//	@Success		200	{object}	response.Envelope{data=map[string]interface{}}
//	@Failure		400	{object}	response.Envelope	"Invalid webhook ID or query params"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Webhook not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks/{id}/deliveries [get]
func (h *WebhookHandler) ListDeliveries(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	webhookIDStr := c.Param("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", webhookIDStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "list deliveries failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "view")
	if err != nil || !allowed {
		log.Warn(ctx, "list deliveries failed - permission denied",
			log.String("webhook_id", webhookID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	// Verify webhook exists and caller has ownership
	webhook, err := h.webhookService.GetByID(ctx, webhookID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook for deliveries",
				log.String("webhook_id", webhookID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook for deliveries",
			log.String("webhook_id", webhookID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "list deliveries failed - ownership mismatch",
			log.String("webhook_id", webhookID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	// Parse and validate query params
	var query request.ListDeliveriesQuery
	if err := c.Bind(&query); err != nil {
		log.Warn(ctx, "failed to bind delivery query params", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	if err := query.Validate(); err != nil {
		log.Warn(ctx, "delivery query validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Build repository query
	repoQuery := repository.ListDeliveriesQuery{
		Limit:  query.GetLimit(),
		Offset: query.GetOffset(),
	}
	if query.Status != nil {
		status := domain.WebhookDeliveryStatus(*query.Status)
		repoQuery.Status = &status
	}
	if query.Event != nil {
		repoQuery.Event = *query.Event
	}

	log.Info(ctx, "listing webhook deliveries",
		log.String("webhook_id", webhookID.String()),
		log.String("user_id", userID.String()),
		log.Int("limit", repoQuery.Limit),
		log.Int("offset", repoQuery.Offset),
	)

	deliveries, total, err := h.webhookService.ListDeliveries(ctx, webhookID, repoQuery)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to list deliveries",
				log.String("webhook_id", webhookID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to list deliveries",
			log.String("webhook_id", webhookID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list deliveries"))
	}

	resp := make([]domain.WebhookDeliveryResponse, len(deliveries))
	for i, d := range deliveries {
		resp[i] = d.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"deliveries": resp,
		"total":      total,
		"limit":      repoQuery.Limit,
		"offset":     repoQuery.Offset,
	}))
}

// GetDelivery retrieves a single delivery by ID.
//
//	@Summary		Get webhook delivery
//	@Description		Retrieve a specific webhook delivery by its ID
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string	true	"Webhook ID"
//	@Param			delivery_id	path	string	true	"Delivery ID"
//	@Success		200	{object}	response.Envelope{data=domain.WebhookDeliveryResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Delivery not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks/{id}/deliveries/{delivery_id} [get]
func (h *WebhookHandler) GetDelivery(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	webhookIDStr := c.Param("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", webhookIDStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	deliveryIDStr := c.Param("delivery_id")
	deliveryID, err := uuid.Parse(deliveryIDStr)
	if err != nil {
		log.Warn(ctx, "invalid delivery ID", log.String("delivery_id", deliveryIDStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid delivery ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "get delivery failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "view")
	if err != nil || !allowed {
		log.Warn(ctx, "get delivery failed - permission denied",
			log.String("delivery_id", deliveryID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	// Verify webhook ownership
	webhook, err := h.webhookService.GetByID(ctx, webhookID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook for delivery",
				log.String("webhook_id", webhookID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook for delivery",
			log.String("webhook_id", webhookID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "get delivery failed - ownership mismatch",
			log.String("webhook_id", webhookID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	log.Info(ctx, "fetching webhook delivery",
		log.String("delivery_id", deliveryID.String()),
		log.String("webhook_id", webhookID.String()),
	)

	delivery, err := h.webhookService.GetDelivery(ctx, deliveryID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve delivery",
				log.String("delivery_id", deliveryID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve delivery",
			log.String("delivery_id", deliveryID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve delivery"))
	}

	// Verify delivery belongs to the webhook
	if delivery.WebhookID != webhookID {
		log.Warn(ctx, "delivery does not belong to webhook",
			log.String("delivery_id", deliveryID.String()),
			log.String("delivery_webhook_id", delivery.WebhookID.String()),
			log.String("expected_webhook_id", webhookID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Delivery does not belong to this webhook"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, delivery.ToResponse()))
}

// ReplayDelivery replays a previously completed delivery.
//
//	@Summary		Replay webhook delivery
//	@Description		Replay a completed or failed delivery by resetting it to queued status
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string	true	"Webhook ID"
//	@Param			delivery_id	path	string	true	"Delivery ID"
//	@Success		200	{object}	response.Envelope{data=domain.WebhookDeliveryResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Delivery not found"
//	@Failure		409	{object}	response.Envelope	"Delivery cannot be replayed"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/webhooks/{id}/deliveries/{delivery_id}/replay [post]
func (h *WebhookHandler) ReplayDelivery(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	webhookIDStr := c.Param("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		log.Warn(ctx, "invalid webhook ID", log.String("id", webhookIDStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook ID"))
	}

	deliveryIDStr := c.Param("delivery_id")
	deliveryID, err := uuid.Parse(deliveryIDStr)
	if err != nil {
		log.Warn(ctx, "invalid delivery ID", log.String("delivery_id", deliveryIDStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid delivery ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "replay delivery failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := resolveOrgDomain(hasOrgID, orgID)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "webhooks", "manage")
	if err != nil || !allowed {
		log.Warn(ctx, "replay delivery failed - permission denied",
			log.String("delivery_id", deliveryID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Insufficient permissions"))
	}

	// Verify webhook ownership
	webhook, err := h.webhookService.GetByID(ctx, webhookID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve webhook for replay",
				log.String("webhook_id", webhookID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve webhook for replay",
			log.String("webhook_id", webhookID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve webhook"))
	}

	if !isOwner(webhook, orgIDPtr, hasOrgID) {
		log.Warn(ctx, "replay delivery failed - ownership mismatch",
			log.String("webhook_id", webhookID.String()),
			log.String("user_id", userID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Not authorized to access this webhook"))
	}

	log.Info(ctx, "replaying webhook delivery",
		log.String("delivery_id", deliveryID.String()),
		log.String("webhook_id", webhookID.String()),
		log.String("user_id", userID.String()),
	)

	delivery, err := h.webhookService.ReplayDelivery(ctx, deliveryID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "replay delivery failed",
				log.String("delivery_id", deliveryID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "replay delivery failed",
			log.String("delivery_id", deliveryID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to replay delivery"))
	}

	// Verify delivery belongs to the webhook
	if delivery.WebhookID != webhookID {
		log.Warn(ctx, "replayed delivery does not belong to webhook",
			log.String("delivery_id", deliveryID.String()),
			log.String("delivery_webhook_id", delivery.WebhookID.String()),
			log.String("expected_webhook_id", webhookID.String()),
		)
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Delivery does not belong to this webhook"))
	}

	log.Info(ctx, "delivery replayed successfully",
		log.String("delivery_id", deliveryID.String()),
		log.String("webhook_id", webhookID.String()),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, delivery.ToResponse()))
}

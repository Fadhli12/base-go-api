package handler

import (
	"log/slog"
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// EmailHandler handles email sending endpoints
type EmailHandler struct {
	emailService *service.EmailService
	queueRepo    repository.EmailQueueRepository
}

// NewEmailHandler creates a new EmailHandler instance
func NewEmailHandler(emailService *service.EmailService, queueRepo repository.EmailQueueRepository) *EmailHandler {
	return &EmailHandler{
		emailService: emailService,
		queueRepo:    queueRepo,
	}
}

// EmailQueueResponse represents an email queue status response
type EmailQueueResponse struct {
	ID           string `json:"id"`
	ToAddress    string `json:"to_address"`
	Subject      string `json:"subject"`
	Template     string `json:"template,omitempty"`
	Status       string `json:"status"`
	Provider     string `json:"provider,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	Attempts     int    `json:"attempts"`
	LastError    string `json:"last_error,omitempty"`
	SentAt       string `json:"sent_at,omitempty"`
	DeliveredAt  string `json:"delivered_at,omitempty"`
	BouncedAt    string `json:"bounced_at,omitempty"`
	BounceReason string `json:"bounce_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// mapEmailQueueToResponse converts domain.EmailQueue to EmailQueueResponse
func mapEmailQueueToResponse(e domain.EmailQueue) EmailQueueResponse {
	resp := EmailQueueResponse{
		ID:        e.ID.String(),
		ToAddress: e.ToAddress,
		Subject:   e.Subject,
		Template:  e.Template,
		Status:     string(e.Status),
		Provider:   e.Provider,
		MessageID:  e.MessageID,
		Attempts:   e.Attempts,
		LastError:  e.LastError,
		CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if e.SentAt != nil {
		resp.SentAt = e.SentAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if e.DeliveredAt != nil {
		resp.DeliveredAt = e.DeliveredAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if e.BouncedAt != nil {
		resp.BouncedAt = e.BouncedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return resp
}

// SendEmail queues an email for delivery
//
//	@Summary		Send an email
//	@Description	Queue an email for async delivery
//	@Tags			email
//	@Accept			json
//	@Produce		json
//	@Param			request	body		request.SendEmailRequest	true	"Email details"
//	@Success		202		{object}	response.Envelope{data=map[string]string}	"Email queued"
//	@Failure		400		{object}	response.Envelope	"Invalid request"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/emails [post]
func (h *EmailHandler) SendEmail(c echo.Context) error {
	var req request.SendEmailRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	emailReq := &service.EmailRequest{
		To:          req.To,
		Subject:     req.Subject,
		Template:    req.Template,
		Data:        req.Data,
		HTMLContent: req.HTMLContent,
		TextContent: req.TextContent,
	}

	if err := h.emailService.QueueEmail(c.Request().Context(), emailReq); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to queue email"))
	}

	return c.JSON(http.StatusAccepted, response.SuccessWithContext(c, map[string]string{
		"message": "Email queued for delivery",
	}))
}

// GetStatus retrieves email queue status by ID
//
//	@Summary		Get email status
//	@Description	Get the status of a queued email
//	@Tags			email
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Email ID"
//	@Success		200	{object}	response.Envelope{data=handler.EmailQueueResponse}
//	@Failure		404	{object}	response.Envelope	"Email not found"
//	@Router			/api/v1/emails/{id} [get]
func (h *EmailHandler) GetStatus(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid email ID"))
	}

	email, err := h.queueRepo.FindByID(c.Request().Context(), id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Email not found"))
	}

	resp := mapEmailQueueToResponse(*email)
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// WebhookDelivery handles delivery webhook from email provider
//
//	@Summary		Handle delivery webhook
//	@Description	Process delivery confirmation from email provider
//	@Tags			email-webhooks
//	@Accept			json
//	@Produce		json
//	@Param			provider	path		string	true	"Provider name (smtp/sendgrid/ses)"
//	@Param			payload		body		object	true	"Webhook payload"
//	@Success		200			{object}	response.Envelope	"Webhook processed"
//	@Router			/api/v1/webhooks/{provider}/delivery [post]
func (h *EmailHandler) WebhookDelivery(c echo.Context) error {
	provider := c.Param("provider")

	// Parse webhook payload based on provider
	// This is a simplified implementation - real implementations would parse provider-specific formats
	var payload struct {
		MessageID string `json:"message_id"`
		Timestamp int64  `json:"timestamp"`
		Event     string `json:"event"`
	}

	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook payload"))
	}

	// Mark email as delivered
	if err := h.emailService.RecordDelivery(c.Request().Context(), payload.MessageID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to process delivery webhook"))
	}

	slog.Info("Delivery webhook processed",
		"provider", provider,
		"message_id", payload.MessageID,
		"event", payload.Event,
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"status": "processed",
	}))
}

// WebhookBounce handles bounce webhook from email provider
//
//	@Summary		Handle bounce webhook
//	@Description	Process bounce notification from email provider
//	@Tags			email-webhooks
//	@Accept			json
//	@Produce		json
//	@Param			provider	path		string	true	"Provider name (smtp/sendgrid/ses)"
//	@Param			payload		body		object	true	"Webhook payload"
//	@Success		200			{object}	response.Envelope	"Webhook processed"
//	@Router			/api/v1/webhooks/{provider}/bounce [post]
func (h *EmailHandler) WebhookBounce(c echo.Context) error {
	provider := c.Param("provider")

	// Parse webhook payload based on provider
	var payload struct {
		Email    string `json:"email"`
		MessageID string `json:"message_id"`
		Type     string `json:"type"` // hard, soft, spam
		Reason   string `json:"reason"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid webhook payload"))
	}

	// Map bounce type
	bounceType := domain.BounceTypeSoft
	switch payload.Type {
	case "hard", "permanent":
		bounceType = domain.BounceTypeHard
	case "spam", "complaint":
		bounceType = domain.BounceTypeSpam
	}

	bounceInfo := &service.EmailBounceInfo{
		Email:        payload.Email,
		MessageID:    payload.MessageID,
		BounceType:   bounceType,
		BounceReason: payload.Reason,
		Timestamp:    payload.Timestamp,
	}

	if err := h.emailService.RecordBounce(c.Request().Context(), bounceInfo); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to process bounce webhook"))
	}

	slog.Info("Bounce webhook processed",
		"provider", provider,
		"email", payload.Email,
		"type", payload.Type,
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{
		"status": "processed",
	}))
}
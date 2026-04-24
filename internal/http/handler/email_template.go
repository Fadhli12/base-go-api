package handler

import (
	"fmt"
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

// EmailTemplateHandler handles email template endpoints
type EmailTemplateHandler struct {
	templateService *service.EmailService
	templateRepo    repository.EmailTemplateRepository
}

// NewEmailTemplateHandler creates a new EmailTemplateHandler instance
func NewEmailTemplateHandler(templateService *service.EmailService, templateRepo repository.EmailTemplateRepository) *EmailTemplateHandler {
	return &EmailTemplateHandler{
		templateService: templateService,
		templateRepo:    templateRepo,
	}
}

// TemplateResponse represents an email template in responses
type TemplateResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	HTMLContent string `json:"html_content"`
	TextContent string `json:"text_content,omitempty"`
	Category    string `json:"category"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TemplateListResponse represents a template list response
type TemplateListResponse struct {
	Data  []TemplateResponse `json:"data"`
	Total int64              `json:"total"`
}

// mapTemplateToResponse converts a domain.EmailTemplate to TemplateResponse
func mapTemplateToResponse(t domain.EmailTemplate) TemplateResponse {
	return TemplateResponse{
		ID:          t.ID.String(),
		Name:        t.Name,
		Subject:     t.Subject,
		HTMLContent: t.HTMLContent,
		TextContent: t.TextContent,
		Category:    t.Category,
		IsActive:    t.IsActive,
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Create handles template creation
//
//	@Summary		Create a new email template
//	@Description	Create a new email template with name, subject, content, and category
//	@Tags			email-templates
//	@Accept			json
//	@Produce		json
//	@Param			request	body		request.CreateTemplateRequest	true	"Template details"
//	@Success		201		{object}	response.Envelope{data=handler.TemplateResponse}
//	@Failure		400		{object}	response.Envelope	"Invalid request"
//	@Failure		409		{object}	response.Envelope	"Template already exists"
//	@Router			/api/v1/email-templates [post]
func (h *EmailTemplateHandler) Create(c echo.Context) error {
	var req request.CreateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	template := &domain.EmailTemplate{
		Name:        req.Name,
		Subject:     req.Subject,
		HTMLContent: req.HTMLContent,
		TextContent: req.TextContent,
		Category:    req.Category,
		IsActive:    true,
	}

	if err := h.templateRepo.Create(c.Request().Context(), template); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create template"))
	}

	resp := mapTemplateToResponse(*template)
	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// GetAll retrieves all email templates
//
//	@Summary		Get all email templates
//	@Description	Retrieve a paginated list of email templates
//	@Tags			email-templates
//	@Accept			json
//	@Produce		json
//	@Param			limit	query		int	false	"Page size"	default(20)
//	@Param			offset	query		int	false	"Page offset"	default(0)
//	@Success		200		{object}	response.Envelope{data=handler.TemplateListResponse}
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/email-templates [get]
func (h *EmailTemplateHandler) GetAll(c echo.Context) error {
	limit := 20
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		var parsed int
		if _, err := fmt.Sscanf(l, "%d", &parsed); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.QueryParam("offset"); o != "" {
		var parsed int
		if _, err := fmt.Sscanf(o, "%d", &parsed); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	templates, total, err := h.templateRepo.FindAll(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve templates"))
	}

	resp := make([]TemplateResponse, len(templates))
	for i, t := range templates {
		resp[i] = mapTemplateToResponse(*t)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, TemplateListResponse{
		Data:  resp,
		Total: total,
	}))
}

// GetByID retrieves a single email template by ID
//
//	@Summary		Get email template by ID
//	@Description	Retrieve a single email template by its ID
//	@Tags			email-templates
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Template ID"
//	@Success		200	{object}	response.Envelope{data=handler.TemplateResponse}
//	@Failure		404	{object}	response.Envelope	"Template not found"
//	@Router			/api/v1/email-templates/{id} [get]
func (h *EmailTemplateHandler) GetByID(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid template ID"))
	}

	template, err := h.templateRepo.FindByID(c.Request().Context(), id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Template not found"))
	}

	resp := mapTemplateToResponse(*template)
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// Update updates an email template
//
//	@Summary		Update an email template
//	@Description	Update an existing email template
//	@Tags			email-templates
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Template ID"
//	@Param			request	body		request.UpdateTemplateRequest	true	"Template updates"
//	@Success		200		{object}	response.Envelope{data=handler.TemplateResponse}
//	@Failure		400		{object}	response.Envelope	"Invalid request"
//	@Failure		404		{object}	response.Envelope	"Template not found"
//	@Router			/api/v1/email-templates/{id} [put]
func (h *EmailTemplateHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid template ID"))
	}

	var req request.UpdateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	template, err := h.templateRepo.FindByID(c.Request().Context(), id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Template not found"))
	}

	// Update fields if provided
	if req.Name != "" {
		template.Name = req.Name
	}
	if req.Subject != "" {
		template.Subject = req.Subject
	}
	if req.HTMLContent != "" {
		template.HTMLContent = req.HTMLContent
	}
	if req.TextContent != "" {
		template.TextContent = req.TextContent
	}
	if req.Category != "" {
		template.Category = req.Category
	}
	if req.IsActive != nil {
		template.IsActive = *req.IsActive
	}

	if err := h.templateRepo.Update(c.Request().Context(), template); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update template"))
	}

	resp := mapTemplateToResponse(*template)
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// Delete soft-deletes an email template
//
//	@Summary		Delete an email template
//	@Description	Soft-delete an email template
//	@Tags			email-templates
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Template ID"
//	@Success		204	"Template deleted"
//	@Failure		404	{object}	response.Envelope	"Template not found"
//	@Router			/api/v1/email-templates/{id} [delete]
func (h *EmailTemplateHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid template ID"))
	}

	if err := h.templateRepo.SoftDelete(c.Request().Context(), id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Template not found"))
	}

	return c.NoContent(http.StatusNoContent)
}
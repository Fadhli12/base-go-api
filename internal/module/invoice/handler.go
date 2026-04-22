package invoice

import (
	"fmt"
	"net/http"
	"time"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles invoice-related HTTP endpoints
type Handler struct {
	service  *Service
	enforcer *permission.Enforcer
}

// NewHandler creates a new Handler instance
func NewHandler(service *Service, enforcer *permission.Enforcer) *Handler {
	return &Handler{
		service:  service,
		enforcer: enforcer,
	}
}

// InvoiceListResponse represents the response for list endpoints
type InvoiceListResponse struct {
	Data       []InvoiceResponse `json:"data"`
	Total      int64             `json:"total"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
}

// Create handles POST /api/v1/invoices
// Creates a new invoice for the authenticated user
//
//	@Summary	Create invoice
//	@Description	Create a new invoice
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.CreateInvoiceRequest	true	"Invoice data"
//	@Success	201	{object}	response.Envelope{data=invoice.InvoiceResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/invoices [post]
func (h *Handler) Create(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	var req request.CreateInvoiceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Generate invoice number (simple sequential for now)
	invoiceNumber := generateInvoiceNumber()

	invoice, err := h.service.Create(c.Request().Context(), userID, invoiceNumber, req.Customer, req.Amount, req.Description, nil)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create invoice"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, invoice.ToResponse()))
}

// GetByID handles GET /api/v1/invoices/:id
// Returns an invoice by ID
//
//	@Summary	Get invoice by ID
//	@Description	Get an invoice by ID. Regular users can only see their own invoices.
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Invoice ID"
//	@Success	200	{object}	response.Envelope{data=invoice.InvoiceResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid invoice ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Invoice not found"
//	@Router		/api/v1/invoices/{id} [get]
func (h *Handler) GetByID(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse invoice ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid invoice ID"))
	}

	// Check if user is admin (has invoice:view:all scope or similar)
	isAdmin := h.isAdmin(c)

	invoice, err := h.service.GetByID(c.Request().Context(), userID, id, isAdmin)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Invoice not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch invoice"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, invoice.ToResponse()))
}

// List handles GET /api/v1/invoices
// Lists invoices - regular users see their own, admins see all
//
//	@Summary	List invoices
//	@Description	List invoices. Regular users see their own invoices, admins see all.
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		limit	query	int	false	"Limit"	default(20)
//	@Param		offset	query	int	false	"Offset"	default(0)
//	@Success	200	{object}	response.Envelope{data=InvoiceListResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/invoices [get]
func (h *Handler) List(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse query parameters
	var query request.InvoiceListQuery
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	limit := query.GetLimit()
	offset := query.GetOffset()

	// Check if user is admin (can see all invoices)
	isAdmin := h.isAdmin(c)

	var invoices []Invoice
	var total int64

	if isAdmin {
		invoices, total, err = h.service.ListAll(c.Request().Context(), limit, offset)
	} else {
		invoices, total, err = h.service.ListByUser(c.Request().Context(), userID, limit, offset)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list invoices"))
	}

	// Convert to response
	responses := make([]InvoiceResponse, len(invoices))
	for i, inv := range invoices {
		responses[i] = inv.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, InvoiceListResponse{
		Data:   responses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}))
}

// Update handles PUT /api/v1/invoices/:id
// Updates an invoice with ownership check
//
//	@Summary	Update invoice
//	@Description	Update an invoice. Users can only update their own invoices.
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string				true	"Invoice ID"
//	@Param		request	body	request.UpdateInvoiceRequest	true	"Invoice data"
//	@Success	200	{object}	response.Envelope{data=invoice.InvoiceResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Invoice not found"
//	@Router		/api/v1/invoices/{id} [put]
func (h *Handler) Update(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse invoice ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid invoice ID"))
	}

	var req request.UpdateInvoiceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	invoice, err := h.service.Update(c.Request().Context(), userID, id, req.Customer, req.Amount, req.Description, nil, InvoiceStatus(req.Status), isAdmin)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Invoice not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update invoice"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, invoice.ToResponse()))
}

// Delete handles DELETE /api/v1/invoices/:id
// Soft-deletes an invoice with ownership check
//
//	@Summary	Delete invoice
//	@Description	Soft delete an invoice. Users can only delete their own invoices.
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Invoice ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid invoice ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Invoice not found"
//	@Router		/api/v1/invoices/{id} [delete]
func (h *Handler) Delete(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse invoice ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid invoice ID"))
	}

	// Check if user is admin
	isAdmin := h.isAdmin(c)

	if err := h.service.Delete(c.Request().Context(), userID, id, isAdmin); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Invoice not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete invoice"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Invoice deleted successfully"}))
}

// UpdateStatus handles PATCH /api/v1/invoices/:id/status
// Updates invoice status (admin only)
//
//	@Summary	Update invoice status
//	@Description	Update the status of an invoice (admin only)
//	@Tags		invoices
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Invoice ID"
//	@Param		status	query	string	true	"Status (draft, pending, paid, cancelled)"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Invoice not found"
//	@Router		/api/v1/invoices/{id}/status [patch]
func (h *Handler) UpdateStatus(c echo.Context) error {
	// Check if user is admin
	if !h.isAdmin(c) {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Admin access required"))
	}

	// Parse invoice ID
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid invoice ID"))
	}

	statusStr := c.QueryParam("status")
	if statusStr == "" {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Status is required"))
	}

	status := InvoiceStatus(statusStr)
	if !IsValidStatus(status) {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid status. Must be one of: draft, pending, paid, cancelled"))
	}

	if err := h.service.UpdateStatus(c.Request().Context(), id, status); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Invoice not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update invoice status"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Invoice status updated successfully"}))
}

// isAdmin checks if the current user has admin privileges for invoices
func (h *Handler) isAdmin(c echo.Context) bool {
	if h.enforcer == nil {
		return false
	}

	claims, err := middleware.GetUserClaims(c)
	if err != nil {
		return false
	}

	// Check if user has permission to manage all invoices (not just own)
	// This would typically be a permission like "invoices:manage" with scope "all"
	// For now, we check if user has "invoices" "manage" permission
	allowed, err := h.enforcer.Enforce(claims.UserID, "default", "invoices", "manage")
	if err != nil {
		return false
	}

	return allowed
}

// generateInvoiceNumber generates a unique invoice number
func generateInvoiceNumber() string {
	// Simple sequential number with timestamp
	// In production, this would use a proper sequence or UUID-based generation
	return fmt.Sprintf("INV-%d", time.Now().UnixNano()/1000000)
}
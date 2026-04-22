package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/labstack/echo/v4"
)

// PermissionHandler handles permission endpoints
type PermissionHandler struct {
	permissionService *service.PermissionService
}

// NewPermissionHandler creates a new PermissionHandler instance
func NewPermissionHandler(permissionService *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
	}
}

// PermissionResponse represents a permission in responses
type PermissionResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Scope    string `json:"scope"`
}

// Create handles permission creation
//
//	@Summary	Create a new permission
//	@Description	Create a new permission with name, resource, action, and optional scope
//	@Tags		permissions
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.CreatePermissionRequest	true	"Permission details"
//	@Success	201	{object}	response.Envelope{data=handler.PermissionResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	409	{object}	response.Envelope	"Permission already exists"
//	@Router		/api/v1/permissions [post]
func (h *PermissionHandler) Create(c echo.Context) error {
	var req request.CreatePermissionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	permission, err := h.permissionService.Create(c.Request().Context(), req.Name, req.Resource, req.Action, req.Scope)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create permission"))
	}

	resp := PermissionResponse{
		ID:       permission.ID.String(),
		Name:     permission.Name,
		Resource: permission.Resource,
		Action:   permission.Action,
		Scope:    permission.Scope,
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// GetAll retrieves all permissions
//
//	@Summary	Get all permissions
//	@Description	Retrieve a list of all permissions
//	@Tags		permissions
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=[]handler.PermissionResponse}
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/permissions [get]
func (h *PermissionHandler) GetAll(c echo.Context) error {
	permissions, err := h.permissionService.GetAll(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve permissions"))
	}

	resp := make([]PermissionResponse, len(permissions))
	for i, p := range permissions {
		resp[i] = PermissionResponse{
			ID:       p.ID.String(),
			Name:     p.Name,
			Resource: p.Resource,
			Action:   p.Action,
			Scope:    p.Scope,
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}
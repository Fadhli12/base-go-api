package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// RoleHandler handles role endpoints
type RoleHandler struct {
	roleService *service.RoleService
}

// NewRoleHandler creates a new RoleHandler instance
func NewRoleHandler(roleService *service.RoleService) *RoleHandler {
	return &RoleHandler{
		roleService: roleService,
	}
}

// RoleResponse represents a role in responses
type RoleResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	IsSystem    bool                   `json:"is_system"`
	Permissions []PermissionResponse   `json:"permissions,omitempty"`
}

// RoleListResponse represents a role with permissions for list responses
type RoleListResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	IsSystem    bool                 `json:"is_system"`
	Permissions []PermissionResponse `json:"permissions"`
}

// mapPermissionToResponse converts a domain.Permission to PermissionResponse
func mapPermissionToResponse(p domain.Permission) PermissionResponse {
	return PermissionResponse{
		ID:       p.ID.String(),
		Name:     p.Name,
		Resource: p.Resource,
		Action:   p.Action,
		Scope:    p.Scope,
	}
}

// mapRoleToResponse converts a domain.Role to RoleResponse
func mapRoleToResponse(r domain.Role) RoleResponse {
	perms := make([]PermissionResponse, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = mapPermissionToResponse(p)
	}
	return RoleResponse{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		IsSystem:    r.IsSystem,
		Permissions: perms,
	}
}

// Create handles role creation
//
//	@Summary	Create a new role
//	@Description	Create a new role with name and optional description
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		request	body	request.CreateRoleRequest	true	"Role details"
//	@Success	201	{object}	response.Envelope{data=handler.RoleResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	409	{object}	response.Envelope	"Role already exists"
//	@Router		/api/v1/roles [post]
func (h *RoleHandler) Create(c echo.Context) error {
	var req request.CreateRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	role, err := h.roleService.Create(c.Request().Context(), req.Name, req.Description)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create role"))
	}

	resp := mapRoleToResponse(*role)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// GetAll retrieves all roles with their permissions
//
//	@Summary	Get all roles
//	@Description	Retrieve a list of all roles with their associated permissions
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	response.Envelope{data=[]handler.RoleListResponse}
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/roles [get]
func (h *RoleHandler) GetAll(c echo.Context) error {
	roles, err := h.roleService.GetAll(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve roles"))
	}

	resp := make([]RoleListResponse, len(roles))
	for i, r := range roles {
		perms := make([]PermissionResponse, len(r.Permissions))
		for j, p := range r.Permissions {
			perms[j] = mapPermissionToResponse(p)
		}
		resp[i] = RoleListResponse{
			ID:          r.ID.String(),
			Name:        r.Name,
			Description: r.Description,
			IsSystem:    r.IsSystem,
			Permissions: perms,
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// Update handles role updates
//
//	@Summary	Update a role
//	@Description	Update a role's name and description
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		id	path	string	true	"Role ID"
//	@Param		request	body	request.UpdateRoleRequest	true	"Role update details"
//	@Success	200	{object}	response.Envelope{data=handler.RoleResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	404	{object}	response.Envelope	"Role not found"
//	@Router		/api/v1/roles/{id} [put]
func (h *RoleHandler) Update(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	var req request.UpdateRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	role, err := h.roleService.Update(c.Request().Context(), id, req.Name, req.Description)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Role not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update role"))
	}

	resp := mapRoleToResponse(*role)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// SoftDelete handles role soft deletion
//
//	@Summary	Delete a role
//	@Description	Soft delete a role (cannot delete system roles)
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		id	path	string	true	"Role ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid role ID"
//	@Failure	403	{object}	response.Envelope	"Cannot delete system role"
//	@Failure	404	{object}	response.Envelope	"Role not found"
//	@Router		/api/v1/roles/{id} [delete]
func (h *RoleHandler) SoftDelete(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	// Get role to check if it's a system role
	role, err := h.roleService.GetByID(c.Request().Context(), id)
	if err != nil {
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Role not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve role"))
	}

	if role.IsSystem {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Cannot delete system role"))
	}

	if err := h.roleService.Delete(c.Request().Context(), id); err != nil {
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Role not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete role"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Role deleted successfully"}))
}

// AttachPermission attaches a permission to a role
//
//	@Summary	Attach permission to role
//	@Description	Attach a permission to a role
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		id	path	string	true	"Role ID"
//	@Param		pid	path	string	true	"Permission ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	404	{object}	response.Envelope	"Role or permission not found"
//	@Router		/api/v1/roles/{id}/permissions/{pid} [post]
func (h *RoleHandler) AttachPermission(c echo.Context) error {
	idStr := c.Param("id")
	roleID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	pidStr := c.Param("pid")
	permissionID, err := uuid.Parse(pidStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid permission ID"))
	}

	if err := h.roleService.AttachPermission(c.Request().Context(), roleID, permissionID); err != nil {
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Role or permission not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to attach permission"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Permission attached successfully"}))
}

// DetachPermission detaches a permission from a role
//
//	@Summary	Detach permission from role
//	@Description	Detach a permission from a role
//	@Tags		roles
//	@Accept		json
//	@Produce	json
//	@Param		id	path	string	true	"Role ID"
//	@Param		pid	path	string	true	"Permission ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/roles/{id}/permissions/{pid} [delete]
func (h *RoleHandler) DetachPermission(c echo.Context) error {
	idStr := c.Param("id")
	roleID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	pidStr := c.Param("pid")
	permissionID, err := uuid.Parse(pidStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid permission ID"))
	}

	if err := h.roleService.DetachPermission(c.Request().Context(), roleID, permissionID); err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to detach permission"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Permission detached successfully"}))
}
package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UserHandler handles user-related endpoints
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// MeResponse represents the response for the /me endpoint
type MeResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserListResponse represents a user in list responses
type UserListResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserDetailResponse represents a detailed user response
type UserDetailResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserRoleResponse represents a user role in responses
type UserRoleResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	IsSystem    bool                 `json:"is_system"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
}

// EffectivePermissionResponse represents an effective permission in responses
type EffectivePermissionResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Scope    string `json:"scope"`
}

// GetCurrentUser handles GET /api/v1/me
// Returns the current authenticated user's information
//
//	@Summary	Get current user
//	@Description	Get the currently authenticated user's profile information
//	@Tags		user
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Envelope{data=handler.MeResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User not found"
//	@Router		/api/v1/me [get]
func (h *UserHandler) GetCurrentUser(c echo.Context) error {
	// Extract user ID from context using helper
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Fetch user from database
	user, err := h.userService.FindByID(c.Request().Context(), userID)
	if err != nil {
		if apperrors.IsAppError(err) && apperrors.GetAppError(err).Code == "NOT_FOUND" {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "USER_NOT_FOUND", "User not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch user"))
	}

	// Convert to response format
	resp := userToResponse(user)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// ListUsers handles GET /api/v1/users
// Returns a list of all users
//
//	@Summary	List all users
//	@Description	Retrieve a list of all users
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Envelope{data=[]handler.UserListResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/users [get]
func (h *UserHandler) ListUsers(c echo.Context) error {
	users, err := h.userService.FindAll(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve users"))
	}

	resp := make([]UserListResponse, len(users))
	for i, u := range users {
		resp[i] = UserListResponse{
			ID:        u.ID.String(),
			Email:     u.Email,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetUserByID handles GET /api/v1/users/:id
// Returns a user by ID
//
//	@Summary	Get user by ID
//	@Description	Retrieve a user by their ID
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=handler.UserDetailResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid user ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User not found"
//	@Router		/api/v1/users/{id} [get]
func (h *UserHandler) GetUserByID(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	user, err := h.userService.FindByID(c.Request().Context(), id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "User not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch user"))
	}

	resp := UserDetailResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// SoftDelete handles DELETE /api/v1/users/:id
// Soft deletes a user
//
//	@Summary	Delete a user
//	@Description	Soft delete a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid user ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User not found"
//	@Router		/api/v1/users/{id} [delete]
func (h *UserHandler) SoftDelete(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	if err := h.userService.SoftDelete(c.Request().Context(), id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "User not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete user"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "User deleted successfully"}))
}

// AssignRole handles POST /api/v1/users/:id/roles
// Assigns a role to a user
//
//	@Summary	Assign role to user
//	@Description	Assign a role to a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string						true	"User ID"
//	@Param		request	body	request.CreateUserRoleRequest	true	"Role assignment"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User or role not found"
//	@Router		/api/v1/users/{id}/roles [post]
func (h *UserHandler) AssignRole(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	var req request.CreateUserRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	roleID, err := uuid.Parse(req.RoleID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	// Get the current user ID for tracking who assigned the role
	assignedBy, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	if err := h.userService.AssignRole(c.Request().Context(), userID, roleID, assignedBy); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to assign role"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Role assigned successfully"}))
}

// RemoveRole handles DELETE /api/v1/users/:id/roles/:roleId
// Removes a role from a user
//
//	@Summary	Remove role from user
//	@Description	Remove a role from a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"User ID"
//	@Param		roleId	path	string	true	"Role ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User or role not found"
//	@Router		/api/v1/users/{id}/roles/{roleId} [delete]
func (h *UserHandler) RemoveRole(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	roleIDStr := c.Param("roleId")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid role ID"))
	}

	if err := h.userService.RemoveRole(c.Request().Context(), userID, roleID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		if err == apperrors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Role assignment not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to remove role"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Role removed successfully"}))
}

// GrantPermission handles POST /api/v1/users/:id/permissions
// Grants a permission to a user (allow effect)
//
//	@Summary	Grant permission to user
//	@Description	Grant or deny a permission directly to a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string							true	"User ID"
//	@Param		request	body	request.CreateUserPermissionRequest	true	"Permission assignment"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User or permission not found"
//	@Router		/api/v1/users/{id}/permissions [post]
func (h *UserHandler) GrantPermission(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	var req request.CreateUserPermissionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	permissionID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid permission ID"))
	}

	// Get the current user ID for tracking who granted the permission
	assignedBy, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	if req.Effect == "deny" {
		if err := h.userService.DenyPermission(c.Request().Context(), userID, permissionID, assignedBy); err != nil {
			if appErr := apperrors.GetAppError(err); appErr != nil {
				return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
			}
			return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to deny permission"))
		}
		return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Permission denied successfully"}))
	}

	if err := h.userService.GrantPermission(c.Request().Context(), userID, permissionID, assignedBy); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to grant permission"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Permission granted successfully"}))
}

// RemovePermission handles DELETE /api/v1/users/:id/permissions/:permId
// Removes a direct permission assignment from a user
//
//	@Summary	Remove permission from user
//	@Description	Remove a direct permission assignment from a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"User ID"
//	@Param		permId	path	string	true	"Permission ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"Permission assignment not found"
//	@Router		/api/v1/users/{id}/permissions/{permId} [delete]
func (h *UserHandler) RemovePermission(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	permIDStr := c.Param("permId")
	permissionID, err := uuid.Parse(permIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid permission ID"))
	}

	if err := h.userService.RemovePermission(c.Request().Context(), userID, permissionID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to remove permission"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Permission removed successfully"}))
}

// GetEffectivePermissions handles GET /api/v1/users/:id/effective-permissions
// Returns the effective permissions for a user after applying role and direct permissions with deny overrides
//
//	@Summary	Get effective permissions
//	@Description	Get the effective permissions for a user after applying role permissions and deny overrides
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=[]handler.EffectivePermissionResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid user ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User not found"
//	@Router		/api/v1/users/{id}/effective-permissions [get]
func (h *UserHandler) GetEffectivePermissions(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	permissions, err := h.userService.GetEffectivePermissions(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get effective permissions"))
	}

	resp := make([]EffectivePermissionResponse, len(permissions))
	for i, p := range permissions {
		resp[i] = EffectivePermissionResponse{
			ID:       p.ID.String(),
			Name:     p.Name,
			Resource: p.Resource,
			Action:   p.Action,
			Scope:    p.Scope,
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetUserRoles handles GET /api/v1/users/:id/roles
// Returns all roles assigned to a user
//
//	@Summary	Get user roles
//	@Description	Get all roles assigned to a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=[]handler.UserRoleResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid user ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	404	{object}	response.Envelope	"User not found"
//	@Router		/api/v1/users/{id}/roles [get]
func (h *UserHandler) GetUserRoles(c echo.Context) error {
	idStr := c.Param("id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	roles, err := h.userService.GetUserRoles(c.Request().Context(), userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get user roles"))
	}

	resp := make([]UserRoleResponse, len(roles))
	for i, r := range roles {
		resp[i] = UserRoleResponse{
			ID:          r.ID.String(),
			Name:        r.Name,
			Description: r.Description,
			IsSystem:    r.IsSystem,
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// userToResponse converts a domain.User to MeResponse
func userToResponse(user *domain.User) MeResponse {
	return MeResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
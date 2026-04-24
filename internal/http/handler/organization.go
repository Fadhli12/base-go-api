package handler

import (
	"fmt"
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

// OrganizationHandler handles organization endpoints
type OrganizationHandler struct {
	orgService *service.OrganizationService
}

// NewOrganizationHandler creates a new OrganizationHandler instance
func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
	}
}

// Create handles organization creation
//
//	@Summary	Create a new organization
//	@Description	Create a new organization with name and slug
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body	request.CreateOrganizationRequest	true	"Organization details"
//	@Success	201	{object}	response.Envelope{data=domain.OrganizationResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	409	{object}	response.Envelope	"Organization slug already exists"
//	@Router		/api/v1/organizations [post]
func (h *OrganizationHandler) Create(c echo.Context) error {
	var req request.CreateOrganizationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	// Get client IP and user agent for audit
	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	org, err := h.orgService.CreateOrganization(c.Request().Context(), userID, req.Name, req.Slug, req.Settings, ipAddress, userAgent)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create organization"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, org.ToResponse()))
}

// GetByID retrieves an organization by ID
//
//	@Summary	Get organization by ID
//	@Description	Retrieve a specific organization by its ID
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Organization ID"
//	@Success	200	{object}	response.Envelope{data=domain.OrganizationResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid organization ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Organization not found"
//	@Router		/api/v1/organizations/{id} [get]
func (h *OrganizationHandler) GetByID(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	org, err := h.orgService.GetOrganization(c.Request().Context(), userID, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve organization"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, org.ToResponse()))
}

// List retrieves all organizations for the current user
//
//	@Summary	List organizations
//	@Description	Retrieve all organizations where the user is a member
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		limit	query	int	false	"Limit"	default(20)
//	@Param		offset	query	int	false	"Offset"	default(0)
//	@Success	200	{object}	response.Envelope{data=[]domain.OrganizationResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/organizations [get]
func (h *OrganizationHandler) List(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	limit := 20
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := parseIntParam(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	orgs, total, err := h.orgService.ListOrganizations(c.Request().Context(), userID, limit, offset)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve organizations"))
	}

	resp := make([]*domain.OrganizationResponse, len(orgs))
	for i, org := range orgs {
		resp[i] = org.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"organizations": resp,
		"total":        total,
		"limit":         limit,
		"offset":        offset,
	}))
}

// Update handles organization updates
//
//	@Summary	Update an organization
//	@Description	Update an organization's name, slug, or settings
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Organization ID"
//	@Param		request	body	request.UpdateOrganizationRequest	true	"Organization update details"
//	@Success	200	{object}	response.Envelope{data=domain.OrganizationResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Organization not found"
//	@Router		/api/v1/organizations/{id} [put]
func (h *OrganizationHandler) Update(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	var req request.UpdateOrganizationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	org, err := h.orgService.UpdateOrganization(c.Request().Context(), userID, id, req.Name, req.Slug, req.Settings, ipAddress, userAgent)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update organization"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, org.ToResponse()))
}

// Delete handles organization soft deletion
//
//	@Summary	Delete an organization
//	@Description	Soft delete an organization (requires manage permission)
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Organization ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid organization ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Organization not found"
//	@Router		/api/v1/organizations/{id} [delete]
func (h *OrganizationHandler) Delete(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	if err := h.orgService.DeleteOrganization(c.Request().Context(), userID, id, ipAddress, userAgent); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete organization"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Organization deleted successfully"}))
}

// AddMember adds a member to an organization
//
//	@Summary	Add member to organization
//	@Description	Add a user to an organization with a specific role
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Organization ID"
//	@Param		request	body	request.AddMemberRequest	true	"Member details"
//	@Success	201	{object}	response.Envelope{data=domain.OrganizationMemberResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	409	{object}	response.Envelope	"User is already a member"
//	@Router		/api/v1/organizations/{id}/members [post]
func (h *OrganizationHandler) AddMember(c echo.Context) error {
	idStr := c.Param("id")
	orgID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	var req request.AddMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	member, err := h.orgService.AddMember(c.Request().Context(), userID, orgID, req.UserID, req.Role, ipAddress, userAgent)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to add member"))
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, member.ToResponse()))
}

// GetMembers retrieves all members of an organization
//
//	@Summary	Get organization members
//	@Description	Retrieve all members of an organization with pagination
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Organization ID"
//	@Param		limit	query	int	false	"Limit"	default(20)
//	@Param		offset	query	int	false	"Offset"	default(0)
//	@Success	200	{object}	response.Envelope{data=map[string]interface{}}
//	@Failure	400	{object}	response.Envelope	"Invalid organization ID"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Router		/api/v1/organizations/{id}/members [get]
func (h *OrganizationHandler) GetMembers(c echo.Context) error {
	idStr := c.Param("id")
	orgID, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	limit := 20
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := parseIntParam(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	members, total, err := h.orgService.GetMembers(c.Request().Context(), userID, orgID, limit, offset)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve members"))
	}

	resp := make([]*domain.OrganizationMemberResponse, len(members))
	for i, member := range members {
		resp[i] = member.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"members": resp,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}))
}

// RemoveMember removes a member from an organization
//
//	@Summary	Remove member from organization
//	@Description	Remove a user from an organization
//	@Tags		organizations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Organization ID"
//	@Param		user_id	path	string	true	"User ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	400	{object}	response.Envelope	"Invalid IDs"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Access denied"
//	@Failure	404	{object}	response.Envelope	"Member not found"
//	@Router		/api/v1/organizations/{id}/members/{user_id} [delete]
func (h *OrganizationHandler) RemoveMember(c echo.Context) error {
	orgIDStr := c.Param("id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid organization ID"))
	}

	memberIDStr := c.Param("user_id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid user ID"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	if err := h.orgService.RemoveMember(c.Request().Context(), userID, orgID, memberID, ipAddress, userAgent); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to remove member"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Member removed successfully"}))
}

// parseIntParam parses an integer query parameter
func parseIntParam(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
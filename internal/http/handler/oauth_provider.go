package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// OAuthProviderHandler handles OAuth provider admin CRUD endpoints
// and the public provider listing endpoint.
// Permission enforcement is done via middleware.RequirePermission on route groups,
// following the project's two-tier permission model.
type OAuthProviderHandler struct {
	providerService *service.OAuthProviderService
	enforcer        *permission.Enforcer
	logger          logger.Logger
}

// NewOAuthProviderHandler creates a new OAuthProviderHandler instance.
func NewOAuthProviderHandler(
	providerService *service.OAuthProviderService,
	enforcer *permission.Enforcer,
	logger logger.Logger,
) *OAuthProviderHandler {
	return &OAuthProviderHandler{
		providerService: providerService,
		enforcer:        enforcer,
		logger:          logger,
	}
}

// RegisterRoutes registers all OAuth provider routes on the Echo group.
// Public route (providers listing) has no auth.
// Admin routes require JWT + oauth:manage permission via middleware.
func (h *OAuthProviderHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
	// Public route (no auth required)
	auth := v1.Group("/auth/oauth")
	auth.GET("/providers", h.ListPublicProviders)

	// Admin routes (JWT + oauth:manage permission)
	providers := v1.Group("/oauth-providers")
	providers.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	providers.Use(middleware.ExtractOrganizationID())
	providers.Use(middleware.RequirePermission(h.enforcer, "oauth", "manage"))

	providers.POST("", h.Create)
	providers.GET("/:id", h.GetByID)
	providers.GET("", h.List)
	providers.PUT("/:id", h.Update)
	providers.DELETE("/:id", h.Delete)
}

// ListPublicProviders lists all enabled OAuth providers for unauthenticated users.
// Returns only name, display_name, and merged scopes.
//
//	@Summary		List enabled OAuth providers
//	@Description		Retrieve all enabled OAuth providers for public consumption (name, display_name, scopes only)
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]domain.PublicOAuthProviderResponse}
//	@Router			/api/v1/auth/oauth/providers [get]
func (h *OAuthProviderHandler) ListPublicProviders(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	log.Info(ctx, "listing public OAuth providers")

	providers, err := h.providerService.FindEnabled(ctx, orgIDPtr)
	if err != nil {
		log.Error(ctx, "failed to list public OAuth providers", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list OAuth providers"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"providers": providers,
	}))
}

// Create handles OAuth provider creation.
// Permission is enforced by middleware.RequirePermission on the route group.
//
//	@Summary		Create an OAuth provider
//	@Description		Create a new OAuth provider configuration (requires oauth:manage permission)
//	@Tags			oauth-providers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body	request.CreateOAuthProviderRequest	true	"Provider details"
//	@Success		201	{object}	response.Envelope{data=domain.OAuthProviderResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid request"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/oauth-providers [post]
func (h *OAuthProviderHandler) Create(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "create OAuth provider failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	var req request.CreateOAuthProviderRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// If organization_id is provided in the request body, use it
	if req.OrganizationID != nil {
		orgIDPtr = req.OrganizationID
	}

	log.Info(ctx, "creating OAuth provider",
		log.String("name", req.Name),
		log.String("user_id", userID.String()),
	)

	result, err := h.providerService.Create(ctx, orgIDPtr, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth provider creation failed",
				log.String("name", req.Name),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth provider creation failed",
			log.String("name", req.Name),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create OAuth provider"))
	}

	log.Info(ctx, "OAuth provider created successfully",
		log.String("provider_id", result.ID.String()),
		log.String("name", result.Name),
	)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, result))
}

// List retrieves all OAuth providers for the current organization context (admin).
// Permission is enforced by middleware.RequirePermission on the route group.
//
//	@Summary		List OAuth providers
//	@Description		Retrieve OAuth providers scoped to the current organization context with pagination
//	@Tags			oauth-providers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query	int	false	"Limit"	default(20)
//	@Param			offset	query	int	false	"Offset"	default(0)
//	@Success		200	{object}	response.Envelope{data=map[string]interface{}}
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/oauth-providers [get]
func (h *OAuthProviderHandler) List(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var orgIDPtr *uuid.UUID
	if hasOrgID && orgID != uuid.Nil {
		orgIDPtr = &orgID
	}

	pagination := ParsePagination(c)

	log.Info(ctx, "listing OAuth providers",
		log.Int("limit", pagination.Limit),
		log.Int("offset", pagination.Offset),
	)

	providers, total, err := h.providerService.FindAll(ctx, orgIDPtr, pagination.Limit, pagination.Offset)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve OAuth providers",
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve OAuth providers", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve OAuth providers"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"providers": providers,
		"total":    total,
		"limit":    pagination.Limit,
		"offset":   pagination.Offset,
	}))
}

// GetByID retrieves a single OAuth provider by ID (admin).
// Permission is enforced by middleware.RequirePermission on the route group.
//
//	@Summary		Get OAuth provider by ID
//	@Description		Retrieve a specific OAuth provider configuration by its ID
//	@Tags			oauth-providers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Provider ID"
//	@Success		200	{object}	response.Envelope{data=domain.OAuthProviderResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid provider ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Provider not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/oauth-providers/{id} [get]
func (h *OAuthProviderHandler) GetByID(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid OAuth provider ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid provider ID"))
	}

	log.Info(ctx, "fetching OAuth provider",
		log.String("provider_id", id.String()),
	)

	result, err := h.providerService.FindByID(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to retrieve OAuth provider",
				log.String("provider_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to retrieve OAuth provider",
			log.String("provider_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve OAuth provider"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// Update handles OAuth provider updates.
// Permission is enforced by middleware.RequirePermission on the route group.
//
//	@Summary		Update an OAuth provider
//	@Description		Update an OAuth provider configuration (requires oauth:manage permission)
//	@Tags			oauth-providers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Provider ID"
//	@Param			request	body	request.UpdateOAuthProviderRequest	true	"Provider update details"
//	@Success		200	{object}	response.Envelope{data=domain.OAuthProviderResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid request"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied"
//	@Failure		404	{object}	response.Envelope	"Provider not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/oauth-providers/{id} [put]
func (h *OAuthProviderHandler) Update(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid OAuth provider ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid provider ID"))
	}

	var req request.UpdateOAuthProviderRequest
	if err := c.Bind(&req); err != nil {
		log.Error(ctx, "failed to bind request", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		log.Warn(ctx, "validation failed", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	log.Info(ctx, "updating OAuth provider",
		log.String("provider_id", id.String()),
	)

	result, err := h.providerService.Update(ctx, id, &req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth provider update failed",
				log.String("provider_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth provider update failed",
			log.String("provider_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update OAuth provider"))
	}

	log.Info(ctx, "OAuth provider updated successfully",
		log.String("provider_id", result.ID.String()),
		log.String("name", result.Name),
	)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// Delete handles OAuth provider soft deletion.
// Permission is enforced by middleware.RequirePermission on the route group.
//
//	@Summary		Delete an OAuth provider
//	@Description		Soft delete an OAuth provider configuration (requires oauth:manage permission)
//	@Tags			oauth-providers
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Provider ID"
//	@Success		204	"Provider deleted successfully"
//	@Failure		400	{object}	response.Envelope	"Invalid provider ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Access denied or system provider"
//	@Failure		404	{object}	response.Envelope	"Provider not found"
//	@Failure		500	{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/oauth-providers/{id} [delete]
func (h *OAuthProviderHandler) Delete(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Warn(ctx, "invalid OAuth provider ID", log.String("id", idStr))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid provider ID"))
	}

	log.Info(ctx, "deleting OAuth provider",
		log.String("provider_id", id.String()),
	)

	if err := h.providerService.Delete(ctx, id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth provider deletion failed",
				log.String("provider_id", id.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth provider deletion failed",
			log.String("provider_id", id.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete OAuth provider"))
	}

	log.Info(ctx, "OAuth provider deleted successfully",
		log.String("provider_id", id.String()),
	)

	return c.NoContent(http.StatusNoContent)
}
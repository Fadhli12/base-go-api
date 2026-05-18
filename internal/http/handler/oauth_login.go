package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// OAuthLoginHandler handles OAuth login flow endpoints:
// initiating login, processing callbacks, and initiating account linking.
type OAuthLoginHandler struct {
	loginService *service.OAuthLoginService
	enforcer     *permission.Enforcer
	logger       logger.Logger
	oauthConfig  config.OAuthConfig
}

// NewOAuthLoginHandler creates a new OAuthLoginHandler instance.
func NewOAuthLoginHandler(
	loginService *service.OAuthLoginService,
	enforcer *permission.Enforcer,
	logger logger.Logger,
	oauthConfig config.OAuthConfig,
) *OAuthLoginHandler {
	return &OAuthLoginHandler{
		loginService: loginService,
		enforcer:     enforcer,
		logger:       logger,
		oauthConfig:  oauthConfig,
	}
}

// RegisterRoutes registers all OAuth login routes on the Echo group.
func (h *OAuthLoginHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
	// Public routes (no auth required)
	auth := v1.Group("/auth/oauth")
	auth.GET("/:provider", h.InitiateLogin)
	auth.GET("/:provider/callback", h.HandleCallback)

	// Protected routes (JWT + oauth:link permission)
	link := v1.Group("/auth/oauth")
	link.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	link.Use(middleware.ExtractOrganizationID())
	link.POST("/:provider/link", h.InitiateLink, middleware.RequirePermission(h.enforcer, "oauth", "link"))
}

// InitiateLogin initiates the OAuth login flow for a given provider.
// Validates the provider name, builds the callback URL, and redirects
// the user to the provider's authorization URL.
//
//	@Summary		Initiate OAuth login
//	@Description		Redirect to the OAuth provider authorization URL to start login
//	@Tags			auth
//	@Param			provider	path	string	true	"OAuth provider name (google, github, microsoft)"
//	@Success		302		"Redirect to provider authorization URL"
//	@Failure		400		{object}	response.Envelope	"Invalid provider"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/auth/oauth/{provider} [get]
func (h *OAuthLoginHandler) InitiateLogin(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	providerName := c.Param("provider")
	if !domain.ValidOAuthProviders[providerName] {
		log.Warn(ctx, "invalid OAuth provider requested",
			log.String("provider", providerName),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid OAuth provider"))
	}

	// Build callback URL from config
	callbackURL := h.oauthConfig.FrontendCallbackURL + "/" + providerName

	log.Info(ctx, "initiating OAuth login",
		log.String("provider", providerName),
	)

	authURL, err := h.loginService.InitiateLogin(ctx, providerName, callbackURL, uuid.Nil, uuid.Nil)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth login initiation failed",
				log.String("provider", providerName),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth login initiation failed",
			log.String("provider", providerName),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to initiate OAuth login"))
	}

	return c.Redirect(http.StatusFound, authURL)
}

// HandleCallback processes the OAuth callback after the provider redirects the user back.
// Extracts the authorization code and state from query params, calls the service to
// handle the callback, and redirects to the frontend with tokens in the URL fragment.
//
//	@Summary		Handle OAuth callback
//	@Description		Process the OAuth provider callback and redirect to frontend with tokens
//	@Tags			auth
//	@Param			provider	path	string	true	"OAuth provider name"
//	@Param			code		query	string	true	"Authorization code from provider"
//	@Param			state		query	string	true	"State parameter for CSRF protection"
//	@Success		302		"Redirect to frontend with tokens in URL fragment"
//	@Failure		400		{object}	response.Envelope	"Invalid parameters"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/auth/oauth/{provider}/callback [get]
func (h *OAuthLoginHandler) HandleCallback(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	providerName := c.Param("provider")
	if !domain.ValidOAuthProviders[providerName] {
		log.Warn(ctx, "invalid OAuth provider in callback",
			log.String("provider", providerName),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid OAuth provider"))
	}

	code := c.QueryParam("code")
	state := c.QueryParam("state")

	if code == "" || state == "" {
		log.Warn(ctx, "missing code or state parameter in OAuth callback",
			log.String("provider", providerName),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Missing code or state parameter"))
	}

	log.Info(ctx, "processing OAuth callback",
		log.String("provider", providerName),
	)

	redirectURL, err := h.loginService.HandleCallback(ctx, providerName, code, state)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth callback failed",
				log.String("provider", providerName),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth callback failed",
			log.String("provider", providerName),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to process OAuth callback"))
	}

	return c.Redirect(http.StatusFound, redirectURL)
}

// InitiateLinkResponse represents the response for initiating an OAuth provider link.
type InitiateLinkResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// InitiateLink starts the OAuth flow for linking a provider to an authenticated user.
// Requires JWT authentication and the oauth:link permission.
// Returns a JSON response with the redirect URL (NOT a 302 redirect).
//
//	@Summary		Initiate OAuth provider linking
//	@Description		Start linking an OAuth provider to the authenticated user's account
//	@Tags			auth
//	@Security		BearerAuth
//	@Param			provider	path	string	true	"OAuth provider name (google, github, microsoft)"
//	@Success		200		{object}	response.Envelope{data=handler.InitiateLinkResponse}
//	@Failure		400		{object}	response.Envelope	"Invalid provider"
//	@Failure		401		{object}	response.Envelope	"Unauthorized"
//	@Failure		403		{object}	response.Envelope	"Insufficient permissions"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/auth/oauth/{provider}/link [post]
func (h *OAuthLoginHandler) InitiateLink(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	providerName := c.Param("provider")
	if !domain.ValidOAuthProviders[providerName] {
		log.Warn(ctx, "invalid OAuth provider for linking",
			log.String("provider", providerName),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid OAuth provider"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "OAuth link failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	if !hasOrgID {
		orgID = uuid.Nil
	}

	// Build callback URL from config
	callbackURL := h.oauthConfig.FrontendCallbackURL + "/" + providerName

	log.Info(ctx, "initiating OAuth provider link",
		log.String("provider", providerName),
		log.String("user_id", userID.String()),
	)

	authURL, err := h.loginService.InitiateLogin(ctx, providerName, callbackURL, userID, orgID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "OAuth link initiation failed",
				log.String("provider", providerName),
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "OAuth link initiation failed",
			log.String("provider", providerName),
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to initiate OAuth provider link"))
	}

	resp := InitiateLinkResponse{
		RedirectURL: authURL,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}
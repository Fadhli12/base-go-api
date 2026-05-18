package handler

import (
	"net/http"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// LinkedAccountResponse represents a linked OAuth account in API responses.
type LinkedAccountResponse struct {
	ID             string `json:"id"`
	ProviderID     string `json:"provider_id"`
	ProviderName   string `json:"provider_name"`
	ProviderUserID string `json:"provider_user_id"`
	Email          string `json:"email,omitempty"`
	EmailVerified  bool   `json:"email_verified"`
	DisplayName    string `json:"display_name,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	LinkedAt       string `json:"linked_at"`
}

// UnlinkResponse represents the response for unlinking an OAuth provider.
type UnlinkResponse struct {
	Message string `json:"message"`
}

// ListAccountsResponse represents the response for listing linked accounts.
type ListAccountsResponse struct {
	Accounts []LinkedAccountResponse `json:"accounts"`
}

// OAuthAccountHandler handles user-facing OAuth account management endpoints:
// unlinking providers and listing linked accounts.
type OAuthAccountHandler struct {
	providerService *service.OAuthProviderService
	accountRepo     repository.OAuthAccountRepository
	providerRepo    repository.OAuthProviderRepository
	userRepo        repository.UserRepository
	enforcer        *permission.Enforcer
	auditService    *service.AuditService
	eventBus        *domain.EventBus // Set via SetEventBus
	logger          logger.Logger
}

// NewOAuthAccountHandler creates a new OAuthAccountHandler instance.
func NewOAuthAccountHandler(
	providerService *service.OAuthProviderService,
	accountRepo repository.OAuthAccountRepository,
	providerRepo repository.OAuthProviderRepository,
	userRepo repository.UserRepository,
	enforcer *permission.Enforcer,
	auditService *service.AuditService,
	logger logger.Logger,
) *OAuthAccountHandler {
	return &OAuthAccountHandler{
		providerService: providerService,
		accountRepo:     accountRepo,
		providerRepo:    providerRepo,
		userRepo:        userRepo,
		enforcer:        enforcer,
		auditService:    auditService,
		logger:          logger,
	}
}

// SetEventBus sets the EventBus for publishing OAuth events.
func (h *OAuthAccountHandler) SetEventBus(bus *domain.EventBus) {
	h.eventBus = bus
}

// RegisterRoutes registers all OAuth account management routes on the Echo group.
func (h *OAuthAccountHandler) RegisterRoutes(v1 *echo.Group, jwtSecret string) {
	// All routes require JWT + oauth:view or oauth:link permission
	oauth := v1.Group("/auth/oauth")
	oauth.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	oauth.Use(middleware.ExtractOrganizationID())

	// Unlink requires oauth:link permission
	oauth.DELETE("/:provider/unlink", h.Unlink, middleware.RequirePermission(h.enforcer, "oauth", "link"))

	// List linked accounts requires oauth:view permission
	oauth.GET("/accounts", h.ListLinkedAccounts, middleware.RequirePermission(h.enforcer, "oauth", "view"))
}

// Unlink removes an OAuth provider link from the authenticated user's account.
// Enforces the "at least one auth method" rule: if the user only has one remaining
// auth method (either a single linked provider with no password, or just a password),
// the unlink is rejected with a 409 Conflict.
//
//	@Summary		Unlink an OAuth provider
//	@Description		Remove a linked OAuth provider from the authenticated user's account
//	@Tags			auth
//	@Security		BearerAuth
//	@Param			provider	path	string	true	"OAuth provider name (google, github, microsoft)"
//	@Success		200		{object}	response.Envelope{data=handler.UnlinkResponse}
//	@Failure		400		{object}	response.Envelope	"Invalid provider"
//	@Failure		401		{object}	response.Envelope	"Unauthorized"
//	@Failure		403		{object}	response.Envelope	"Insufficient permissions"
//	@Failure		404		{object}	response.Envelope	"Provider or account not found"
//	@Failure		409		{object}	response.Envelope	"Cannot unlink last auth method"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/auth/oauth/{provider}/unlink [delete]
func (h *OAuthAccountHandler) Unlink(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	providerName := c.Param("provider")
	if !domain.ValidOAuthProviders[providerName] {
		log.Warn(ctx, "invalid OAuth provider for unlinking",
			log.String("provider", providerName),
		)
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid OAuth provider"))
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "OAuth unlink failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	log.Info(ctx, "unlinking OAuth provider",
		log.String("provider", providerName),
		log.String("user_id", userID.String()),
	)

	// 1. Find provider by name
	provider, err := h.providerRepo.FindByName(ctx, providerName)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to find OAuth provider",
				log.String("provider", providerName),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to find OAuth provider",
			log.String("provider", providerName),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to find OAuth provider"))
	}

	// 2. Find the user's OAuthAccount for this provider
	account, err := h.accountRepo.FindByUserAndProvider(ctx, userID, provider.ID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to find OAuth account for unlinking",
				log.String("provider", providerName),
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to find OAuth account for unlinking",
			log.String("provider", providerName),
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to find linked account"))
	}

	// 3. Count auth methods to enforce the "at least one auth method" rule
	authMethodCount, err := h.accountRepo.CountAuthMethodsByUserID(ctx, userID)
	if err != nil {
		log.Error(ctx, "failed to count auth methods",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to verify auth methods"))
	}

	// If only one auth method remains, reject the unlink
	if authMethodCount <= 1 {
		log.Warn(ctx, "cannot unlink last authentication method",
			log.String("user_id", userID.String()),
			log.String("provider", providerName),
			log.Int64("auth_method_count", authMethodCount),
		)
		return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "CONFLICT", "Cannot unlink last authentication method"))
	}

	// 4. Soft-delete the account
	if err := h.accountRepo.SoftDelete(ctx, account.ID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to unlink OAuth account",
				log.String("account_id", account.ID.String()),
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to unlink OAuth account",
			log.String("account_id", account.ID.String()),
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to unlink account"))
	}

	// 5. Publish auth.oauth.unlinked event via EventBus
	h.publishUnlinkedEvent(account, provider)

	// 6. Audit log the unlink action
	_ = h.auditService.LogAction(
		ctx,
		userID,
		"oauth_account.unlinked",
		"oauth_account",
		account.ID.String(),
		nil,
		nil,
		c.RealIP(),
		c.Request().UserAgent(),
	)

	log.Info(ctx, "OAuth provider unlinked successfully",
		log.String("provider", providerName),
		log.String("user_id", userID.String()),
		log.String("account_id", account.ID.String()),
	)

	resp := UnlinkResponse{
		Message: "Provider unlinked successfully",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// ListLinkedAccounts returns all OAuth accounts linked to the authenticated user.
//
//	@Summary		List linked OAuth accounts
//	@Description		Retrieve all OAuth provider accounts linked to the authenticated user
//	@Tags			auth
//	@Security		BearerAuth
//	@Success		200		{object}	response.Envelope{data=handler.ListAccountsResponse}
//	@Failure		401		{object}	response.Envelope	"Unauthorized"
//	@Failure		403		{object}	response.Envelope	"Insufficient permissions"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/auth/oauth/accounts [get]
func (h *OAuthAccountHandler) ListLinkedAccounts(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn(ctx, "list linked accounts failed - not authenticated")
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	log.Info(ctx, "listing linked OAuth accounts",
		log.String("user_id", userID.String()),
	)

	accounts, err := h.accountRepo.FindByUserID(ctx, userID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "failed to list linked OAuth accounts",
				log.String("user_id", userID.String()),
				log.String("error_code", appErr.Code),
				logger.Err(err),
			)
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "failed to list linked OAuth accounts",
			log.String("user_id", userID.String()),
			logger.Err(err),
		)
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list linked accounts"))
	}

	// Map accounts to response DTOs
	accountResponses := make([]LinkedAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		providerNameStr := ""
		if account.Provider != nil {
			providerNameStr = account.Provider.Name
		}
		accountResponses = append(accountResponses, LinkedAccountResponse{
			ID:             account.ID.String(),
			ProviderID:     account.ProviderID.String(),
			ProviderName:   providerNameStr,
			ProviderUserID: account.ProviderUserID,
			Email:          account.Email,
			EmailVerified:  account.EmailVerified,
			DisplayName:    account.DisplayName,
			AvatarURL:      account.AvatarURL,
			LinkedAt:       account.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	resp := ListAccountsResponse{
		Accounts: accountResponses,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// publishUnlinkedEvent publishes auth.oauth.unlinked via EventBus (non-blocking).
func (h *OAuthAccountHandler) publishUnlinkedEvent(account *domain.OAuthAccount, provider *domain.OAuthProvider) {
	if h.eventBus == nil {
		return
	}

	go func() {
		payload := map[string]interface{}{
			"account_id":        account.ID.String(),
			"user_id":          account.UserID.String(),
			"provider":         provider.Name,
			"provider_user_id": account.ProviderUserID,
			"email":            account.Email,
			"display_name":     account.DisplayName,
		}

		var orgID *uuid.UUID
		if provider.OrganizationID != nil {
			orgID = provider.OrganizationID
		}

		_ = h.eventBus.Publish(domain.WebhookEvent{
			Type:    domain.OAuthEventUnlinked,
			Payload: payload,
			OrgID:   orgID,
		})
	}()
}
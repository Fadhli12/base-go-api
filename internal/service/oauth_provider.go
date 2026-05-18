package service

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// OAuthProviderService handles admin CRUD for OAuth provider configurations
// with encryption and audit logging.
type OAuthProviderService struct {
	providerRepo repository.OAuthProviderRepository
	encryption   *OAuthEncryptionService
	auditService *AuditService
	enforcer     *permission.Enforcer
	config       config.OAuthConfig
	logger       logger.Logger
	eventBus     *domain.EventBus
}

// NewOAuthProviderService creates a new OAuthProviderService instance.
func NewOAuthProviderService(
	providerRepo repository.OAuthProviderRepository,
	encryption *OAuthEncryptionService,
	auditService *AuditService,
	enforcer *permission.Enforcer,
	cfg config.OAuthConfig,
	logger logger.Logger,
) *OAuthProviderService {
	return &OAuthProviderService{
		providerRepo: providerRepo,
		encryption:   encryption,
		auditService: auditService,
		enforcer:     enforcer,
		config:       cfg,
		logger:       logger,
	}
}

// SetEventBus sets the event bus for publishing events. Called after construction.
func (s *OAuthProviderService) SetEventBus(bus *domain.EventBus) {
	s.eventBus = bus
}

// Create creates a new OAuth provider configuration.
// Validates provider name, encrypts client_secret, stores, and returns response.
func (s *OAuthProviderService) Create(
	ctx context.Context,
	orgID *uuid.UUID,
	req *request.CreateOAuthProviderRequest,
) (*domain.OAuthProviderResponse, error) {
	// 1. Validate provider name
	if !domain.ValidOAuthProviders[req.Name] {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid OAuth provider name", 422)
	}

	// 2. Validate redirect URL
	if err := s.validateRedirectURL(req.RedirectURL); err != nil {
		return nil, err
	}

	// 3. Encrypt client_secret
	encryptedSecret, err := s.encryption.Encrypt(req.ClientSecret)
	if err != nil {
		s.logger.Error(ctx, "failed to encrypt client secret", logger.Err(err))
		return nil, apperrors.WrapInternal(err)
	}

	// 4. Marshal additional scopes to JSON
	var scopesJSON datatypes.JSON
	if len(req.AdditionalScopes) > 0 {
		scopesBytes, marshalErr := json.Marshal(req.AdditionalScopes)
		if marshalErr != nil {
			return nil, apperrors.WrapInternal(marshalErr)
		}
		scopesJSON = scopesBytes
	} else {
		scopesJSON = datatypes.JSON("[]")
	}

	// 5. Build entity
	provider := &domain.OAuthProvider{
		Name:                  req.Name,
		DisplayName:           req.DisplayName,
		ClientID:              req.ClientID,
		ClientSecretEncrypted: encryptedSecret,
		RedirectURL:           req.RedirectURL,
		AdditionalScopes:     scopesJSON,
		IsEnabled:            true,
		IsSystem:             false,
		OrganizationID:       orgID,
	}

	// Handle optional Config
	if req.Config != nil {
		provider.Config = datatypes.JSON(req.Config)
	} else {
		provider.Config = datatypes.JSON("{}")
	}

	// 6. Store via repository
	if err := s.providerRepo.Create(ctx, provider); err != nil {
		s.logger.Error(ctx, "failed to create OAuth provider",
			s.logger.String("name", req.Name),
			logger.Err(err),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "OAuth provider created",
		s.logger.String("provider_id", provider.ID.String()),
		s.logger.String("name", provider.Name),
	)

	// 7. Audit log
	if s.auditService != nil {
		actorID := getActorID(ctx)
		afterState := provider.ToResponse()
		// Never include client_secret in audit log
		_ = s.auditService.LogAction(ctx, actorID, "oauth_provider.created", "oauth_provider", provider.ID.String(), nil, afterState, "", "")
	}

	resp := provider.ToResponse()
	return &resp, nil
}

// FindByID returns a single OAuth provider by UUID.
func (s *OAuthProviderService) FindByID(ctx context.Context, id uuid.UUID) (*domain.OAuthProviderResponse, error) {
	provider, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := provider.ToResponse()
	return &resp, nil
}

// FindEnabled returns all enabled providers for public listing (no auth required).
func (s *OAuthProviderService) FindEnabled(ctx context.Context, orgID *uuid.UUID) ([]*domain.PublicOAuthProviderResponse, error) {
	providers, err := s.providerRepo.FindEnabled(ctx, orgID)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}

	result := make([]*domain.PublicOAuthProviderResponse, len(providers))
	for i, p := range providers {
		resp := p.ToPublicResponse()
		result[i] = &resp
	}
	return result, nil
}

// FindAll returns paginated OAuth providers for admin listing.
func (s *OAuthProviderService) FindAll(
	ctx context.Context,
	orgID *uuid.UUID,
	limit, offset int,
) ([]*domain.OAuthProviderResponse, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	providers, total, err := s.providerRepo.FindAll(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.WrapInternal(err)
	}

	result := make([]*domain.OAuthProviderResponse, len(providers))
	for i, p := range providers {
		resp := p.ToResponse()
		result[i] = &resp
	}
	return result, total, nil
}

// Update updates an OAuth provider with partial updates.
// Re-encrypts client_secret if provided.
func (s *OAuthProviderService) Update(
	ctx context.Context,
	id uuid.UUID,
	req *request.UpdateOAuthProviderRequest,
) (*domain.OAuthProviderResponse, error) {
	// 1. Find existing provider
	provider, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Capture before state for audit
	beforeState := provider.ToResponse()

	// 2. If client_secret is provided, re-encrypt
	if req.ClientSecret != nil {
		encryptedSecret, encErr := s.encryption.Encrypt(*req.ClientSecret)
		if encErr != nil {
			s.logger.Error(ctx, "failed to encrypt client secret", logger.Err(encErr))
			return nil, apperrors.WrapInternal(encErr)
		}
		provider.ClientSecretEncrypted = encryptedSecret
	}

	// 3. Apply partial updates (only non-nil fields)
	if req.DisplayName != nil {
		provider.DisplayName = *req.DisplayName
	}
	if req.ClientID != nil {
		provider.ClientID = *req.ClientID
	}
	if req.RedirectURL != nil {
		if valErr := s.validateRedirectURL(*req.RedirectURL); valErr != nil {
			return nil, valErr
		}
		provider.RedirectURL = *req.RedirectURL
	}
	if req.AdditionalScopes != nil {
		scopesBytes, marshalErr := json.Marshal(*req.AdditionalScopes)
		if marshalErr != nil {
			return nil, apperrors.WrapInternal(marshalErr)
		}
		provider.AdditionalScopes = scopesBytes
	}
	if req.Config != nil {
		provider.Config = datatypes.JSON(req.Config)
	}
	if req.IsEnabled != nil {
		provider.IsEnabled = *req.IsEnabled
	}

	// 4. Save via repository
	if err := s.providerRepo.Update(ctx, provider); err != nil {
		s.logger.Error(ctx, "failed to update OAuth provider",
			s.logger.String("provider_id", id.String()),
			logger.Err(err),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "OAuth provider updated",
		s.logger.String("provider_id", provider.ID.String()),
	)

	// 5. Audit log — client_secret excluded from after-state
	if s.auditService != nil {
		actorID := getActorID(ctx)
		afterState := provider.ToResponse()
		_ = s.auditService.LogAction(ctx, actorID, "oauth_provider.updated", "oauth_provider", provider.ID.String(), beforeState, afterState, "", "")
	}

	resp := provider.ToResponse()
	return &resp, nil
}

// Delete soft-deletes an OAuth provider. Prevents deletion of system providers.
func (s *OAuthProviderService) Delete(ctx context.Context, id uuid.UUID) error {
	// 1. Find provider by ID
	provider, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 2. Check is_system flag — cannot delete system providers
	if provider.IsSystem {
		return apperrors.NewAppError("FORBIDDEN", "cannot delete system OAuth provider", 403)
	}

	// 3. Soft delete via repository
	if err := s.providerRepo.SoftDelete(ctx, id); err != nil {
		s.logger.Error(ctx, "failed to soft delete OAuth provider",
			s.logger.String("provider_id", id.String()),
			logger.Err(err),
		)
		return err
	}

	s.logger.Info(ctx, "OAuth provider soft deleted",
		s.logger.String("provider_id", id.String()),
	)

	// 4. Audit log
	if s.auditService != nil {
		actorID := getActorID(ctx)
		_ = s.auditService.LogAction(ctx, actorID, "oauth_provider.deleted", "oauth_provider", id.String(), nil, nil, "", "")
	}

	return nil
}

// validateRedirectURL checks that the redirect URL is valid and uses HTTPS
// unless AllowHTTP is true (for local development).
func (s *OAuthProviderService) validateRedirectURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return apperrors.NewAppError("VALIDATION_ERROR", "invalid redirect URL", 422)
	}

	if parsed.Scheme != "https" && !s.config.AllowHTTP {
		return apperrors.NewAppError("VALIDATION_ERROR", "redirect URL must use HTTPS (set OAUTH_ALLOW_HTTP=true for local development)", 422)
	}

	if parsed.Host == "" {
		return apperrors.NewAppError("VALIDATION_ERROR", "redirect URL must have a valid host", 422)
	}

	return nil
}

// getActorID extracts the actor ID from context, or returns uuid.Nil if not found.
// This is a helper for audit logging.
func getActorID(ctx context.Context) uuid.UUID {
	// Attempt to get user ID from context if available
	if val := ctx.Value("user_id"); val != nil {
		if id, ok := val.(uuid.UUID); ok {
			return id
		}
		if str, ok := val.(string); ok {
			if id, err := uuid.Parse(str); err == nil {
				return id
			}
		}
	}
	return uuid.Nil
}
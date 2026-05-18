package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/ssrf"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ──────────────────────────────────────────────
// Profile normalization helpers
// ──────────────────────────────────────────────

// NormalizeGoogleProfile extracts user info from Google's /oauth2/v3/userinfo endpoint.
// Google returns: sub, email, email_verified, name, picture.
func NormalizeGoogleProfile(raw map[string]interface{}) domain.ProviderProfile {
	return domain.ProviderProfile{
		ProviderID:    getStringField(raw, "sub"),
		Email:         getStringField(raw, "email"),
		EmailVerified: getBoolField(raw, "email_verified"),
		DisplayName:   getStringField(raw, "name"),
		AvatarURL:     getStringField(raw, "picture"),
	}
}

// NormalizeGitHubProfile extracts user info from GitHub's /user endpoint.
// GitHub returns: id (integer), login, email (may be null), avatar_url.
// GitHub IDs are integers — convert with fmt.Sprintf.
func NormalizeGitHubProfile(raw map[string]interface{}) domain.ProviderProfile {
	return domain.ProviderProfile{
		ProviderID:    fmt.Sprintf("%v", raw["id"]),
		Email:         getStringField(raw, "email"),
		EmailVerified: false, // GitHub doesn't verify in /user response
		DisplayName:   getStringField(raw, "login"),
		AvatarURL:     getStringField(raw, "avatar_url"),
	}
}

// NormalizeMicrosoftProfile extracts user info from Microsoft Graph /me endpoint.
// Microsoft returns: id, mail/userPrincipalName, displayName.
func NormalizeMicrosoftProfile(raw map[string]interface{}) domain.ProviderProfile {
	return domain.ProviderProfile{
		ProviderID:    getStringField(raw, "id"),
		Email:         getMicrosoftEmail(raw),
		EmailVerified: false, // Microsoft doesn't provide verification in /me
		DisplayName:   getStringField(raw, "displayName"),
		AvatarURL:     "", // Microsoft doesn't include photo in /me
	}
}

// getStringField safely extracts a string from a raw profile map.
func getStringField(raw map[string]interface{}, key string) string {
	if v, ok := raw[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// getBoolField safely extracts a bool from a raw profile map.
func getBoolField(raw map[string]interface{}, key string) bool {
	if v, ok := raw[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getMicrosoftEmail returns the email from a Microsoft profile.
// Checks "mail" first, then falls back to "userPrincipalName".
func getMicrosoftEmail(raw map[string]interface{}) string {
	if mail := getStringField(raw, "mail"); mail != "" {
		return mail
	}
	return getStringField(raw, "userPrincipalName")
}

// ──────────────────────────────────────────────
// OAuthLoginService
// ──────────────────────────────────────────────

// OAuthLoginService handles OAuth login flows: initiation, callback processing,
// user creation, and account linking.
type OAuthLoginService struct {
	providerRepo     repository.OAuthProviderRepository
	accountRepo      repository.OAuthAccountRepository
	userRepo         repository.UserRepository
	roleRepo         repository.RoleRepository
	userRoleRepo     repository.UserRoleRepository
	refreshTokenRepo repository.RefreshTokenRepository
	stateManager     OAuthStateManager
	encryption       *OAuthEncryptionService
	tokenService     *TokenService
	audit            *AuditService
	enforcer         *permission.Enforcer
	config           config.OAuthConfig
	logger           logger.Logger
	ssrfClient       *http.Client
	eventBus         *domain.EventBus // Set via SetEventBus
}

// NewOAuthLoginService creates a new OAuthLoginService instance.
func NewOAuthLoginService(
	providerRepo repository.OAuthProviderRepository,
	accountRepo repository.OAuthAccountRepository,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	userRoleRepo repository.UserRoleRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	stateManager OAuthStateManager,
	encryption *OAuthEncryptionService,
	tokenService *TokenService,
	audit *AuditService,
	enforcer *permission.Enforcer,
	cfg config.OAuthConfig,
	log logger.Logger,
	ssrfCfg ssrf.SSRFConfig,
) *OAuthLoginService {
	return &OAuthLoginService{
		providerRepo:     providerRepo,
		accountRepo:      accountRepo,
		userRepo:         userRepo,
		roleRepo:         roleRepo,
		userRoleRepo:     userRoleRepo,
		refreshTokenRepo: refreshTokenRepo,
		stateManager:     stateManager,
		encryption:        encryption,
		tokenService:      tokenService,
		audit:             audit,
		enforcer:          enforcer,
		config:            cfg,
		logger:            log,
		ssrfClient:        ssrf.NewClient(&ssrfCfg, nil),
	}
}

// SetEventBus sets the EventBus for publishing OAuth events.
// Follows the same setter pattern as WebhookService and ActivityService.
func (s *OAuthLoginService) SetEventBus(bus *domain.EventBus) {
	s.eventBus = bus
}

// ──────────────────────────────────────────────
// Login flow: initiation
// ──────────────────────────────────────────────

// InitiateLogin generates state, stores it in Redis, builds the provider authorization URL,
// and returns the URL for the client to redirect to.
func (s *OAuthLoginService) InitiateLogin(
	ctx context.Context,
	providerName string,
	callbackURL string,
	userID, orgID uuid.UUID,
) (string, error) {
	// 1. Find provider by name
	provider, err := s.providerRepo.FindByName(ctx, providerName)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return "", apperrors.NewAppError("NOT_FOUND", fmt.Sprintf("OAuth provider %q not found", providerName), 404)
		}
		return "", apperrors.WrapInternal(err)
	}

	// 2. Validate provider is enabled
	if !provider.IsEnabled {
		return "", apperrors.NewAppError("FORBIDDEN", fmt.Sprintf("OAuth provider %q is disabled", providerName), 403)
	}

	// 3. Determine intent: login vs link
	intent := "login"
	if userID != uuid.Nil {
		intent = "link"
	}

	// 4. Create state (stores in Redis with PKCE code_verifier)
	nonce, codeVerifier, err := s.stateManager.CreateState(ctx, callbackURL, providerName, intent, userID, orgID)
	if err != nil {
		s.logger.Error(ctx, "failed to create oauth state",
			s.logger.String("provider", providerName),
			logger.Err(err),
		)
		return "", apperrors.WrapInternal(err)
	}

	// 5. Compute PKCE code_challenge from code_verifier (S256)
	codeChallenge := ComputePKCECodeChallenge(codeVerifier)

	// 6. Decrypt client_secret for authorization URL (we only need client_id for the auth URL)
	clientID := provider.ClientID

	// 7. Build authorization URL
	authURL, err := s.buildAuthorizationURL(provider, clientID, callbackURL, nonce, codeChallenge)
	if err != nil {
		s.logger.Error(ctx, "failed to build authorization URL",
			s.logger.String("provider", providerName),
			logger.Err(err),
		)
		return "", apperrors.WrapInternal(err)
	}

	s.logger.Info(ctx, "oauth login initiated",
		s.logger.String("provider", providerName),
		s.logger.String("intent", intent),
	)

	return authURL, nil
}

// buildAuthorizationURL constructs the provider-specific authorization URL.
func (s *OAuthLoginService) buildAuthorizationURL(
	provider *domain.OAuthProvider,
	clientID string,
	redirectURI string,
	state string,
	codeChallenge string,
) (string, error) {
	baseURL := s.getProviderAuthorizationURL(provider)
	if baseURL == "" {
		return "", apperrors.NewAppError("BAD_REQUEST", fmt.Sprintf("unsupported OAuth provider: %s", provider.Name), 400)
	}

	scopes := provider.GetEffectiveScopes()

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", apperrors.WrapInternal(fmt.Errorf("invalid authorization URL: %w", err))
	}

	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	// Provider-specific query params
	if provider.Name == domain.OAuthProviderMicrosoft {
		// Microsoft requires tenant in URL; extract from config JSONB
		tenantID := getTenantIDFromConfig(provider.Config)
		if tenantID != "" {
			// Replace common or consumers tenant with the configured one
			u.Path = strings.Replace(u.Path, "{tenant_id}", tenantID, 1)
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ──────────────────────────────────────────────
// Login flow: callback
// ──────────────────────────────────────────────

// HandleCallback processes the OAuth callback: validates state, exchanges code,
// fetches profile, and either logs in an existing user or creates a new one.
func (s *OAuthLoginService) HandleCallback(
	ctx context.Context,
	providerName string,
	code string,
	stateNonce string,
) (redirectURL string, err error) {
	// 1. Validate state from Redis (also deletes to prevent replay)
	state, err := s.stateManager.ValidateState(ctx, stateNonce)
	if err != nil {
		s.logger.Error(ctx, "failed to validate oauth state",
			s.logger.String("nonce", stateNonce),
			logger.Err(err),
		)
		return "", apperrors.NewAppError("INVALID_STATE", "oauth state not found or expired", 400)
	}

	// Verify the provider from state matches the callback provider
	if state.Provider != providerName {
		return "", apperrors.NewAppError("INVALID_STATE", fmt.Sprintf("provider mismatch: state=%s, callback=%s", state.Provider, providerName), 400)
	}

	// 2. Find provider by name
	provider, err := s.providerRepo.FindByName(ctx, providerName)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return "", apperrors.NewAppError("NOT_FOUND", fmt.Sprintf("OAuth provider %q not found", providerName), 404)
		}
		return "", apperrors.WrapInternal(err)
	}

	// 3. Validate provider still enabled
	if !provider.IsEnabled {
		return "", apperrors.NewAppError("FORBIDDEN", fmt.Sprintf("OAuth provider %q is disabled", providerName), 403)
	}

	// 4. Exchange authorization code for tokens
	tokenResponse, err := s.exchangeCodeForTokens(ctx, provider, code, state.CallbackURL, state.CodeVerifier)
	if err != nil {
		s.logger.Error(ctx, "failed to exchange authorization code",
			s.logger.String("provider", providerName),
			logger.Err(err),
		)
		return "", apperrors.WrapInternal(fmt.Errorf("failed to exchange authorization code: %w", err))
	}

	// 5. Get access token from response
	accessToken, ok := getStringFieldOK(tokenResponse, "access_token")
	if !ok || accessToken == "" {
		return "", apperrors.NewAppError("AUTH_ERROR", "no access_token in provider response", 502)
	}

	// 6. Fetch user profile from provider's userinfo endpoint
	rawProfile, err := s.fetchUserProfile(ctx, provider, accessToken)
	if err != nil {
		s.logger.Error(ctx, "failed to fetch user profile",
			s.logger.String("provider", providerName),
			logger.Err(err),
		)
		return "", apperrors.WrapInternal(fmt.Errorf("failed to fetch user profile: %w", err))
	}

	// 7. Normalize profile based on provider name
	profile := s.normalizeProfile(provider.Name, rawProfile)
	if profile.ProviderID == "" {
		return "", apperrors.NewAppError("AUTH_ERROR", "provider did not return a user ID", 502)
	}

	// 8. Route based on intent
	switch state.Intent {
	case "link":
		return s.handleLinkIntent(ctx, provider, profile, state)
	case "login":
		fallthrough
	default:
		return s.handleLoginIntent(ctx, provider, profile, state, rawProfile)
	}
}

// ──────────────────────────────────────────────
// Login intent: find or create user
// ──────────────────────────────────────────────

// handleLoginIntent processes login intent:
//  1. Find existing OAuthAccount by provider_id + provider_user_id
//  2. If found → login (generate tokens)
//  3. If not found:
//     a. Email not verified → error fragment
//     b. Email matches existing user → error fragment (email_already_exists)
//     c. No email match → auto-create user + OAuthAccount + generate tokens
func (s *OAuthLoginService) handleLoginIntent(
	ctx context.Context,
	provider *domain.OAuthProvider,
	profile domain.ProviderProfile,
	state *domain.OAuthState,
	rawProfile map[string]interface{},
) (string, error) {
	// 1. Try to find existing OAuthAccount
	existingAccount, err := s.accountRepo.FindByProviderAndProviderUserID(ctx, provider.ID, profile.ProviderID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return "", apperrors.WrapInternal(err)
	}

	if existingAccount != nil {
		// Existing account found — find the user and generate tokens
		user, err := s.userRepo.FindByID(ctx, existingAccount.UserID)
		if err != nil {
			return "", apperrors.WrapInternal(err)
		}

		// Note: We don't update the OAuthAccount profile fields here because
		// the OAuthAccountRepository interface doesn't expose an Update method.
		// Profile updates should be handled through a dedicated account management endpoint.
		// The profile data from the latest OAuth login is used for creating new accounts only.

		accessToken, refreshToken, err := s.generateTokensForUser(ctx, user)
		if err != nil {
			return "", err
		}

		s.logger.Info(ctx, "oauth login successful",
			s.logger.String("provider", provider.Name),
			s.logger.String("user_id", user.ID.String()),
		)

		return s.buildSuccessFragmentURL(state.CallbackURL, accessToken, refreshToken, int(s.tokenService.GetRefreshExpiry().Seconds())), nil
	}

	// 2. No existing OAuth account
	// 2a. If email exists but is not verified, return error
	if profile.Email != "" && !profile.EmailVerified {
		return s.buildErrorFragmentURL(state.CallbackURL, "email_not_verified", "email address is not verified by the provider"), nil
	}

	// 2b. If email exists and matches an existing user (but not linked), return error
	if profile.Email != "" {
		existingUser, err := s.userRepo.FindByEmail(ctx, profile.Email)
		if err == nil && existingUser != nil {
			// Email already registered — user needs to link manually or log in first
			return s.buildErrorFragmentURL(state.CallbackURL, "email_already_exists", "an account with this email already exists, please log in and link the provider"), nil
		}
	}

	// 2c. Auto-create user
	user, err := s.autoCreateUser(ctx, profile, provider)
	if err != nil {
		return "", err
	}

	// Create OAuthAccount linking
	account := &domain.OAuthAccount{
		UserID:         user.ID,
		ProviderID:     provider.ID,
		ProviderUserID: profile.ProviderID,
		Email:          profile.Email,
		EmailVerified:  profile.EmailVerified,
		DisplayName:    profile.DisplayName,
		AvatarURL:      profile.AvatarURL,
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return "", apperrors.WrapInternal(err)
	}

	// Publish auth.oauth.linked event
	s.publishOAuthLinkedEvent(account, provider)

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokensForUser(ctx, user)
	if err != nil {
		return "", err
	}

	s.logger.Info(ctx, "oauth auto-created user and logged in",
		s.logger.String("provider", provider.Name),
		s.logger.String("user_id", user.ID.String()),
		s.logger.String("email", user.Email),
	)

	return s.buildSuccessFragmentURL(state.CallbackURL, accessToken, refreshToken, int(s.tokenService.GetRefreshExpiry().Seconds())), nil
}

// handleLinkIntent processes link intent:
// Links an existing authenticated user to an OAuth provider.
func (s *OAuthLoginService) handleLinkIntent(
	ctx context.Context,
	provider *domain.OAuthProvider,
	profile domain.ProviderProfile,
	state *domain.OAuthState,
) (string, error) {
	// Verify user ID was provided in state (must be authenticated for link)
	if state.UserID == uuid.Nil {
		return s.buildErrorFragmentURL(state.CallbackURL, "auth_required", "authentication required to link a provider"), nil
	}

	// Find the authenticated user
	user, err := s.userRepo.FindByID(ctx, state.UserID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return s.buildErrorFragmentURL(state.CallbackURL, "user_not_found", "authenticated user not found"), nil
		}
		return "", apperrors.WrapInternal(err)
	}

	// Check if this provider account is already linked to another user
	existingAccount, err := s.accountRepo.FindByProviderAndProviderUserID(ctx, provider.ID, profile.ProviderID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return "", apperrors.WrapInternal(err)
	}
	if existingAccount != nil && existingAccount.UserID != user.ID {
		return s.buildErrorFragmentURL(state.CallbackURL, "provider_already_linked", "this provider account is already linked to another user"), nil
	}

	// Check if user already has this provider linked
	existingLink, err := s.accountRepo.FindByUserAndProvider(ctx, user.ID, provider.ID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return "", apperrors.WrapInternal(err)
	}
	if existingLink != nil {
		return s.buildErrorFragmentURL(state.CallbackURL, "already_linked", "you have already linked this provider"), nil
	}

	// Create OAuthAccount
	account := &domain.OAuthAccount{
		UserID:         user.ID,
		ProviderID:     provider.ID,
		ProviderUserID: profile.ProviderID,
		Email:          profile.Email,
		EmailVerified:  profile.EmailVerified,
		DisplayName:    profile.DisplayName,
		AvatarURL:      profile.AvatarURL,
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return "", apperrors.WrapInternal(err)
	}

	// Publish auth.oauth.linked event
	s.publishOAuthLinkedEvent(account, provider)

	s.logger.Info(ctx, "oauth provider linked",
		s.logger.String("provider", provider.Name),
		s.logger.String("user_id", user.ID.String()),
		s.logger.String("account_id", account.ID.String()),
	)

	return s.buildSuccessFragmentURL(state.CallbackURL, "", "", 0), nil
}

// ──────────────────────────────────────────────
// Auto-create user
// ──────────────────────────────────────────────

// autoCreateUser creates a new user from OAuth profile data.
// If no email is provided by the provider, sets a placeholder email
// with "oauth-{uuid}@social.placeholder" format.
// The password hash is set to a random unmatchable value.
func (s *OAuthLoginService) autoCreateUser(
	ctx context.Context,
	profile domain.ProviderProfile,
	provider *domain.OAuthProvider,
) (*domain.User, error) {
	email := profile.Email
	if email == "" {
		// Generate placeholder email: oauth-{uuid}@social.placeholder
		email = fmt.Sprintf("oauth-%s@social.placeholder", uuid.New().String())
	}

	// Generate unmatchable password hash (64-char random hex)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to generate random password: %w", err))
	}
	passwordHash := "oauth-" + hex.EncodeToString(randomBytes)

	user := &domain.User{
		Email:        email,
		PasswordHash: passwordHash,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to create user: %w", err))
	}

	// Assign default "viewer" role (non-blocking, same pattern as AuthService.Register)
	if s.roleRepo != nil && s.userRoleRepo != nil {
		go func() {
			bgCtx := context.Background()
			viewerRole, err := s.roleRepo.FindByName(bgCtx, "viewer")
			if err == nil && viewerRole != nil {
				_ = s.userRoleRepo.Assign(bgCtx, user.ID, viewerRole.ID, user.ID)
			}
		}()
	}

	// Audit log the social login event
	s.audit.LogAction(ctx, user.ID, "auth.oauth.login", "user", user.ID.String(), nil, nil, "", "")

	return user, nil
}

// ──────────────────────────────────────────────
// Token generation
// ──────────────────────────────────────────────

// generateTokensForUser generates access and refresh tokens for an existing user.
// Follows the same pattern as AuthService.Login.
func (s *OAuthLoginService) generateTokensForUser(ctx context.Context, user *domain.User) (string, string, error) {
	// Generate access token
	accessToken, err := s.tokenService.GenerateAccessToken(user.ID.String(), user.Email)
	if err != nil {
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to generate access token: %w", err))
	}

	// Generate refresh token
	refreshToken, refreshHash, err := s.tokenService.GenerateRefreshToken()
	if err != nil {
		return "", "", apperrors.WrapInternal(fmt.Errorf("failed to generate refresh token: %w", err))
	}

	// Create token family (same pattern as AuthService.Login)
	familyID := uuid.New()

	// Store refresh token in database
	expiresAt := time.Now().Add(s.tokenService.GetRefreshExpiry())
	token := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		FamilyID:  familyID,
		ExpiresAt: expiresAt,
	}

	if s.refreshTokenRepo != nil {
		if err := s.refreshTokenRepo.Create(ctx, token); err != nil {
			return "", "", apperrors.WrapInternal(fmt.Errorf("failed to store refresh token: %w", err))
		}
	}

	return accessToken, refreshToken, nil
}

// ──────────────────────────────────────────────
// HTTP operations: code exchange & profile fetch
// ──────────────────────────────────────────────

// exchangeCodeForTokens makes a POST to the provider's token endpoint
// to exchange the authorization code for access/refresh tokens.
func (s *OAuthLoginService) exchangeCodeForTokens(
	ctx context.Context,
	provider *domain.OAuthProvider,
	code string,
	redirectURI string,
	codeVerifier string,
) (map[string]interface{}, error) {
	tokenURL := s.getProviderTokenURL(provider)
	if tokenURL == "" {
		return nil, apperrors.NewAppError("BAD_REQUEST", fmt.Sprintf("unsupported OAuth provider: %s", provider.Name), 400)
	}

	// Decrypt client_secret
	clientSecret, err := s.encryption.Decrypt(provider.ClientSecretEncrypted)
	if err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to decrypt client secret: %w", err))
	}

	// Build request body
	data := url.Values{}
	data.Set("client_id", provider.ClientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", codeVerifier)
	data.Set("grant_type", "authorization_code")

	// For GitHub, explicitly request JSON response
	if provider.Name == domain.OAuthProviderGitHub {
		data.Set("accept", "application/json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to create token request: %w", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.ssrfClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("token exchange request failed: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, apperrors.NewAppError("PROVIDER_ERROR",
			fmt.Sprintf("token exchange failed with status %d: %s", resp.StatusCode, string(body)), 502)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to decode token response: %w", err))
	}

	return result, nil
}

// fetchUserProfile makes a GET to the provider's userinfo endpoint
// to retrieve the authenticated user's profile.
func (s *OAuthLoginService) fetchUserProfile(
	ctx context.Context,
	provider *domain.OAuthProvider,
	accessToken string,
) (map[string]interface{}, error) {
	userInfoURL := s.getProviderUserInfoURL(provider)
	if userInfoURL == "" {
		return nil, apperrors.NewAppError("BAD_REQUEST", fmt.Sprintf("unsupported OAuth provider: %s", provider.Name), 400)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to create profile request: %w", err))
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	// GitHub requires User-Agent header
	if provider.Name == domain.OAuthProviderGitHub {
		req.Header.Set("User-Agent", "GoAPI-Base/1.0")
	}

	resp, err := s.ssrfClient.Do(req)
	if err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("profile request failed: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, apperrors.NewAppError("PROVIDER_ERROR",
			fmt.Sprintf("profile request failed with status %d: %s", resp.StatusCode, string(body)), 502)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, apperrors.WrapInternal(fmt.Errorf("failed to decode profile response: %w", err))
	}

	return result, nil
}

// ──────────────────────────────────────────────
// Provider URL resolution
// ──────────────────────────────────────────────

// getProviderAuthorizationURL returns the base authorization URL for the given provider.
func (s *OAuthLoginService) getProviderAuthorizationURL(provider *domain.OAuthProvider) string {
	switch provider.Name {
	case domain.OAuthProviderGoogle:
		return "https://accounts.google.com/o/oauth2/v2/auth"
	case domain.OAuthProviderGitHub:
		return "https://github.com/login/oauth/authorize"
	case domain.OAuthProviderMicrosoft:
		tenantID := getTenantIDFromConfig(provider.Config)
		if tenantID == "" {
			tenantID = "common"
		}
		return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID)
	default:
		return ""
	}
}

// getProviderTokenURL returns the token exchange URL for the given provider.
func (s *OAuthLoginService) getProviderTokenURL(provider *domain.OAuthProvider) string {
	switch provider.Name {
	case domain.OAuthProviderGoogle:
		return "https://oauth2.googleapis.com/token"
	case domain.OAuthProviderGitHub:
		return "https://github.com/login/oauth/access_token"
	case domain.OAuthProviderMicrosoft:
		tenantID := getTenantIDFromConfig(provider.Config)
		if tenantID == "" {
			tenantID = "common"
		}
		return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	default:
		return ""
	}
}

// getProviderUserInfoURL returns the userinfo/profile endpoint URL for the given provider.
func (s *OAuthLoginService) getProviderUserInfoURL(provider *domain.OAuthProvider) string {
	switch provider.Name {
	case domain.OAuthProviderGoogle:
		return "https://www.googleapis.com/oauth2/v3/userinfo"
	case domain.OAuthProviderGitHub:
		return "https://api.github.com/user"
	case domain.OAuthProviderMicrosoft:
		return "https://graph.microsoft.com/v1.0/me"
	default:
		return ""
	}
}

// ──────────────────────────────────────────────
// Profile normalization
// ──────────────────────────────────────────────

// normalizeProfile dispatches to the appropriate provider-specific normalizer.
func (s *OAuthLoginService) normalizeProfile(providerName string, raw map[string]interface{}) domain.ProviderProfile {
	switch providerName {
	case domain.OAuthProviderGoogle:
		return NormalizeGoogleProfile(raw)
	case domain.OAuthProviderGitHub:
		return NormalizeGitHubProfile(raw)
	case domain.OAuthProviderMicrosoft:
		return NormalizeMicrosoftProfile(raw)
	default:
		// Fallback: extract common fields
		return domain.ProviderProfile{
			ProviderID:    getStringField(raw, "id"),
			Email:         getStringField(raw, "email"),
			EmailVerified: getBoolField(raw, "email_verified"),
			DisplayName:   getStringField(raw, "name"),
		}
	}
}

// ──────────────────────────────────────────────
// Fragment URL builders
// ──────────────────────────────────────────────

// buildSuccessFragmentURL builds the callback URL with tokens in the URL fragment.
// Format: callbackURL#access_token={jwt}&refresh_token={hex}&token_type=Bearer&expires_in={seconds}
func (s *OAuthLoginService) buildSuccessFragmentURL(callbackURL, accessToken, refreshToken string, expiresIn int) string {
	u, err := url.Parse(callbackURL)
	if err != nil {
		// Fallback: append as query-like fragment
		return callbackURL + "#access_token=" + accessToken
	}
	fragment := fmt.Sprintf("access_token=%s&refresh_token=%s&token_type=Bearer&expires_in=%d",
		url.QueryEscape(accessToken), url.QueryEscape(refreshToken), expiresIn)
	u.Fragment = fragment
	return u.String()
}

// buildErrorFragmentURL builds the callback URL with error info in the URL fragment.
// Format: callbackURL#error={code}&message={message}
func (s *OAuthLoginService) buildErrorFragmentURL(callbackURL, errorCode, message string) string {
	u, err := url.Parse(callbackURL)
	if err != nil {
		// Fallback: append as query-like fragment
		return callbackURL + "#error=" + errorCode
	}
	fragment := fmt.Sprintf("error=%s&message=%s",
		url.QueryEscape(errorCode), url.QueryEscape(message))
	u.Fragment = fragment
	return u.String()
}

// ──────────────────────────────────────────────
// Event publishing
// ──────────────────────────────────────────────

// publishOAuthLinkedEvent publishes auth.oauth.linked via EventBus (non-blocking).
func (s *OAuthLoginService) publishOAuthLinkedEvent(account *domain.OAuthAccount, provider *domain.OAuthProvider) {
	if s.eventBus == nil {
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
		if account.Provider != nil && account.Provider.OrganizationID != nil {
			orgID = account.Provider.OrganizationID
		} else if provider.OrganizationID != nil {
			orgID = provider.OrganizationID
		}

		if err := s.eventBus.Publish(domain.WebhookEvent{
			Type:    domain.OAuthEventLinked,
			Payload: payload,
			OrgID:   orgID,
		}); err != nil {
			// Log but don't block — event publishing is best-effort
		}
	}()
}

// ──────────────────────────────────────────────
// Utility functions
// ──────────────────────────────────────────────

// getStringFieldOK returns the string value and whether the key existed in the map.
func getStringFieldOK(raw map[string]interface{}, key string) (string, bool) {
	if v, ok := raw[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s, true
		}
		return fmt.Sprintf("%v", v), true
	}
	return "", false
}

// getTenantIDFromConfig extracts the tenant_id from Microsoft provider config JSONB.
func getTenantIDFromConfig(configJSON datatypes.JSON) string {
	if configJSON == nil {
		return ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return ""
	}
	if tenantID, ok := cfg["tenant_id"].(string); ok {
		return tenantID
	}
	return ""
}
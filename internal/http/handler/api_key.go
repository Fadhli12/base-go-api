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

// APIKeyHandler handles API key endpoints
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler instance
func NewAPIKeyHandler(apiKeyService *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKeyRequest represents the response for a successful API key creation
type CreateAPIKeyResponse struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Name      string   `json:"name"`
	Prefix    string   `json:"prefix"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
	Key       string   `json:"key"` // Only returned on creation
	CreatedAt string   `json:"created_at"`
}

// ListAPIKeysResponse represents the response for listing API keys
type ListAPIKeysResponse struct {
	APIKeys []APIKeyListItem `json:"api_keys"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// APIKeyListItem represents an API key in list responses
type APIKeyListItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	IsRevoked  bool     `json:"is_revoked"`
	IsExpired  bool     `json:"is_expired"`
	IsActive   bool     `json:"is_active"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// RevokeAPIKeyResponse represents the response for revoking an API key
type RevokeAPIKeyResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// Create handles API key creation
//
//	@Summary		Create a new API key
//	@Description	Create a new API key with specified scopes for the authenticated user
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		request.CreateAPIKeyRequest	true	"API key details"
//	@Success		201		{object}	response.Envelope{data=handler.CreateAPIKeyResponse}
//	@Failure		400		{object}	response.Envelope	"Invalid request"
//	@Failure		401		{object}	response.Envelope	"Unauthorized"
//	@Failure		403		{object}	response.Envelope	"Permission denied"
//	@Failure		409		{object}	response.Envelope	"API key name already exists"
//	@Router			/api/v1/api-keys [post]
func (h *APIKeyHandler) Create(c echo.Context) error {
	// Get user ID from context (set by JWT middleware)
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	var req request.CreateAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	result, err := h.apiKeyService.Create(c.Request().Context(), userID, req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create API key"))
	}

	resp := CreateAPIKeyResponse{
		ID:        result.APIKey.ID,
		UserID:    result.APIKey.UserID,
		Name:      result.APIKey.Name,
		Prefix:    result.APIKey.Prefix,
		Scopes:    result.APIKey.Scopes,
		ExpiresAt: result.APIKey.ExpiresAt,
		Key:       result.Secret, // Only time key is exposed
		CreatedAt: result.APIKey.CreatedAt,
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, resp))
}

// List handles listing API keys for the authenticated user
//
//	@Summary		List API keys
//	@Description	List all API keys for the authenticated user with pagination
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Number of results (default: 20, max: 100)"
//	@Param			offset	query		int	false	"Pagination offset (default: 0)"
//	@Success		200		{object}	response.Envelope{data=handler.ListAPIKeysResponse}
//	@Failure		401		{object}	response.Envelope	"Unauthorized"
//	@Failure		500		{object}	response.Envelope	"Internal server error"
//	@Router			/api/v1/api-keys [get]
func (h *APIKeyHandler) List(c echo.Context) error {
	// Get user ID from context (set by JWT middleware)
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	// Parse query parameters
	var req request.ListAPIKeysRequest
	if err := c.Bind(&req); err != nil {
		// Use defaults on bind error
		req.Limit = 20
		req.Offset = 0
	}

	// Set defaults
	req.SetDefaults()

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	apiKeys, total, err := h.apiKeyService.List(c.Request().Context(), userID, req.Limit, req.Offset)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve API keys"))
	}

	items := make([]APIKeyListItem, len(apiKeys))
	for i, key := range apiKeys {
		resp := key.ToResponse()
		items[i] = APIKeyListItem{
			ID:         resp.ID,
			Name:       resp.Name,
			Prefix:     resp.Prefix,
			Scopes:     resp.Scopes,
			ExpiresAt:  resp.ExpiresAt,
			LastUsedAt: resp.LastUsedAt,
			IsRevoked:  resp.IsRevoked,
			IsExpired:  resp.IsExpired,
			IsActive:   resp.IsActive,
			CreatedAt:  resp.CreatedAt,
			UpdatedAt:  resp.UpdatedAt,
		}
	}

	resp := ListAPIKeysResponse{
		APIKeys: items,
		Total:   total,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// GetByID handles retrieving a specific API key
//
//	@Summary		Get API key by ID
//	@Description	Get details of a specific API key by ID
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"API key ID"
//	@Success		200	{object}	response.Envelope{data=handler.APIKeyListItem}
//	@Failure		400	{object}	response.Envelope	"Invalid ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Forbidden"
//	@Failure		404	{object}	response.Envelope	"Not found"
//	@Router			/api/v1/api-keys/{id} [get]
func (h *APIKeyHandler) GetByID(c echo.Context) error {
	// Get user ID from context (set by JWT middleware)
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	// Parse path parameter
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid API key ID"))
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request().Context(), userID, keyID)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve API key"))
	}

	resp := apiKey.ToResponse()
	item := APIKeyListItem{
		ID:         resp.ID,
		Name:       resp.Name,
		Prefix:     resp.Prefix,
		Scopes:     resp.Scopes,
		ExpiresAt:  resp.ExpiresAt,
		LastUsedAt: resp.LastUsedAt,
		IsRevoked:  resp.IsRevoked,
		IsExpired:  resp.IsExpired,
		IsActive:   resp.IsActive,
		CreatedAt:  resp.CreatedAt,
		UpdatedAt:  resp.UpdatedAt,
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, item))
}

// Revoke handles revoking an API key
//
//	@Summary		Revoke an API key
//	@Description	Revoke (soft delete) an API key by ID
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"API key ID"
//	@Success		200	{object}	response.Envelope{data=handler.RevokeAPIKeyResponse}
//	@Failure		400	{object}	response.Envelope	"Invalid ID"
//	@Failure		401	{object}	response.Envelope	"Unauthorized"
//	@Failure		403	{object}	response.Envelope	"Forbidden"
//	@Failure		404	{object}	response.Envelope	"Not found"
//	@Failure		409	{object}	response.Envelope	"Already revoked"
//	@Router			/api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) Revoke(c echo.Context) error {
	// Get user ID from context (set by JWT middleware)
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "User not authenticated"))
	}

	// Parse path parameter
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid API key ID"))
	}

	if err := h.apiKeyService.Revoke(c.Request().Context(), userID, keyID); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to revoke API key"))
	}

	resp := RevokeAPIKeyResponse{
		ID:      keyIDStr,
		Message: "API key revoked successfully",
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, resp))
}

// getUserIDFromContext extracts user ID from echo context
func getUserIDFromContext(c echo.Context) (uuid.UUID, error) {
	userIDStr := c.Get("user_id")
	if userIDStr == nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	userID, ok := userIDStr.(uuid.UUID)
	if !ok {
		// Try string conversion
		userIDStrTyped, ok := userIDStr.(string)
		if !ok {
			return uuid.Nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid user ID in context")
		}
		return uuid.Parse(userIDStrTyped)
	}

	return userID, nil
}

// mapAPIKeyToResponse converts a domain.APIKey to APIKeyListItem
func mapAPIKeyToResponse(key *domain.APIKey) APIKeyListItem {
	resp := key.ToResponse()
	return APIKeyListItem{
		ID:         resp.ID,
		Name:       resp.Name,
		Prefix:     resp.Prefix,
		Scopes:     resp.Scopes,
		ExpiresAt:  resp.ExpiresAt,
		LastUsedAt: resp.LastUsedAt,
		IsRevoked:  resp.IsRevoked,
		IsExpired:  resp.IsExpired,
		IsActive:   resp.IsActive,
		CreatedAt:  resp.CreatedAt,
		UpdatedAt:  resp.UpdatedAt,
	}
}
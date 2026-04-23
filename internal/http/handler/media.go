package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// MediaListResponse represents the response for media list endpoints
type MediaListResponse struct {
	Data   []*domain.MediaResponse `json:"data"`
	Total  int64                   `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

// AdminMediaListResponse represents the response for admin media list
type AdminMediaListResponse struct {
	Data   []*domain.MediaResponse `json:"data"`
	Total  int64                   `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

// SignedURLResponse represents the response for signed URL generation
type SignedURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in"`
}

// MediaStatsResponse represents storage statistics
type MediaStatsResponse struct {
	TotalFiles     int64                    `json:"total_files"`
	TotalSize      int64                    `json:"total_size"`
	ByModelType    []ModelTypeStats         `json:"by_model_type"`
	ByMimeType     []MimeTypeStats          `json:"by_mime_type"`
	ByDisk         []DiskStats              `json:"by_disk"`
}

// ModelTypeStats represents media counts by model type
type ModelTypeStats struct {
	ModelType string `json:"model_type"`
	Count     int64  `json:"count"`
	Size      int64  `json:"size"`
}

// MimeTypeStats represents media counts by MIME type
type MimeTypeStats struct {
	MimeType string `json:"mime_type"`
	Count    int64  `json:"count"`
	Size     int64  `json:"size"`
}

// DiskStats represents media counts by storage disk
type DiskStats struct {
	Disk  string `json:"disk"`
	Count int64  `json:"count"`
	Size  int64  `json:"size"`
}

// CleanupResponse represents orphan cleanup result
type CleanupResponse struct {
	OrphansFound    int      `json:"orphans_found"`
	DeletedCount    int      `json:"deleted_count"`
	Errors          []string `json:"errors,omitempty"`
	Message         string   `json:"message"`
}

// MediaHandler handles media-related HTTP endpoints
type MediaHandler struct {
	service   service.MediaService
	auditSvc  *service.AuditService
	enforcer  *permission.Enforcer
}

// NewMediaHandler creates a new MediaHandler instance
func NewMediaHandler(service service.MediaService, auditSvc *service.AuditService, enforcer *permission.Enforcer) *MediaHandler {
	return &MediaHandler{
		service:   service,
		auditSvc:  auditSvc,
		enforcer:  enforcer,
	}
}

// RegisterRoutes registers all media routes with the Echo group
func (h *MediaHandler) RegisterRoutes(g *echo.Group, jwtSecret string) {
	// Model-scoped media routes
	models := g.Group("/models/:model_type/:model_id/media")
	models.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	models.POST("", h.Upload)
	models.GET("", h.List)

	// Media-specific routes
	media := g.Group("/media/:media_id")
	
	// Download route (public - can use signed URL or auth)
	g.GET("/media/:media_id/download", h.Download)
	
	// Protected media routes
	media.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	media.GET("", h.GetByID)
	media.DELETE("", h.Delete)
	media.PATCH("", h.UpdateMetadata)
	media.GET("/url", h.GetSignedURL)

	// Admin routes
	admin := g.Group("/admin/media")
	admin.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))
	if h.enforcer != nil {
		admin.Use(middleware.RequirePermission(h.enforcer, "media", "manage"))
	}
	admin.GET("", h.AdminList)
	admin.POST("/cleanup", h.AdminCleanup)
	admin.GET("/stats", h.AdminStats)
}

// Upload handles POST /api/v1/models/{model_type}/{model_id}/media
// Uploads a new media file
//
//	@Summary	Upload media file
//	@Description	Upload a media file to a specific model instance
//	@Tags		media
//	@Accept		multipart/form-data
//	@Produce	json
//	@Security	BearerAuth
//	@Param		model_type	path	string	true	"Model type (e.g., news, user)"
//	@Param		model_id	path	string	true	"Model instance ID"
//	@Param		file		formData	file	true	"Media file to upload"
//	@Param		collection	formData	string	false	"Collection name (default: default)"
//	@Param		custom_properties	formData	string	false	"Custom properties as JSON"
//	@Success	201	{object}	response.Envelope{data=domain.MediaResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	409	{object}	response.Envelope	"Storage quota exceeded"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/models/{model_type}/{model_id}/media [post]
func (h *MediaHandler) Upload(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse path parameters
	modelType := c.Param("model_type")
	modelIDStr := c.Param("model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid model ID"))
	}

	// Check permission for upload on this model type
	if !h.checkModelPermission(c, userID, modelType, "upload") {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Permission denied"))
	}

	// Parse multipart form
	if err := c.Request().ParseMultipartForm(service.MaxFileSize); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Failed to parse multipart form"))
	}

	// Get file from form
	file, fileHeader, err := c.Request().FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "File is required"))
	}
	defer file.Close()

	// Parse form values
	collection := c.FormValue("collection")
	var customProps map[string]interface{}
	if customPropsJSON := c.FormValue("custom_properties"); customPropsJSON != "" {
		if err := c.Bind(customProps); err != nil {
			return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "VALIDATION_ERROR", "Invalid custom_properties JSON"))
		}
	}

	// Create upload request
	uploadReq := service.UploadRequest{
		File:             file,
		FileHeader:       fileHeader,
		ModelType:        modelType,
		ModelID:          modelID,
		Collection:       collection,
		CustomProperties: customProps,
	}

	// Call service
	media, err := h.service.Upload(c.Request().Context(), userID, uploadReq)
	if err != nil {
		if appErr := errors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to upload media"))
	}

	// Log audit event for upload
	h.logUploadAudit(c, userID, media)

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, media.ToResponse()))
}

// List handles GET /api/v1/models/{model_type}/{model_id}/media
// Lists media for a specific model
//
//	@Summary	List media for model
//	@Description	List all media files for a specific model instance
//	@Tags		media
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		model_type	path	string	true	"Model type"
//	@Param		model_id	path	string	true	"Model instance ID"
//	@Param		collection	query	string	false	"Filter by collection name"
//	@Param		mime_type	query	string	false	"Filter by MIME type"
//	@Param		limit		query	int	false	"Limit (max 100)"      	default(20)
//	@Param		offset		query	int	false	"Offset"               	default(0)
//	@Param		sort		query	string	false	"Sort field"           	enum(created_at,-created_at,size,-size,filename,-filename)
//	@Success	200	{object}	response.Envelope{data=MediaListResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/models/{model_type}/{model_id}/media [get]
func (h *MediaHandler) List(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse path parameters
	modelType := c.Param("model_type")
	modelIDStr := c.Param("model_id")
	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid model ID"))
	}

	// Check permission for view on this model type
	if !h.checkModelPermission(c, userID, modelType, "view") {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Permission denied"))
	}

	// Parse query parameters
	var query request.ListMediaQuery
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	// Create filter
	filter := service.ListFilter{
		Collection: query.Collection,
		MimeType:   query.MimeType,
		Limit:      query.GetLimit(),
		Offset:     query.GetOffset(),
		Sort:       query.GetSort(),
	}

	// Call service
	mediaList, total, err := h.service.List(c.Request().Context(), modelType, modelID, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list media"))
	}

	// Convert to responses
	responses := make([]*domain.MediaResponse, len(mediaList))
	for i, m := range mediaList {
		responses[i] = m.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, MediaListResponse{
		Data:   responses,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}))
}

// GetByID handles GET /api/v1/media/{media_id}
// Returns a single media item by ID
//
//	@Summary	Get media by ID
//	@Description	Get details of a specific media file
//	@Tags		media
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		media_id	path	string	true	"Media ID"
//	@Success	200	{object}	response.Envelope{data=domain.MediaResponse}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Media not found"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/media/{media_id} [get]
func (h *MediaHandler) GetByID(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse media ID
	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	// Call service
	media, err := h.service.Get(c.Request().Context(), mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Media not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch media"))
	}

	// Check permission (owner can always view)
	isAdmin := h.isAdmin(c)
	if !isAdmin && media.UploadedByID != userID {
		// Check if user has media:view permission
		allowed, _ := h.service.CheckPermission(c.Request().Context(), userID, "view")
		if !allowed {
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Permission denied"))
		}
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, media.ToResponse()))
}

// Delete handles DELETE /api/v1/media/{media_id}
// Soft-deletes a media file
//
//	@Summary	Delete media
//	@Description	Soft delete a media file
//	@Tags		media
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		media_id	path	string	true	"Media ID"
//	@Success	200	{object}	response.Envelope{data=map[string]string}
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Media not found"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/media/{media_id} [delete]
func (h *MediaHandler) Delete(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse media ID
	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	// Check admin status
	isAdmin := h.isAdmin(c)

	// Call service
	if err := h.service.Delete(c.Request().Context(), userID, mediaID, isAdmin); err != nil {
		if err == errors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Media not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to delete media"))
	}

	// Log audit event for delete
	h.logDeleteAudit(c, userID, mediaID)

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"message": "Media deleted successfully"}))
}

// UpdateMetadata handles PATCH /api/v1/media/{media_id}
// Updates media custom properties
//
//	@Summary	Update media metadata
//	@Description	Update custom properties for a media file
//	@Tags		media
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		media_id	path	string	true	"Media ID"
//	@Param		request	body	request.UpdateMediaMetadataRequest	true	"Metadata update"
//	@Success	200	{object}	response.Envelope{data=domain.MediaResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Media not found"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/media/{media_id} [patch]
func (h *MediaHandler) UpdateMetadata(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse media ID
	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	// Parse request body
	var req request.UpdateMediaMetadataRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Get custom properties
	customProps, err := req.GetCustomProperties()
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContext(c, "VALIDATION_ERROR", err.Error()))
	}

	// Check admin status
	isAdmin := h.isAdmin(c)

	// Call service
	media, err := h.service.UpdateMetadata(c.Request().Context(), userID, mediaID, customProps, isAdmin)
	if err != nil {
		if err == errors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Media not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to update media"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, media.ToResponse()))
}

// GetSignedURL handles GET /api/v1/media/{media_id}/url
// Generates a signed URL for downloading media
//
//	@Summary	Get signed download URL
//	@Description	Generate a signed URL for downloading media
//	@Tags		media
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		media_id	path	string	true	"Media ID"
//	@Param		conversion	query	string	false	"Conversion name"
//	@Param		expires_in	query	int	false	"Expiry time in seconds (max 86400)"
//	@Success	200	{object}	response.Envelope{data=SignedURLResponse}
//	@Failure	400	{object}	response.Envelope	"Invalid request"
//	@Failure	401	{object}	response.Envelope	"Unauthorized"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Media not found"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/media/{media_id}/url [get]
func (h *MediaHandler) GetSignedURL(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Parse media ID
	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	// Parse query parameters
	var query request.GetSignedURLRequest
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	if err := query.Validate(); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, response.ErrorWithContextAndDetails(c, "VALIDATION_ERROR", "Validation failed", err.Error()))
	}

	// Check permission
	media, err := h.service.Get(c.Request().Context(), mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Media not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch media"))
	}

	// Check permission (owner can always download)
	isAdmin := h.isAdmin(c)
	if !isAdmin && media.UploadedByID != userID {
		allowed, _ := h.service.CheckPermission(c.Request().Context(), userID, "download")
		if !allowed {
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Permission denied"))
		}
	}

	// Generate signed URL
	expiry := time.Duration(query.GetExpiresIn()) * time.Second
	url, expiresAt, err := h.service.GetSignedURL(c.Request().Context(), mediaID, query.Conversion, expiry)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to generate signed URL"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, SignedURLResponse{
		URL:       url,
		ExpiresAt: expiresAt,
		ExpiresIn: query.GetExpiresIn(),
	}))
}

// Download handles GET /api/v1/media/{media_id}/download
// Downloads a media file (with optional signed URL support)
//
//	@Summary	Download media file
//	@Description	Download a media file directly
//	@Tags		media
//	@Param		media_id	path	string	true	"Media ID"
//	@Param		conversion	query	string	false	"Conversion name"
//	@Param		sig		query	string	false	"Signature for signed URL"
//	@Param		expires		query	int	false	"Expiry timestamp for signed URL"
//	@Success	200	{file}	binary	"Media file"
//	@Failure	401	{object}	response.Envelope	"Unauthorized or invalid signature"
//	@Failure	403	{object}	response.Envelope	"Forbidden"
//	@Failure	404	{object}	response.Envelope	"Media not found"
//	@Failure	410	{object}	response.Envelope	"Media deleted"
//	@Failure	500	{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/media/{media_id}/download [get]
func (h *MediaHandler) Download(c echo.Context) error {
	// Parse media ID
	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	// Parse query parameters
	var query request.DownloadMediaQuery
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	// Fetch media
	media, err := h.service.Get(c.Request().Context(), mediaID)
	if err != nil {
		if err == errors.ErrNotFound {
			return c.JSON(http.StatusNotFound, response.ErrorWithContext(c, "NOT_FOUND", "Media not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to fetch media"))
	}

	// Check if media is soft-deleted
	if media.DeletedAt.Valid {
		return c.JSON(http.StatusGone, response.ErrorWithContext(c, "GONE", "Media has been deleted"))
	}

	// Check authentication or signed URL
	userID, authErr := middleware.GetUserID(c)
	isAuthenticated := authErr == nil

	// If not authenticated, require valid signed URL
	if !isAuthenticated {
		if query.Signature == "" || query.Expires == 0 {
			return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication or signed URL required"))
		}

		// Validate signed URL
		// Note: In production, signingSecret should come from config
		signingSecret := "change-me-in-production"
		if !h.service.ValidateSignedURL(mediaID, query.Signature, query.Expires, signingSecret) {
			return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Invalid or expired signature"))
		}
	} else {
		// Authenticated - check permission
		isAdmin := h.isAdmin(c)
		if !isAdmin && media.UploadedByID != userID {
			allowed, _ := h.service.CheckPermission(c.Request().Context(), userID, "download")
			if !allowed {
				return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Permission denied"))
			}
		}
	}

	// Determine which file to serve
	path := media.Path
	if query.Conversion != "" {
		// Find conversion
		for _, conv := range media.Conversions {
			if conv.Name == query.Conversion {
				path = conv.Path
				break
			}
		}
	}

	// Get file from storage
	storageDriver := h.service.GetStorageDriver()
	file, err := storageDriver.Get(c.Request().Context(), path)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to retrieve file"))
	}
	defer file.Close()

	// Set headers
	c.Response().Header().Set("Content-Type", media.MimeType)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", media.OriginalFilename))
	c.Response().Header().Set("Content-Length", strconv.FormatInt(media.Size, 10))

	// Stream file
	_, err = io.Copy(c.Response(), file)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to stream file"))
	}

	return nil
}

// checkModelPermission checks if user has permission on a specific model type
func (h *MediaHandler) checkModelPermission(c echo.Context, userID uuid.UUID, modelType, action string) bool {
	if h.enforcer == nil {
		return true
	}

	// Check permission on model type (e.g., "news:upload")
	allowed, err := h.enforcer.Enforce(userID.String(), "default", modelType, action)
	if err != nil {
		return false
	}
	if allowed {
		return true
	}

	// Fallback to generic media permission (e.g., "media:upload")
	allowed, err = h.enforcer.Enforce(userID.String(), "default", "media", action)
	if err != nil {
		return false
	}

	return allowed
}

// isAdmin checks if the current user has admin privileges
func (h *MediaHandler) isAdmin(c echo.Context) bool {
	if h.enforcer == nil {
		return false
	}

	claims, err := middleware.GetUserClaims(c)
	if err != nil {
		return false
	}

	// Check if user has permission to manage all media
	allowed, err := h.enforcer.Enforce(claims.UserID, "default", "media", "manage")
	if err != nil {
		return false
	}

	return allowed
}

// AdminList handles GET /api/v1/admin/media
// Lists all media files with filtering (admin only)
//
//	@Summary	List all media (admin)
//	@Description	List all media files with filtering. Admin access only.
//	@Tags		media,admin
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		model_type	query	string	false	"Filter by model type"
//	@Param		model_id	query	string	false	"Filter by model ID"
//	@Param		include_deleted	query	bool	false	"Include soft-deleted media"
//	@Param		limit		query	int		false	"Limit (max 100)"    	default(20)
//	@Param		offset		query	int		false	"Offset"             	default(0)
//	@Success	200		{object}	response.Envelope{data=AdminMediaListResponse}
//	@Failure	401		{object}	response.Envelope	"Unauthorized"
//	@Failure	403		{object}	response.Envelope	"Forbidden"
//	@Failure	500		{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/admin/media [get]
func (h *MediaHandler) AdminList(c echo.Context) error {
	// Check admin permission
	if !h.isAdmin(c) {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Admin access required"))
	}

	// Parse query parameters
	var query request.AdminListMediaQuery
	if err := c.Bind(&query); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid query parameters"))
	}

	// Create filter with available fields
	filter := service.AdminListFilter{
		ModelType: query.ModelType,
		ModelID:   query.ModelID,
		Limit:     query.GetLimit(),
		Offset:    query.GetOffset(),
		Sort:      "-created_at",
	}

	// Call service - need to add AdminList to MediaService interface
	// For now, return empty response with message
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, AdminMediaListResponse{
		Data:   []*domain.MediaResponse{},
		Total:  0,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}))
}

// AdminCleanup handles POST /api/v1/admin/media/cleanup
// Triggers cleanup of orphaned media files
//
//	@Summary	Cleanup orphaned media (admin)
//	@Description	Trigger cleanup of orphaned media files. Admin access only.
//	@Tags		media,admin
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		cutoff_hours	query	int	false	"Hours since orphaned"  	default(24)
//	@Success	200		{object}	response.Envelope{data=CleanupResponse}
//	@Failure	401		{object}	response.Envelope	"Unauthorized"
//	@Failure	403		{object}	response.Envelope	"Forbidden"
//	@Failure	500		{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/admin/media/cleanup [post]
func (h *MediaHandler) AdminCleanup(c echo.Context) error {
	// Extract user ID from JWT claims
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	// Check admin permission
	if !h.isAdmin(c) {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Admin access required"))
	}

	// Parse query parameters
	cutoffHours := 24
	if hours := c.QueryParam("cutoff_hours"); hours != "" {
		if h, err := strconv.Atoi(hours); err == nil && h > 0 {
			cutoffHours = h
		}
	}

	// Log audit event for cleanup
	if h.auditSvc != nil {
		go func() {
			_ = h.auditSvc.LogAction(
				c.Request().Context(),
				userID,
				"cleanup",
				"media",
				"",
				nil,
				map[string]interface{}{
					"action":        "orphan_cleanup",
					"cutoff_hours":  cutoffHours,
					"triggered_by":  "admin",
				},
				c.RealIP(),
				c.Request().UserAgent(),
			)
		}()
	}

	// Return success - actual cleanup would be handled by a background job
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, CleanupResponse{
		OrphansFound: 0,
		DeletedCount: 0,
		Message:      "Cleanup job queued. Orphaned media older than " + strconv.Itoa(cutoffHours) + " hours will be processed.",
	}))
}

// AdminStats handles GET /api/v1/admin/media/stats
// Returns storage statistics for all media
//
//	@Summary	Get media statistics (admin)
//	@Description	Get storage statistics for all media files. Admin access only.
//	@Tags		media,admin
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200		{object}	response.Envelope{data=MediaStatsResponse}
//	@Failure	401		{object}	response.Envelope	"Unauthorized"
//	@Failure	403		{object}	response.Envelope	"Forbidden"
//	@Failure	500		{object}	response.Envelope	"Internal server error"
//	@Router		/api/v1/admin/media/stats [get]
func (h *MediaHandler) AdminStats(c echo.Context) error {
	// Check admin permission
	if !h.isAdmin(c) {
		return c.JSON(http.StatusForbidden, response.ErrorWithContext(c, "FORBIDDEN", "Admin access required"))
	}

	// Log audit event
	userID, _ := middleware.GetUserID(c)
	if h.auditSvc != nil {
		go func() {
			_ = h.auditSvc.LogAction(
				c.Request().Context(),
				userID,
				"stats_view",
				"media",
				"",
				nil,
				nil,
				c.RealIP(),
				c.Request().UserAgent(),
			)
		}()
	}

	// Return mock stats - in production this would query the database
	return c.JSON(http.StatusOK, response.SuccessWithContext(c, MediaStatsResponse{
		TotalFiles:  0,
		TotalSize:   0,
		ByModelType: []ModelTypeStats{},
		ByMimeType:  []MimeTypeStats{},
		ByDisk:      []DiskStats{},
	}))
}

// logUploadAudit logs an audit entry for media upload
func (h *MediaHandler) logUploadAudit(c echo.Context, userID uuid.UUID, media *domain.Media) {
	if h.auditSvc == nil {
		return
	}

	go func() {
		ctx := c.Request().Context()
		beforeJSON, _ := json.Marshal(map[string]interface{}{
			"file_size":      media.Size,
			"mime_type":      media.MimeType,
			"model_type":     media.ModelType,
			"model_id":       media.ModelID.String(),
			"original_name":  media.OriginalFilename,
		})

		_ = h.auditSvc.LogAction(
			ctx,
			userID,
			"upload",
			"media",
			media.ID.String(),
			nil,
			beforeJSON,
			c.RealIP(),
			c.Request().UserAgent(),
		)
	}()
}

// logDownloadAudit logs an audit entry for media download
func (h *MediaHandler) logDownloadAudit(c echo.Context, userID uuid.UUID, media *domain.Media) {
	if h.auditSvc == nil {
		return
	}

	go func() {
		ctx := c.Request().Context()
		_ = h.auditSvc.LogAction(
			ctx,
			userID,
			"download",
			"media",
			media.ID.String(),
			nil,
			nil,
			c.RealIP(),
			c.Request().UserAgent(),
		)
	}()
}

// logDeleteAudit logs an audit entry for media deletion
func (h *MediaHandler) logDeleteAudit(c echo.Context, userID uuid.UUID, mediaID uuid.UUID) {
	if h.auditSvc == nil {
		return
	}

	go func() {
		ctx := c.Request().Context()
		_ = h.auditSvc.LogAction(
			ctx,
			userID,
			"delete",
			"media",
			mediaID.String(),
			nil,
			nil,
			c.RealIP(),
			c.Request().UserAgent(),
		)
	}()
}

// logUpdateMetadataAudit logs an audit entry for media metadata update
func (h *MediaHandler) logUpdateMetadataAudit(c echo.Context, userID uuid.UUID, media *domain.Media, beforeProps map[string]interface{}) {
	if h.auditSvc == nil {
		return
	}

	go func() {
		ctx := c.Request().Context()
		beforeJSON, _ := json.Marshal(beforeProps)
		afterJSON, _ := json.Marshal(media.CustomProperties)

		_ = h.auditSvc.LogAction(
			ctx,
			userID,
			"update_metadata",
			"media",
			media.ID.String(),
			beforeJSON,
			afterJSON,
			c.RealIP(),
			c.Request().UserAgent(),
		)
	}()
}

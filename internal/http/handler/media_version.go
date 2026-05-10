package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// MediaVersionHandler handles media version HTTP endpoints.
type MediaVersionHandler struct {
	versionService service.VersionService
	mediaService   service.MediaService
	enforcer       *permission.Enforcer
}

// NewMediaVersionHandler creates a new MediaVersionHandler instance.
func NewMediaVersionHandler(
	versionService service.VersionService,
	mediaService service.MediaService,
	enforcer *permission.Enforcer,
) *MediaVersionHandler {
	return &MediaVersionHandler{
		versionService: versionService,
		mediaService:   mediaService,
		enforcer:       enforcer,
	}
}

// RegisterRoutes registers media version routes with the Echo group.
func (h *MediaVersionHandler) RegisterRoutes(g *echo.Group, jwtSecret string) {
	versions := g.Group("/media/:media_id/versions")
	versions.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     jwtSecret,
		ContextKey: "user",
	}))

	// View routes (media_version:view)
	if h.enforcer != nil {
		viewGroup := versions.Group("")
		viewGroup.Use(middleware.RequirePermission(h.enforcer, "media_version", "view"))
		viewGroup.GET("", h.ListVersions)
		viewGroup.GET("/:version", h.GetVersion)
	} else {
		versions.GET("", h.ListVersions)
		versions.GET("/:version", h.GetVersion)
	}

	// Upload route (media_version:upload)
	if h.enforcer != nil {
		versions.POST("", h.Upload, middleware.RequirePermission(h.enforcer, "media_version", "upload"))
	} else {
		versions.POST("", h.Upload)
	}

	// Download routes (media_version:download)
	if h.enforcer != nil {
		downloadGroup := versions.Group("")
		downloadGroup.Use(middleware.RequirePermission(h.enforcer, "media_version", "download"))
		downloadGroup.GET("/:version/download", h.DownloadVersion)
		downloadGroup.GET("/:version/url", h.GetVersionSignedURL)
	} else {
		versions.GET("/:version/download", h.DownloadVersion)
		versions.GET("/:version/url", h.GetVersionSignedURL)
	}

	// Restore route (media_version:restore)
	if h.enforcer != nil {
		versions.POST("/:version/restore", h.RestoreVersion, middleware.RequirePermission(h.enforcer, "media_version", "restore"))
	} else {
		versions.POST("/:version/restore", h.RestoreVersion)
	}

	// Delete route (media_version:delete)
	if h.enforcer != nil {
		versions.DELETE("/:version", h.DeleteVersion, middleware.RequirePermission(h.enforcer, "media_version", "delete"))
	} else {
		versions.DELETE("/:version", h.DeleteVersion)
	}
}

// Upload handles POST /api/v1/media/:media_id/versions
func (h *MediaVersionHandler) Upload(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	if err := c.Request().ParseMultipartForm(service.MaxFileSize); err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Failed to parse multipart form"))
	}

	file, fileHeader, err := c.Request().FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "File is required"))
	}
	defer file.Close()

	ctx := c.Request().Context()
	ipAddress := c.RealIP()
	userAgent := c.Request().UserAgent()

	version, svcErr := h.versionService.UploadVersion(
		ctx,
		userID,
		mediaID,
		file,
		fileHeader.Filename,
		fileHeader.Size,
		fileHeader.Header.Get("Content-Type"),
		ipAddress,
		userAgent,
	)
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusCreated, response.SuccessWithContext(c, version.ToResponse(version.Version)))
}

// ListVersions handles GET /api/v1/media/:media_id/versions
func (h *MediaVersionHandler) ListVersions(c echo.Context) error {
	_, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	limit := 20
	offset := 0
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
		if limit > 100 {
			limit = 100
		}
	}
	if o, err := strconv.Atoi(c.QueryParam("offset")); err == nil && o >= 0 {
		offset = o
	}

	ctx := c.Request().Context()
	result, svcErr := h.versionService.ListVersions(ctx, mediaID, limit, offset)
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// GetVersion handles GET /api/v1/media/:media_id/versions/:version
func (h *MediaVersionHandler) GetVersion(c echo.Context) error {
	_, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil || versionNum < 1 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid version number"))
	}

	ctx := c.Request().Context()
	result, svcErr := h.versionService.GetVersion(ctx, mediaID, versionNum)
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, result))
}

// DownloadVersion handles GET /api/v1/media/:media_id/versions/:version/download
func (h *MediaVersionHandler) DownloadVersion(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil || versionNum < 1 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid version number"))
	}

	ctx := c.Request().Context()
	file, version, svcErr := h.versionService.DownloadVersion(
		ctx,
		userID,
		mediaID,
		versionNum,
		c.RealIP(),
		c.Request().UserAgent(),
	)
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}
	defer file.Close()

	c.Response().Header().Set("Content-Type", version.MimeType)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", version.OriginalFilename))
	c.Response().Header().Set("Content-Length", strconv.FormatInt(version.Size, 10))
	_, err = io.Copy(c.Response(), file)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to stream file"))
	}

	return nil
}

// GetVersionSignedURL handles GET /api/v1/media/:media_id/versions/:version/url
func (h *MediaVersionHandler) GetVersionSignedURL(c echo.Context) error {
	_, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil || versionNum < 1 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid version number"))
	}

	expiresIn := 3600
	if expStr := c.QueryParam("expires_in"); expStr != "" {
		if ei, err := strconv.Atoi(expStr); err == nil && ei > 0 {
			expiresIn = ei
		}
	}

	ctx := c.Request().Context()
	url, expiresAt, svcErr := h.versionService.GetVersionSignedURL(ctx, mediaID, versionNum, expiresIn)
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, SignedURLResponse{
		URL:       url,
		ExpiresAt: expiresAt,
		ExpiresIn: expiresIn,
	}))
}

func (h *MediaVersionHandler) mapServiceError(c echo.Context, err error) error {
	appErr := errors.GetAppError(err)
	if appErr != nil {
		return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
	}
	return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Internal server error"))
}

// RestoreVersion handles POST /api/v1/media/:media_id/versions/:version/restore
func (h *MediaVersionHandler) RestoreVersion(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil || versionNum < 1 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid version number"))
	}

	ctx := c.Request().Context()
	media, svcErr := h.versionService.RestoreVersion(ctx, userID, mediaID, versionNum, c.RealIP(), c.Request().UserAgent())
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, media.ToResponse()))
}

// DeleteVersion handles DELETE /api/v1/media/:media_id/versions/:version
func (h *MediaVersionHandler) DeleteVersion(c echo.Context) error {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	mediaIDStr := c.Param("media_id")
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid media ID"))
	}

	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil || versionNum < 1 {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid version number"))
	}

	ctx := c.Request().Context()
	svcErr := h.versionService.DeleteVersion(ctx, userID, mediaID, versionNum, c.RealIP(), c.Request().UserAgent())
	if svcErr != nil {
		return h.mapServiceError(c, svcErr)
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"message":  "Version deleted successfully",
		"media_id": mediaID,
		"version":  versionNum,
	}))
}

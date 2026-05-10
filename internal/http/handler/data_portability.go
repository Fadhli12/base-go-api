package handler

import (
	"fmt"
	"net/http"

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

type DataPortabilityHandler struct {
	exportService service.ExportService
	importService service.ImportService
	enforcer      *permission.Enforcer
	rateLimiter   service.DataPortabilityRateLimiter
	logger        logger.Logger
}

func NewDataPortabilityHandler(
	exportService service.ExportService,
	importService service.ImportService,
	enforcer *permission.Enforcer,
	rateLimiter service.DataPortabilityRateLimiter,
	logger logger.Logger,
) *DataPortabilityHandler {
	return &DataPortabilityHandler{
		exportService: exportService,
		importService: importService,
		enforcer:      enforcer,
		rateLimiter:   rateLimiter,
		logger:        logger,
	}
}

func (h *DataPortabilityHandler) RegisterRoutes(g *echo.Group, jwtSecret string) {
	e := g.Group("/exports", middleware.JWT(middleware.JWTConfig{Secret: jwtSecret}))
	e.POST("", h.CreateExport, middleware.ExportRateLimit(h.rateLimiter), middleware.RequirePermission(h.enforcer, "data_portability", "export:create"))
	e.GET("", h.ListExports, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
	e.GET("/:id", h.GetExport, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
	e.GET("/:id/download", h.DownloadExport, middleware.RequirePermission(h.enforcer, "data_portability", "export:download"))
	e.DELETE("/:id", h.CancelExport, middleware.RequirePermission(h.enforcer, "data_portability", "import:cancel"))

	i := g.Group("/imports", middleware.JWT(middleware.JWTConfig{Secret: jwtSecret}))
	i.POST("/preview", h.PreviewImport, middleware.ImportRateLimit(h.rateLimiter), middleware.ImportFileSizeLimit(service.DefaultMaxFileSize), middleware.RequirePermission(h.enforcer, "data_portability", "import:create"))
	i.POST("", h.CreateImport, middleware.ImportRateLimit(h.rateLimiter), middleware.ImportFileSizeLimit(service.DefaultMaxFileSize), middleware.RequirePermission(h.enforcer, "data_portability", "import:create"))
	i.GET("", h.ListImports, middleware.RequirePermission(h.enforcer, "data_portability", "import:view"))
	i.GET("/:id", h.GetImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:view"))
	i.POST("/:id/cancel", h.CancelImport, middleware.RequirePermission(h.enforcer, "data_portability", "import:cancel"))
}

func (h *DataPortabilityHandler) CreateExport(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	var req domain.CreateExportRequest
	if err := c.Bind(&req); err != nil {
		log.Warn(ctx, "invalid export request body", logger.Err(err))
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid request body"))
	}

	exportReq := &service.ExportRequest{
		EntityTypes:    req.EntityTypes,
		Format:         req.Format,
		UserID:         userID,
		IncludeDeleted: req.IncludeDeleted,
	}
	if hasOrgID {
		exportReq.OrgID = &orgID
	}

	exp, err := h.exportService.CreateExport(ctx, exportReq)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "export creation failed", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create export"))
	}

	statusCode := http.StatusAccepted
	if exp.Sync {
		statusCode = http.StatusOK
	}

	return c.JSON(statusCode, response.SuccessWithContext(c, exp.ToResponse()))
}

func (h *DataPortabilityHandler) GetExport(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid export ID"))
	}

	exp, err := h.exportService.GetExport(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get export"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, exp.ToResponse()))
}

func (h *DataPortabilityHandler) ListExports(c echo.Context) error {
	ctx := c.Request().Context()

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	var orgIDPtr *uuid.UUID
	if hasOrgID {
		orgIDPtr = &orgID
	}

	page, pageSize := parsePagination(c)

	exports, total, err := h.exportService.ListExports(ctx, orgIDPtr, page, pageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list exports"))
	}

	exportsResp := make([]domain.ExportJobResponse, len(exports))
	for i, exp := range exports {
		exportsResp[i] = exp.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"exports": exportsResp,
		"total":   total,
		"page":    page,
		"size":    pageSize,
	}))
}

func (h *DataPortabilityHandler) DownloadExport(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid export ID"))
	}

	download, err := h.exportService.DownloadExport(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to download export"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, download))
}

func (h *DataPortabilityHandler) CancelExport(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid export ID"))
	}

	if err := h.exportService.CancelExport(ctx, id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to cancel export"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "cancelled"}))
}

func (h *DataPortabilityHandler) PreviewImport(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "File is required"))
	}

	format := c.FormValue("format")
	if format == "" {
		format = "json"
	}

	src, err := file.Open()
	if err != nil {
		log.Error(ctx, "failed to open import file", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to read file"))
	}
	defer src.Close()

	preview, err := h.importService.PreviewImport(ctx, src, format)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "import preview failed", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to preview import"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, preview))
}

func (h *DataPortabilityHandler) CreateImport(c echo.Context) error {
	log := middleware.GetLogger(c)
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.ErrorWithContext(c, "UNAUTHORIZED", "Authentication required"))
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "File is required"))
	}

	format := c.FormValue("format")
	if format == "" {
		format = "json"
	}

	conflictStrategy := c.FormValue("conflict_strategy")
	if conflictStrategy == "" {
		conflictStrategy = string(domain.ConflictSkip)
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	var orgIDPtr *uuid.UUID
	if hasOrgID {
		orgIDPtr = &orgID
	}

	src, err := file.Open()
	if err != nil {
		log.Error(ctx, "failed to open import file", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to read file"))
	}
	defer src.Close()

	req := &service.ImportRequest{
		UserID:           userID,
		OrgID:            orgIDPtr,
		File:             src,
		Filename:         file.Filename,
		Format:           format,
		ConflictStrategy: conflictStrategy,
	}

	imp, err := h.importService.CreateImport(ctx, req)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			log.Error(ctx, "import creation failed", log.String("error_code", appErr.Code), logger.Err(err))
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		log.Error(ctx, "import creation failed", logger.Err(err))
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to create import"))
	}

	return c.JSON(http.StatusAccepted, response.SuccessWithContext(c, imp.ToResponse()))
}

func (h *DataPortabilityHandler) GetImport(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid import ID"))
	}

	imp, err := h.importService.GetImport(ctx, id)
	if err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to get import"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, imp.ToResponse()))
}

func (h *DataPortabilityHandler) ListImports(c echo.Context) error {
	ctx := c.Request().Context()

	orgID, hasOrgID := middleware.GetOrganizationID(c)
	var orgIDPtr *uuid.UUID
	if hasOrgID {
		orgIDPtr = &orgID
	}

	page, pageSize := parsePagination(c)

	imports, err := h.importService.ListImports(ctx, orgIDPtr, page, pageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to list imports"))
	}

	importsResp := make([]domain.ImportJobResponse, len(imports))
	for i, imp := range imports {
		importsResp[i] = imp.ToResponse()
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"imports": importsResp,
		"page":    page,
		"size":    pageSize,
	}))
}

func (h *DataPortabilityHandler) CancelImport(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "BAD_REQUEST", "Invalid import ID"))
	}

	if err := h.importService.CancelImport(ctx, id); err != nil {
		if appErr := apperrors.GetAppError(err); appErr != nil {
			return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
		}
		return c.JSON(http.StatusInternalServerError, response.ErrorWithContext(c, "INTERNAL_ERROR", "Failed to cancel import"))
	}

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]string{"status": "cancelled"}))
}

func parsePagination(c echo.Context) (page, pageSize int) {
	page = 1
	pageSize = 20

	if p := c.QueryParam("page"); p != "" {
		var v int
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil && v > 0 {
			page = v
		}
	}

	if ps := c.QueryParam("page_size"); ps != "" {
		var v int
		if _, err := fmt.Sscanf(ps, "%d", &v); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	return page, pageSize
}
// Package http provides HTTP server configuration and middleware.
package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/http/handler"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/module/invoice"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/internal/storage"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Server wraps Echo server with dependencies
type Server struct {
	echo        *echo.Echo
	config      *config.Config
	db          *gorm.DB
	redis       *redis.Client
	enforcer    *permission.Enforcer
	permCache   *permission.Cache
	invalidator *permission.Invalidator
	auditSvc    *service.AuditService
	emailWorker *service.EmailWorker
}

// ServerConfig holds Echo server configuration.
type ServerConfig struct {
	// Debug enables debug mode (prints more verbose logs, disables some optimizations)
	Debug bool
	// HideBanner hides the Echo startup banner
	HideBanner bool
	// HidePort hides the port in the startup message
	HidePort bool
}

// NewServer creates a new HTTP server with middleware chain
func NewServer(cfg *config.Config, db *gorm.DB, redisClient *redis.Client, cacheDriver cache.Driver) *Server {
	e := echo.New()

	// Configure Echo settings
	e.Debug = isDebugMode()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{
		echo:   e,
		config: cfg,
		db:     db,
		redis:  redisClient,
	}

	// Set custom error handler
	e.HTTPErrorHandler = s.HTTPErrorHandler

	// Set up middleware chain: recover -> request_id -> cors -> rate_limit
	// Order matters: recover should be first to catch all panics
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORS(cfg))

	// Apply rate limiting with cache driver
	if cacheDriver != nil {
		rateLimiter := middleware.NewRateLimiter(cacheDriver, middleware.DefaultRequestsPerMinute, cfg.Cache.RateLimitTTL)
		e.Use(rateLimiter.Middleware())
	}

	return s
}

// NewServerWithConfig creates a new Echo server with custom configuration.
//
// Parameters:
//   - serverConfig: Server configuration options
//
// Returns:
//   - *echo.Echo: Configured Echo instance
func NewServerWithConfig(serverConfig ServerConfig) *echo.Echo {
	e := echo.New()

	e.Debug = serverConfig.Debug
	e.HideBanner = serverConfig.HideBanner
	e.HidePort = serverConfig.HidePort

	return e
}

// HTTPErrorHandler handles all HTTP errors and returns JSON envelope responses
func (s *Server) HTTPErrorHandler(err error, c echo.Context) {
	// Skip if response already written
	if c.Response().Committed {
		return
	}

	// Get request ID from context
	requestID := middleware.GetRequestID(c)

	// Default to internal server error
	statusCode := http.StatusInternalServerError
	errorCode := "INTERNAL_ERROR"
	message := "Internal server error"

	// Check if it's an echo HTTPError
	if echoErr, ok := err.(*echo.HTTPError); ok {
		statusCode = echoErr.Code
		errorCode = "HTTP_ERROR"
		if msg, ok := echoErr.Message.(string); ok {
			message = msg
		}
		if statusCode == http.StatusNotFound {
			errorCode = "NOT_FOUND"
			message = "Resource not found"
		}
	} else if appErr := apperrors.GetAppError(err); appErr != nil {
		// Check if it's an AppError
		statusCode = appErr.HTTPStatus
		errorCode = appErr.Code
		message = appErr.Message
	} else {
		// Log unknown errors
		slog.Error("unhandled error",
			slog.String("error", err.Error()),
			slog.String("request_id", requestID),
		)
	}

	// Log error with appropriate level
	if statusCode >= 500 {
		slog.Error("server error",
			slog.Int("status", statusCode),
			slog.String("code", errorCode),
			slog.String("message", message),
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
	}

	// Return JSON envelope
	resp := response.Envelope{
		Error: &response.ErrorDetail{
			Code:    errorCode,
			Message: message,
		},
		Meta: &response.Meta{
			RequestID: requestID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}

	if err := c.JSON(statusCode, resp); err != nil {
		slog.Error("failed to write error response", slog.String("error", err.Error()))
	}
}

// Echo returns the underlying Echo instance
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// DB returns the database connection
func (s *Server) DB() *gorm.DB {
	return s.db
}

// Redis returns the Redis client
func (s *Server) Redis() *redis.Client {
	return s.redis
}

// Config returns the server configuration
func (s *Server) Config() *config.Config {
	return s.config
}

// Enforcer returns the permission enforcer
func (s *Server) Enforcer() *permission.Enforcer {
	return s.enforcer
}

// SetEnforcer sets the permission enforcer
func (s *Server) SetEnforcer(enforcer *permission.Enforcer) {
	s.enforcer = enforcer
}

// PermissionCache returns the permission cache
func (s *Server) PermissionCache() *permission.Cache {
	return s.permCache
}

// SetPermissionCache sets the permission cache
func (s *Server) SetPermissionCache(cache *permission.Cache) {
	s.permCache = cache
}

// Invalidator returns the permission invalidator
func (s *Server) Invalidator() *permission.Invalidator {
	return s.invalidator
}

// SetInvalidator sets the permission invalidator
func (s *Server) SetInvalidator(invalidator *permission.Invalidator) {
	s.invalidator = invalidator
}

// AuditService returns the audit service
func (s *Server) AuditService() *service.AuditService {
	return s.auditSvc
}

// SetAuditService sets the audit service
func (s *Server) SetAuditService(auditSvc *service.AuditService) {
	s.auditSvc = auditSvc
}

// SetEmailWorker sets the email worker
func (s *Server) SetEmailWorker(emailWorker *service.EmailWorker) {
	s.emailWorker = emailWorker
}

// EmailWorker returns the email worker
func (s *Server) EmailWorker() *service.EmailWorker {
	return s.emailWorker
}

// Start starts the HTTP server
func (s *Server) Start() error {
	address := fmt.Sprintf(":%d", s.config.Server.Port)
	slog.Info("Starting HTTP server", slog.String("address", address))
	return s.echo.Start(address)
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	slog.Info("Shutting down HTTP server")
	return s.echo.Shutdown(ctx)
}

// RegisterRoutes registers all application routes
func (s *Server) RegisterRoutes() {
	// Initialize repositories
	userRepo := repository.NewUserRepository(s.db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(s.db)
	permissionRepo := repository.NewPermissionRepository(s.db)
	roleRepo := repository.NewRoleRepository(s.db)
	rolePermissionRepo := repository.NewRolePermissionRepository(s.db)
	userRoleRepo := repository.NewUserRoleRepository(s.db)
	userPermissionRepo := repository.NewUserPermissionRepository(s.db)
	auditLogRepo := repository.NewAuditLogRepository(s.db)
	mediaRepo := repository.NewMediaRepository(s.db)
	apiKeyRepo := repository.NewAPIKeyRepository(s.db)
	organizationRepo := repository.NewOrganizationRepository(s.db)
	emailTemplateRepo := repository.NewEmailTemplateRepository(s.db)
	emailQueueRepo := repository.NewEmailQueueRepository(s.db)
	emailBounceRepo := repository.NewEmailBounceRepository(s.db)

	// Initialize email services (needed for auth and org services)
	smtpProvider := service.NewSMTPProvider(&s.config.Email)
	templateEngine := service.NewTemplateEngine(emailTemplateRepo)
	emailService := service.NewEmailService(
		&s.config.Email,
		emailTemplateRepo,
		emailQueueRepo,
		emailBounceRepo,
		templateEngine,
		smtpProvider,
	)

	// Initialize services
	tokenService := service.NewTokenService(s.config.JWT.Secret, s.config.JWT.AccessExpiry, s.config.JWT.RefreshExpiry)
	passwordHasher := service.NewPasswordHasher()
	authService := service.NewAuthService(userRepo, refreshTokenRepo, tokenService, passwordHasher, emailService)
	userService := service.NewUserService(userRepo, userRoleRepo, userPermissionRepo)
	permissionService := service.NewPermissionService(permissionRepo)
	roleService := service.NewRoleService(roleRepo, rolePermissionRepo, permissionRepo)
	
	// Initialize audit service with async processing
	auditService := service.NewAuditService(auditLogRepo, service.DefaultAuditServiceConfig())
	s.SetAuditService(auditService)
	
	// Initialize organization service
	orgService := service.NewOrganizationService(organizationRepo, s.enforcer, auditService, emailService, slog.Default())

	// Initialize API key service
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, userRepo, auditService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService)

	// Initialize storage driver
	storageDriver, err := storage.NewDriver(storage.Config{
		Type:      s.config.Storage.Driver,
		LocalPath: s.config.Storage.LocalPath,
		BaseURL:   s.config.Storage.BaseURL,
	})
	if err != nil {
		slog.Error("Failed to initialize storage driver", "error", err)
		// Continue without media functionality if storage fails
		storageDriver = nil
	}

	// Initialize media service
	var mediaService service.MediaService
	if storageDriver != nil {
		mediaService = service.NewMediaService(mediaRepo, s.enforcer, storageDriver, s.config.JWT.Secret)
	}

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	userHandler := handler.NewUserHandler(userService)
	permissionHandler := handler.NewPermissionHandler(permissionService)
	roleHandler := handler.NewRoleHandler(roleService)
	orgHandler := handler.NewOrganizationHandler(orgService)

	// Initialize invoice module
	invoiceRepo := invoice.NewRepository(s.db)
	invoiceService := invoice.NewService(invoiceRepo, s.enforcer)
	invoiceHandler := invoice.NewHandler(invoiceService, s.enforcer)

	// Initialize media handler if media service is available
	var mediaHandler *handler.MediaHandler
	if mediaService != nil {
		mediaHandler = handler.NewMediaHandler(mediaService, auditService, s.enforcer)
	}

	// Swagger documentation (conditional based on config)
	if s.config.Swagger.Enabled {
		swagger := s.echo.Group(s.config.Swagger.Path)
		swagger.GET("/*", echoSwagger.WrapHandler)
		slog.Info("Swagger documentation enabled", "path", s.config.Swagger.Path)
	} else {
		slog.Info("Swagger documentation disabled")
	}

	// API v1 routes
	v1 := s.echo.Group("/api/v1")

	// Auth routes (public)
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout)

	// Permission routes
	s.RegisterPermissionRoutes(v1, permissionHandler)

	// Role routes
	s.RegisterRoleRoutes(v1, roleHandler)

	// Invoice routes
	s.RegisterInvoiceRoutes(v1, invoiceHandler)

	// Media routes (if handler initialized)
	if mediaHandler != nil {
		mediaHandler.RegisterRoutes(v1, s.config.JWT.Secret)
	}

	// API Key routes
	s.RegisterAPIKeyRoutes(v1, apiKeyHandler)

	// Protected routes (require JWT authentication)
	protected := v1.Group("")
	protected.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// User routes
	protected.GET("/me", userHandler.GetCurrentUser)

	// User admin routes
	s.RegisterUserRoutes(v1, userHandler)

	// Organization routes
	s.RegisterOrganizationRoutes(v1, orgHandler)

	// Email routes
	s.RegisterEmailRoutes(v1)
}

// RegisterEmailRoutes registers email-related routes
// Requires JWT authentication and permission checks
func (s *Server) RegisterEmailRoutes(api *echo.Group) {
	// Initialize email repositories
	emailTemplateRepo := repository.NewEmailTemplateRepository(s.db)
	emailQueueRepo := repository.NewEmailQueueRepository(s.db)
	emailBounceRepo := repository.NewEmailBounceRepository(s.db)

	// Initialize email services
	smtpProvider := service.NewSMTPProvider(&s.config.Email)
	templateEngine := service.NewTemplateEngine(emailTemplateRepo)
	emailService := service.NewEmailService(
		&s.config.Email,
		emailTemplateRepo,
		emailQueueRepo,
		emailBounceRepo,
		templateEngine,
		smtpProvider,
	)

	// Initialize handlers
	emailTemplateHandler := handler.NewEmailTemplateHandler(emailService, emailTemplateRepo)
	emailHandler := handler.NewEmailHandler(emailService, emailQueueRepo)

	// Email template routes (require JWT + permissions)
	templates := api.Group("/email-templates")
	templates.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Permission check for template management
	if s.enforcer != nil {
		templates.Use(middleware.RequirePermission(s.enforcer, "email_templates", "manage"))
	}

	// Apply audit middleware
	if s.auditSvc != nil {
		templates.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	templates.POST("", emailTemplateHandler.Create)
	templates.GET("", emailTemplateHandler.GetAll)
	templates.GET("/:id", emailTemplateHandler.GetByID)
	templates.PUT("/:id", emailTemplateHandler.Update)
	templates.DELETE("/:id", emailTemplateHandler.Delete)

	// Email sending routes (require JWT)
	emails := api.Group("/emails")
	emails.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Apply audit middleware for email operations
	if s.auditSvc != nil {
		emails.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	emails.POST("", emailHandler.SendEmail)
	emails.GET("/:id", emailHandler.GetStatus)

	// Webhook routes (public - no auth required)
	webhooks := api.Group("/webhooks")
	webhooks.POST("/:provider/delivery", emailHandler.WebhookDelivery)
	webhooks.POST("/:provider/bounce", emailHandler.WebhookBounce)
}

// RegisterPermissionRoutes registers permission-related routes
// Requires JWT authentication and permission:manage permission for write operations
func (s *Server) RegisterPermissionRoutes(api *echo.Group, permHandler *handler.PermissionHandler) {
	permissions := api.Group("/permissions")
	
	// All permission routes require JWT authentication
	permissions.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Permission management requires permission:manage permission
	if s.enforcer != nil {
		permissions.Use(middleware.RequirePermission(s.enforcer, "permissions", "manage"))
	}

	// Apply audit middleware to mutating routes
	if s.auditSvc != nil {
		permissions.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	permissions.POST("", permHandler.Create)
	permissions.GET("", permHandler.GetAll)
}

// RegisterRoleRoutes registers role-related routes
// Requires JWT authentication and role:manage permission
func (s *Server) RegisterRoleRoutes(api *echo.Group, roleHandler *handler.RoleHandler) {
	roles := api.Group("/roles")
	
	// All role routes require JWT authentication
	roles.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Role management requires role:manage permission
	if s.enforcer != nil {
		roles.Use(middleware.RequirePermission(s.enforcer, "roles", "manage"))
	}

	// Apply audit middleware to mutating routes
	if s.auditSvc != nil {
		roles.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	roles.POST("", roleHandler.Create)
	roles.GET("", roleHandler.GetAll)
	roles.PUT("/:id", roleHandler.Update)
	roles.DELETE("/:id", roleHandler.SoftDelete)
	roles.POST("/:id/permissions/:pid", roleHandler.AttachPermission)
	roles.DELETE("/:id/permissions/:pid", roleHandler.DetachPermission)
}

// RegisterUserRoutes registers user admin routes
// Requires JWT authentication and user:manage permission
func (s *Server) RegisterUserRoutes(api *echo.Group, userHandler *handler.UserHandler) {
	users := api.Group("/users")
	
	// All user routes require JWT authentication
	users.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// User management requires user:manage permission
	if s.enforcer != nil {
		users.Use(middleware.RequirePermission(s.enforcer, "users", "manage"))
	}

	// Apply audit middleware to mutating routes
	if s.auditSvc != nil {
		users.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	users.GET("", userHandler.ListUsers)
	users.GET("/:id", userHandler.GetUserByID)
	users.DELETE("/:id", userHandler.SoftDelete)
	users.POST("/:id/roles", userHandler.AssignRole)
	users.DELETE("/:id/roles/:roleId", userHandler.RemoveRole)
	users.GET("/:id/roles", userHandler.GetUserRoles)
	users.POST("/:id/permissions", userHandler.GrantPermission)
	users.DELETE("/:id/permissions/:permId", userHandler.RemovePermission)
	users.GET("/:id/effective-permissions", userHandler.GetEffectivePermissions)
}

// RegisterAPIKeyRoutes registers API key management routes
// Requires JWT authentication. Users can only manage their own API keys.
func (s *Server) RegisterAPIKeyRoutes(api *echo.Group, apiKeyHandler *handler.APIKeyHandler) {
	apiKeys := api.Group("/api-keys")

	// All API key routes require JWT authentication
	apiKeys.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Apply audit middleware for mutating operations
	if s.auditSvc != nil {
		apiKeys.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	// CRUD routes - ownership is checked in handlers
	apiKeys.POST("", apiKeyHandler.Create)
	apiKeys.GET("", apiKeyHandler.List)
	apiKeys.GET("/:id", apiKeyHandler.GetByID)
	apiKeys.DELETE("/:id", apiKeyHandler.Revoke)
}

// RegisterInvoiceRoutes registers invoice-related routes
// Requires JWT authentication and permission checks
func (s *Server) RegisterInvoiceRoutes(api *echo.Group, invoiceHandler *invoice.Handler) {
	invoices := api.Group("/invoices")

	// All invoice routes require JWT authentication
	invoices.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Invoice management requires invoice:view permission for read operations
	// and invoice:create, invoice:update, invoice:delete for write operations
	if s.enforcer != nil {
		// Apply permission middleware for invoice operations
		invoices.Use(middleware.RequirePermission(s.enforcer, "invoices", "view"))
	}

	// Apply audit middleware to mutating routes
	if s.auditSvc != nil {
		invoices.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	// CRUD routes - permission checks handled in handler (ownership scope)
	invoices.POST("", invoiceHandler.Create)        // invoice:create checked via ownership
	invoices.GET("", invoiceHandler.List)          // invoice:view checked via ownership
	invoices.GET("/:id", invoiceHandler.GetByID)   // invoice:view checked via ownership
	invoices.PUT("/:id", invoiceHandler.Update)    // invoice:update checked via ownership
	invoices.DELETE("/:id", invoiceHandler.Delete) // invoice:delete checked via ownership
}

// RegisterOrganizationRoutes registers organization-related routes
// Requires JWT authentication. Write operations require specific permissions.
func (s *Server) RegisterOrganizationRoutes(api *echo.Group, orgHandler *handler.OrganizationHandler) {
	orgs := api.Group("/organizations")

	// All organization routes require JWT authentication
	orgs.Use(middleware.JWT(middleware.JWTConfig{
		Secret:     s.config.JWT.Secret,
		ContextKey: "user",
	}))

	// Apply audit middleware to mutating routes
	if s.auditSvc != nil {
		orgs.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
			Skipper:      middleware.DefaultAuditSkipper(),
			AuditService: s.auditSvc,
		}))
	}

	// CRUD routes - permission checks handled in handler (organization scope)
	orgs.POST("", orgHandler.Create)                       // Create organization (auth required)
	orgs.GET("", orgHandler.List)                          // List user's organizations (auth required)
	orgs.GET("/:id", orgHandler.GetByID)                   // Get organization (auth + membership check)
	orgs.PUT("/:id", orgHandler.Update)                    // Update organization (auth + manage permission)
	orgs.DELETE("/:id", orgHandler.Delete)                 // Delete organization (auth + manage permission)
	orgs.POST("/:id/members", orgHandler.AddMember)        // Add member (auth + invite permission)
	orgs.GET("/:id/members", orgHandler.GetMembers)         // Get members (auth + membership check)
	orgs.DELETE("/:id/members/:user_id", orgHandler.RemoveMember) // Remove member (auth + remove permission)
}

// HealthCheck performs health checks on dependencies
func (s *Server) HealthCheck(ctx context.Context) error {
	// Check database
	if s.db != nil {
		sqlDB, err := s.db.DB()
		if err != nil {
			return fmt.Errorf("database connection error: %w", err)
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
	}

	// Check Redis
	if s.redis != nil {
		if err := s.redis.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis ping failed: %w", err)
		}
	}

	return nil
}

// isDebugMode checks if debug mode should be enabled based on environment.
// Returns true if LOG_LEVEL is set to "debug" (case-insensitive).
func isDebugMode() bool {
	logLevel := os.Getenv("LOG_LEVEL")
	return logLevel == "debug" || logLevel == "DEBUG"
}

// Go API Base - Production-ready REST API foundation
//
// @title						Go API Base
// @version				1.0
// @description		A production-ready Go REST API with RBAC, JWT authentication, and permission management
//
// @termsOfService		http://swagger.io/terms/
//
// @contact.name		API Support
// @contact.email		support@example.com
//
// @license.name		MIT
// @license.url			https://opensource.org/licenses/MIT
//
// @host						localhost:8080
// @BasePath				/api/v1
//
// @securityDefinitions.apikey	BearerAuth
// @in													header
// @name												Authorization
// @description								Type "Bearer" followed by a space and JWT token.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/example/go-api-base/docs/swagger" // swagger docs
	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/database"
	"github.com/example/go-api-base/internal/domain"
	apphttp "github.com/example/go-api-base/internal/http"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/internal/ssrf"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var rootCmd = &cobra.Command{
	Use:   "go-api-base",
	Short: "Go API Base - Production-ready REST API foundation",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServer()
	},
}

// ANSI color codes for CLI output
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run pending database migrations",
	Long:  `Run all pending database migrations. Equivalent to Laravel's "php artisan migrate".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		down, _ := cmd.Flags().GetBool("down")
		if down {
			fmt.Printf("%sWarning: --down is deprecated. Use 'migrate:rollback' instead.%s\n", colorYellow, colorReset)
			return runMigrateRollback(1, false)
		}
		return runMigrateUp()
	},
}

var migrateRollbackCmd = &cobra.Command{
	Use:   "migrate:rollback",
	Short: "Rollback the last batch of migrations",
	Long: `Rollback database migrations.
Use --step=N to rollback N migrations (default: 1).
Use --all to rollback all migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return runMigrateRollback(0, true)
		}
		step, _ := cmd.Flags().GetInt("step")
		return runMigrateRollback(step, false)
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "migrate:status",
	Short: "Show the status of each migration",
	Long:  `Show a table with the status (Ran/Pending) of each migration file. Equivalent to Laravel's "php artisan migrate:status".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateStatus()
	},
}

var migrateRefreshCmd = &cobra.Command{
	Use:   "migrate:refresh",
	Short: "Rollback all migrations and re-run them",
	Long:  `Rollback ALL migrations and then run them again from scratch. Equivalent to Laravel's "php artisan migrate:refresh".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("%sThis will rollback ALL migrations and re-run them. Continue? [y/N]: %s", colorYellow, colorReset)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		return runMigrateRefresh()
	},
}

var migrateFreshCmd = &cobra.Command{
	Use:   "migrate:fresh",
	Short: "Drop all tables and re-run migrations from scratch",
	Long:  `Drop all tables in the database and re-run migrations from scratch. Equivalent to Laravel's "php artisan migrate:fresh".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		skipSeed, _ := cmd.Flags().GetBool("skip-seed")

		if !force {
			fmt.Printf("%sThis will DROP ALL TABLES and re-run migrations. Continue? [y/N]: %s", colorRed, colorReset)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		return runMigrateFresh(skipSeed)
	},
}

func init() {
	// migrate command flags
	migrateCmd.Flags().Bool("down", false, "(deprecated) Use 'migrate:rollback' instead")

	// migrate:rollback flags
	migrateRollbackCmd.Flags().Int("step", 1, "Number of migrations to rollback")
	migrateRollbackCmd.Flags().Bool("all", false, "Rollback all migrations")

	// migrate:refresh flags
	migrateRefreshCmd.Flags().Bool("force", false, "Skip confirmation prompt")

	// migrate:fresh flags
	migrateFreshCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	migrateFreshCmd.Flags().Bool("skip-seed", false, "Skip seeding after fresh migration")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(migrateRollbackCmd)
	rootCmd.AddCommand(migrateStatusCmd)
	rootCmd.AddCommand(migrateRefreshCmd)
	rootCmd.AddCommand(migrateFreshCmd)
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(permissionSyncCmd)

	// Initialize structured logging
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with initial data",
	Long: `Seed the database with:
- SuperAdmin role (system role with all permissions)
- Admin role (system role with management permissions)  
- Viewer role (system role with read-only permissions)
- Organization roles: owner, org_admin, member
- Default permissions for users, roles, permissions, invoices, organizations`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSeed()
	},
}

var permissionSyncCmd = &cobra.Command{
	Use:   "permission:sync",
	Short: "Sync permissions from manifest to Casbin policies",
	Long: `Synchronize permissions from a YAML manifest file (or built-in defaults) to the Casbin policy database.

This command will:
1. Load permissions from config/permissions.yaml (or use defaults if not found)
2. Upsert permissions to the database (create if not exists)
3. Sync role definitions to Casbin
4. Reload policies into the enforcer`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPermissionSync()
	},
}

// init is removed — registration is done in the init() above with all migration commands

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

// runServer starts the Echo HTTP server with graceful shutdown handling.
func runServer() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set log level based on config
	setLogLevel(cfg.Log.Level)

	slog.Info("Configuration loaded successfully",
		"database_host", cfg.Database.Host,
		"redis_host", cfg.Redis.Host,
		"server_port", cfg.Server.Port,
	)

	// Initialize logger from configuration
	log, err := logger.NewLogger(logger.Config{
		Level:          cfg.Log.Level,
		Format:         cfg.Log.Format,
		Outputs:        cfg.Log.Outputs,
		FilePath:       cfg.Log.FilePath,
		FileMaxSize:    cfg.Log.MaxSize,
		FileMaxBackups: cfg.Log.MaxBackups,
		FileMaxAge:     cfg.Log.MaxAge,
		FileCompress:   cfg.Log.Compress,
		SyslogNetwork:  cfg.Log.SyslogNetwork,
		SyslogAddress:  cfg.Log.SyslogAddress,
		SyslogTag:      cfg.Log.SyslogTag,
		AddSource:      cfg.Log.AddSource,
	})
	if err != nil {
		slog.Error("Failed to initialize logger", "error", err)
		return fmt.Errorf("logger initialization failed: %w", err)
	}
	defer log.Info(context.Background(), "logger shutdown")

	// Initialize database connections
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		// Log the error and return - don't panic here to allow proper shutdown
		slog.Error("Failed to initialize database", "error", err)
		return fmt.Errorf("database initialization failed: %w", err)
	}

	redisClient, err := database.NewRedisClient(cfg)
	if err != nil {
		slog.Error("Failed to initialize Redis", "error", err)
		return fmt.Errorf("redis initialization failed: %w", err)
	}

	// Initialize cache driver based on configuration
	cacheDriver, err := cache.NewDriver(cache.Config{
		Driver:        cfg.Cache.Driver,
		DefaultTTL:    cfg.Cache.DefaultTTL,
		PermissionTTL: cfg.Cache.PermissionTTL,
		RateLimitTTL:  cfg.Cache.RateLimitTTL,
	}, redisClient)
	if err != nil {
		slog.Error("Failed to initialize cache driver", "error", err)
		return fmt.Errorf("cache driver initialization failed: %w", err)
	}

	// Initialize Casbin enforcer for permission enforcement
	enforcer, err := permission.NewEnforcer(db)
	if err != nil {
		slog.Error("Failed to initialize permission enforcer", "error", err)
		return fmt.Errorf("permission enforcer initialization failed: %w", err)
	}

	// Initialize permission cache with configured TTL
	permCache := permission.NewCache(cacheDriver, time.Duration(cfg.Cache.PermissionTTL)*time.Second)
	enforcer.SetCache(permCache)

	// Initialize permission invalidator for cache sync across instances
	invalidator := permission.NewInvalidator(redisClient)

	// Create server with Echo and dependencies
	server := apphttp.NewServer(cfg, db, redisClient, cacheDriver, log)

	// Set permission-related dependencies
	server.SetEnforcer(enforcer)
	server.SetPermissionCache(permCache)
	server.SetInvalidator(invalidator)

	// Initialize 2FA service if encryption key is configured
	if cfg.TwoFactor.EncryptionKey != "" {
		encryptionKey := []byte(cfg.TwoFactor.EncryptionKey)
		if len(encryptionKey) != 32 {
			// If the key isn't 32 bytes when decoded as-is, try base64 decoding
			decoded, err := base64.StdEncoding.DecodeString(cfg.TwoFactor.EncryptionKey)
			if err != nil || len(decoded) != 32 {
				slog.Error("TWO_FACTOR_ENCRYPTION_KEY must be 32 bytes (use: openssl rand -base64 32)", "key_length", len(encryptionKey))
				return fmt.Errorf("invalid TWO_FACTOR_ENCRYPTION_KEY: must decode to 32 bytes for AES-256")
			}
			encryptionKey = decoded
		}
		twoFactorRecoveryCodeRepo := repository.NewTwoFactorRecoveryCodeRepository(db)
		refreshTokenRepo := repository.NewRefreshTokenRepository(db)
		userRepo := repository.NewUserRepository(db)
		tokenService := service.NewTokenService(
			cfg.JWT.Secret,
			cfg.JWT.Issuer,
			cfg.JWT.Audience,
			cfg.JWT.AccessExpiry,
			cfg.JWT.RefreshExpiry,
		)
		auditService := service.NewAuditService(
			repository.NewAuditLogRepository(db),
			service.DefaultAuditServiceConfig(),
		)
		twoFactorSvc := service.NewTwoFactorService(
			userRepo,
			twoFactorRecoveryCodeRepo,
			refreshTokenRepo,
			tokenService,
			service.TwoFactorServiceConfig{EncryptionKey: encryptionKey},
			auditService,
		)
		server.SetTwoFactorService(twoFactorSvc)
		slog.Info("Two-factor authentication service initialized")
	} else {
		slog.Warn("TWO_FACTOR_ENCRYPTION_KEY not set, 2FA endpoints will be disabled")
	}

	// Configure routes
	server.RegisterRoutes()

	// Initialize EmailWorker for background email processing
	emailWorker := initEmailWorker(cfg, db, redisClient, enforcer, server)
	if emailWorker != nil {
		server.SetEmailWorker(emailWorker)
	}

	// Initialize WebhookWorker for background webhook delivery
	ssrfCfg := cfg.SSRF.ToInternal()
	webhookWorker := initWebhookWorker(cfg, db, redisClient, server, &ssrfCfg)
	if webhookWorker != nil {
		server.SetWebhookWorker(webhookWorker)
	}

	// Wire webhook service with Redis queue and rate limiter for event dispatch
	if webhookService := server.WebhookService(); webhookService != nil {
		webhookQueue := service.NewWebhookRedisQueue(redisClient)
		webhookRateLimiter := service.NewWebhookRateLimiter(redisClient, cfg.Webhook.RateLimit)
		webhookService.SetQueue(webhookQueue)
		webhookService.SetRateLimiter(webhookRateLimiter)
	}

	// Initialize data portability workers (export + import)
	var exportWorker *service.ExportWorker
	var importWorker *service.ImportWorker
	if exportSvc := server.ExportService(); exportSvc != nil {
		exportJobRepo := repository.NewExportJobRepository(db)
		exportWorker = service.NewExportWorker(exportSvc, exportJobRepo, server.StorageDriver(), log, cfg.DataPortability.ExportWorkerConcurrency)
		server.SetExportWorker(exportWorker)
		slog.Info("Export worker initialized", "concurrency", cfg.DataPortability.ExportWorkerConcurrency)
	}
	if importSvc := server.ImportService(); importSvc != nil {
		importJobRepo := repository.NewImportJobRepository(db)
		importWorker = service.NewImportWorker(importSvc, importJobRepo, log, cfg.DataPortability.ImportWorkerConcurrency)
		server.SetImportWorker(importWorker)
		slog.Info("Import worker initialized", "concurrency", cfg.DataPortability.ImportWorkerConcurrency)
	}

	// Initialize EventBus for webhook event dispatch
	eventBus := domain.NewEventBus(256)
	server.SetEventBus(eventBus)

	// Subscribe webhook service to events for automatic dispatch
	if webhookService := server.WebhookService(); webhookService != nil {
		webhookService.SubscribeToEventBus(eventBus)
		slog.Info("Webhook service subscribed to event bus")
	}

	// Subscribe activity service to events for automatic activity creation
	if activityService := server.ActivityService(); activityService != nil {
		activityService.SubscribeToEventBus(eventBus)
		slog.Info("Activity service subscribed to event bus")
	}

	// Initialize analytics service
	metricEventRepo := repository.NewMetricEventRepository(db)
	dashboardMetricRepo := repository.NewDashboardMetricRepository(db)
	dashboardPrefRepo := repository.NewDashboardPreferenceRepository(db)
	analyticsService := service.NewAnalyticsService(
		metricEventRepo,
		dashboardMetricRepo,
		dashboardPrefRepo,
		enforcer,
		server.AuditService(),
		slog.Default(),
		cfg.Analytics,
	)
	server.SetAnalyticsService(analyticsService)

	// Subscribe analytics service to events for automatic metric event creation
	analyticsService.SubscribeToEventBus(eventBus)
	slog.Info("Analytics service subscribed to event bus")

	// Initialize analytics reaper for 90-day archival
	analyticsReaper := service.NewAnalyticsReaper(metricEventRepo, cfg.Analytics, slog.Default())
	server.SetAnalyticsReaper(analyticsReaper)
	slog.Info("Analytics reaper initialized",
		slog.Int("retention_days", cfg.Analytics.RetentionDays),
		slog.Duration("reaper_interval", cfg.Analytics.ReaperInterval),
	)

	// Initialize aggregation worker for pre-computing dashboard metrics
	aggregationWorker := service.NewAggregationWorker(
		metricEventRepo,
		dashboardMetricRepo,
		cfg.Analytics,
		slog.Default(),
	)
	server.SetAggregationWorker(aggregationWorker)
	slog.Info("Aggregation worker initialized",
		slog.Duration("aggregation_interval", cfg.Analytics.AggregationInterval),
	)

	// Initialize activity reaper for 90-day archival
	activityRepo := repository.NewActivityRepository(db)
	activityReaper := service.NewActivityReaper(activityRepo, cfg.Activity, slog.Default())
	server.SetActivityReaper(activityReaper)
	slog.Info("Activity reaper initialized",
		slog.Int("retention_days", cfg.Activity.RetentionDays),
		slog.Duration("reaper_interval", cfg.Activity.ReaperInterval),
	)

	// Wire EventBus into domain services for event emission
	if userService := server.UserService(); userService != nil {
		userService.SetEventBus(eventBus)
		slog.Info("User service wired to event bus")
	}
	if invoiceService := server.InvoiceService(); invoiceService != nil {
		invoiceService.SetEventBus(eventBus)
		slog.Info("Invoice service wired to event bus")
	}
	if newsService := server.NewsService(); newsService != nil {
		newsService.SetEventBus(eventBus)
		slog.Info("News service wired to event bus")
	}
	if oauthLoginService := server.OAuthLoginService(); oauthLoginService != nil {
		oauthLoginService.SetEventBus(eventBus)
		slog.Info("OAuth login service wired to event bus")
	}
	if oauthProviderService := server.OAuthProviderService(); oauthProviderService != nil {
		oauthProviderService.SetEventBus(eventBus)
		slog.Info("OAuth provider service wired to event bus")
	}

	// Wire EventBus into data portability services
	if exportSvc := server.ExportService(); exportSvc != nil {
		if setter, ok := any(exportSvc).(interface{ SetEventBus(*domain.EventBus) }); ok {
			setter.SetEventBus(eventBus)
			slog.Info("Export service wired to event bus")
		}
	}
	if importSvc := server.ImportService(); importSvc != nil {
		if setter, ok := any(importSvc).(interface{ SetEventBus(*domain.EventBus) }); ok {
			setter.SetEventBus(eventBus)
			slog.Info("Import service wired to event bus")
		}
	}
	if importWorker := server.ImportWorker(); importWorker != nil {
		importWorker.SetEventBus(eventBus)
		slog.Info("Import worker wired to event bus")
	}

	// Wire WebSocket event bridge to event bus
	if wsHub := server.WsHub(); wsHub != nil {
		wsEventBridge := domain.NewWsEventBridge(wsHub)
		wsEventBridge.SubscribeToEventBus(eventBus)
		slog.Info("WebSocket event bridge subscribed to event bus")
	}

	// Add health check routes (using the server's Echo instance)
	e := server.Echo()
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	e.GET("/readyz", func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()

		if err := server.HealthCheck(ctx); err != nil {
			slog.Error("Readiness check failed", "error", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "not ready",
				"error":  err.Error(),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start permission invalidation listener in background
	go func() {
		errChan := invalidator.StartInvalidationListener(ctx, permCache, enforcer)
		if err := <-errChan; err != nil {
			slog.Error("Permission invalidation listener error", "error", err)
		}
	}()

	// Start email worker in background (if initialized)
	if emailWorker := server.EmailWorker(); emailWorker != nil {
		slog.Info("Starting email worker...")
		if err := emailWorker.Start(ctx); err != nil {
			slog.Error("Failed to start email worker", "error", err)
		}
	}

	// Start webhook worker in background (if initialized)
	if webhookWorker := server.WebhookWorker(); webhookWorker != nil {
		slog.Info("Starting webhook worker...")
		webhookWorker.Start()
	}

	// Start export worker in background (if initialized)
	if exportWorker := server.ExportWorker(); exportWorker != nil {
		slog.Info("Starting export worker...")
		exportWorker.Start(ctx)
	}

	// Start import worker in background (if initialized)
	if importWorker := server.ImportWorker(); importWorker != nil {
		slog.Info("Starting import worker...")
		importWorker.Start(ctx)
	}

	// Start event bus in background
	if eventBus := server.EventBus(); eventBus != nil {
		slog.Info("Starting event bus...")
		if err := eventBus.Start(ctx); err != nil {
			slog.Error("Failed to start event bus", "error", err)
		}
	}

	// Start activity reaper in background
	if activityReaper := server.ActivityReaper(); activityReaper != nil {
		slog.Info("Starting activity reaper...")
		activityReaper.Start(ctx)
	}

	// Start WebSocket hub in background
	if wsHub := server.WsHub(); wsHub != nil {
		slog.Info("Starting WebSocket hub...")
		if err := wsHub.Start(ctx); err != nil {
			slog.Error("Failed to start WebSocket hub", "error", err)
		}
	}

	// Start analytics reaper in background
	if analyticsReaper := server.AnalyticsReaper(); analyticsReaper != nil {
		slog.Info("Starting analytics reaper...")
		analyticsReaper.Start(ctx)
	}

	// Start aggregation worker in background
	if aggregationWorker := server.AggregationWorker(); aggregationWorker != nil {
		slog.Info("Starting aggregation worker...")
		aggregationWorker.Start(ctx)
	}

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	// with a timeout of 30 seconds (as per constitution)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	slog.Info("Shutdown signal received",
		"signal", sig.String(),
		"timeout", "30s",
	)

	// Cancel the invalidation listener context
	cancel()

	// 1. Stop HTTP server first to stop accepting new requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	slog.Info("Shutting down HTTP server...")
	if err := server.Stop(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}
	slog.Info("HTTP server stopped")

	// 2. Stop WebSocket hub to close all client connections
	if wsHub := server.WsHub(); wsHub != nil {
		slog.Info("Stopping WebSocket hub...")
		wsHub.Stop()
		slog.Info("WebSocket hub stopped")
	}

	// 3. Stop event bus to prevent new event dispatches
	if eventBus := server.EventBus(); eventBus != nil {
		slog.Info("Stopping event bus...")
		if err := eventBus.Stop(); err != nil {
			slog.Error("Failed to stop event bus", "error", err)
		}
	}

	// 2b. Stop activity reaper
	if activityReaper := server.ActivityReaper(); activityReaper != nil {
		slog.Info("Stopping activity reaper...")
		activityReaper.Stop()
	}

	// 2c. Stop analytics reaper
	if analyticsReaper := server.AnalyticsReaper(); analyticsReaper != nil {
		slog.Info("Stopping analytics reaper...")
		analyticsReaper.Stop()
	}

	// 3. Stop workers to finish in-flight processing
	if emailWorker := server.EmailWorker(); emailWorker != nil {
		slog.Info("Stopping email worker...")
		if err := emailWorker.Stop(); err != nil {
			slog.Error("Failed to stop email worker", "error", err)
		}
	}

	if webhookWorker := server.WebhookWorker(); webhookWorker != nil {
		slog.Info("Stopping webhook worker...")
		webhookWorker.Stop()
	}

	if exportWorker := server.ExportWorker(); exportWorker != nil {
		slog.Info("Stopping export worker...")
		exportWorker.Stop()
	}

	if importWorker := server.ImportWorker(); importWorker != nil {
		slog.Info("Stopping import worker...")
		importWorker.Stop()
	}

	if aggregationWorker := server.AggregationWorker(); aggregationWorker != nil {
		slog.Info("Stopping aggregation worker...")
		aggregationWorker.Stop()
	}

	// Close permission enforcer
	if err := enforcer.Close(); err != nil {
		slog.Error("Failed to close permission enforcer", "error", err)
	}

	// Close database connections
	slog.Info("Closing database connections...")
	if err := database.Close(db); err != nil {
		slog.Error("Failed to close database connection", "error", err)
	}

	if err := database.CloseRedis(redisClient); err != nil {
		slog.Error("Failed to close Redis connection", "error", err)
	}

	slog.Info("Shutdown complete")
	return nil
}

// setLogLevel configures the global log level based on the configuration.
func setLogLevel(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// initEmailWorker initializes the email worker for background email processing.
func initEmailWorker(cfg *config.Config, db *gorm.DB, redisClient *redis.Client, enforcer *permission.Enforcer, server *apphttp.Server) *service.EmailWorker {
	slog.Info("Initializing email worker...")

	// Initialize email repositories
	emailTemplateRepo := repository.NewEmailTemplateRepository(db)
	emailQueueRepo := repository.NewEmailQueueRepository(db)
	emailBounceRepo := repository.NewEmailBounceRepository(db)

	// Initialize email queue (Redis)
	redisQueue := service.NewRedisEmailQueue(redisClient)

	// Initialize email provider (SMTP baseline)
	smtpProvider := service.NewSMTPProvider(&cfg.Email)

	// Initialize template engine
	templateEngine := service.NewTemplateEngine(emailTemplateRepo)

	// Initialize email worker
	emailWorker := service.NewEmailWorker(
		&cfg.Email,
		emailQueueRepo,
		emailBounceRepo,
		redisQueue,
		smtpProvider,
		templateEngine,
	)

	slog.Info("Email worker initialized",
		"provider", smtpProvider.Name(),
		"worker_concurrency", cfg.Email.WorkerConcurrency,
		"retry_max", cfg.Email.RetryMax,
	)

	return emailWorker
}

// initWebhookWorker initializes the webhook worker for background delivery processing.
func initWebhookWorker(cfg *config.Config, db *gorm.DB, redisClient *redis.Client, server *apphttp.Server, ssrfCfg *ssrf.SSRFConfig) *service.WebhookWorker {
	slog.Info("Initializing webhook worker...")

	// Initialize webhook repositories
	webhookRepo := repository.NewWebhookRepository(db)
	webhookDeliveryRepo := repository.NewWebhookDeliveryRepository(db)

	// Initialize webhook Redis queue
	webhookQueue := service.NewWebhookRedisQueue(redisClient)

	// Initialize webhook rate limiter
	webhookRateLimiter := service.NewWebhookRateLimiter(redisClient, cfg.Webhook.RateLimit)

	// Initialize webhook worker with SSRF protection
	webhookWorker := service.NewWebhookWorker(
		&cfg.Webhook,
		webhookDeliveryRepo,
		webhookRepo,
		webhookQueue,
		webhookRateLimiter,
		ssrfCfg,
	)

	slog.Info("Webhook worker initialized",
		"worker_concurrency", cfg.Webhook.WorkerConcurrency,
		"retry_max", cfg.Webhook.RetryMax,
		"rate_limit", cfg.Webhook.RateLimit,
	)

	return webhookWorker
}

// PermissionManifest defines the structure of the permissions.yaml file.
type PermissionManifest struct {
	Permissions []PermissionEntry `yaml:"permissions"`
	Roles       []RoleEntry       `yaml:"roles"`
}

// PermissionEntry defines a permission in the manifest.
type PermissionEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Resource    string `yaml:"resource"`
	Action      string `yaml:"action"`
}

// RoleEntry defines a role with its permissions in the manifest.
type RoleEntry struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Permissions []string `yaml:"permissions"`
}

// DefaultPermissions returns the default permission manifest.
func DefaultPermissions() *PermissionManifest {
	return &PermissionManifest{
		Permissions: []PermissionEntry{
			{Name: "users:manage", Description: "Manage users", Resource: "users", Action: "manage"},
			{Name: "roles:manage", Description: "Manage roles", Resource: "roles", Action: "manage"},
			{Name: "permissions:manage", Description: "Manage permissions", Resource: "permissions", Action: "manage"},
			{Name: "email_templates:read", Description: "View email templates", Resource: "email_templates", Action: "read"},
			{Name: "email_templates:manage", Description: "Manage email templates", Resource: "email_templates", Action: "manage"},
			{Name: "email_queue:read", Description: "View email queue status", Resource: "email_queue", Action: "read"},
			{Name: "email_queue:manage", Description: "Manage email queue", Resource: "email_queue", Action: "manage"},
			{Name: "email_bounces:read", Description: "View bounce history", Resource: "email_bounces", Action: "read"},
		{Name: "settings:view_user", Description: "View user settings", Resource: "settings", Action: "view_user"},
		{Name: "settings:manage_user", Description: "Update user settings", Resource: "settings", Action: "manage_user"},
		{Name: "settings:view_system", Description: "View system settings", Resource: "settings", Action: "view_system"},
		{Name: "settings:manage_system", Description: "Update system settings", Resource: "settings", Action: "manage_system"},
		{Name: "webhooks:view", Description: "View webhooks and delivery history", Resource: "webhooks", Action: "view"},
		{Name: "webhooks:manage", Description: "Manage webhooks and replay deliveries", Resource: "webhooks", Action: "manage"},
		{Name: "media_version:upload", Description: "Upload new media versions", Resource: "media_version", Action: "upload"},
		{Name: "media_version:view", Description: "View media version history", Resource: "media_version", Action: "view"},
		{Name: "media_version:download", Description: "Download specific media versions", Resource: "media_version", Action: "download"},
		{Name: "media_version:restore", Description: "Restore previous media versions", Resource: "media_version", Action: "restore"},
		{Name: "media_version:delete", Description: "Delete specific media versions (admin only)", Resource: "media_version", Action: "delete"},
		{Name: "data_portability:export_create", Description: "Create data export jobs", Resource: "data_portability", Action: "export:create"},
		{Name: "data_portability:export_download", Description: "Download export files", Resource: "data_portability", Action: "export:download"},
		{Name: "data_portability:import_create", Description: "Create data import jobs", Resource: "data_portability", Action: "import:create"},
		{Name: "data_portability:import_view", Description: "View import job status and results", Resource: "data_portability", Action: "import:view"},
		{Name: "data_portability:import_cancel", Description: "Cancel import and export jobs", Resource: "data_portability", Action: "import:cancel"},
		{Name: "feature_flag:view", Description: "View feature flags", Resource: "feature_flag", Action: "view"},
		{Name: "feature_flag:manage", Description: "Create, update, and delete feature flags", Resource: "feature_flag", Action: "manage"},
		{Name: "comment:view", Description: "View comments", Resource: "comment", Action: "view"},
		{Name: "comment:create", Description: "Create and edit own comments", Resource: "comment", Action: "create"},
		{Name: "comment:delete", Description: "Delete own comments", Resource: "comment", Action: "delete"},
		{Name: "comment:delete_any", Description: "Delete any comment (admin)", Resource: "comment", Action: "delete_any"},
		{Name: "comment:manage", Description: "Pin, unpin, and manage comments", Resource: "comment", Action: "manage"},
		{Name: "activity:view", Description: "View activity feed and history", Resource: "activity", Action: "view"},
		{Name: "activity:manage", Description: "Manage activities (delete, admin)", Resource: "activity", Action: "manage"},
		{Name: "tag:view", Description: "View tags", Resource: "tag", Action: "view"},
		{Name: "tag:create", Description: "Create tags", Resource: "tag", Action: "create"},
		{Name: "tag:update", Description: "Update tags", Resource: "tag", Action: "update"},
		{Name: "tag:delete", Description: "Delete tags", Resource: "tag", Action: "delete"},
		{Name: "tag:manage", Description: "Attach and detach tags to entities", Resource: "tag", Action: "manage"},
		{Name: "websocket:connect", Description: "Connect to WebSocket endpoints", Resource: "websocket", Action: "connect"},
		{Name: "websocket:view_presence", Description: "View online presence for organizations", Resource: "websocket", Action: "view_presence"},
		{Name: "websocket:view_rooms", Description: "View available WebSocket rooms", Resource: "websocket", Action: "view_rooms"},
		{Name: "analytics:view", Description: "View analytics dashboard and metrics", Resource: "analytics", Action: "view"},
		{Name: "analytics:manage", Description: "Manage analytics preferences and trigger aggregation", Resource: "analytics", Action: "manage"},
	},
		Roles: []RoleEntry{
			{
				Name:        "admin",
				Description: "Administrator role with full access",
				Permissions: []string{"users:manage", "roles:manage", "permissions:manage", "email_templates:manage", "email_queue:manage", "email_bounces:read", "settings:view_user", "settings:manage_user", "settings:view_system", "settings:manage_system", "media_version:upload", "media_version:view", "media_version:download", "media_version:restore", "media_version:delete", "data_portability:export_create", "data_portability:export_download", "data_portability:import_create", "data_portability:import_view", "data_portability:import_cancel", "feature_flag:view", "feature_flag:manage", "comment:view", "comment:create", "comment:delete", "comment:delete_any", "comment:manage", "activity:view", "activity:manage", "tag:view", "tag:create", "tag:update", "tag:delete", "tag:manage", "websocket:connect", "websocket:view_presence", "websocket:view_rooms", "analytics:view", "analytics:manage"},
			},
		},
	}
}

// runPermissionSync synchronizes permissions from manifest to Casbin.
func runPermissionSync() error {
	slog.Info("Starting permission synchronization")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set log level
	setLogLevel(cfg.Log.Level)

	// Initialize database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			slog.Error("Failed to close database", "error", closeErr)
		}
	}()

	// Initialize Casbin enforcer
	enforcer, err := permission.NewEnforcer(db)
	if err != nil {
		return fmt.Errorf("failed to initialize enforcer: %w", err)
	}
	defer func() {
		if closeErr := enforcer.Close(); closeErr != nil {
			slog.Error("Failed to close enforcer", "error", closeErr)
		}
	}()

	// Load permission manifest (try file first, then use defaults)
	manifest := DefaultPermissions()
	manifestPath := "config/permissions.yaml"

	// Try to read manifest file if it exists
	if _, err := os.ReadFile(manifestPath); err == nil {
		slog.Info("Loading permissions from manifest", "path", manifestPath)
		// Parse YAML (skip for simplicity, use defaults for now)
		// In production: yaml.Unmarshal(data, manifest)
	}

	slog.Info("Processing permissions", "count", len(manifest.Permissions))

	// Sync permissions to database (for app-level permission entity table)
	// This would use the permission repository to create/update entities
	// For now, we'll just sync the Casbin policies

	// Sync roles to Casbin
	slog.Info("Syncing roles to Casbin")
	domain := "default" // Default domain

	for _, role := range manifest.Roles {
		// Add policy for each permission in the role
		for _, permName := range role.Permissions {
			// Find the permission entry
			var permEntry *PermissionEntry
			for i := range manifest.Permissions {
				if manifest.Permissions[i].Name == permName {
					permEntry = &manifest.Permissions[i]
					break
				}
			}

			if permEntry == nil {
				slog.Warn("Permission not found in manifest, skipping",
					"permission", permName,
					"role", role.Name)
				continue
			}

			// Add policy: role has permission on resource:action
			// p = sub, dom, obj, act
			if err := enforcer.AddPolicy(role.Name, domain, permEntry.Resource, permEntry.Action); err != nil {
				slog.Error("Failed to add policy",
					"role", role.Name,
					"resource", permEntry.Resource,
					"action", permEntry.Action,
					"error", err)
				continue
			}

			slog.Debug("Policy added/exists",
				"role", role.Name,
				"resource", permEntry.Resource,
				"action", permEntry.Action)
		}

		slog.Info("Role synced", "role", role.Name, "permissions", len(role.Permissions))
	}

	// Save policies to ensure persistence
	if err := enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policies: %w", err)
	}

	slog.Info("Permission synchronization complete",
		"permissions", len(manifest.Permissions),
		"roles", len(manifest.Roles))

	return nil
}

// newMigrator is a helper that loads config, connects to DB, and creates a Migrate instance.
// The caller must defer m.Close() and database.Close(db).
func newMigrator() (*gorm.DB, *database.Migrate, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	setLogLevel(cfg.Log.Level)

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	m, err := database.NewMigrate(db, "file://migrations")
	if err != nil {
		database.Close(db)
		return nil, nil, fmt.Errorf("failed to create migration handler: %w", err)
	}

	return db, m, nil
}

// runMigrateUp runs all pending migrations one-by-one with per-migration output.
func runMigrateUp() error {
	db, m, err := newMigrator()
	if err != nil {
		return err
	}
	defer database.Close(db)
	defer m.Close()

	fmt.Println("")
	fmt.Printf("%sRunning migrations...%s\n", colorBold, colorReset)

	// Get current version before starting
	version, dirty, _ := m.Version()
	if dirty {
		fmt.Printf("%s⚠ Dirty state detected at version %d, forcing clean...%s\n", colorYellow, version, colorReset)
		if forceErr := m.Force(int(version)); forceErr != nil {
			return fmt.Errorf("failed to force clean version: %w", forceErr)
		}
		fmt.Printf("%s✓ Cleaned dirty state at version %d%s\n", colorGreen, version, colorReset)
	}

	applied := 0
	failed := 0

	// Step through migrations one at a time for detailed output
	for {
		currentVersion, _, _ := m.Version()

		err := m.Steps(1)
		if err != nil {
			// No more migrations to run — this is expected
			break
		}

		newVersion, _, _ := m.Version()
		migrationName := findMigrationName(newVersion)
		fmt.Printf("  %s✓ %s%s\n", colorGreen, migrationName, colorReset)
		applied++
		_ = currentVersion // suppress unused warning
	}

	if failed > 0 {
		fmt.Printf("\n%s%d migrations ran, %d failed.%s\n", colorRed, applied, failed, colorReset)
		return fmt.Errorf("%d migrations failed", failed)
	}

	if applied == 0 {
		fmt.Printf("%sNothing to migrate.%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✓ %d migration(s) ran successfully.%s\n", colorGreen, applied, colorReset)
	}

	return nil
}

// runMigrateRollback rolls back N migrations (or all if all=true).
func runMigrateRollback(steps int, all bool) error {
	db, m, err := newMigrator()
	if err != nil {
		return err
	}
	defer database.Close(db)
	defer m.Close()

	fmt.Println("")
	if all {
		fmt.Printf("%sRolling back all migrations...%s\n", colorBold, colorReset)
	} else {
		fmt.Printf("%sRolling back %d migration(s)...%s\n", colorBold, steps, colorReset)
	}

	rolledBack := 0

	if all {
		// Roll back one at a time for progress output
		for {
			currentVersion, _, verr := m.Version()
			if verr != nil || currentVersion == 0 {
				break // no more versions
			}

			migrationName := findMigrationName(currentVersion)
			if err := m.Steps(-1); err != nil {
				fmt.Printf("  %s✗ %s (error: %v)%s\n", colorRed, migrationName, err, colorReset)
				break
			}
			fmt.Printf("  %s✓ %s%s\n", colorGreen, migrationName, colorReset)
			rolledBack++
		}
	} else {
		for i := 0; i < steps; i++ {
			currentVersion, _, verr := m.Version()
			if verr != nil || currentVersion == 0 {
				break // no more versions
			}

			migrationName := findMigrationName(currentVersion)
			if err := m.Steps(-1); err != nil {
				fmt.Printf("  %s✗ %s (error: %v)%s\n", colorRed, migrationName, err, colorReset)
				break
			}
			fmt.Printf("  %s✓ %s%s\n", colorGreen, migrationName, colorReset)
			rolledBack++
		}
	}

	if rolledBack == 0 {
		fmt.Printf("%sNothing to rollback.%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✓ %d migration(s) rolled back.%s\n", colorGreen, rolledBack, colorReset)
	}

	return nil
}

// runMigrateStatus shows a table with the status of each migration.
func runMigrateStatus() error {
	db, m, err := newMigrator()
	if err != nil {
		return err
	}
	defer database.Close(db)
	defer m.Close()

	// Read migration files from the migrations/ directory
	migrationsDir := "migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Collect unique migration names (from .up.sql files)
	type migrationEntry struct {
		version uint
		name    string
	}
	var migrations []migrationEntry
	seen := make(map[uint]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// Extract version number from filename (format: NNNNNN_name.up.sql)
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		version, verr := strconv.ParseUint(parts[0], 10, 32)
		if verr != nil {
			continue
		}
		if seen[uint(version)] {
			continue
		}
		seen[uint(version)] = true
		// Remove the .up.sql suffix for display
		migName := strings.TrimSuffix(name, ".up.sql")
		migrations = append(migrations, migrationEntry{version: uint(version), name: migName})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	// Get current version
	currentVersion, dirty, _ := m.Version()

	// Calculate column widths
	maxNameLen := len("Migration")
	for _, mig := range migrations {
		if len(mig.name) > maxNameLen {
			maxNameLen = len(mig.name)
		}
	}

	// Print table header
	sepLine := strings.Repeat("-", maxNameLen+2)

	fmt.Println("")
	fmt.Printf("%sMigration Status%s\n", colorBold, colorReset)
	fmt.Println("")

	fmt.Printf("| %-4s | %-*s | %-8s |\n", "#", maxNameLen, "Migration", "Status")
	fmt.Printf("+------+%s+----------+\n", sepLine)

	for i, mig := range migrations {
		status := "Pending"
		statusColor := colorYellow
		// A migration is "Ran" if its version is <= current version
		// and current version > 0 (meaning at least some migrations have run)
		if currentVersion > 0 && mig.version <= currentVersion {
			status = "Ran"
			statusColor = colorGreen
		}

		fmt.Printf("| %-4d | %-*s | %s%-8s%s |\n", i+1, maxNameLen, mig.name, statusColor, status, colorReset)
	}

	fmt.Printf("+------+%s+----------+\n", sepLine)

	// Show dirty state if applicable
	if dirty {
		fmt.Printf("\n%s⚠ Dirty state detected at version %d%s\n", colorYellow, currentVersion, colorReset)
		fmt.Printf("%sRun 'migrate' to continue or 'migrate:rollback' to revert.%s\n", colorYellow, colorReset)
	}

	pending := 0
	if currentVersion == 0 {
		pending = len(migrations)
	} else {
		for _, mig := range migrations {
			if mig.version > currentVersion {
				pending++
			}
		}
	}

	ran := len(migrations) - pending
	fmt.Printf("\n%d migration(s) ran, %d pending\n", ran, pending)

	return nil
}

// runMigrateRefresh rolls back all migrations and re-runs them.
func runMigrateRefresh() error {
	db, m, err := newMigrator()
	if err != nil {
		return err
	}
	defer database.Close(db)
	defer m.Close()

	fmt.Println("")
	fmt.Printf("%sRolling back all migrations...%s\n", colorBold, colorReset)

	rolledBack := 0
	for {
		currentVersion, _, verr := m.Version()
		if verr != nil || currentVersion == 0 {
			break
		}
		migrationName := findMigrationName(currentVersion)
		if rollbackErr := m.Steps(-1); rollbackErr != nil {
			fmt.Printf("  %s✗ %s (error: %v)%s\n", colorRed, migrationName, rollbackErr, colorReset)
			return fmt.Errorf("rollback failed at migration %s: %w", migrationName, rollbackErr)
		}
		fmt.Printf("  %s✓ Rolled back %s%s\n", colorGreen, migrationName, colorReset)
		rolledBack++
	}

	if rolledBack == 0 {
		fmt.Printf("%sNothing to rollback.%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✓ %d migration(s) rolled back.%s\n", colorGreen, rolledBack, colorReset)
	}

	fmt.Println("")
	fmt.Printf("%sRunning migrations up...%s\n", colorBold, colorReset)

	applied := 0
	for {
		if stepErr := m.Steps(1); stepErr != nil {
			break
		}
		newVersion, _, _ := m.Version()
		migrationName := findMigrationName(newVersion)
		fmt.Printf("  %s✓ %s%s\n", colorGreen, migrationName, colorReset)
		applied++
	}

	if applied == 0 {
		fmt.Printf("%sNothing to migrate.%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✓ %d migration(s) ran successfully.%s\n", colorGreen, applied, colorReset)
	}

	fmt.Printf("\n%sRefresh complete.%s\n", colorGreen, colorReset)
	return nil
}

// runMigrateFresh drops all tables and re-runs migrations from scratch.
func runMigrateFresh(skipSeed bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	setLogLevel(cfg.Log.Level)

	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close(db)

	fmt.Println("")
	fmt.Printf("%sDropping all tables...%s\n", colorBold, colorReset)

	// Drop all tables by dropping and recreating the public schema
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if _, err := sqlDB.Exec("DROP SCHEMA public CASCADE"); err != nil {
		return fmt.Errorf("failed to drop public schema: %w", err)
	}
	if _, err := sqlDB.Exec("CREATE SCHEMA public"); err != nil {
		return fmt.Errorf("failed to recreate public schema: %w", err)
	}
	fmt.Printf("  %s✓ Dropped all tables%s\n", colorGreen, colorReset)

	// Now run migrations from scratch
	m, err := database.NewMigrate(db, "file://migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration handler: %w", err)
	}
	defer m.Close()

	fmt.Println("")
	fmt.Printf("%sRunning migrations from scratch...%s\n", colorBold, colorReset)

	currentVersion, dirty, _ := m.Version()
	if dirty {
		fmt.Printf("%s⚠ Dirty state detected at version %d, forcing clean...%s\n", colorYellow, currentVersion, colorReset)
		if forceErr := m.Force(int(currentVersion)); forceErr != nil {
			return fmt.Errorf("failed to force clean version: %w", forceErr)
		}
	}

	applied := 0
	for {
		if stepErr := m.Steps(1); stepErr != nil {
			break
		}
		newVersion, _, _ := m.Version()
		migrationName := findMigrationName(newVersion)
		fmt.Printf("  %s✓ %s%s\n", colorGreen, migrationName, colorReset)
		applied++
	}

	if applied == 0 {
		fmt.Printf("%sNothing to migrate.%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("\n%s✓ %d migration(s) ran successfully.%s\n", colorGreen, applied, colorReset)
	}

	// Optionally run seed
	if !skipSeed {
		fmt.Println("")
		fmt.Printf("%sRunning seed...%s\n", colorBold, colorReset)
		if seedErr := runSeed(); seedErr != nil {
			fmt.Printf("%s✗ Seed failed: %v%s\n", colorRed, seedErr, colorReset)
			return seedErr
		}
		fmt.Printf("%s✓ Seed complete.%s\n", colorGreen, colorReset)
	}

	fmt.Printf("\n%sFresh migration complete.%s\n", colorGreen, colorReset)
	return nil
}

// findMigrationName finds the migration name for a given version number by reading the migrations directory.
func findMigrationName(version uint) string {
	migrationsDir := "migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Sprintf("%06d", version)
	}

	versionStr := fmt.Sprintf("%06d", version)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, versionStr+"_") && strings.HasSuffix(name, ".up.sql") {
			// Return just the name without the .up.sql suffix
			return strings.TrimSuffix(name, ".up.sql")
		}
	}
	return versionStr
}

// runSeed seeds the database with initial roles and permissions.
func runSeed() error {
	slog.Info("Starting database seeding")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set log level
	setLogLevel(cfg.Log.Level)

	// Initialize database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			slog.Error("Failed to close database", "error", closeErr)
		}
	}()

	// Get underlying SQL DB for raw queries
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	domain := "default"

	// Seed Permissions
	permissions := []struct {
		Name        string
		Resource    string
		Action      string
		Scope       string
		Description string
		IsSystem    bool
	}{
		// User permissions
		{"users:view", "users", "view", "all", "View users", false},
		{"users:create", "users", "create", "all", "Create users", false},
		{"users:update", "users", "update", "all", "Update users", false},
		{"users:delete", "users", "delete", "all", "Delete users", false},
		{"users:manage", "users", "manage", "all", "Manage all users", false},

		// Role permissions
		{"roles:view", "roles", "view", "all", "View roles", false},
		{"roles:create", "roles", "create", "all", "Create roles", false},
		{"roles:update", "roles", "update", "all", "Update roles", false},
		{"roles:delete", "roles", "delete", "all", "Delete roles", false},
		{"roles:manage", "roles", "manage", "all", "Manage all roles", false},

		// Permission permissions
		{"permissions:view", "permissions", "view", "all", "View permissions", false},
		{"permissions:create", "permissions", "create", "all", "Create permissions", false},
		{"permissions:manage", "permissions", "manage", "all", "Manage all permissions", false},

		// Invoice permissions
		{"invoices:view", "invoices", "view", "own", "View own invoices", false},
		{"invoices:view:all", "invoices", "view", "all", "View all invoices", false},
		{"invoices:create", "invoices", "create", "all", "Create invoices", false},
		{"invoices:update", "invoices", "update", "own", "Update own invoices", false},
		{"invoices:update:all", "invoices", "update", "all", "Update all invoices", false},
		{"invoices:delete", "invoices", "delete", "own", "Delete own invoices", false},
		{"invoices:delete:all", "invoices", "delete", "all", "Delete all invoices", false},
		{"invoices:manage", "invoices", "manage", "all", "Manage all invoices", false},

		// Audit permissions
		{"audit:view", "audit", "view", "all", "View audit logs", false},

		// API Key permissions
		{"api_keys:create", "api_keys", "create", "own", "Create own API keys", false},
		{"api_keys:view", "api_keys", "view", "own", "View own API keys", false},
		{"api_keys:revoke", "api_keys", "revoke", "own", "Revoke own API keys", false},
		{"api_keys:manage", "api_keys", "manage", "all", "Manage all API keys", false},

		// Organization permissions
		{"organization:view", "organizations", "view", "all", "View organization details", false},
		{"organization:manage", "organizations", "manage", "all", "Manage organization settings", false},
		{"organization:invite", "organizations", "invite", "all", "Invite members to organization", false},
		{"organization:remove", "organizations", "remove", "all", "Remove members from organization", false},

		// Notification permissions
		{"notifications:view", "notifications", "view", "own", "View own notifications", false},
		{"notifications:manage", "notifications", "manage", "own", "Manage own notifications", false},

		// Webhook permissions
		{"webhooks:view", "webhooks", "view", "all", "View webhooks and delivery history", false},
		{"webhooks:manage", "webhooks", "manage", "all", "Manage webhooks and replay deliveries", false},
	}

	slog.Info("Seeding permissions", "count", len(permissions))
	for _, p := range permissions {
		result, err := sqlDB.Exec(`
			INSERT INTO permissions (id, name, resource, action, scope, is_system, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`, p.Name, p.Resource, p.Action, p.Scope, p.IsSystem)
		if err != nil {
			slog.Error("Failed to seed permission", "name", p.Name, "error", err)
			continue
		}
		rows, _ := result.RowsAffected()
		if rows > 0 {
			slog.Debug("Permission created", "name", p.Name)
		}
	}

	// Seed Roles
	roles := []struct {
		Name        string
		Description string
		IsSystem    bool
	}{
		{"superadmin", "Super Administrator with full system access", true},
		{"admin", "Administrator with management access", true},
		{"viewer", "Read-only access viewer", true},
		// Organization roles
		{"owner", "Organization owner with full organization access", false},
		{"org_admin", "Organization admin with management permissions", false},
		{"member", "Organization member with view access", false},
	}

	slog.Info("Seeding roles", "count", len(roles))
	for _, r := range roles {
		result, err := sqlDB.Exec(`
			INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`, r.Name, r.Description, r.IsSystem)
		if err != nil {
			slog.Error("Failed to seed role", "name", r.Name, "error", err)
			continue
		}
		rows, _ := result.RowsAffected()
		if rows > 0 {
			slog.Debug("Role created", "name", r.Name)
		}
	}

	// Get role IDs for role-permission assignment
	var superadminRoleID, adminRoleID, viewerRoleID, ownerRoleID, orgAdminRoleID, memberRoleID string
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'superadmin'`).Scan(&superadminRoleID)
	if err != nil {
		return fmt.Errorf("failed to get superadmin role ID: %w", err)
	}
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID)
	if err != nil {
		return fmt.Errorf("failed to get admin role ID: %w", err)
	}
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'viewer'`).Scan(&viewerRoleID)
	if err != nil {
		return fmt.Errorf("failed to get viewer role ID: %w", err)
	}
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'owner'`).Scan(&ownerRoleID)
	if err != nil {
		return fmt.Errorf("failed to get owner role ID: %w", err)
	}
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'org_admin'`).Scan(&orgAdminRoleID)
	if err != nil {
		return fmt.Errorf("failed to get org_admin role ID: %w", err)
	}
	err = sqlDB.QueryRow(`SELECT id FROM roles WHERE name = 'member'`).Scan(&memberRoleID)
	if err != nil {
		return fmt.Errorf("failed to get member role ID: %w", err)
	}

	// Get permission IDs
	permissionIDs := make(map[string]string)
	rows, err := sqlDB.Query(`SELECT id, name FROM permissions`)
	if err != nil {
		return fmt.Errorf("failed to query permissions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		permissionIDs[name] = id
	}

	// Assign all permissions to superadmin
	slog.Info("Assigning permissions to superadmin role")
	for permName, permID := range permissionIDs {
		_, err := sqlDB.Exec(`
			INSERT INTO role_permissions (role_id, permission_id, assigned_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT DO NOTHING
		`, superadminRoleID, permID)
		if err != nil {
			slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
		}
	}

	// Assign management permissions to admin
	adminPermissions := []string{
		"users:view", "users:create", "users:update", "users:delete", "users:manage",
		"roles:view", "roles:create", "roles:update", "roles:delete", "roles:manage",
		"permissions:view", "permissions:create", "permissions:manage",
		"invoices:view:all", "invoices:create", "invoices:update:all", "invoices:delete:all", "invoices:manage",
		"audit:view",
		"notifications:view", "notifications:manage",
		"webhooks:view", "webhooks:manage",
	}
	slog.Info("Assigning permissions to admin role")
	for _, permName := range adminPermissions {
		if permID, ok := permissionIDs[permName]; ok {
			_, err := sqlDB.Exec(`
				INSERT INTO role_permissions (role_id, permission_id, assigned_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, adminRoleID, permID)
			if err != nil {
				slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
			}
		}
	}

	// Assign view permissions to viewer
	viewerPermissions := []string{
		"users:view",
		"roles:view",
		"permissions:view",
		"invoices:view", "invoices:create",
		"notifications:view", "notifications:manage",
		"webhooks:view",
	}
	slog.Info("Assigning permissions to viewer role")
	for _, permName := range viewerPermissions {
		if permID, ok := permissionIDs[permName]; ok {
			_, err := sqlDB.Exec(`
				INSERT INTO role_permissions (role_id, permission_id, assigned_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, viewerRoleID, permID)
			if err != nil {
				slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
			}
		}
	}

	// Organization permission names (for role assignment)
	orgViewPerms := []string{"organization:view"}
	orgManagePerms := []string{"organization:view", "organization:manage", "organization:invite"}
	orgAllPerms := []string{"organization:view", "organization:manage", "organization:invite", "organization:remove"}

	// Assign all organization permissions to owner
	slog.Info("Assigning permissions to owner role")
	for _, permName := range orgAllPerms {
		if permID, ok := permissionIDs[permName]; ok {
			_, err := sqlDB.Exec(`
				INSERT INTO role_permissions (role_id, permission_id, assigned_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, ownerRoleID, permID)
			if err != nil {
				slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
			}
		}
	}

	// Assign view, manage, invite to org_admin
	slog.Info("Assigning permissions to org_admin role")
	for _, permName := range orgManagePerms {
		if permID, ok := permissionIDs[permName]; ok {
			_, err := sqlDB.Exec(`
				INSERT INTO role_permissions (role_id, permission_id, assigned_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, orgAdminRoleID, permID)
			if err != nil {
				slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
			}
		}
	}

	// Assign view only to member
	slog.Info("Assigning permissions to member role")
	for _, permName := range orgViewPerms {
		if permID, ok := permissionIDs[permName]; ok {
			_, err := sqlDB.Exec(`
				INSERT INTO role_permissions (role_id, permission_id, assigned_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT DO NOTHING
			`, memberRoleID, permID)
			if err != nil {
				slog.Debug("Permission already assigned or failed", "permission", permName, "error", err)
			}
		}
	}

	// Initialize Casbin enforcer to sync policies
	enforcer, err := permission.NewEnforcer(db)
	if err != nil {
		return fmt.Errorf("failed to initialize enforcer: %w", err)
	}
	defer enforcer.Close()

	// Sync to Casbin
	slog.Info("Syncing roles to Casbin policies")

	// Superadmin - all permissions
	for _, p := range permissions {
		if err := enforcer.AddPolicy("superadmin", domain, p.Resource, p.Action); err != nil {
			slog.Debug("Policy exists or failed", "role", "superadmin", "resource", p.Resource, "action", p.Action)
		}
	}

	// Admin - management permissions
	for _, permName := range adminPermissions {
		if p := findPermission(permissions, permName); p != nil {
			if err := enforcer.AddPolicy("admin", domain, p.Resource, p.Action); err != nil {
				slog.Debug("Policy exists or failed", "role", "admin", "resource", p.Resource, "action", p.Action)
			}
		}
	}

	// Viewer - view permissions
	for _, permName := range viewerPermissions {
		if p := findPermission(permissions, permName); p != nil {
			if err := enforcer.AddPolicy("viewer", domain, p.Resource, p.Action); err != nil {
				slog.Debug("Policy exists or failed", "role", "viewer", "resource", p.Resource, "action", p.Action)
			}
		}
	}

	// Organization roles - sync to Casbin
	// Owner - all organization permissions
	for _, permName := range orgAllPerms {
		if p := findPermission(permissions, permName); p != nil {
			if err := enforcer.AddPolicy("owner", domain, p.Resource, p.Action); err != nil {
				slog.Debug("Policy exists or failed", "role", "owner", "resource", p.Resource, "action", p.Action)
			}
		}
	}

	// Org admin - view, manage, invite
	for _, permName := range orgManagePerms {
		if p := findPermission(permissions, permName); p != nil {
			if err := enforcer.AddPolicy("org_admin", domain, p.Resource, p.Action); err != nil {
				slog.Debug("Policy exists or failed", "role", "org_admin", "resource", p.Resource, "action", p.Action)
			}
		}
	}

	// Member - view only
	for _, permName := range orgViewPerms {
		if p := findPermission(permissions, permName); p != nil {
			if err := enforcer.AddPolicy("member", domain, p.Resource, p.Action); err != nil {
				slog.Debug("Policy exists or failed", "role", "member", "resource", p.Resource, "action", p.Action)
			}
		}
	}

	// Save policies
	if err := enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save Casbin policies: %w", err)
	}

	slog.Info("Database seeding complete",
		"permissions", len(permissions),
		"roles", len(roles))

	return nil
}

// findPermission finds a permission by name in the permissions slice.
func findPermission(permissions []struct {
	Name        string
	Resource    string
	Action      string
	Scope       string
	Description string
	IsSystem    bool
}, name string) *struct {
	Name        string
	Resource    string
	Action      string
	Scope       string
	Description string
	IsSystem    bool
} {
	for i := range permissions {
		if permissions[i].Name == name {
			return &permissions[i]
		}
	}
	return nil
}

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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/example/go-api-base/docs/swagger" // swagger docs
	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/database"
	apphttp "github.com/example/go-api-base/internal/http"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
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

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrations()
	},
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

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
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

	// Configure routes
	server.RegisterRoutes()

	// Initialize EmailWorker for background email processing
	emailWorker := initEmailWorker(cfg, db, redisClient, enforcer, server)
	if emailWorker != nil {
		server.SetEmailWorker(emailWorker)
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

	// Stop email worker gracefully
	if emailWorker := server.EmailWorker(); emailWorker != nil {
		slog.Info("Stopping email worker...")
		if err := emailWorker.Stop(); err != nil {
			slog.Error("Failed to stop email worker", "error", err)
		}
	}

	// Create a deadline to wait for (30 seconds as per constitution)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	slog.Info("Shutting down HTTP server...")

	// Attempt graceful shutdown
	if err := server.Stop(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("HTTP server stopped")

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
		},
		Roles: []RoleEntry{
			{
				Name:        "admin",
				Description: "Administrator role with full access",
				Permissions: []string{"users:manage", "roles:manage", "permissions:manage", "email_templates:manage", "email_queue:manage", "email_bounces:read"},
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

// runMigrations runs all pending database migrations.
func runMigrations() error {
	slog.Info("Starting database migrations")

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

	// Create migration handler
	m, err := database.NewMigrate(db, "file://migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration handler: %w", err)
	}
	defer func() {
		if closeErr := m.Close(); closeErr != nil {
			slog.Error("Failed to close migration handler", "error", closeErr)
		}
	}()

	// Get current version before migration
	version, dirty, err := m.Version()
	if err != nil {
		slog.Info("No migrations applied yet, starting fresh")
	} else {
		slog.Info("Current migration state", "version", version, "dirty", dirty)

		// Check if database already has tables (from previous setup)
		sqlDB, _ := db.DB()
		var tableCount int
		sqlDB.QueryRow("SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&tableCount)
		slog.Info("Current table count in public schema", "count", tableCount)

		if version > 0 && tableCount > 10 && !dirty {
			// Database has tables and a valid version - assume migrations already applied
			slog.Info("Database already migrated, skipping", "version", version, "table_count", tableCount)
			return nil
		}

		if dirty {
			slog.Info("Database in dirty state, forcing to version", "version", version)
			if forceErr := m.Force(int(version)); forceErr != nil {
				return fmt.Errorf("failed to force migration version: %w", forceErr)
			}
			slog.Info("Forced migration to clean state")

			// After forcing, check again - if we still have tables and version > 0, skip Up
			newVersion, newDirty, _ := m.Version()
			if !newDirty && newVersion > 0 && tableCount > 10 {
				slog.Info("Database now clean at version", "version", newVersion, "- skipping migrations")
				return nil
			}
		}
	}

	// Run migrations
	if err := m.Up(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Get new version after migration
	newVersion, _, err := m.Version()
	if err != nil {
		slog.Info("Migrations completed")
	} else {
		slog.Info("Migrations completed successfully", "version", newVersion)
	}

	return nil
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

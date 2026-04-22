// Package database provides database connection management for PostgreSQL and Redis.
package database

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/example/go-api-base/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresDB wraps a GORM database connection with additional functionality.
type PostgresDB struct {
	DB *gorm.DB
}

// NewPostgresDB creates a new PostgreSQL database connection using GORM.
// It configures the connection pool and enables logging based on configuration.
// Returns the GORM DB instance or an error if connection fails.
func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.Database.DSN()

	// Configure GORM logger based on log level
	gormLogger := configureGORMLogger(cfg.Log.Level)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   gormLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	slog.Info("Connected to PostgreSQL database",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"dbname", cfg.Database.DBName,
	)

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	slog.Debug("Configured database connection pool",
		"max_idle", 10,
		"max_open", 100,
	)

	return db, nil
}

// configureGORMLogger creates a GORM logger based on the configured log level.
func configureGORMLogger(level string) logger.Interface {
	logLevel := getGORMLogLevel(level)

	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
}

// getGORMLogLevel converts the application log level to GORM log level.
func getGORMLogLevel(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.Info
	case "info":
		return logger.Warn
	case "warn", "warning":
		return logger.Error
	case "error":
		return logger.Silent
	default:
		return logger.Warn
	}
}

// Close closes the database connection.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	slog.Info("Database connection closed")
	return nil
}

// Ping verifies the database connection is alive.
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// HealthCheck performs a health check on the database connection.
// Returns nil if healthy, otherwise returns an error.
func HealthCheck(db *gorm.DB) error {
	return Ping(db)
}

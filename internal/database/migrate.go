// Package database provides database connectivity and migration management.
package database

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // File source driver
	"gorm.io/gorm"
)

// Migrate handles database migrations using golang-migrate.
type Migrate struct {
	m *migrate.Migrate
}

// NewMigrate creates a new migration handler.
//
// Parameters:
//   - db: GORM database connection
//   - migrationsPath: Path to migration files (e.g., "file://migrations")
//
// Returns:
//   - *Migrate: Migration handler
//   - error: If migration initialization fails
func NewMigrate(db *gorm.DB, migrationsPath string) (*Migrate, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrate{m: m}, nil
}

// Up runs all pending migrations.
//
// Returns:
//   - error: If migration fails (or migrate.ErrNoChange if no migrations to run)
func (m *Migrate) Up() error {
	if err := m.m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil // No migrations to run is not an error
		}
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

// Down rolls back all migrations.
//
// Returns:
//   - error: If rollback fails (or migrate.ErrNoChange if no migrations to rollback)
func (m *Migrate) Down() error {
	if err := m.m.Down(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil // No migrations to rollback is not an error
		}
		return fmt.Errorf("migration down failed: %w", err)
	}
	return nil
}

// Version returns the current migration version.
//
// Returns:
//   - version: Current migration version number
//   - dirty: True if migration is in a dirty state
//   - error: If version check fails
func (m *Migrate) Version() (uint, bool, error) {
	version, dirty, err := m.m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil // No migrations run yet
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// Steps migrates up or down by the specified number of steps.
// Use negative steps to migrate down.
//
// Parameters:
//   - steps: Number of migrations to apply (positive) or rollback (negative)
//
// Returns:
//   - error: If migration fails
func (m *Migrate) Steps(steps int) error {
	if err := m.m.Steps(steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("migration steps failed: %w", err)
	}
	return nil
}

// Close closes the migration instance and releases resources.
//
// Returns:
//   - error: If close fails
func (m *Migrate) Close() error {
	sourceErr, dbErr := m.m.Close()
	if sourceErr != nil {
		return fmt.Errorf("failed to close migration source: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("failed to close migration database: %w", dbErr)
	}
	return nil
}

// Force forces the migration version to the given value.
// This is useful to resolve dirty migration states.
//
// Parameters:
//   - version: Target version number
//
// Returns:
//   - error: If force fails
func (m *Migrate) Force(version int) error {
	if err := m.m.Force(version); err != nil {
		return fmt.Errorf("migration force failed: %w", err)
	}
	return nil
}

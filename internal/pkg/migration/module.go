package migration

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"

	"go.uber.org/zap"
)

// Runner defines the interface for running migrations
// This abstracts the implementation details
type Runner interface {
	// Up runs all available migrations
	Up() error
	// Down rolls back the last migration
	Down() error
	// Steps runs n migrations (positive for up, negative for down)
	Steps(n int) error
	// Version returns the current migration version
	Version() (uint, bool, error)
	// Force sets the migration version without running migrations
	Force(version int) error
}

// Migrator implements Runner interface using golang-migrate
type Migrator struct {
	migrate *migrate.Migrate
	log     *zap.Logger
}

// NewMigrator creates a new migrator instance
// migrationsPath: path to migration files (e.g., "migrations" or "service/notification/migrations")
func NewMigrator(db *sql.DB, migrationsPath string, log *zap.Logger) (*Migrator, error) {
	// Create postgres driver instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("create postgres driver: %w", err)
	}

	// Get absolute path for migrations
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	// Create file source instance
	source, err := (&file.File{}).Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open migration source: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance("file", source, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}

	return &Migrator{
		migrate: m,
		log:     log,
	}, nil
}

// Up implements Runner interface
func (m *Migrator) Up() error {
	m.log.Info("Running migrations up")
	if err := m.migrate.Up(); err != nil {
		if err == migrate.ErrNoChange {
			m.log.Info("No migrations to run")
			return nil
		}
		return fmt.Errorf("migrate up: %w", err)
	}
	m.log.Info("Migrations completed successfully")
	return nil
}

// Down implements Runner interface
func (m *Migrator) Down() error {
	m.log.Info("Rolling back last migration")
	if err := m.migrate.Down(); err != nil {
		if err == migrate.ErrNoChange {
			m.log.Info("No migrations to rollback")
			return nil
		}
		return fmt.Errorf("migrate down: %w", err)
	}
	m.log.Info("Migration rollback completed")
	return nil
}

// Steps implements Runner interface
func (m *Migrator) Steps(n int) error {
	if n > 0 {
		m.log.Info("Running migrations forward", zap.Int("steps", n))
	} else {
		m.log.Info("Running migrations backward", zap.Int("steps", -n))
	}
	if err := m.migrate.Steps(n); err != nil {
		if err == migrate.ErrNoChange {
			m.log.Info("No migrations to run")
			return nil
		}
		return fmt.Errorf("migrate steps: %w", err)
	}
	m.log.Info("Migrations completed", zap.Int("steps", n))
	return nil
}

// Version implements Runner interface
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("get version: %w", err)
	}
	return version, dirty, nil
}

// Force implements Runner interface
func (m *Migrator) Force(version int) error {
	m.log.Info("Forcing migration version", zap.Int("version", version))
	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("force version: %w", err)
	}
	m.log.Info("Migration version forced", zap.Int("version", version))
	return nil
}

// Close closes the migrator
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return fmt.Errorf("close source: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("close database: %w", dbErr)
	}
	return nil
}

// RunMigrations is a helper function to run migrations for a service
// It handles the full migration lifecycle including dirty state detection
func RunMigrations(db *sql.DB, migrationsPath string, log *zap.Logger) error {
	log.Info("Running migrations", zap.String("path", migrationsPath))

	// Create migrator
	migrator, err := NewMigrator(db, migrationsPath, log)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		if closeErr := migrator.Close(); closeErr != nil {
			log.Warn("Failed to close migrator", zap.Error(closeErr))
		}
	}()

	// Check current version
	version, dirty, err := migrator.Version()
	if err != nil {
		return fmt.Errorf("get migration version: %w", err)
	}
	if dirty {
		log.Warn("Database is in dirty state, forcing version", zap.Uint("version", version))
		if err := migrator.Force(int(version)); err != nil {
			return fmt.Errorf("force version: %w", err)
		}
	}

	// Run migrations up
	if err := migrator.Up(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Log final version
	finalVersion, _, err := migrator.Version()
	if err != nil {
		log.Warn("Failed to get final version", zap.Error(err))
	} else {
		log.Info("Migrations completed", zap.Uint("version", finalVersion))
	}

	return nil
}

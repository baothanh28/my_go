package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/migration"
)

// RunMigrations runs notification service migrations using SQL files
// All migrations (tables, triggers, functions) are defined in SQL files
func RunMigrations(db *database.Database, log *logger.Logger) error {
	log.Info("Running notification migrations from SQL files")

	// Get SQL DB connection for golang-migrate
	sqlDB, err := db.SQLDB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	// Get migration path
	// In Docker container, working directory is /app, so migration files are at:
	// /app/internal/service/notification/migration
	// When running locally, use relative path from project root
	migrationsPath := "internal/service/notification/migration"

	// Try to resolve absolute path (works in Docker container)
	if absPath, err := filepath.Abs(migrationsPath); err == nil {
		// Check if path exists
		if _, err := os.Stat(absPath); err == nil {
			migrationsPath = absPath
		}
	}

	// Use golang-migrate to run SQL migrations
	// This handles dollar quoting ($$), triggers, functions, etc. correctly
	return migration.RunMigrations(sqlDB, migrationsPath, log.Logger)
}

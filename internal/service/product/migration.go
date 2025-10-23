package product

import (
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
)

// RunMigrations runs product service migrations
func RunMigrations(db *database.Database, log *logger.Logger) error {
	log.Info("Running product migrations")
	return db.RunMigrations(&Product{})
}

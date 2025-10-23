package auth

import (
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
)

// RunMigrations runs auth service migrations
func RunMigrations(db *database.Database, log *logger.Logger) error {
	log.Info("Running auth migrations")
	return db.RunMigrations(&User{})
}

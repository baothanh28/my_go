package supabase

import (
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
)

// RunMigrations runs supabase_login service migrations
func RunMigrations(db *database.Database, log *logger.Logger) error {
	log.Info("Running supabase_login migrations")
	return db.RunMigrations(&User{})
}

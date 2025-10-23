package database

import (
	"context"

	"myapp/internal/pkg/logger"

	"go.uber.org/fx"
)

// Module exports the database module for FX
var Module = fx.Module("database",
	fx.Provide(NewDatabase),
	fx.Invoke(registerHooks),
)

// registerHooks registers lifecycle hooks for database
func registerHooks(lc fx.Lifecycle, db *Database, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Database module started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Closing database connection")
			return db.Close()
		},
	})
}

package main

import (
	"fmt"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/service/notification"
	"myapp/internal/service/notification/repository"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

// newMigrateCmd creates the migrate command
func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run notification database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrations()
		},
	}
}

// runMigrations runs database migrations
func runMigrations() error {
	fmt.Println("Running notification service migrations...")

	var log *logger.Logger
	var db *database.Database

	app := fx.New(
		notification.NotificationAppMigration, // Use minimal migration app (no worker, server)
		fx.NopLogger,
		fx.Invoke(func(logger *logger.Logger, database *database.Database) {
			log = logger
			db = database
		}),
	)

	if err := startApp(app, "migration"); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	if err := repository.RunMigrations(db, log); err != nil {
		_ = stopApp(app, "migration")
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("Notification migrations completed successfully!")

	return stopApp(app, "migration")
}


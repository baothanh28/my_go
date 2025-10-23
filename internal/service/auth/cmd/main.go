package main

import (
	"context"
	"fmt"
	"os"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/service/auth"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

var (
	version = "1.0.0"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "auth-service",
	Short: "Authentication service",
	Long:  `Standalone authentication service with user management and JWT tokens.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the auth service",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run auth database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		runMigrations()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Auth Service Version: %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runServer() {
	app := fx.New(
		auth.AuthApp,
		fx.NopLogger,
	)

	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start auth service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Auth service started successfully on http://localhost:8081")

	<-app.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop auth service: %v\n", err)
		os.Exit(1)
	}
}

func runMigrations() {
	fmt.Println("Running auth service migrations...")

	var log *logger.Logger
	var db *database.Database

	app := fx.New(
		auth.AuthApp,
		fx.NopLogger,
		fx.Invoke(func(logger *logger.Logger, database *database.Database) {
			log = logger
			db = database
		}),
	)

	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	if err := auth.RunMigrations(db, log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Auth migrations completed successfully!")

	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"fmt"
	"os"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/service/supabase"

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
	Use:   "supabase-login-service",
	Short: "Supabase login service",
	Long:  `Standalone service to exchange Supabase tokens for app JWTs.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supabase login service",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		runMigrations()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Supabase Login Service Version: %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runServer() {
	app := fx.New(
		supabase.App,
		fx.NopLogger,
	)
	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()
	if err := app.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Supabase login service started on http://localhost:8081")
	<-app.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop: %v\n", err)
		os.Exit(1)
	}
}

func runMigrations() {
	fmt.Println("Running supabase_login migrations...")
	var log *logger.Logger
	var db *database.Database
	app := fx.New(
		supabase.App,
		fx.NopLogger,
		fx.Invoke(func(l *logger.Logger, d *database.Database) {
			log = l
			db = d
		}),
	)
	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()
	if err := app.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	if err := supabase.RunMigrations(db, log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run migrations: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Migrations completed successfully!")
	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()
	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop: %v\n", err)
		os.Exit(1)
	}
}

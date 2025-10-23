package main

import (
	"context"
	"fmt"
	"os"

	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/service/product"

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
	Use:   "product-service",
	Short: "Product management service",
	Long:  `Standalone product service with CRUD operations for products.`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the product service",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run product database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		runMigrations()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Product Service Version: %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runServer() {
	app := fx.New(
		product.ProductApp,
		fx.NopLogger,
	)

	startCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start product service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Product service started successfully on http://localhost:8082")

	<-app.Done()

	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop product service: %v\n", err)
		os.Exit(1)
	}
}

func runMigrations() {
	fmt.Println("Running product service migrations...")

	var log *logger.Logger
	var db *database.Database

	app := fx.New(
		product.ProductApp,
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

	if err := product.RunMigrations(db, log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Product migrations completed successfully!")

	stopCtx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop: %v\n", err)
		os.Exit(1)
	}
}

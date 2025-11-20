package main

import (
	"fmt"

	"myapp/internal/service/notification"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

// newAPICmd creates the API-only command
func newAPICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "Start the notification API server only (without worker)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI()
		},
	}
}

// runAPI starts the API server without worker
func runAPI() error {
	app := fx.New(
		notification.NotificationAppAPIOnly,
		fx.NopLogger,
	)

	if err := startApp(app, "notification API server"); err != nil {
		return err
	}

	fmt.Printf("Notification API server started successfully on http://localhost:%s\n", defaultPort)
	fmt.Println("Worker is NOT running - only API endpoints are available")

	<-app.Done()

	return stopApp(app, "notification API server")
}


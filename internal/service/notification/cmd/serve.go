package main

import (
	"fmt"

	"myapp/internal/service/notification"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

// newServeCmd creates the serve command
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the notification service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer()
		},
	}
}

// runServer starts the full notification service with worker
func runServer() error {
	app := fx.New(
		notification.NotificationApp,
		fx.NopLogger,
	)

	if err := startApp(app, "notification service"); err != nil {
		return err
	}

	fmt.Printf("Notification service started successfully on http://localhost:%s\n", defaultPort)
	<-app.Done()

	return stopApp(app, "notification service")
}


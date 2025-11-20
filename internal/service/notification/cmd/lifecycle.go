package main

import (
	"context"
	"fmt"

	"go.uber.org/fx"
)

// startApp starts an fx application with proper context handling
func startApp(app *fx.App, serviceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start %s: %w", serviceName, err)
	}

	return nil
}

// stopApp stops an fx application with proper context handling
func stopApp(app *fx.App, serviceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), fx.DefaultTimeout)
	defer cancel()

	if err := app.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop %s: %w", serviceName, err)
	}

	return nil
}


package server

import (
	"context"
	"time"

	"myapp/internal/pkg/logger"

	"go.uber.org/fx"
)

// Module exports the server module for FX
var Module = fx.Module("server",
	fx.Provide(
		NewEchoServer,
	),
	fx.Invoke(registerHooks),
)

// registerHooks registers lifecycle hooks for server
func registerHooks(lc fx.Lifecycle, server *Server, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start server in a goroutine
			go func() {
				if err := server.Start(); err != nil {
					log.Error("Server error")
				}
			}()
			log.Info("Server module started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// Create shutdown context with timeout
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			log.Info("Stopping server")
			return server.Shutdown(shutdownCtx)
		},
	})
}

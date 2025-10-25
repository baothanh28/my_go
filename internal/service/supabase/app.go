package supabase

import (
	"myapp/internal/pkg/config"
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"

	"go.uber.org/fx"
)

// App provides all supabase_login service dependencies including infrastructure
var App = fx.Options(
	// Infrastructure modules
	config.Module,
	logger.Module,
	database.Module,
	server.Module,

	// Service components
	fx.Provide(
		NewServiceConfig,
		NewRepository,
		NewService,
		NewHandler,
	),

	// Register routes
	fx.Invoke(registerRoutes),
)

// registerRoutes registers routes on the Echo server
func registerRoutes(srv *server.Server, handler *Handler) {
	e := srv.GetEcho()
	RegisterRoutes(e, handler)
}

package auth

import (
	"myapp/internal/pkg/config"
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"

	"go.uber.org/fx"
)

// AuthApp provides all auth service dependencies including infrastructure
var AuthApp = fx.Options(
	// Infrastructure modules
	config.WithServiceDir("internal/service/auth"),
	config.Module,
	logger.Module,
	database.Module,
	server.Module,

	// Auth service components
	fx.Provide(
		NewServiceConfig,
		NewAuthRepository,
		NewAuthService,
		NewAuthHandler,
	),

	// Register routes
	fx.Invoke(registerAuthRoutes),
)

// registerAuthRoutes registers auth routes on the Echo server
func registerAuthRoutes(srv *server.Server, handler *AuthHandler) {
	e := srv.GetEcho()
	RegisterAuthRoutes(e, handler)
}

package product

import (
	"myapp/internal/pkg/config"
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"
	"myapp/internal/service/auth"

	"go.uber.org/fx"
)

// ProductApp provides all product service dependencies including infrastructure
var ProductApp = fx.Options(
	// Infrastructure modules
	config.Module,
	logger.Module,
	database.Module,
	server.Module,

	// Auth service (needed for JWT middleware)
	fx.Provide(
		auth.NewAuthService,
	),

	// Product service components
	fx.Provide(
		NewProductConfig,
		NewProductRepository,
		NewProductService,
		NewProductHandler,
	),

	// Register routes
	fx.Invoke(registerProductRoutes),
)

// registerProductRoutes registers product routes on the Echo server
func registerProductRoutes(srv *server.Server, handler *ProductHandler, authService *auth.AuthService) {
	e := srv.GetEcho()
	RegisterProductRoutes(e, handler, authService)
}

package health

import (
	"context"
	"database/sql"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/logger"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module exports the health module for FX
var Module = fx.Module("health",
	fx.Provide(NewHealthService),
	fx.Invoke(registerHooks),
)

// HealthServiceParams defines the dependencies for the health service
type HealthServiceParams struct {
	fx.In

	Config      *config.Config
	Logger      *logger.Logger
	DB          *sql.DB               `optional:"true"`
	RedisClient redis.UniversalClient `optional:"true"`
}

// NewHealthService constructs a new health service with auto-registered providers
func NewHealthService(params HealthServiceParams) *Service {
	// Create service configuration
	serviceConfig := DefaultServiceConfig()

	// Configure based on app config if available
	if params.Config != nil {
		// You can add health check configuration to your config.yaml
		// For now, use sensible defaults
		serviceConfig.AsyncMode = true
		serviceConfig.AggregationStrategy = StrategyAll
	}

	service := NewService(serviceConfig)

	// Auto-register database provider if available
	if params.DB != nil {
		dbProvider := NewPostgresProvider("database", params.DB)
		service.RegisterProvider(dbProvider)
		params.Logger.Info("Registered database health provider")
	}

	// Auto-register Redis provider if available
	if params.RedisClient != nil {
		redisProvider := NewRedisProvider(RedisProviderConfig{
			Name:       "redis",
			Client:     params.RedisClient,
			DegradedMS: 100,
		})
		service.RegisterProvider(redisProvider)
		params.Logger.Info("Registered Redis health provider")
	}

	params.Logger.Info("Health service initialized")
	return service
}

// registerHooks registers lifecycle hooks for the health service
func registerHooks(lc fx.Lifecycle, service *Service, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Health service started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Stopping health service")
			service.Stop()
			return nil
		},
	})
}

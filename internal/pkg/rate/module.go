package rate

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module exports the rate limiter module for FX
var Module = fx.Module("rate",
	fx.Provide(
		NewLimiterFromConfig,
		NewStorageFromConfig,
	),
	fx.Invoke(registerHooks),
)

// LimiterParams holds dependencies for creating a limiter
type LimiterParams struct {
	fx.In

	Config  *LimiterConfig
	Storage Storage
	Logger  Logger           `optional:"true"`
	Metrics MetricsCollector `optional:"true"`
}

// NewLimiterFromConfig creates a new limiter from configuration
func NewLimiterFromConfig(params LimiterParams) (Limiter, error) {
	if err := params.Config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rate limiter config: %w", err)
	}

	opts := []Option{}

	if params.Logger != nil {
		opts = append(opts, WithLogger(params.Logger))
	}

	if params.Metrics != nil {
		opts = append(opts, WithMetrics(params.Metrics))
	}

	return New(params.Config.ToConfig(), params.Storage, opts...)
}

// StorageParams holds dependencies for creating storage
type StorageParams struct {
	fx.In

	Config      *LimiterConfig
	RedisClient *redis.Client `optional:"true"`
}

// NewStorageFromConfig creates a new storage from configuration
func NewStorageFromConfig(params StorageParams) (Storage, error) {
	if err := params.Config.Storage.Validate(); err != nil {
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}

	switch params.Config.Storage.Type {
	case "memory":
		return NewMemoryStorage(), nil

	case "redis":
		if params.RedisClient == nil {
			// Create new Redis client
			client := redis.NewClient(&redis.Options{
				Addr:     params.Config.Storage.Redis.Addr,
				Password: params.Config.Storage.Redis.Password,
				DB:       params.Config.Storage.Redis.DB,
				PoolSize: params.Config.Storage.Redis.PoolSize,
			})

			// Test connection
			if err := client.Ping(context.Background()).Err(); err != nil {
				return nil, fmt.Errorf("redis connection failed: %w", err)
			}

			return NewRedisStorage(client, params.Config.Storage.KeyPrefix), nil
		}

		return NewRedisStorage(params.RedisClient, params.Config.Storage.KeyPrefix), nil

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", params.Config.Storage.Type)
	}
}

// registerHooks registers lifecycle hooks
func registerHooks(lc fx.Lifecycle, limiter Limiter, logger Logger) {
	if logger == nil {
		logger = &NoOpLogger{}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("rate limiter module started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("closing rate limiter")
			return limiter.Close()
		},
	})
}

// ProvideLimiterConfig provides limiter configuration
func ProvideLimiterConfig(config *LimiterConfig) fx.Option {
	return fx.Provide(func() *LimiterConfig {
		return config
	})
}

// ProvideLogger provides a logger implementation
func ProvideLogger(logger Logger) fx.Option {
	return fx.Provide(func() Logger {
		return logger
	})
}

// ProvideMetrics provides a metrics collector implementation
func ProvideMetrics(metrics MetricsCollector) fx.Option {
	return fx.Provide(func() MetricsCollector {
		return metrics
	})
}

// NewHTTPMiddlewareFromConfig creates HTTP middleware from limiter
func NewHTTPMiddlewareFromConfig(limiter Limiter) *HTTPMiddleware {
	return NewHTTPMiddleware(limiter)
}

// NewGRPCInterceptorsFromConfig creates gRPC interceptors from limiter
type GRPCInterceptors struct {
	Unary  func() interface{}
	Stream func() interface{}
}

// ProvideHTTPMiddleware provides HTTP middleware
func ProvideHTTPMiddleware() fx.Option {
	return fx.Provide(NewHTTPMiddlewareFromConfig)
}

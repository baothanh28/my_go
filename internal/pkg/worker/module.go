package worker

import (
	"context"

	"myapp/internal/pkg/logger"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module exports the worker module for FX
var Module = fx.Module("worker")

// Params holds the dependencies for creating a worker
type Params struct {
	fx.In

	Redis  *redisv9.Client
	Logger *logger.Logger
	Config *WorkerModuleConfig `optional:"true"`
}

// WorkerModuleConfig holds configuration for the worker module
type WorkerModuleConfig struct {
	// Worker configuration
	Worker Config

	// Redis provider configuration
	Redis RedisProviderConfig

	// Enable default middlewares
	EnableLogging  bool
	EnableMetrics  bool
	EnableRecovery bool
	EnableTracing  bool
	EnableTimeout  bool
}

// DefaultWorkerModuleConfig returns a config with sensible defaults
func DefaultWorkerModuleConfig() *WorkerModuleConfig {
	return &WorkerModuleConfig{
		Worker:         DefaultConfig(),
		Redis:          DefaultRedisProviderConfig("tasks", "workers", "worker-1"),
		EnableLogging:  true,
		EnableMetrics:  true,
		EnableRecovery: true,
		EnableTracing:  true,
		EnableTimeout:  false, // Timeout is handled in worker.go
	}
}

// NewWorker creates a new worker with Redis provider
func NewWorker(p Params) (*Worker, error) {
	// Use default config if not provided
	config := p.Config
	if config == nil {
		config = DefaultWorkerModuleConfig()
	}

	// Create Redis provider
	provider, err := NewRedisProvider(p.Redis, config.Redis, p.Logger)
	if err != nil {
		return nil, err
	}

	// Create worker
	worker := New(provider, config.Worker, p.Logger)

	// Register default middlewares
	if config.EnableRecovery {
		worker.Use(RecoveryMiddleware(p.Logger))
	}
	if config.EnableLogging {
		worker.Use(LoggingMiddleware(p.Logger))
	}
	if config.EnableMetrics {
		metricsCollector := NewMetricsCollector(p.Logger)
		worker.Use(MetricsMiddleware(metricsCollector))
	}
	if config.EnableTracing {
		worker.Use(TracingMiddleware())
	}
	if config.EnableTimeout {
		worker.Use(TimeoutMiddleware(p.Logger))
	}

	return worker, nil
}

// ProvideWorker provides a worker to the FX container
func ProvideWorker() fx.Option {
	return fx.Options(
		fx.Provide(NewWorker),
		fx.Invoke(registerHooks),
	)
}

// registerHooks registers lifecycle hooks for the worker
func registerHooks(lc fx.Lifecycle, worker *Worker, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Worker module starting")
			// Start worker in a goroutine
			go func() {
				if err := worker.Start(ctx); err != nil {
					log.Error("Worker stopped with error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Worker module stopping")
			return worker.Stop(ctx)
		},
	})
}

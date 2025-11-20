package scheduler

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module provides scheduler dependencies for fx.
var Module = fx.Module("scheduler",
	fx.Provide(
		NewSchedulerFromConfig,
		NewExecutorFromConfig,
		NewLockFromConfig,
		NewBackendFromConfig,
	),
)

// SchedulerParams holds dependencies for creating a scheduler.
type SchedulerParams struct {
	fx.In

	Config  *SchedulerConfig
	Backend BackendProvider
	Logger  Logger           `optional:"true"`
	Metrics MetricsCollector `optional:"true"`
}

// NewSchedulerFromConfig creates a new scheduler from configuration.
func NewSchedulerFromConfig(params SchedulerParams) (Scheduler, error) {
	logger := params.Logger
	if logger == nil {
		logger = &NoOpLogger{}
	}

	metrics := params.Metrics
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}

	executor := NewDefaultJobExecutor(logger, metrics)
	lock := NewDistributedLock(params.Backend, logger, metrics).
		WithLockTTL(params.Config.LockTTL).
		WithRefreshInterval(params.Config.LockRefreshInterval)

	config := &Config{
		TickInterval:  params.Config.TickInterval,
		MaxConcurrent: params.Config.MaxConcurrent,
	}

	return NewScheduler(params.Backend, executor, lock, logger, metrics, config), nil
}

// ExecutorParams holds dependencies for creating an executor.
type ExecutorParams struct {
	fx.In

	Logger  Logger           `optional:"true"`
	Metrics MetricsCollector `optional:"true"`
}

// NewExecutorFromConfig creates a new executor from configuration.
func NewExecutorFromConfig(params ExecutorParams) JobExecutor {
	logger := params.Logger
	if logger == nil {
		logger = &NoOpLogger{}
	}

	metrics := params.Metrics
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}

	return NewDefaultJobExecutor(logger, metrics)
}

// LockParams holds dependencies for creating a distributed lock.
type LockParams struct {
	fx.In

	Config  *SchedulerConfig
	Backend BackendProvider
	Logger  Logger           `optional:"true"`
	Metrics MetricsCollector `optional:"true"`
}

// NewLockFromConfig creates a new distributed lock from configuration.
func NewLockFromConfig(params LockParams) *DistributedLock {
	logger := params.Logger
	if logger == nil {
		logger = &NoOpLogger{}
	}

	metrics := params.Metrics
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}

	return NewDistributedLock(params.Backend, logger, metrics).
		WithLockTTL(params.Config.LockTTL).
		WithRefreshInterval(params.Config.LockRefreshInterval)
}

// BackendParams holds dependencies for creating a backend.
type BackendParams struct {
	fx.In

	Config      *SchedulerConfig
	RedisClient *redis.Client `optional:"true"`
}

// NewBackendFromConfig creates a new backend from configuration.
func NewBackendFromConfig(params BackendParams) (BackendProvider, error) {
	if err := params.Config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	switch params.Config.BackendType {
	case "redis":
		if params.RedisClient == nil {
			// Create new Redis client
			client := redis.NewClient(&redis.Options{
				Addr:     params.Config.RedisAddr,
				Password: params.Config.RedisPassword,
				DB:       params.Config.RedisDB,
			})
			return NewRedisBackend(client), nil
		}
		return NewRedisBackend(params.RedisClient), nil

	case "memory":
		return NewMemoryBackend(), nil

	default:
		return nil, fmt.Errorf("unsupported backend type: %s", params.Config.BackendType)
	}
}

// ProvideSchedulerConfig provides scheduler configuration.
func ProvideSchedulerConfig(config *SchedulerConfig) fx.Option {
	return fx.Provide(func() *SchedulerConfig {
		return config
	})
}

// ProvideLogger provides a logger implementation.
func ProvideLogger(logger Logger) fx.Option {
	return fx.Provide(func() Logger {
		return logger
	})
}

// ProvideMetrics provides a metrics collector implementation.
func ProvideMetrics(metrics MetricsCollector) fx.Option {
	return fx.Provide(func() MetricsCollector {
		return metrics
	})
}

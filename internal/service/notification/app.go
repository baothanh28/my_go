package notification

import (
	"context"
	"fmt"
	"time"

	pkgconfig "myapp/internal/pkg/config"
	"myapp/internal/pkg/database"
	"myapp/internal/pkg/health"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"
	workerpkg "myapp/internal/pkg/worker"

	"myapp/internal/service/notification/channel"
	"myapp/internal/service/notification/config"
	"myapp/internal/service/notification/handler"
	"myapp/internal/service/notification/repository"
	"myapp/internal/service/notification/service"
	"myapp/internal/service/notification/worker"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Import sub-packages

// NotificationApp provides all notification service dependencies
var NotificationApp = fx.Options(
	// Infrastructure modules
	pkgconfig.WithServiceDir("internal/service/notification"),
	pkgconfig.Module,
	logger.Module,
	database.Module,
	server.Module,
	health.Module,
	// Removed: redis.Module, idempotency.Module

	// Notification service components
	fx.Provide(
		config.NewServiceConfig,
		repository.NewNotificationRepository,
		service.NewNotificationService,
		handler.NewNotificationHandler,
		provideInMemoryQueue,
		provideInMemoryProvider,
		provideNotificationPoller,
		provideNotificationWorker,
		channel.NewChannelRegistry,
	),

	// Register routes
	fx.Invoke(registerNotificationRoutes),

	// Register worker health provider
	fx.Invoke(provideWorkerHealthProvider),

	// Start background services
	fx.Invoke(startBackgroundServices),
)

// NotificationAppAPIOnly provides notification service dependencies without worker
var NotificationAppAPIOnly = fx.Options(
	// Infrastructure modules
	pkgconfig.WithServiceDir("internal/service/notification"),
	pkgconfig.Module,
	logger.Module,
	database.Module,
	server.Module,

	// Notification service components (không có worker)
	fx.Provide(
		config.NewServiceConfig,
		repository.NewNotificationRepository,
		service.NewNotificationService,
		handler.NewNotificationHandler,
		// Không include: NewSenderRegistry, provideInMemoryQueue, provideInMemoryProvider,
		// provideNotificationPoller, provideNotificationWorker
	),

	// Register routes
	fx.Invoke(registerNotificationRoutes),

	// KHÔNG invoke startBackgroundServices - chỉ chạy API server
)

// NotificationAppMigration provides minimal dependencies for running migrations
// Only includes: config, logger, database (no server, worker, or health checks)
var NotificationAppMigration = fx.Options(
	// Infrastructure modules (minimal)
	pkgconfig.WithServiceDir("internal/service/notification"),
	pkgconfig.Module,
	logger.Module,
	database.Module,
	// No server, worker, health, or other services
)

// InMemoryQueueParams holds dependencies for creating in-memory queue
type InMemoryQueueParams struct {
	fx.In
	Config *config.ServiceConfig
}

// provideInMemoryQueue provides an in-memory queue
func provideInMemoryQueue(params InMemoryQueueParams) *worker.InMemoryQueue {
	queueSize := params.Config.Notification.Poller.MaxQueueSize
	if queueSize <= 0 {
		queueSize = 2000 // Default
	}
	return worker.NewInMemoryQueue(queueSize)
}

// InMemoryProviderParams holds dependencies for creating in-memory provider
type InMemoryProviderParams struct {
	fx.In
	Queue  *worker.InMemoryQueue
	Repo   *repository.NotificationRepository
	Logger *logger.Logger
}

// provideInMemoryProvider provides an in-memory provider
func provideInMemoryProvider(params InMemoryProviderParams) workerpkg.Provider {
	return worker.NewInMemoryProvider(
		params.Queue,
		params.Repo,
		params.Logger,
	)
}

// NotificationPollerParams holds dependencies for creating a notification poller
type NotificationPollerParams struct {
	fx.In
	DB     *database.Database
	Repo   *repository.NotificationRepository
	Queue  *worker.InMemoryQueue
	Config *config.ServiceConfig
	Logger *logger.Logger
}

// provideNotificationPoller provides a notification poller
func provideNotificationPoller(params NotificationPollerParams) *worker.NotificationPoller {
	return worker.NewNotificationPoller(
		params.DB,
		params.Repo,
		params.Queue,
		params.Config,
		params.Logger,
	)
}

// NotificationWorkerParams holds dependencies for creating a notification worker
type NotificationWorkerParams struct {
	fx.In
	WorkerProvider  workerpkg.Provider
	Config          *config.ServiceConfig
	Logger          *logger.Logger
	Repository      *repository.NotificationRepository
	ChannelRegistry *channel.ChannelRegistry
	Queue           *worker.InMemoryQueue
}

// provideNotificationWorker provides a notification worker
func provideNotificationWorker(params NotificationWorkerParams) (*worker.NotificationWorker, error) {
	return worker.NewNotificationWorker(
		params.WorkerProvider,
		params.Config,
		params.Logger,
		params.Repository,
		params.ChannelRegistry,
		params.Queue,
	)
}

// WorkerHealthProviderParams holds dependencies for creating worker health provider
type WorkerHealthProviderParams struct {
	fx.In
	HealthService *health.Service
	Worker        *worker.NotificationWorker
	Config        *config.ServiceConfig
	Logger        *logger.Logger
}

// provideWorkerHealthProvider registers worker health provider
func provideWorkerHealthProvider(params WorkerHealthProviderParams) error {
	// Create worker health checker adapter
	checker := &workerHealthCheckerAdapter{
		worker: params.Worker,
	}

	// Create worker health provider
	maxQueueSize := params.Config.Notification.Poller.MaxQueueSize

	workerProvider := health.NewWorkerProvider(health.WorkerProviderConfig{
		Name:           "notification-worker",
		Checker:        checker,
		MaxQueueLength: maxQueueSize,
	})

	// Register with health service
	params.HealthService.RegisterProvider(workerProvider)
	params.Logger.Info("Registered notification worker health provider")

	return nil
}

// workerHealthCheckerAdapter adapts NotificationWorker to WorkerHealthChecker interface
type workerHealthCheckerAdapter struct {
	worker *worker.NotificationWorker
}

func (a *workerHealthCheckerAdapter) IsRunning() bool {
	return a.worker.IsRunning()
}

func (a *workerHealthCheckerAdapter) GetQueueLength() int {
	return a.worker.GetQueueLength()
}

func (a *workerHealthCheckerAdapter) GetQueueCapacity() int {
	return a.worker.GetQueueCapacity()
}

// NotificationRoutesParams holds dependencies for registering routes
type NotificationRoutesParams struct {
	fx.In
	Server  *server.Server
	Handler *handler.NotificationHandler
}

// registerNotificationRoutes registers notification routes
func registerNotificationRoutes(params NotificationRoutesParams) {
	e := params.Server.GetEcho()
	// For now, register without auth middleware - can be added later

	// Basic routes without auth for now
	protectedGroup := e.Group("/api/v1/notifications")
	protectedGroup.POST("", params.Handler.CreateNotification)
	protectedGroup.GET("/users/:user_id/failed", params.Handler.GetFailedNotifications)
	protectedGroup.GET("/failed", params.Handler.GetFailedNotifications)
	protectedGroup.POST("/:id/retry", params.Handler.RetryNotification)

	// New token registration route
	protectedGroup.POST("/tokens/register", params.Handler.RegisterToken)
}

// BackgroundServicesParams holds dependencies for starting background services
type BackgroundServicesParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Poller    *worker.NotificationPoller
	Worker    *worker.NotificationWorker
	Repo      *repository.NotificationRepository
	Config    *config.ServiceConfig
	Logger    *logger.Logger
}

// startBackgroundServices starts background services (poller, worker)
func startBackgroundServices(params BackgroundServicesParams) {
	// Create controlled contexts for poller and worker
	pollerCtx, pollerCancel := context.WithCancel(context.Background())
	workerCtx, workerCancel := context.WithCancel(context.Background())

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start poller
			if params.Config.Notification.Poller.Enabled {
				if err := params.Poller.Start(pollerCtx); err != nil {
					return fmt.Errorf("failed to start poller: %w", err)
				}
				params.Logger.Info("Notification poller started")
			}

			// Start worker
			go func() {
				if err := params.Worker.Start(workerCtx); err != nil {
					params.Logger.Error("Worker stopped", zap.Error(err))
				}
			}()

			// Start background job to reset stale processing deliveries
			go func() {
				ticker := time.NewTicker(1 * time.Minute) // Check every minute
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						timeoutMinutes := params.Config.Notification.Poller.ProcessingTimeoutMinutes
						if timeoutMinutes <= 0 {
							timeoutMinutes = 5 // Default
						}
						if err := params.Repo.ResetProcessingToPending(timeoutMinutes); err != nil {
							params.Logger.Error("Failed to reset stale processing deliveries", zap.Error(err))
						}
					}
				}
			}()

			params.Logger.Info("Background services started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			// Stop poller
			if params.Config.Notification.Poller.Enabled {
				pollerCancel()
				stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := params.Poller.Stop(stopCtx); err != nil {
					params.Logger.Error("Failed to stop poller", zap.Error(err))
				}
			}

			// Stop worker
			workerCancel()
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := params.Worker.Stop(stopCtx); err != nil {
				params.Logger.Error("Failed to stop worker", zap.Error(err))
			}

			params.Logger.Info("Background services stopped")
			return nil
		},
	})
}

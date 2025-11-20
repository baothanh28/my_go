package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/worker"
	"myapp/internal/service/notification/channel"
	"myapp/internal/service/notification/config"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/repository"

	"go.uber.org/zap"
)

// NotificationWorker processes notifications from in-memory queue
type NotificationWorker struct {
	worker          *worker.Worker
	config          *config.ServiceConfig
	logger          *logger.Logger
	repo            *repository.NotificationRepository
	channelRegistry *channel.ChannelRegistry
	queue           *InMemoryQueue

	// Health check fields
	// Use atomic for lock-free reads (faster than RLock for simple bool)
	running int32 // 1 = running, 0 = stopped
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	workerProvider worker.Provider,
	config *config.ServiceConfig,
	log *logger.Logger,
	repo *repository.NotificationRepository,
	channelRegistry *channel.ChannelRegistry,
	queue *InMemoryQueue,
) (*NotificationWorker, error) {
	w := &NotificationWorker{
		config:          config,
		logger:          log,
		repo:            repo,
		channelRegistry: channelRegistry,
		queue:           queue,
		running:         0, // 0 = not running
	}

	// Create worker
	workerConfig := worker.Config{
		Concurrency:     config.Notification.WorkerConcurrency,
		PollInterval:    100 * time.Millisecond,
		ErrorBackoff:    1 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		BaseBackoff:     time.Duration(config.Notification.RetryBackoffSec) * time.Second,
		BackoffStrategy: worker.BackoffExponential,
	}

	w.worker = worker.New(workerProvider, workerConfig, log)

	// Register handler
	w.worker.Register("notification", w)

	return w, nil
}

// Process implements worker.Handler interface
func (w *NotificationWorker) Process(ctx context.Context, task *worker.Task) error {
	// Parse payload
	var payload model.NotificationPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		w.logger.Error("Failed to parse notification payload", zap.Error(err))
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	// Get delivery ID and target ID from metadata
	deliveryIDStr := task.Metadata["delivery_id"]
	if deliveryIDStr == "" {
		return fmt.Errorf("missing delivery_id in metadata")
	}

	targetIDStr := task.Metadata["target_id"]
	if targetIDStr == "" {
		return fmt.Errorf("missing target_id in metadata")
	}

	var deliveryID, targetID int64
	if _, err := fmt.Sscanf(deliveryIDStr, "%d", &deliveryID); err != nil {
		return fmt.Errorf("invalid delivery_id: %w", err)
	}
	if _, err := fmt.Sscanf(targetIDStr, "%d", &targetID); err != nil {
		return fmt.Errorf("invalid target_id: %w", err)
	}

	// Check idempotency (database-based)
	alreadyProcessed, err := w.repo.CheckIdempotency(deliveryID)
	if err != nil {
		w.logger.Error("Failed to check idempotency", zap.Error(err), zap.Int64("delivery_id", deliveryID))
		return fmt.Errorf("failed to check idempotency: %w", err)
	}

	if alreadyProcessed {
		w.logger.Info("Notification already processed", zap.Int64("delivery_id", deliveryID))
		return nil // Not an error, just skip
	}

	// Get target from database
	target, err := w.repo.GetTargetByID(targetID)
	if err != nil {
		w.logger.Error("Failed to get target", zap.Error(err), zap.Int64("target_id", targetID))
		return fmt.Errorf("failed to get target: %w", err)
	}

	// Send notification
	_, err = w.sendNotification(ctx, target, payload, deliveryID)
	return err
}

// sendNotification sends a notification using the appropriate channel
func (w *NotificationWorker) sendNotification(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload, deliveryID int64) (interface{}, error) {
	startTime := time.Now()

	// Increment attempt count
	if err := w.repo.IncrementAttempt(target.ID, ""); err != nil {
		w.logger.Warn("Failed to increment attempt count", zap.Error(err))
	}

	// Determine channel type from payload or target
	channelType := "expo" // Default
	if st, ok := target.Payload["sender_type"].(string); ok && st != "" {
		channelType = st
	}

	// Get channel
	channel, ok := w.channelRegistry.GetChannel(channelType)
	if !ok {
		err := fmt.Errorf("channel not found: %s", channelType)
		w.logger.Error("Channel not found", zap.String("channel_type", channelType))
		w.repo.IncrementAttempt(target.ID, err.Error())
		return nil, err
	}

	// Send notification
	result := channel.Send(ctx, target, payload)
	duration := time.Since(startTime)

	if result.Success {
		// Mark as delivered
		if err := w.repo.MarkDelivered(target.ID); err != nil {
			w.logger.Error("Failed to mark as delivered", zap.Error(err))
		}

		w.logger.Info("Notification sent successfully",
			zap.Int64("delivery_id", deliveryID),
			zap.Int64("target_id", target.ID),
			zap.String("user_id", target.UserID),
			zap.String("channel", channel.Name()),
			zap.String("trace_id", payload.TraceID),
			zap.Duration("duration_ms", duration),
		)

		return nil, nil
	}

	// Handle failure
	errorMsg := ""
	if result.Error != nil {
		errorMsg = result.Error.Error()
	}

	// Increment attempt with error
	if err := w.repo.IncrementAttempt(target.ID, errorMsg); err != nil {
		w.logger.Warn("Failed to update attempt count", zap.Error(err))
	}

	w.logger.Error("Notification send failed",
		zap.Int64("delivery_id", deliveryID),
		zap.Int64("target_id", target.ID),
		zap.String("user_id", target.UserID),
		zap.String("channel", channel.Name()),
		zap.Bool("retryable", result.Retryable),
		zap.String("error", errorMsg),
		zap.Duration("duration_ms", duration),
	)

	// If retryable, the worker will handle retry (reset to pending)
	// If not retryable, mark as failed
	if !result.Retryable {
		return nil, fmt.Errorf("non-retryable error: %w", result.Error)
	}

	return nil, fmt.Errorf("retryable error: %w", result.Error)
}

// Start starts the worker
func (w *NotificationWorker) Start(ctx context.Context) error {
	atomic.StoreInt32(&w.running, 1) // Set to running

	w.logger.Info("Starting notification worker",
		zap.Int("concurrency", w.config.Notification.WorkerConcurrency),
	)

	err := w.worker.Start(ctx)

	atomic.StoreInt32(&w.running, 0) // Set to stopped

	return err
}

// Stop stops the worker
func (w *NotificationWorker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping notification worker")

	atomic.StoreInt32(&w.running, 0) // Set to stopped

	return w.worker.Stop(ctx)
}

// IsRunning returns true if the worker is currently running
// Uses atomic operation for lock-free read (much faster than RLock)
func (w *NotificationWorker) IsRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// GetQueueLength returns the current queue length
func (w *NotificationWorker) GetQueueLength() int {
	if w.queue == nil {
		return -1
	}
	return w.queue.Length()
}

// GetQueueCapacity returns the queue capacity
func (w *NotificationWorker) GetQueueCapacity() int {
	if w.queue == nil {
		return -1
	}
	return w.queue.Capacity()
}

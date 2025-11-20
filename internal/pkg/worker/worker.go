package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
)

// Worker manages task processing with concurrency control
type Worker struct {
	provider    Provider
	registry    map[string]Handler
	middlewares []Middleware
	config      Config
	logger      *logger.Logger
	wg          sync.WaitGroup
	stopCh      chan struct{}
	mu          sync.RWMutex
}

// New creates a new Worker instance
func New(provider Provider, config Config, log *logger.Logger) *Worker {
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	return &Worker{
		provider:    provider,
		registry:    make(map[string]Handler),
		middlewares: []Middleware{},
		config:      config,
		logger:      log,
		stopCh:      make(chan struct{}),
	}
}

// Register registers a handler for a specific task name
func (w *Worker) Register(name string, handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.registry[name] = handler
	w.logger.Info("Handler registered", zap.String("name", name))
}

// Use adds a middleware to the worker
func (w *Worker) Use(mw Middleware) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.middlewares = append(w.middlewares, mw)
}

// Start begins processing tasks with the configured concurrency
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting worker", zap.Int("concurrency", w.config.Concurrency))

	// Start worker goroutines
	for i := 0; i < w.config.Concurrency; i++ {
		w.wg.Add(1)
		go w.processLoop(ctx, i)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		w.logger.Info("Worker context cancelled")
	case <-w.stopCh:
		w.logger.Info("Worker stop signal received")
	}

	return w.shutdown()
}

// Stop gracefully stops the worker
func (w *Worker) Stop(ctx context.Context) error {
	select {
	case <-w.stopCh:
		// Already stopped
		return nil
	default:
		close(w.stopCh)
	}

	// Wait for shutdown with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("Worker stopped gracefully")
		return nil
	case <-ctx.Done():
		w.logger.Warn("Worker shutdown timeout exceeded")
		return ctx.Err()
	}
}

// shutdown performs graceful shutdown
func (w *Worker) shutdown() error {
	w.logger.Info("Shutting down worker", zap.Duration("timeout", w.config.ShutdownTimeout))

	ctx, cancel := context.WithTimeout(context.Background(), w.config.ShutdownTimeout)
	defer cancel()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("All workers finished")
	case <-ctx.Done():
		w.logger.Warn("Shutdown timeout exceeded, forcing stop")
		return ctx.Err()
	}

	// Close provider
	if err := w.provider.Close(); err != nil {
		w.logger.Error("Failed to close provider", zap.Error(err))
		return err
	}

	return nil
}

// processLoop is the main processing loop for each worker goroutine
func (w *Worker) processLoop(ctx context.Context, workerID int) {
	defer w.wg.Done()

	log := w.logger.With(zap.Int("worker_id", workerID))
	log.Info("Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Info("Worker stopping: context cancelled")
			return
		case <-w.stopCh:
			log.Info("Worker stopping: stop signal")
			return
		default:
		}

		// Fetch next task
		task, err := w.provider.Fetch(ctx)
		if err != nil {
			log.Error("Failed to fetch task", zap.Error(err))
			time.Sleep(w.config.ErrorBackoff)
			continue
		}

		// No task available
		if task == nil {
			time.Sleep(w.config.PollInterval)
			continue
		}

		// Process the task
		w.processTask(ctx, task, log)
	}
}

// processTask handles a single task with timeout and recovery
func (w *Worker) processTask(ctx context.Context, task *Task, log *logger.Logger) {
	taskLog := log.With(
		zap.String("task_id", task.ID),
		zap.Int("retry", task.Retry),
	)

	// Check if task is expired
	if task.IsExpired() {
		taskLog.Warn("Task expired, sending to DLQ")
		if err := w.provider.Nack(ctx, task, false); err != nil {
			taskLog.Error("Failed to nack expired task", zap.Error(err))
		}
		return
	}

	// Get task type from metadata
	taskType := task.Metadata["type"]
	if taskType == "" {
		taskLog.Error("Task missing type metadata")
		if err := w.provider.Nack(ctx, task, false); err != nil {
			taskLog.Error("Failed to nack invalid task", zap.Error(err))
		}
		return
	}

	// Get handler
	w.mu.RLock()
	handler, exists := w.registry[taskType]
	w.mu.RUnlock()

	if !exists {
		taskLog.Error("No handler registered for task type", zap.String("type", taskType))
		if err := w.provider.Nack(ctx, task, false); err != nil {
			taskLog.Error("Failed to nack unhandled task", zap.Error(err))
		}
		return
	}

	// Apply middlewares
	w.mu.RLock()
	if len(w.middlewares) > 0 {
		handler = Chain(w.middlewares...)(handler)
	}
	w.mu.RUnlock()

	// Create context with timeout
	taskCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute handler
	startTime := time.Now()
	err := handler.Process(taskCtx, task)
	duration := time.Since(startTime)

	taskLog = taskLog.With(zap.Duration("duration", duration))

	// Handle result
	if err != nil {
		taskLog.Error("Task processing failed", zap.Error(err))
		w.handleTaskError(ctx, task, err, taskLog)
	} else {
		taskLog.Info("Task processed successfully")
		if ackErr := w.provider.Ack(ctx, task); ackErr != nil {
			taskLog.Error("Failed to acknowledge task", zap.Error(ackErr))
		}
	}
}

// handleTaskError handles task processing errors with retry logic
func (w *Worker) handleTaskError(ctx context.Context, task *Task, err error, log *logger.Logger) {
	// Check if task should be retried
	if task.ShouldRetry() {
		log.Info("Retrying task", zap.Int("next_retry", task.Retry+1))
		task.IncrementRetry()

		// Calculate backoff delay
		delay := w.calculateBackoff(task.Retry)
		task.ScheduledAt = time.Now().Add(delay)

		// Requeue task
		if err := w.provider.Nack(ctx, task, true); err != nil {
			log.Error("Failed to requeue task", zap.Error(err))
		}
	} else {
		log.Warn("Task max retries exceeded, sending to DLQ")
		// Send to dead letter queue
		if err := w.provider.Nack(ctx, task, false); err != nil {
			log.Error("Failed to send task to DLQ", zap.Error(err))
		}
	}
}

// calculateBackoff calculates the backoff delay based on retry count
func (w *Worker) calculateBackoff(retry int) time.Duration {
	switch w.config.BackoffStrategy {
	case BackoffExponential:
		return w.config.BaseBackoff * time.Duration(1<<uint(retry))
	case BackoffLinear:
		return w.config.BaseBackoff * time.Duration(retry+1)
	default:
		return w.config.BaseBackoff
	}
}

// GetHandler returns the handler for a given task type
func (w *Worker) GetHandler(name string) (Handler, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	handler, exists := w.registry[name]
	if !exists {
		return nil, fmt.Errorf("handler not found: %s", name)
	}
	return handler, nil
}

package rate

import (
	"context"
	"fmt"
)

// WorkerKeyFunc extracts the rate limit key from a worker task
type WorkerKeyFunc func(taskType string, taskID string) string

// WorkerMiddleware creates a worker middleware for rate limiting
type WorkerMiddleware struct {
	limiter Limiter
	keyFunc WorkerKeyFunc
}

// NewWorkerMiddleware creates a new worker rate limiting middleware
func NewWorkerMiddleware(limiter Limiter, keyFunc WorkerKeyFunc) *WorkerMiddleware {
	if keyFunc == nil {
		keyFunc = DefaultWorkerKeyFunc
	}

	return &WorkerMiddleware{
		limiter: limiter,
		keyFunc: keyFunc,
	}
}

// Wrap wraps a worker handler with rate limiting
// This assumes the worker package has a similar interface to what we saw
func (m *WorkerMiddleware) Wrap(next WorkerHandler) WorkerHandler {
	return WorkerHandlerFunc(func(ctx context.Context, task WorkerTask) error {
		// Extract key
		key := m.keyFunc(task.Type(), task.ID())

		// Check rate limit
		allowed, err := m.limiter.Allow(ctx, key)
		if err != nil {
			// Decide whether to fail or continue based on configuration
			return fmt.Errorf("rate limit check failed: %w", err)
		}

		if !allowed {
			// Task is rate limited - could return error or reschedule
			return ErrRateLimitExceeded
		}

		// Process task
		return next.Process(ctx, task)
	})
}

// DefaultWorkerKeyFunc creates a key based on task type
func DefaultWorkerKeyFunc(taskType string, taskID string) string {
	return fmt.Sprintf("worker:%s", taskType)
}

// WorkerTaskKeyFunc creates a key based on individual task
func WorkerTaskKeyFunc() WorkerKeyFunc {
	return func(taskType string, taskID string) string {
		return fmt.Sprintf("worker:%s:%s", taskType, taskID)
	}
}

// WorkerHandler is a generic interface for worker handlers
type WorkerHandler interface {
	Process(ctx context.Context, task WorkerTask) error
}

// WorkerHandlerFunc is a function adapter for WorkerHandler
type WorkerHandlerFunc func(ctx context.Context, task WorkerTask) error

func (f WorkerHandlerFunc) Process(ctx context.Context, task WorkerTask) error {
	return f(ctx, task)
}

// WorkerTask is a generic interface for worker tasks
type WorkerTask interface {
	Type() string
	ID() string
}

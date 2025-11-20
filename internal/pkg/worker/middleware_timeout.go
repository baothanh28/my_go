package worker

import (
	"context"
	"fmt"

	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
)

// TimeoutMiddleware creates a middleware that enforces task timeouts
func TimeoutMiddleware(log *logger.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, task *Task) error {
			// Skip if no timeout specified
			if task.Timeout <= 0 {
				return next.Process(ctx, task)
			}

			// Create context with timeout
			timeoutCtx, cancel := context.WithTimeout(ctx, task.Timeout)
			defer cancel()

			// Process with timeout
			errCh := make(chan error, 1)
			go func() {
				errCh <- next.Process(timeoutCtx, task)
			}()

			select {
			case err := <-errCh:
				return err
			case <-timeoutCtx.Done():
				log.Error("Task timeout exceeded",
					zap.String("task_id", task.ID),
					zap.String("task_type", task.Metadata["type"]),
					zap.Duration("timeout", task.Timeout),
				)
				return fmt.Errorf("task timeout exceeded: %s", task.Timeout)
			}
		})
	}
}

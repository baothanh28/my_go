package worker

import (
	"context"
	"fmt"
	"runtime/debug"

	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
)

// RecoveryMiddleware creates a middleware that recovers from panics
func RecoveryMiddleware(log *logger.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, task *Task) (err error) {
			defer func() {
				if r := recover(); r != nil {
					// Capture stack trace
					stack := debug.Stack()

					log.Error("Task handler panicked",
						zap.String("task_id", task.ID),
						zap.String("task_type", task.Metadata["type"]),
						zap.Any("panic", r),
						zap.String("stack", string(stack)),
					)

					// Convert panic to error
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()

			return next.Process(ctx, task)
		})
	}
}

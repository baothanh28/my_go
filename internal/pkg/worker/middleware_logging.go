package worker

import (
	"context"
	"time"

	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
)

// LoggingMiddleware creates a middleware that logs task processing
func LoggingMiddleware(log *logger.Logger) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, task *Task) error {
			taskLog := log.With(
				zap.String("task_id", task.ID),
				zap.String("task_type", task.Metadata["type"]),
				zap.Int("retry", task.Retry),
			)

			taskLog.Info("Task processing started")
			start := time.Now()

			err := next.Process(ctx, task)

			duration := time.Since(start)
			taskLog = taskLog.With(zap.Duration("duration", duration))

			if err != nil {
				taskLog.Error("Task processing failed", zap.Error(err))
			} else {
				taskLog.Info("Task processing completed")
			}

			return err
		})
	}
}

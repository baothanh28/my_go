package worker

import (
	"context"

	"myapp/internal/pkg/logctx"
)

// TracingMiddleware creates a middleware that adds correlation ID to context
func TracingMiddleware() Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, task *Task) error {
			// Use task ID as correlation ID
			correlationID := task.ID

			// Check if metadata has a custom correlation ID
			if cid, ok := task.Metadata["correlation_id"]; ok {
				correlationID = cid
			}

			// Add correlation ID to context
			ctx = logctx.WithCorrelationID(ctx, correlationID)

			return next.Process(ctx, task)
		})
	}
}

package worker

import (
	"context"
)

// Provider defines the interface for task queue providers
type Provider interface {
	// Fetch retrieves the next task from the queue
	// Returns nil task if no task is available
	Fetch(ctx context.Context) (*Task, error)

	// Ack acknowledges successful processing of a task
	Ack(ctx context.Context, task *Task) error

	// Nack negatively acknowledges a task
	// If requeue is true, the task will be returned to the queue
	Nack(ctx context.Context, task *Task, requeue bool) error

	// Close cleans up provider resources
	Close() error
}

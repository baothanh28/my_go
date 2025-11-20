package worker

import (
	"context"
)

// Handler defines the interface for processing tasks
type Handler interface {
	// Process executes the task logic
	Process(ctx context.Context, task *Task) error
}

// HandlerFunc is a function adapter that implements the Handler interface
type HandlerFunc func(ctx context.Context, task *Task) error

// Process implements the Handler interface
func (f HandlerFunc) Process(ctx context.Context, task *Task) error {
	return f(ctx, task)
}

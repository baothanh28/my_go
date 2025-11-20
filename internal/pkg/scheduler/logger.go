package scheduler

import "context"

// Logger defines the logging interface for the scheduler.
type Logger interface {
	Debug(ctx context.Context, msg string, fields map[string]interface{})
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Warn(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, fields map[string]interface{})
}

// NoOpLogger is a logger that does nothing.
type NoOpLogger struct{}

func (n *NoOpLogger) Debug(ctx context.Context, msg string, fields map[string]interface{}) {}
func (n *NoOpLogger) Info(ctx context.Context, msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) Warn(ctx context.Context, msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) Error(ctx context.Context, msg string, fields map[string]interface{}) {}

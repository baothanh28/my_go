# Worker Package

A generic, reusable worker package for processing tasks from Redis Streams with support for retries, middlewares, and graceful shutdown.

## Features

- ✅ **Generic Task Processing**: Handle any type of task with custom handlers
- ✅ **Multiple Provider Support**: Currently supports Redis Streams (easy to extend)
- ✅ **Retry Logic**: Configurable retry with exponential/linear backoff
- ✅ **Middleware Pipeline**: Extensible middleware system for logging, metrics, recovery, etc.
- ✅ **Graceful Shutdown**: Proper cleanup on shutdown
- ✅ **Concurrency Control**: Configurable number of worker goroutines
- ✅ **Dead Letter Queue**: Automatic DLQ for failed tasks
- ✅ **Context-Aware**: Full context propagation for tracing and cancellation

## Architecture

```
┌──────────────┐
│   Provider   │ (Redis Streams, Kafka, SQS, etc.)
└──────┬───────┘
       │ Fetch
       ▼
┌──────────────┐
│    Worker    │ (Manages worker pool & lifecycle)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Middleware  │ (Recovery → Logging → Metrics → Tracing)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Handler    │ (Your business logic)
└──────────────┘
```

## Quick Start

### 1. Basic Usage

```go
package main

import (
	"context"
	"time"
	
	"myapp/internal/pkg/worker"
	"myapp/internal/pkg/logger"
	
	redisv9 "github.com/redis/go-redis/v9"
)

func main() {
	// Create logger
	log, _ := logger.NewLogger(cfg)
	
	// Create Redis client
	rdb := redisv9.NewClient(&redisv9.Options{
		Addr: "localhost:6379",
	})
	
	// Create Redis provider
	providerConfig := worker.DefaultRedisProviderConfig("tasks", "workers", "worker-1")
	provider, _ := worker.NewRedisProvider(rdb, providerConfig, log)
	
	// Create worker
	workerConfig := worker.DefaultConfig()
	w := worker.New(provider, workerConfig, log)
	
	// Register handlers
	w.Register("send_email", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		// Your task processing logic here
		log.Info("Sending email", zap.String("to", task.Metadata["to"]))
		return nil
	}))
	
	// Add middlewares
	w.Use(worker.RecoveryMiddleware(log))
	w.Use(worker.LoggingMiddleware(log))
	
	// Start worker
	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		log.Fatal("Worker failed", zap.Error(err))
	}
}
```

### 2. Using with Uber FX

```go
package main

import (
	"myapp/internal/pkg/worker"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/redis"
	
	"go.uber.org/fx"
)

func main() {
	fx.New(
		// Provide dependencies
		logger.Module,
		redis.Module,
		
		// Provide worker
		worker.ProvideWorker(),
		
		// Register handlers
		fx.Invoke(registerHandlers),
	).Run()
}

func registerHandlers(w *worker.Worker) {
	w.Register("send_email", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		// Your task processing logic
		return nil
	}))
	
	w.Register("process_order", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		// Your task processing logic
		return nil
	}))
}
```

### 3. Enqueuing Tasks

```go
package main

import (
	"context"
	"encoding/json"
	"time"
	
	"myapp/internal/pkg/worker"
	
	redisv9 "github.com/redis/go-redis/v9"
)

func enqueueTask(rdb *redisv9.Client) {
	// Create provider (for enqueuing)
	providerConfig := worker.DefaultRedisProviderConfig("tasks", "workers", "producer")
	provider, _ := worker.NewRedisProvider(rdb, providerConfig, log)
	
	// Create task
	payload := map[string]string{
		"to":      "user@example.com",
		"subject": "Welcome!",
		"body":    "Welcome to our service",
	}
	payloadBytes, _ := json.Marshal(payload)
	
	task := &worker.Task{
		Payload: payloadBytes,
		Metadata: map[string]string{
			"type": "send_email",
		},
		MaxRetry:  3,
		Timeout:   30 * time.Second,
		CreatedAt: time.Now(),
	}
	
	// Enqueue
	taskID, err := provider.EnqueueTask(context.Background(), task)
	if err != nil {
		log.Fatal("Failed to enqueue task", zap.Error(err))
	}
	
	log.Info("Task enqueued", zap.String("task_id", taskID))
}
```

## Configuration

### Worker Config

```go
config := worker.Config{
	Concurrency:     10,                         // Number of worker goroutines
	BackoffStrategy: worker.BackoffExponential,  // Backoff strategy
	BaseBackoff:     1 * time.Second,            // Base backoff delay
	MaxBackoff:      5 * time.Minute,            // Maximum backoff delay
	ShutdownTimeout: 30 * time.Second,           // Graceful shutdown timeout
	PollInterval:    1 * time.Second,            // Poll interval when queue is empty
	ErrorBackoff:    5 * time.Second,            // Backoff after fetch error
}
```

### Redis Provider Config

```go
config := worker.RedisProviderConfig{
	Stream:          "tasks",                // Redis stream name
	Group:           "workers",              // Consumer group name
	Consumer:        "worker-1",             // Consumer name (instance ID)
	Count:           1,                      // Messages per fetch
	Block:           1 * time.Second,        // Block duration
	ClaimMinIdle:    5 * time.Minute,        // Min idle time before claiming
	ClaimCount:      10,                     // Messages to claim per batch
	EnableAutoClaim: true,                   // Enable auto-claiming stale messages
	DLQStream:       "tasks:dlq",            // Dead letter queue stream
	MaxLen:          10000,                  // Maximum stream length
}
```

## Middleware

### Built-in Middlewares

1. **RecoveryMiddleware**: Recovers from panics in handlers
2. **LoggingMiddleware**: Logs task processing
3. **MetricsMiddleware**: Collects basic metrics
4. **TracingMiddleware**: Adds correlation ID to context
5. **TimeoutMiddleware**: Enforces task timeouts

### Custom Middleware

```go
func CustomMiddleware(log *logger.Logger) worker.Middleware {
	return func(next worker.Handler) worker.Handler {
		return worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
			// Before handler
			log.Info("Before processing")
			
			// Call next handler
			err := next.Process(ctx, task)
			
			// After handler
			log.Info("After processing")
			
			return err
		})
	}
}

// Use middleware
w.Use(CustomMiddleware(log))
```

## Advanced Usage

### Custom Provider

Implement the `Provider` interface to use a different queue system:

```go
type Provider interface {
	Fetch(ctx context.Context) (*Task, error)
	Ack(ctx context.Context, task *Task) error
	Nack(ctx context.Context, task *Task, requeue bool) error
	Close() error
}
```

### Task Handler with Context

```go
w.Register("example", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Get correlation ID from context
	if correlationID, ok := logctx.CorrelationID(ctx); ok {
		log.Info("Processing task", zap.String("correlation_id", correlationID))
	}
	
	// Your business logic
	return nil
}))
```

## Monitoring

### Metrics

The built-in `MetricsCollector` tracks:
- Task processed count by type and status
- Task processing duration
- Task retry count

For production, integrate with Prometheus or other metrics systems.

### Logs

All operations are logged with structured logging using zap:
- Task fetching
- Task processing (start/complete/error)
- Retries
- DLQ operations
- Worker lifecycle events

## Testing

```go
package myhandler_test

import (
	"context"
	"testing"
	
	"myapp/internal/pkg/worker"
)

func TestEmailHandler(t *testing.T) {
	handler := worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		// Your handler logic
		return nil
	})
	
	task := &worker.Task{
		ID:      "test-task-1",
		Payload: []byte(`{"to":"test@example.com"}`),
		Metadata: map[string]string{
			"type": "send_email",
		},
	}
	
	err := handler.Process(context.Background(), task)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
```

## Best Practices

1. **Idempotent Handlers**: Design handlers to be idempotent (safe to retry)
2. **Timeout Configuration**: Set appropriate timeouts for tasks
3. **Error Handling**: Return errors for retriable failures, panic for unrecoverable errors
4. **Graceful Shutdown**: Always handle shutdown signals properly
5. **Monitoring**: Monitor worker metrics and logs
6. **Dead Letter Queue**: Regularly review and reprocess DLQ items
7. **Concurrency**: Tune concurrency based on workload and resources
8. **Metadata**: Use task metadata for routing and filtering

## Troubleshooting

### Worker not processing tasks

1. Check Redis connection
2. Verify consumer group exists
3. Check task metadata has "type" field
4. Verify handler is registered for task type

### Tasks failing repeatedly

1. Check handler logic for errors
2. Verify task timeout is sufficient
3. Check for panic recovery in logs
4. Review DLQ for failed tasks

### High memory usage

1. Reduce worker concurrency
2. Limit stream max length
3. Clear processed messages regularly
4. Check for goroutine leaks

## License

Internal use only.


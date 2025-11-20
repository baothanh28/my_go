package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/worker"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Example demonstrates basic worker usage
func Example_basicUsage() {
	// Create logger
	cfg := &config.Config{
		Logger: config.LoggerConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
	}
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

	// Register handler
	w.Register("send_email", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		var payload map[string]string
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			return err
		}
		log.Info("Sending email", zap.String("to", payload["to"]))
		return nil
	}))

	// Add middlewares
	w.Use(worker.RecoveryMiddleware(log))
	w.Use(worker.LoggingMiddleware(log))

	// Start worker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = w.Start(ctx)
}

// Example demonstrates how to enqueue tasks
func Example_enqueueTask() {
	// Setup (same as above)
	cfg := &config.Config{
		Logger: config.LoggerConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
	}
	log, _ := logger.NewLogger(cfg)

	rdb := redisv9.NewClient(&redisv9.Options{
		Addr: "localhost:6379",
	})

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

	// Enqueue task
	taskID, err := provider.EnqueueTask(context.Background(), task)
	if err != nil {
		fmt.Printf("Failed to enqueue: %v\n", err)
		return
	}

	fmt.Printf("Task enqueued: %s\n", taskID)
}

// Example demonstrates custom middleware
func Example_customMiddleware() {
	cfg := &config.Config{
		Logger: config.LoggerConfig{
			Level:      "info",
			Format:     "json",
			OutputPath: "stdout",
		},
	}
	log, _ := logger.NewLogger(cfg)

	rdb := redisv9.NewClient(&redisv9.Options{
		Addr: "localhost:6379",
	})

	providerConfig := worker.DefaultRedisProviderConfig("tasks", "workers", "worker-1")
	provider, _ := worker.NewRedisProvider(rdb, providerConfig, log)

	workerConfig := worker.DefaultConfig()
	w := worker.New(provider, workerConfig, log)

	// Custom middleware
	customMiddleware := func(next worker.Handler) worker.Handler {
		return worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
			log.Info("Before processing", zap.String("task_id", task.ID))
			err := next.Process(ctx, task)
			log.Info("After processing", zap.String("task_id", task.ID))
			return err
		})
	}

	// Use custom middleware
	w.Use(customMiddleware)

	// Register handler
	w.Register("example", worker.HandlerFunc(func(ctx context.Context, task *worker.Task) error {
		log.Info("Processing task")
		return nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = w.Start(ctx)
}

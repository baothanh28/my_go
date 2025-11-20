# Scheduler Package

A generic, reusable scheduler package for running recurring or delayed tasks in Go applications with distributed locking support.

## Features

- **Multiple Schedule Types**: Support for cron expressions, fixed intervals, and one-time schedules
- **Distributed Execution**: Uses distributed locks to ensure only one instance executes a job at a time
- **Retry Logic**: Configurable retry policies with exponential, linear, or fixed backoff strategies
- **Timeout Management**: Per-job timeouts with automatic cancellation
- **Panic Recovery**: Automatically recovers from panics in job handlers
- **Multiple Backends**: Pluggable backends (Redis, in-memory)
- **Observability**: Built-in logging and metrics interfaces
- **Graceful Shutdown**: Ensures in-flight jobs complete before shutdown
- **Dependency Injection**: Integrated with Uber FX

## Installation

```bash
go get github.com/robfig/cron/v3
go get github.com/redis/go-redis/v9
go get github.com/google/uuid
go get go.uber.org/fx
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "your-project/internal/pkg/scheduler"
)

func main() {
    // Create backend (in-memory for testing)
    backend := scheduler.NewMemoryBackend()
    
    // Create logger and metrics (or use your own implementations)
    logger := &scheduler.NoOpLogger{}
    metrics := &scheduler.NoOpMetrics{}
    
    // Create executor and lock
    executor := scheduler.NewDefaultJobExecutor(logger, metrics)
    lock := scheduler.NewDistributedLock(backend, logger, metrics)
    
    // Create scheduler with configuration
    config := scheduler.DefaultConfig()
    sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)
    
    // Create a job
    cronSchedule, _ := scheduler.NewCronSchedule("*/5 * * * *") // Every 5 minutes
    
    job := &scheduler.Job{
        Name:     "my-job",
        Schedule: cronSchedule,
        Timeout:  30 * time.Second,
        Handler: func(ctx context.Context) error {
            fmt.Println("Job executed!")
            return nil
        },
        RetryPolicy: scheduler.DefaultRetryPolicy(),
    }
    
    // Register and start
    if err := sched.Register(job); err != nil {
        panic(err)
    }
    
    ctx := context.Background()
    if err := sched.Start(ctx); err != nil {
        panic(err)
    }
    
    // Run for some time
    time.Sleep(1 * time.Hour)
    
    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    sched.Stop(shutdownCtx)
}
```

### Using with Uber FX

```go
package main

import (
    "context"
    "time"
    
    "go.uber.org/fx"
    "your-project/internal/pkg/scheduler"
)

func main() {
    app := fx.New(
        // Provide configuration
        scheduler.ProvideSchedulerConfig(&scheduler.SchedulerConfig{
            TickInterval:  5 * time.Second,
            MaxConcurrent: 10,
            BackendType:   "redis",
            RedisAddr:     "localhost:6379",
        }),
        
        // Include scheduler module
        scheduler.Module,
        
        // Provide custom logger and metrics
        scheduler.ProvideLogger(myLogger),
        scheduler.ProvideMetrics(myMetrics),
        
        // Register jobs
        fx.Invoke(registerJobs),
    )
    
    app.Run()
}

func registerJobs(sched scheduler.Scheduler, lc fx.Lifecycle) error {
    // Create jobs
    cronSchedule, _ := scheduler.NewCronSchedule("0 * * * *") // Every hour
    
    job := &scheduler.Job{
        Name:     "hourly-cleanup",
        Schedule: cronSchedule,
        Timeout:  5 * time.Minute,
        Handler: func(ctx context.Context) error {
            // Your job logic here
            return nil
        },
    }
    
    if err := sched.Register(job); err != nil {
        return err
    }
    
    // Lifecycle hooks
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            return sched.Start(ctx)
        },
        OnStop: func(ctx context.Context) error {
            return sched.Stop(ctx)
        },
    })
    
    return nil
}
```

## Schedule Types

### Cron Schedule

```go
schedule, err := scheduler.NewCronSchedule("0 */6 * * *") // Every 6 hours
if err != nil {
    // Handle error
}
```

Cron expression format:
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

### Interval Schedule

```go
schedule := scheduler.NewIntervalSchedule(15 * time.Minute) // Every 15 minutes
```

### One-Time Schedule

```go
runAt := time.Now().Add(1 * time.Hour)
schedule := scheduler.NewOnceSchedule(runAt) // Run once at specific time
```

## Retry Policies

### Exponential Backoff

```go
retryPolicy := &scheduler.RetryPolicy{
    MaxRetries:      3,
    InitialInterval: 1 * time.Second,
    MaxInterval:     30 * time.Second,
    Multiplier:      2.0,
    Strategy:        scheduler.RetryStrategyExponential,
}
```

Retry delays: 1s, 2s, 4s

### Linear Backoff

```go
retryPolicy := &scheduler.RetryPolicy{
    MaxRetries:      3,
    InitialInterval: 5 * time.Second,
    MaxInterval:     30 * time.Second,
    Multiplier:      1.0,
    Strategy:        scheduler.RetryStrategyLinear,
}
```

Retry delays: 5s, 10s, 15s

### Fixed Backoff

```go
retryPolicy := &scheduler.RetryPolicy{
    MaxRetries:      3,
    InitialInterval: 10 * time.Second,
    MaxInterval:     10 * time.Second,
    Multiplier:      1.0,
    Strategy:        scheduler.RetryStrategyFixed,
}
```

Retry delays: 10s, 10s, 10s

## Job Management

### Pause a Job

```go
err := sched.Pause("my-job")
```

### Resume a Job

```go
err := sched.Resume("my-job")
```

### Remove a Job

```go
err := sched.Remove("my-job")
```

### Get Job Status

```go
job, err := sched.GetJob("my-job")
if err != nil {
    // Handle error
}

fmt.Printf("Status: %s\n", job.Metadata.Status)
fmt.Printf("Next run: %s\n", job.Metadata.NextRunAt)
fmt.Printf("Run count: %d\n", job.Metadata.RunCount)
```

## Backend Providers

### Redis Backend

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

backend := scheduler.NewRedisBackend(client)
```

### In-Memory Backend (for testing)

```go
backend := scheduler.NewMemoryBackend()
```

## Custom Logger

Implement the `Logger` interface:

```go
type MyLogger struct {
    // Your logger implementation
}

func (l *MyLogger) Debug(ctx context.Context, msg string, fields map[string]interface{}) {
    // Log debug
}

func (l *MyLogger) Info(ctx context.Context, msg string, fields map[string]interface{}) {
    // Log info
}

func (l *MyLogger) Warn(ctx context.Context, msg string, fields map[string]interface{}) {
    // Log warning
}

func (l *MyLogger) Error(ctx context.Context, msg string, fields map[string]interface{}) {
    // Log error
}
```

## Custom Metrics

Implement the `MetricsCollector` interface:

```go
type MyMetrics struct {
    // Your metrics implementation
}

func (m *MyMetrics) JobStarted(jobName string) {
    // Record job started metric
}

func (m *MyMetrics) JobCompleted(jobName string, duration time.Duration) {
    // Record job completion metric
}

func (m *MyMetrics) JobFailed(jobName string, err error) {
    // Record job failure metric
}

// Implement other methods...
```

## Configuration

```go
config := &scheduler.SchedulerConfig{
    TickInterval:        5 * time.Second,   // How often to check for due jobs
    MaxConcurrent:       10,                // Maximum concurrent job executions
    LockTTL:             30 * time.Second,  // Distributed lock TTL
    LockRefreshInterval: 10 * time.Second,  // How often to refresh locks
    BackendType:         "redis",           // "redis" or "memory"
    RedisAddr:           "localhost:6379",
    RedisPassword:       "",
    RedisDB:             0,
}
```

## Best Practices

1. **Idempotent Handlers**: Always make job handlers idempotent to handle at-least-once delivery semantics
2. **Timeout Configuration**: Set appropriate timeouts based on expected job duration
3. **Retry Strategy**: Choose retry strategy based on job characteristics
4. **Monitoring**: Implement custom logger and metrics for production observability
5. **Graceful Shutdown**: Always use graceful shutdown to ensure jobs complete properly
6. **Error Handling**: Return errors from handlers to trigger retry logic
7. **Context Handling**: Respect context cancellation in job handlers

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Scheduler                            │
│  ┌────────────┐  ┌────────────┐  ┌──────────────────┐      │
│  │  Job       │  │  Executor  │  │  Distributed     │      │
│  │  Registry  │  │            │  │  Lock            │      │
│  └────────────┘  └────────────┘  └──────────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           │
                           │
                   ┌───────┴────────┐
                   │                │
           ┌───────▼──────┐  ┌──────▼─────┐
           │    Redis     │  │  In-Memory │
           │   Backend    │  │  Backend   │
           └──────────────┘  └────────────┘
```

## License

This package is part of your project's internal packages.


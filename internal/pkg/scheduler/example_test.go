package scheduler_test

import (
	"context"
	"fmt"
	"time"

	"myapp/internal/pkg/scheduler"
)

// Example_basicUsage demonstrates basic scheduler usage.
func Example_basicUsage() {
	// Create backend (in-memory for this example)
	backend := scheduler.NewMemoryBackend()

	// Create logger and metrics (using no-op implementations)
	logger := &scheduler.NoOpLogger{}
	metrics := &scheduler.NoOpMetrics{}

	// Create executor and lock
	executor := scheduler.NewDefaultJobExecutor(logger, metrics)
	lock := scheduler.NewDistributedLock(backend, logger, metrics)

	// Create scheduler with default configuration
	config := scheduler.DefaultConfig()
	sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)

	// Create a simple interval-based job
	intervalSchedule := scheduler.NewIntervalSchedule(10 * time.Second)

	job := &scheduler.Job{
		Name:     "hello-world",
		Schedule: intervalSchedule,
		Timeout:  5 * time.Second,
		Handler: func(ctx context.Context) error {
			fmt.Println("Hello, World!")
			return nil
		},
		RetryPolicy: scheduler.DefaultRetryPolicy(),
	}

	// Register the job
	if err := sched.Register(job); err != nil {
		panic(err)
	}

	// Start the scheduler
	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		panic(err)
	}

	// Let it run for a bit
	time.Sleep(25 * time.Second)

	// Gracefully stop the scheduler
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sched.Stop(shutdownCtx)
}

// Example_cronSchedule demonstrates using cron expressions.
func Example_cronSchedule() {
	backend := scheduler.NewMemoryBackend()
	logger := &scheduler.NoOpLogger{}
	metrics := &scheduler.NoOpMetrics{}
	executor := scheduler.NewDefaultJobExecutor(logger, metrics)
	lock := scheduler.NewDistributedLock(backend, logger, metrics)

	config := scheduler.DefaultConfig()
	sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)

	// Create a cron schedule - runs every hour at minute 0
	cronSchedule, err := scheduler.NewCronSchedule("0 * * * *")
	if err != nil {
		panic(err)
	}

	job := &scheduler.Job{
		Name:     "hourly-report",
		Schedule: cronSchedule,
		Timeout:  5 * time.Minute,
		Handler: func(ctx context.Context) error {
			fmt.Println("Generating hourly report...")
			// Report generation logic here
			return nil
		},
	}

	sched.Register(job)
	sched.Start(context.Background())

	// Output:
	// Generating hourly report...
}

// Example_oneTimeJob demonstrates a one-time scheduled job.
func Example_oneTimeJob() {
	backend := scheduler.NewMemoryBackend()
	logger := &scheduler.NoOpLogger{}
	metrics := &scheduler.NoOpMetrics{}
	executor := scheduler.NewDefaultJobExecutor(logger, metrics)
	lock := scheduler.NewDistributedLock(backend, logger, metrics)

	config := scheduler.DefaultConfig()
	sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)

	// Schedule a job to run once in 1 hour
	runAt := time.Now().Add(1 * time.Hour)
	onceSchedule := scheduler.NewOnceSchedule(runAt)

	job := &scheduler.Job{
		Name:     "delayed-notification",
		Schedule: onceSchedule,
		Timeout:  30 * time.Second,
		Handler: func(ctx context.Context) error {
			fmt.Println("Sending delayed notification...")
			return nil
		},
	}

	sched.Register(job)
	sched.Start(context.Background())
}

// Example_retryPolicy demonstrates custom retry configuration.
func Example_retryPolicy() {
	backend := scheduler.NewMemoryBackend()
	logger := &scheduler.NoOpLogger{}
	metrics := &scheduler.NoOpMetrics{}
	executor := scheduler.NewDefaultJobExecutor(logger, metrics)
	lock := scheduler.NewDistributedLock(backend, logger, metrics)

	config := scheduler.DefaultConfig()
	sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)

	// Custom exponential backoff retry policy
	retryPolicy := &scheduler.RetryPolicy{
		MaxRetries:      5,
		InitialInterval: 2 * time.Second,
		MaxInterval:     1 * time.Minute,
		Multiplier:      2.0,
		Strategy:        scheduler.RetryStrategyExponential,
	}

	intervalSchedule := scheduler.NewIntervalSchedule(5 * time.Minute)

	job := &scheduler.Job{
		Name:        "unreliable-api-call",
		Schedule:    intervalSchedule,
		Timeout:     30 * time.Second,
		RetryPolicy: retryPolicy,
		Handler: func(ctx context.Context) error {
			// Simulate API call that might fail
			// Will retry with delays: 2s, 4s, 8s, 16s, 32s
			return callExternalAPI(ctx)
		},
	}

	sched.Register(job)
	sched.Start(context.Background())
}

func callExternalAPI(ctx context.Context) error {
	// Simulated API call
	return nil
}

// Example_jobManagement demonstrates pausing, resuming, and removing jobs.
func Example_jobManagement() {
	backend := scheduler.NewMemoryBackend()
	logger := &scheduler.NoOpLogger{}
	metrics := &scheduler.NoOpMetrics{}
	executor := scheduler.NewDefaultJobExecutor(logger, metrics)
	lock := scheduler.NewDistributedLock(backend, logger, metrics)

	config := scheduler.DefaultConfig()
	sched := scheduler.NewScheduler(backend, executor, lock, logger, metrics, config)

	intervalSchedule := scheduler.NewIntervalSchedule(1 * time.Minute)

	job := &scheduler.Job{
		Name:     "maintenance-job",
		Schedule: intervalSchedule,
		Timeout:  30 * time.Second,
		Handler: func(ctx context.Context) error {
			fmt.Println("Running maintenance...")
			return nil
		},
	}

	sched.Register(job)
	sched.Start(context.Background())

	// Pause the job
	sched.Pause("maintenance-job")
	fmt.Println("Job paused")

	// Resume the job
	sched.Resume("maintenance-job")
	fmt.Println("Job resumed")

	// Get job status
	jobInfo, _ := sched.GetJob("maintenance-job")
	fmt.Printf("Status: %s, Run count: %d\n", jobInfo.Metadata.Status, jobInfo.Metadata.RunCount)

	// Remove the job
	sched.Remove("maintenance-job")
	fmt.Println("Job removed")

	// Output:
	// Job paused
	// Job resumed
	// Status: pending, Run count: 0
	// Job removed
}

// Example_redisBackend demonstrates using Redis as the backend.
func Example_redisBackend() {
	// Note: This requires a running Redis instance
	//
	// import "github.com/redis/go-redis/v9"
	//
	// client := redis.NewClient(&redis.Options{
	//     Addr:     "localhost:6379",
	//     Password: "",
	//     DB:       0,
	// })
	//
	// backend := scheduler.NewRedisBackend(client)
	//
	// Rest is the same as other examples...
}

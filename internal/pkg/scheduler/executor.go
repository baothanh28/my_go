package scheduler

import (
	"context"
	"fmt"
	"time"
)

// JobExecutor handles job execution with timeout, retry, and panic recovery.
type JobExecutor interface {
	Execute(ctx context.Context, job *Job) error
}

// DefaultJobExecutor is the default implementation of JobExecutor.
type DefaultJobExecutor struct {
	logger  Logger
	metrics MetricsCollector
}

// NewDefaultJobExecutor creates a new default job executor.
func NewDefaultJobExecutor(logger Logger, metrics MetricsCollector) *DefaultJobExecutor {
	return &DefaultJobExecutor{
		logger:  logger,
		metrics: metrics,
	}
}

// Execute executes a job with timeout, retry, and panic recovery.
func (e *DefaultJobExecutor) Execute(ctx context.Context, job *Job) error {
	startTime := time.Now()

	// Track execution
	e.metrics.JobStarted(job.Name)
	defer func() {
		duration := time.Since(startTime)
		e.metrics.JobCompleted(job.Name, duration)
	}()

	// Log execution start
	e.logger.Info(ctx, "executing job", map[string]interface{}{
		"job":       job.Name,
		"run_count": job.Metadata.RunCount,
	})

	var lastErr error
	maxAttempts := 1

	if job.RetryPolicy != nil {
		maxAttempts = job.RetryPolicy.MaxRetries + 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := job.RetryPolicy.NextRetryDelay(attempt - 1)
			if delay > 0 {
				e.logger.Info(ctx, "retrying job after delay", map[string]interface{}{
					"job":     job.Name,
					"attempt": attempt + 1,
					"delay":   delay,
				})

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		// Execute with timeout
		err := e.executeWithTimeout(ctx, job, attempt+1)
		if err == nil {
			// Success
			e.logger.Info(ctx, "job executed successfully", map[string]interface{}{
				"job":      job.Name,
				"attempt":  attempt + 1,
				"duration": time.Since(startTime),
			})
			return nil
		}

		lastErr = err
		e.logger.Error(ctx, "job execution failed", map[string]interface{}{
			"job":     job.Name,
			"attempt": attempt + 1,
			"error":   err.Error(),
		})

		e.metrics.JobFailed(job.Name, err)
	}

	// All attempts failed
	e.logger.Error(ctx, "job failed after all retries", map[string]interface{}{
		"job":      job.Name,
		"attempts": maxAttempts,
		"error":    lastErr.Error(),
	})

	return fmt.Errorf("job failed after %d attempts: %w", maxAttempts, lastErr)
}

func (e *DefaultJobExecutor) executeWithTimeout(ctx context.Context, job *Job, attempt int) (err error) {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
			e.logger.Error(ctx, "job panicked", map[string]interface{}{
				"job":     job.Name,
				"attempt": attempt,
				"panic":   r,
			})
			e.metrics.JobPanicked(job.Name, err)
		}
	}()

	// Execute handler in goroutine to respect timeout
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic: %v", r)
			}
		}()
		errChan <- job.Handler(timeoutCtx)
	}()

	// Wait for completion or timeout
	select {
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() == context.DeadlineExceeded {
			e.metrics.JobTimedOut(job.Name)
			return fmt.Errorf("job execution timeout after %s", job.Timeout)
		}
		return timeoutCtx.Err()
	case err := <-errChan:
		return err
	}
}

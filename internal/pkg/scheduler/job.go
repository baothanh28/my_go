package scheduler

import (
	"context"
	"time"
)

// JobStatus represents the current state of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job defines a scheduled task with all its configuration.
type Job struct {
	Name        string        `json:"name"`
	Schedule    Schedule      `json:"schedule"`
	RetryPolicy *RetryPolicy  `json:"retry_policy,omitempty"`
	Timeout     time.Duration `json:"timeout"`
	Handler     JobHandler    `json:"-"`
	Metadata    JobMetadata   `json:"metadata"`
}

// JobMetadata contains runtime information about a job.
type JobMetadata struct {
	Status      JobStatus  `json:"status"`
	NextRunAt   time.Time  `json:"next_run_at"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	RunCount    int64      `json:"run_count"`
	FailCount   int64      `json:"fail_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LockedBy    string     `json:"locked_by,omitempty"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// JobHandler is the user-defined function executed by the scheduler.
// It should be idempotent to handle at-least-once delivery semantics.
type JobHandler func(ctx context.Context) error

// RetryPolicy defines how a job should be retried on failure.
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval"`
	Multiplier      float64       `json:"multiplier"`
	Strategy        RetryStrategy `json:"strategy"`
}

// RetryStrategy defines the retry behavior.
type RetryStrategy string

const (
	RetryStrategyExponential RetryStrategy = "exponential"
	RetryStrategyLinear      RetryStrategy = "linear"
	RetryStrategyFixed       RetryStrategy = "fixed"
)

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		Strategy:        RetryStrategyExponential,
	}
}

// NextRetryDelay calculates the delay before the next retry attempt.
func (r *RetryPolicy) NextRetryDelay(attempt int) time.Duration {
	if attempt >= r.MaxRetries {
		return 0
	}

	var delay time.Duration

	switch r.Strategy {
	case RetryStrategyExponential:
		// Exponential backoff: initialInterval * multiplier^attempt
		multiplier := 1.0
		for i := 0; i < attempt; i++ {
			multiplier *= r.Multiplier
		}
		delay = time.Duration(float64(r.InitialInterval) * multiplier)

	case RetryStrategyLinear:
		// Linear backoff: initialInterval * (1 + attempt)
		delay = r.InitialInterval * time.Duration(1+attempt)

	case RetryStrategyFixed:
		// Fixed backoff: always use initialInterval
		delay = r.InitialInterval

	default:
		delay = r.InitialInterval
	}

	if delay > r.MaxInterval {
		delay = r.MaxInterval
	}

	return delay
}

// Validate checks if the job configuration is valid.
func (j *Job) Validate() error {
	if j.Name == "" {
		return ErrInvalidJobName
	}

	if j.Schedule == nil {
		return ErrInvalidSchedule
	}

	if j.Handler == nil {
		return ErrInvalidHandler
	}

	if j.Timeout <= 0 {
		return ErrInvalidTimeout
	}

	return nil
}

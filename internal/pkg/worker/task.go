package worker

import (
	"time"
)

// Task represents a unit of work to be processed by the worker
type Task struct {
	// ID is the unique identifier for the task
	ID string

	// Payload contains the raw data for the task
	Payload []byte

	// Metadata holds additional information about the task
	Metadata map[string]string

	// Retry is the current retry attempt count
	Retry int

	// MaxRetry is the maximum number of retry attempts allowed
	MaxRetry int

	// Timeout is the maximum duration for task execution
	Timeout time.Duration

	// CreatedAt is the timestamp when the task was created
	CreatedAt time.Time

	// ScheduledAt is when the task should be processed (for delayed tasks)
	ScheduledAt time.Time
}

// ShouldRetry returns true if the task can be retried
func (t *Task) ShouldRetry() bool {
	return t.Retry < t.MaxRetry
}

// IncrementRetry increments the retry counter
func (t *Task) IncrementRetry() {
	t.Retry++
}

// IsExpired returns true if the task has exceeded its timeout
func (t *Task) IsExpired() bool {
	if t.Timeout <= 0 {
		return false
	}
	return time.Since(t.CreatedAt) > t.Timeout
}

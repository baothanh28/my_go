package scheduler

import (
	"context"
	"time"
)

// BackendProvider handles persistence and distributed locking for jobs.
type BackendProvider interface {
	// SaveJob persists a job to the backend.
	SaveJob(ctx context.Context, job *Job) error

	// LoadJobs retrieves all jobs from the backend.
	LoadJobs(ctx context.Context) ([]*Job, error)

	// LoadJob retrieves a specific job by name.
	LoadJob(ctx context.Context, jobName string) (*Job, error)

	// UpdateMetadata updates job metadata (status, next run time, etc.).
	UpdateMetadata(ctx context.Context, jobName string, metadata *JobMetadata) error

	// DeleteJob removes a job from the backend.
	DeleteJob(ctx context.Context, jobName string) error

	// AcquireLock attempts to acquire a distributed lock for a job.
	// Returns true if lock was acquired, false otherwise.
	AcquireLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) (bool, error)

	// ReleaseLock releases a distributed lock.
	ReleaseLock(ctx context.Context, lockKey string, owner string) error

	// RefreshLock extends the TTL of an existing lock.
	RefreshLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) error

	// GetJobsDueForExecution returns jobs that should be executed now.
	GetJobsDueForExecution(ctx context.Context, now time.Time) ([]*Job, error)

	// Close closes the backend connection.
	Close() error
}

// LockInfo contains information about a distributed lock.
type LockInfo struct {
	Key       string
	Owner     string
	ExpiresAt time.Time
}

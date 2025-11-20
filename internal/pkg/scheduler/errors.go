package scheduler

import "errors"

var (
	// Job validation errors
	ErrInvalidJobName  = errors.New("job name cannot be empty")
	ErrInvalidSchedule = errors.New("schedule cannot be nil")
	ErrInvalidHandler  = errors.New("handler cannot be nil")
	ErrInvalidTimeout  = errors.New("timeout must be greater than zero")

	// Job operation errors
	ErrJobAlreadyExists  = errors.New("job already exists")
	ErrJobNotFound       = errors.New("job not found")
	ErrJobAlreadyRunning = errors.New("job is already running")
	ErrJobPaused         = errors.New("job is paused")

	// Scheduler errors
	ErrSchedulerNotStarted     = errors.New("scheduler not started")
	ErrSchedulerAlreadyStarted = errors.New("scheduler already started")
	ErrSchedulerStopped        = errors.New("scheduler stopped")

	// Lock errors
	ErrLockAcquisitionFailed = errors.New("failed to acquire lock")
	ErrLockNotHeld           = errors.New("lock not held")

	// Backend errors
	ErrBackendNotAvailable    = errors.New("backend not available")
	ErrBackendOperationFailed = errors.New("backend operation failed")
)

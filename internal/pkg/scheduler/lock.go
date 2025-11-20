package scheduler

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultLockTTL is the default TTL for distributed locks.
	DefaultLockTTL = 30 * time.Second

	// DefaultLockRefreshInterval is how often to refresh locks.
	DefaultLockRefreshInterval = 10 * time.Second
)

// DistributedLock manages distributed locking for job execution.
type DistributedLock struct {
	backend         BackendProvider
	logger          Logger
	metrics         MetricsCollector
	lockTTL         time.Duration
	refreshInterval time.Duration
}

// NewDistributedLock creates a new distributed lock manager.
func NewDistributedLock(
	backend BackendProvider,
	logger Logger,
	metrics MetricsCollector,
) *DistributedLock {
	return &DistributedLock{
		backend:         backend,
		logger:          logger,
		metrics:         metrics,
		lockTTL:         DefaultLockTTL,
		refreshInterval: DefaultLockRefreshInterval,
	}
}

// WithLockTTL sets the lock TTL.
func (d *DistributedLock) WithLockTTL(ttl time.Duration) *DistributedLock {
	d.lockTTL = ttl
	return d
}

// WithRefreshInterval sets the lock refresh interval.
func (d *DistributedLock) WithRefreshInterval(interval time.Duration) *DistributedLock {
	d.refreshInterval = interval
	return d
}

// AcquireAndExecute acquires a lock and executes the job with automatic lock refresh.
func (d *DistributedLock) AcquireAndExecute(
	ctx context.Context,
	job *Job,
	owner string,
	executor JobExecutor,
) error {
	lockKey := fmt.Sprintf("job:%s", job.Name)

	// Try to acquire lock
	acquired, err := d.backend.AcquireLock(ctx, lockKey, d.lockTTL, owner)
	if err != nil {
		d.metrics.LockFailed(job.Name)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		d.logger.Debug(ctx, "failed to acquire lock (already held)", map[string]interface{}{
			"job": job.Name,
		})
		return ErrLockAcquisitionFailed
	}

	d.logger.Debug(ctx, "lock acquired", map[string]interface{}{
		"job": job.Name,
	})
	d.metrics.LockAcquired(job.Name)

	// Ensure lock is released
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := d.backend.ReleaseLock(releaseCtx, lockKey, owner); err != nil {
			d.logger.Error(ctx, "failed to release lock", map[string]interface{}{
				"job":   job.Name,
				"error": err.Error(),
			})
		} else {
			d.logger.Debug(ctx, "lock released", map[string]interface{}{
				"job": job.Name,
			})
			d.metrics.LockReleased(job.Name)
		}
	}()

	// Start lock refresh goroutine
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	defer cancelRefresh()

	refreshDone := make(chan struct{})
	go d.refreshLockPeriodically(refreshCtx, lockKey, owner, job.Name, refreshDone)
	defer func() { <-refreshDone }()

	// Execute job
	return executor.Execute(ctx, job)
}

func (d *DistributedLock) refreshLockPeriodically(
	ctx context.Context,
	lockKey string,
	owner string,
	jobName string,
	done chan<- struct{},
) {
	defer close(done)

	ticker := time.NewTicker(d.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.backend.RefreshLock(ctx, lockKey, d.lockTTL, owner); err != nil {
				d.logger.Warn(ctx, "failed to refresh lock", map[string]interface{}{
					"job":   jobName,
					"error": err.Error(),
				})
			} else {
				d.logger.Debug(ctx, "lock refreshed", map[string]interface{}{
					"job": jobName,
				})
				d.metrics.LockRefreshed(jobName)
			}
		}
	}
}

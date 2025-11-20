package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryBackend is an in-memory implementation of BackendProvider for testing.
type MemoryBackend struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	locks map[string]*LockInfo
}

// NewMemoryBackend creates a new in-memory backend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		jobs:  make(map[string]*Job),
		locks: make(map[string]*LockInfo),
	}
}

func (m *MemoryBackend) SaveJob(ctx context.Context, job *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy to avoid external modifications
	jobCopy := *job
	jobCopy.Metadata.UpdatedAt = time.Now()

	if _, exists := m.jobs[job.Name]; !exists {
		jobCopy.Metadata.CreatedAt = time.Now()
	}

	m.jobs[job.Name] = &jobCopy
	return nil
}

func (m *MemoryBackend) LoadJobs(ctx context.Context) ([]*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}

	return jobs, nil
}

func (m *MemoryBackend) LoadJob(ctx context.Context, jobName string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobName]
	if !exists {
		return nil, ErrJobNotFound
	}

	jobCopy := *job
	return &jobCopy, nil
}

func (m *MemoryBackend) UpdateMetadata(ctx context.Context, jobName string, metadata *JobMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobName]
	if !exists {
		return ErrJobNotFound
	}

	metadata.UpdatedAt = time.Now()
	job.Metadata = *metadata
	return nil
}

func (m *MemoryBackend) DeleteJob(ctx context.Context, jobName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[jobName]; !exists {
		return ErrJobNotFound
	}

	delete(m.jobs, jobName)
	return nil
}

func (m *MemoryBackend) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up expired locks
	if lock, exists := m.locks[lockKey]; exists {
		if time.Now().After(lock.ExpiresAt) {
			delete(m.locks, lockKey)
		} else if lock.Owner != owner {
			return false, nil
		}
	}

	// Acquire or refresh lock
	m.locks[lockKey] = &LockInfo{
		Key:       lockKey,
		Owner:     owner,
		ExpiresAt: time.Now().Add(ttl),
	}

	return true, nil
}

func (m *MemoryBackend) ReleaseLock(ctx context.Context, lockKey string, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[lockKey]
	if !exists {
		return ErrLockNotHeld
	}

	if lock.Owner != owner {
		return fmt.Errorf("lock owned by %s, not %s", lock.Owner, owner)
	}

	delete(m.locks, lockKey)
	return nil
}

func (m *MemoryBackend) RefreshLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, exists := m.locks[lockKey]
	if !exists {
		return ErrLockNotHeld
	}

	if lock.Owner != owner {
		return fmt.Errorf("lock owned by %s, not %s", lock.Owner, owner)
	}

	lock.ExpiresAt = time.Now().Add(ttl)
	return nil
}

func (m *MemoryBackend) GetJobsDueForExecution(ctx context.Context, now time.Time) ([]*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var dueJobs []*Job

	for _, job := range m.jobs {
		if job.Metadata.Status == JobStatusPaused ||
			job.Metadata.Status == JobStatusCancelled {
			continue
		}

		if !job.Metadata.NextRunAt.IsZero() &&
			(job.Metadata.NextRunAt.Before(now) || job.Metadata.NextRunAt.Equal(now)) {
			jobCopy := *job
			dueJobs = append(dueJobs, &jobCopy)
		}
	}

	return dueJobs, nil
}

func (m *MemoryBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobs = make(map[string]*Job)
	m.locks = make(map[string]*LockInfo)
	return nil
}

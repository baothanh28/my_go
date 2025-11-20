package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Scheduler manages registration and execution of scheduled jobs.
type Scheduler interface {
	Register(job *Job) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Pause(jobName string) error
	Resume(jobName string) error
	Remove(jobName string) error
	GetJob(jobName string) (*Job, error)
	GetAllJobs() ([]*Job, error)
}

// DefaultScheduler is the default implementation of Scheduler.
type DefaultScheduler struct {
	backend  BackendProvider
	executor JobExecutor
	lock     *DistributedLock
	logger   Logger
	metrics  MetricsCollector

	instanceID   string
	tickInterval time.Duration

	mu       sync.RWMutex
	jobs     map[string]*Job
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Worker pool
	workerPool    chan struct{}
	maxConcurrent int
}

// Config holds scheduler configuration.
type Config struct {
	TickInterval  time.Duration
	MaxConcurrent int
}

// DefaultConfig returns default scheduler configuration.
func DefaultConfig() *Config {
	return &Config{
		TickInterval:  5 * time.Second,
		MaxConcurrent: 10,
	}
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(
	backend BackendProvider,
	executor JobExecutor,
	lock *DistributedLock,
	logger Logger,
	metrics MetricsCollector,
	config *Config,
) *DefaultScheduler {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = &NoOpLogger{}
	}

	if metrics == nil {
		metrics = &NoOpMetrics{}
	}

	return &DefaultScheduler{
		backend:       backend,
		executor:      executor,
		lock:          lock,
		logger:        logger,
		metrics:       metrics,
		instanceID:    uuid.New().String(),
		tickInterval:  config.TickInterval,
		maxConcurrent: config.MaxConcurrent,
		jobs:          make(map[string]*Job),
		stopChan:      make(chan struct{}),
		workerPool:    make(chan struct{}, config.MaxConcurrent),
	}
}

// Register registers a new job with the scheduler.
func (s *DefaultScheduler) Register(job *Job) error {
	if err := job.Validate(); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if job already exists
	if _, exists := s.jobs[job.Name]; exists {
		return ErrJobAlreadyExists
	}

	// Initialize metadata
	now := time.Now()
	job.Metadata = JobMetadata{
		Status:    JobStatusPending,
		NextRunAt: job.Schedule.NextRun(now),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// If retry policy is not set, use default
	if job.RetryPolicy == nil {
		job.RetryPolicy = DefaultRetryPolicy()
	}

	// Save to backend
	if err := s.backend.SaveJob(context.Background(), job); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}

	// Add to local registry
	s.jobs[job.Name] = job

	s.logger.Info(context.Background(), "job registered", map[string]interface{}{
		"job":         job.Name,
		"schedule":    job.Schedule.String(),
		"next_run_at": job.Metadata.NextRunAt,
	})

	s.metrics.JobsRegistered(len(s.jobs))

	return nil
}

// Start starts the scheduler.
func (s *DefaultScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return ErrSchedulerAlreadyStarted
	}

	s.logger.Info(ctx, "starting scheduler", map[string]interface{}{
		"instance_id":    s.instanceID,
		"tick_interval":  s.tickInterval,
		"max_concurrent": s.maxConcurrent,
	})

	// Load jobs from backend
	if err := s.loadJobsFromBackend(ctx); err != nil {
		return fmt.Errorf("failed to load jobs: %w", err)
	}

	s.running = true
	s.stopChan = make(chan struct{})

	// Start scheduler loop
	s.wg.Add(1)
	go s.run(ctx)

	s.logger.Info(ctx, "scheduler started", map[string]interface{}{
		"instance_id": s.instanceID,
		"jobs_loaded": len(s.jobs),
	})

	return nil
}

// Stop stops the scheduler gracefully.
func (s *DefaultScheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return ErrSchedulerNotStarted
	}
	s.running = false
	s.mu.Unlock()

	s.logger.Info(ctx, "stopping scheduler", map[string]interface{}{
		"instance_id": s.instanceID,
	})

	// Signal stop
	close(s.stopChan)

	// Wait for scheduler loop to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		s.logger.Info(ctx, "scheduler stopped gracefully", map[string]interface{}{
			"instance_id": s.instanceID,
		})
	case <-ctx.Done():
		s.logger.Warn(ctx, "scheduler stop timeout", map[string]interface{}{
			"instance_id": s.instanceID,
		})
		return ctx.Err()
	}

	return nil
}

// Pause pauses a job.
func (s *DefaultScheduler) Pause(jobName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobName]
	if !exists {
		return ErrJobNotFound
	}

	job.Metadata.Status = JobStatusPaused
	job.Metadata.UpdatedAt = time.Now()

	if err := s.backend.UpdateMetadata(context.Background(), jobName, &job.Metadata); err != nil {
		return fmt.Errorf("failed to update job metadata: %w", err)
	}

	s.logger.Info(context.Background(), "job paused", map[string]interface{}{
		"job": jobName,
	})

	return nil
}

// Resume resumes a paused job.
func (s *DefaultScheduler) Resume(jobName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobName]
	if !exists {
		return ErrJobNotFound
	}

	if job.Metadata.Status != JobStatusPaused {
		return fmt.Errorf("job is not paused")
	}

	job.Metadata.Status = JobStatusPending
	job.Metadata.NextRunAt = job.Schedule.NextRun(time.Now())
	job.Metadata.UpdatedAt = time.Now()

	if err := s.backend.UpdateMetadata(context.Background(), jobName, &job.Metadata); err != nil {
		return fmt.Errorf("failed to update job metadata: %w", err)
	}

	s.logger.Info(context.Background(), "job resumed", map[string]interface{}{
		"job":         jobName,
		"next_run_at": job.Metadata.NextRunAt,
	})

	return nil
}

// Remove removes a job from the scheduler.
func (s *DefaultScheduler) Remove(jobName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[jobName]; !exists {
		return ErrJobNotFound
	}

	// Delete from backend
	if err := s.backend.DeleteJob(context.Background(), jobName); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	// Remove from local registry
	delete(s.jobs, jobName)

	s.logger.Info(context.Background(), "job removed", map[string]interface{}{
		"job": jobName,
	})

	s.metrics.JobsRegistered(len(s.jobs))

	return nil
}

// GetJob retrieves a job by name.
func (s *DefaultScheduler) GetJob(jobName string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobName]
	if !exists {
		return nil, ErrJobNotFound
	}

	return job, nil
}

// GetAllJobs returns all registered jobs.
func (s *DefaultScheduler) GetAllJobs() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (s *DefaultScheduler) loadJobsFromBackend(ctx context.Context) error {
	jobs, err := s.backend.LoadJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		s.jobs[job.Name] = job
	}

	s.logger.Info(ctx, "jobs loaded from backend", map[string]interface{}{
		"count": len(jobs),
	})

	return nil
}

func (s *DefaultScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			s.logger.Debug(ctx, "scheduler loop stopping", nil)
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *DefaultScheduler) tick(ctx context.Context) {
	now := time.Now()

	// Get jobs due for execution
	dueJobs, err := s.backend.GetJobsDueForExecution(ctx, now)
	if err != nil {
		s.logger.Error(ctx, "failed to get due jobs", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if len(dueJobs) == 0 {
		return
	}

	s.logger.Debug(ctx, "found due jobs", map[string]interface{}{
		"count": len(dueJobs),
	})

	s.metrics.JobsQueued(len(dueJobs))

	// Execute due jobs
	for _, job := range dueJobs {
		// Create a copy to avoid race conditions
		jobCopy := *job

		// Acquire worker slot
		select {
		case s.workerPool <- struct{}{}:
			s.wg.Add(1)
			go s.executeJob(ctx, &jobCopy)
		default:
			s.logger.Warn(ctx, "worker pool full, skipping job", map[string]interface{}{
				"job": job.Name,
			})
		}
	}
}

func (s *DefaultScheduler) executeJob(ctx context.Context, job *Job) {
	defer s.wg.Done()
	defer func() { <-s.workerPool }()

	s.logger.Info(ctx, "executing job", map[string]interface{}{
		"job":      job.Name,
		"instance": s.instanceID,
	})

	// Update job metadata
	job.Metadata.Status = JobStatusRunning
	job.Metadata.LockedBy = s.instanceID
	now := time.Now()
	job.Metadata.LastRunAt = &now
	job.Metadata.RunCount++

	if err := s.backend.UpdateMetadata(ctx, job.Name, &job.Metadata); err != nil {
		s.logger.Error(ctx, "failed to update job metadata", map[string]interface{}{
			"job":   job.Name,
			"error": err.Error(),
		})
	}

	// Execute with distributed lock
	err := s.lock.AcquireAndExecute(ctx, job, s.instanceID, s.executor)

	// Update job after execution
	s.updateJobAfterExecution(ctx, job, err)
}

func (s *DefaultScheduler) updateJobAfterExecution(ctx context.Context, job *Job, execErr error) {
	now := time.Now()

	if execErr != nil {
		if execErr == ErrLockAcquisitionFailed {
			// Another instance is executing this job, skip
			s.logger.Debug(ctx, "job already executing on another instance", map[string]interface{}{
				"job": job.Name,
			})
			return
		}

		job.Metadata.Status = JobStatusFailed
		job.Metadata.LastError = execErr.Error()
		job.Metadata.FailCount++

		s.logger.Error(ctx, "job execution failed", map[string]interface{}{
			"job":   job.Name,
			"error": execErr.Error(),
		})
	} else {
		job.Metadata.Status = JobStatusCompleted
		job.Metadata.LastError = ""

		s.logger.Info(ctx, "job execution completed", map[string]interface{}{
			"job": job.Name,
		})
	}

	// Calculate next run time
	nextRun := job.Schedule.NextRun(now)
	if nextRun.IsZero() {
		// One-time job, mark as completed
		job.Metadata.Status = JobStatusCompleted
		s.logger.Info(ctx, "one-time job completed, will not reschedule", map[string]interface{}{
			"job": job.Name,
		})
	} else {
		// Reset status to pending for next run
		if job.Metadata.Status == JobStatusCompleted {
			job.Metadata.Status = JobStatusPending
		}
		job.Metadata.NextRunAt = nextRun
	}

	job.Metadata.LockedBy = ""
	job.Metadata.LockedUntil = nil
	job.Metadata.UpdatedAt = now

	// Save updated metadata
	if err := s.backend.UpdateMetadata(ctx, job.Name, &job.Metadata); err != nil {
		s.logger.Error(ctx, "failed to update job metadata after execution", map[string]interface{}{
			"job":   job.Name,
			"error": err.Error(),
		})
	}

	// Update local copy
	s.mu.Lock()
	if localJob, exists := s.jobs[job.Name]; exists {
		localJob.Metadata = job.Metadata
	}
	s.mu.Unlock()
}

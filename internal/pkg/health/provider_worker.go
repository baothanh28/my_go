package health

import (
	"context"
	"time"
)

// WorkerHealthChecker is an interface for checking worker health
// This allows different worker implementations to provide their own health check logic
type WorkerHealthChecker interface {
	// IsRunning returns true if the worker is currently running
	IsRunning() bool
	// GetQueueLength returns the current queue length (if applicable)
	// Returns -1 if queue length is not available
	GetQueueLength() int
	// GetQueueCapacity returns the queue capacity (if applicable)
	// Returns -1 if queue capacity is not available
	GetQueueCapacity() int
}

// WorkerProviderConfig configures the worker health provider
type WorkerProviderConfig struct {
	// Name is the name of the worker provider
	Name string
	// Checker is the worker health checker implementation
	Checker WorkerHealthChecker
	// MaxQueueLength is the maximum queue length before marking as degraded
	// If 0, queue length is not checked
	MaxQueueLength int
	// DegradedQueueLength is the queue length threshold for degraded status
	// If queue length exceeds this, status will be DEGRADED
	// If 0, uses 80% of MaxQueueLength
	DegradedQueueLength int
}

// WorkerProvider provides health checking for workers
type WorkerProvider struct {
	config WorkerProviderConfig
}

// NewWorkerProvider creates a new worker health provider
func NewWorkerProvider(config WorkerProviderConfig) *WorkerProvider {
	// Set default degraded threshold if not specified
	if config.DegradedQueueLength == 0 && config.MaxQueueLength > 0 {
		config.DegradedQueueLength = int(float64(config.MaxQueueLength) * 0.8)
	}

	return &WorkerProvider{
		config: config,
	}
}

// Name returns the name of the provider
func (p *WorkerProvider) Name() string {
	return p.config.Name
}

// Check performs the health check
func (p *WorkerProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.config.Name,
		Status:    StatusDown,
		Details:   make(map[string]interface{}),
		CheckedAt: time.Now(),
	}

	// Check if worker is running
	if !p.config.Checker.IsRunning() {
		result.Status = StatusDown
		result.Error = "worker is not running"
		result.Details["running"] = false
		return result
	}

	result.Details["running"] = true

	// Check queue metrics if available
	queueLength := p.config.Checker.GetQueueLength()
	queueCapacity := p.config.Checker.GetQueueCapacity()

	if queueLength >= 0 {
		result.Details["queue_length"] = queueLength
	}

	if queueCapacity >= 0 {
		result.Details["queue_capacity"] = queueCapacity
		result.Details["queue_usage_percent"] = float64(queueLength) / float64(queueCapacity) * 100
	}

	// Determine status based on queue metrics
	if p.config.MaxQueueLength > 0 && queueLength >= 0 {
		if queueLength >= p.config.MaxQueueLength {
			result.Status = StatusDown
			result.Error = "queue is full"
			return result
		}

		if queueLength >= p.config.DegradedQueueLength {
			result.Status = StatusDegraded
			result.Details["reason"] = "queue length approaching capacity"
			return result
		}
	}

	// Worker is running and queue is healthy
	result.Status = StatusUp
	return result
}

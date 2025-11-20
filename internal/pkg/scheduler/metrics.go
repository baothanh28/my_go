package scheduler

import "time"

// MetricsCollector defines the metrics interface for the scheduler.
type MetricsCollector interface {
	// Job execution metrics
	JobStarted(jobName string)
	JobCompleted(jobName string, duration time.Duration)
	JobFailed(jobName string, err error)
	JobTimedOut(jobName string)
	JobPanicked(jobName string, err error)

	// Lock metrics
	LockAcquired(jobName string)
	LockReleased(jobName string)
	LockFailed(jobName string)
	LockRefreshed(jobName string)

	// Scheduler metrics
	JobsRegistered(count int)
	JobsQueued(count int)
	JobsRunning(count int)
}

// NoOpMetrics is a metrics collector that does nothing.
type NoOpMetrics struct{}

func (n *NoOpMetrics) JobStarted(jobName string)                           {}
func (n *NoOpMetrics) JobCompleted(jobName string, duration time.Duration) {}
func (n *NoOpMetrics) JobFailed(jobName string, err error)                 {}
func (n *NoOpMetrics) JobTimedOut(jobName string)                          {}
func (n *NoOpMetrics) JobPanicked(jobName string, err error)               {}
func (n *NoOpMetrics) LockAcquired(jobName string)                         {}
func (n *NoOpMetrics) LockReleased(jobName string)                         {}
func (n *NoOpMetrics) LockFailed(jobName string)                           {}
func (n *NoOpMetrics) LockRefreshed(jobName string)                        {}
func (n *NoOpMetrics) JobsRegistered(count int)                            {}
func (n *NoOpMetrics) JobsQueued(count int)                                {}
func (n *NoOpMetrics) JobsRunning(count int)                               {}

package rate

import "time"

// MetricsCollector is the interface for collecting rate limiter metrics
type MetricsCollector interface {
	// RecordRequest records a rate limit request
	RecordRequest(strategy Strategy, allowed bool)

	// RecordAllowed records when a request is allowed
	RecordAllowed(strategy Strategy, key string)

	// RecordDenied records when a request is denied
	RecordDenied(strategy Strategy, key string, retryAfter time.Duration)

	// RecordError records an error
	RecordError(strategy Strategy, err error)

	// RecordFailOpen records a fail-open event
	RecordFailOpen(strategy Strategy)

	// RecordLatency records the latency of a rate limit check
	RecordLatency(strategy Strategy, duration time.Duration)
}

// NoOpMetrics is a no-op metrics collector implementation
type NoOpMetrics struct{}

func (m *NoOpMetrics) RecordRequest(strategy Strategy, allowed bool) {}
func (m *NoOpMetrics) RecordAllowed(strategy Strategy, key string)   {}
func (m *NoOpMetrics) RecordDenied(strategy Strategy, key string, retryAfter time.Duration) {
}
func (m *NoOpMetrics) RecordError(strategy Strategy, err error)                {}
func (m *NoOpMetrics) RecordFailOpen(strategy Strategy)                        {}
func (m *NoOpMetrics) RecordLatency(strategy Strategy, duration time.Duration) {}

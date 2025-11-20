package health

import (
	"context"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	// StatusUp indicates the component is healthy
	StatusUp HealthStatus = "UP"
	// StatusDown indicates the component is unhealthy
	StatusDown HealthStatus = "DOWN"
	// StatusDegraded indicates the component is partially healthy
	StatusDegraded HealthStatus = "DEGRADED"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Name      string                 `json:"name"`
	Status    HealthStatus           `json:"status"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CheckedAt time.Time              `json:"checked_at"`
	Error     string                 `json:"error,omitempty"`
}

// HealthProvider is the interface that all health check providers must implement
type HealthProvider interface {
	// Name returns the name of the provider
	Name() string
	// Check performs the health check and returns the result
	Check(ctx context.Context) HealthCheckResult
}

// HealthAggregator aggregates multiple health providers
type HealthAggregator interface {
	// RegisterProvider registers a health provider
	RegisterProvider(p HealthProvider)
	// Check runs all health checks and returns aggregated results
	Check(ctx context.Context) ([]HealthCheckResult, HealthStatus)
	// GetCachedResults returns cached results if async mode is enabled
	GetCachedResults() ([]HealthCheckResult, HealthStatus)
}

// AggregationStrategy defines how to aggregate health statuses
type AggregationStrategy string

const (
	// StrategyAll requires all providers to be UP for overall UP status
	StrategyAll AggregationStrategy = "ALL"
	// StrategyAny requires at least one provider to be UP for overall UP status
	StrategyAny AggregationStrategy = "ANY"
	// StrategyCritical requires critical providers to be UP
	StrategyCritical AggregationStrategy = "CRITICAL"
)

// HealthResponse is the JSON response for health check endpoint
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    []HealthCheckResult    `json:"checks"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

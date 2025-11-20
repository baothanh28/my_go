package health

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PostgresProvider checks PostgreSQL database health
type PostgresProvider struct {
	name string
	db   *sql.DB
}

// NewPostgresProvider creates a new Postgres health provider
func NewPostgresProvider(name string, db *sql.DB) *PostgresProvider {
	if name == "" {
		name = "postgres"
	}
	return &PostgresProvider{
		name: name,
		db:   db,
	}
}

// Name returns the provider name
func (p *PostgresProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *PostgresProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Measure latency
	start := time.Now()

	// Perform ping
	err := p.db.PingContext(ctx)
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()

	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to ping database: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	// Get database stats
	stats := p.db.Stats()
	result.Details["open_connections"] = stats.OpenConnections
	result.Details["in_use"] = stats.InUse
	result.Details["idle"] = stats.Idle
	result.Details["wait_count"] = stats.WaitCount
	result.Details["wait_duration_ms"] = stats.WaitDuration.Milliseconds()

	// Check if database is responding well
	if latency > 1*time.Second {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency detected"
		return result
	}

	// Check connection pool health
	maxOpen := stats.MaxOpenConnections
	if maxOpen > 0 && stats.OpenConnections >= maxOpen {
		result.Status = StatusDegraded
		result.Details["message"] = "connection pool exhausted"
		return result
	}

	result.Status = StatusUp
	return result
}

// DatabaseProvider is a generic database health provider
type DatabaseProvider struct {
	name       string
	db         *sql.DB
	query      string
	degradedMS int64
	timeoutMS  int64
}

// DatabaseProviderConfig configures the database health provider
type DatabaseProviderConfig struct {
	Name       string
	DB         *sql.DB
	Query      string // Custom query to execute (default: SELECT 1)
	DegradedMS int64  // Latency threshold for degraded status (default: 1000ms)
	TimeoutMS  int64  // Query timeout (default: 5000ms)
}

// NewDatabaseProvider creates a generic database health provider
func NewDatabaseProvider(config DatabaseProviderConfig) *DatabaseProvider {
	if config.Name == "" {
		config.Name = "database"
	}
	if config.Query == "" {
		config.Query = "SELECT 1"
	}
	if config.DegradedMS == 0 {
		config.DegradedMS = 1000
	}
	if config.TimeoutMS == 0 {
		config.TimeoutMS = 5000
	}

	return &DatabaseProvider{
		name:       config.Name,
		db:         config.DB,
		query:      config.Query,
		degradedMS: config.DegradedMS,
		timeoutMS:  config.TimeoutMS,
	}
}

// Name returns the provider name
func (p *DatabaseProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *DatabaseProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Create timeout context
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(p.timeoutMS)*time.Millisecond)
	defer cancel()

	// Measure latency
	start := time.Now()

	// Execute test query
	var dummy interface{}
	err := p.db.QueryRowContext(queryCtx, p.query).Scan(&dummy)
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()
	result.Details["query"] = p.query

	// Handle errors
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded || queryCtx.Err() == context.DeadlineExceeded {
			result.Status = StatusDown
			result.Error = "query timeout"
			result.Details["error"] = "timeout"
			return result
		}

		// sql.ErrNoRows is actually OK for health check
		if err != sql.ErrNoRows {
			result.Status = StatusDown
			result.Error = fmt.Sprintf("query failed: %v", err)
			result.Details["error"] = err.Error()
			return result
		}
	}

	// Get database stats
	stats := p.db.Stats()
	result.Details["open_connections"] = stats.OpenConnections
	result.Details["in_use"] = stats.InUse
	result.Details["idle"] = stats.Idle

	// Check latency threshold
	if latency.Milliseconds() > p.degradedMS {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency"
		return result
	}

	result.Status = StatusUp
	return result
}

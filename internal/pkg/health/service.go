package health

import (
	"context"
	"sync"
	"time"
)

// ServiceConfig configures the health service
type ServiceConfig struct {
	// AsyncMode enables background health checking
	AsyncMode bool
	// CheckInterval is the interval for async health checks
	CheckInterval time.Duration
	// DefaultTimeout is the default timeout for health checks
	DefaultTimeout time.Duration
	// AggregationStrategy defines how to aggregate statuses
	AggregationStrategy AggregationStrategy
	// CriticalProviders are providers that must be UP for overall UP status
	CriticalProviders []string
}

// DefaultServiceConfig returns default configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		AsyncMode:           false,
		CheckInterval:       30 * time.Second,
		DefaultTimeout:      5 * time.Second,
		AggregationStrategy: StrategyAll,
		CriticalProviders:   []string{},
	}
}

// Service is the main health check service
type Service struct {
	config    ServiceConfig
	providers []HealthProvider
	mu        sync.RWMutex

	// For async mode
	cachedResults []HealthCheckResult
	cachedStatus  HealthStatus
	lastCheck     time.Time
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewService creates a new health service
func NewService(config ServiceConfig) *Service {
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 5 * time.Second
	}
	if config.AggregationStrategy == "" {
		config.AggregationStrategy = StrategyAll
	}

	s := &Service{
		config:        config,
		providers:     make([]HealthProvider, 0),
		cachedResults: make([]HealthCheckResult, 0),
		cachedStatus:  StatusDown,
		stopCh:        make(chan struct{}),
	}

	if config.AsyncMode {
		s.startAsyncChecking()
	}

	return s
}

// RegisterProvider registers a health provider
func (s *Service) RegisterProvider(p HealthProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers = append(s.providers, p)
}

// Check runs all health checks synchronously
func (s *Service) Check(ctx context.Context) ([]HealthCheckResult, HealthStatus) {
	s.mu.RLock()
	providers := s.providers
	s.mu.RUnlock()

	if len(providers) == 0 {
		return []HealthCheckResult{}, StatusDown
	}

	results := make([]HealthCheckResult, len(providers))
	var wg sync.WaitGroup

	// Run all checks in parallel
	for i, provider := range providers {
		wg.Add(1)
		go func(idx int, p HealthProvider) {
			defer wg.Done()

			// Create a timeout context
			checkCtx, cancel := context.WithTimeout(ctx, s.config.DefaultTimeout)
			defer cancel()

			// Run check with timeout
			resultCh := make(chan HealthCheckResult, 1)
			go func() {
				resultCh <- p.Check(checkCtx)
			}()

			select {
			case result := <-resultCh:
				results[idx] = result
			case <-checkCtx.Done():
				results[idx] = HealthCheckResult{
					Name:      p.Name(),
					Status:    StatusDown,
					Details:   map[string]interface{}{"error": "timeout"},
					CheckedAt: time.Now(),
					Error:     "health check timeout",
				}
			}
		}(i, provider)
	}

	wg.Wait()

	// Aggregate status
	overallStatus := s.aggregateStatus(results)

	// Update cache if in async mode
	if s.config.AsyncMode {
		s.mu.Lock()
		s.cachedResults = results
		s.cachedStatus = overallStatus
		s.lastCheck = time.Now()
		s.mu.Unlock()
	}

	return results, overallStatus
}

// GetCachedResults returns cached results if async mode is enabled
func (s *Service) GetCachedResults() ([]HealthCheckResult, HealthStatus) {
	if !s.config.AsyncMode {
		// If not in async mode, run check immediately
		return s.Check(context.Background())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return cached results
	resultsCopy := make([]HealthCheckResult, len(s.cachedResults))
	copy(resultsCopy, s.cachedResults)

	return resultsCopy, s.cachedStatus
}

// aggregateStatus aggregates multiple health check results
func (s *Service) aggregateStatus(results []HealthCheckResult) HealthStatus {
	if len(results) == 0 {
		return StatusDown
	}

	switch s.config.AggregationStrategy {
	case StrategyAll:
		return s.aggregateAll(results)
	case StrategyAny:
		return s.aggregateAny(results)
	case StrategyCritical:
		return s.aggregateCritical(results)
	default:
		return s.aggregateAll(results)
	}
}

// aggregateAll requires all providers to be UP
func (s *Service) aggregateAll(results []HealthCheckResult) HealthStatus {
	upCount := 0
	downCount := 0
	degradedCount := 0

	for _, result := range results {
		switch result.Status {
		case StatusUp:
			upCount++
		case StatusDown:
			downCount++
		case StatusDegraded:
			degradedCount++
		}
	}

	// If any provider is down, overall status is down
	if downCount > 0 {
		return StatusDown
	}

	// If any provider is degraded, overall status is degraded
	if degradedCount > 0 {
		return StatusDegraded
	}

	// All providers are up
	return StatusUp
}

// aggregateAny requires at least one provider to be UP
func (s *Service) aggregateAny(results []HealthCheckResult) HealthStatus {
	for _, result := range results {
		if result.Status == StatusUp {
			return StatusUp
		}
	}

	// Check if any degraded
	for _, result := range results {
		if result.Status == StatusDegraded {
			return StatusDegraded
		}
	}

	return StatusDown
}

// aggregateCritical requires critical providers to be UP
func (s *Service) aggregateCritical(results []HealthCheckResult) HealthStatus {
	criticalMap := make(map[string]bool)
	for _, name := range s.config.CriticalProviders {
		criticalMap[name] = true
	}

	criticalDown := false
	criticalDegraded := false
	hasNonCriticalDown := false

	for _, result := range results {
		isCritical := criticalMap[result.Name]

		if isCritical {
			if result.Status == StatusDown {
				criticalDown = true
			} else if result.Status == StatusDegraded {
				criticalDegraded = true
			}
		} else {
			if result.Status == StatusDown {
				hasNonCriticalDown = true
			}
		}
	}

	// If any critical provider is down, overall is down
	if criticalDown {
		return StatusDown
	}

	// If any critical provider is degraded, overall is degraded
	if criticalDegraded {
		return StatusDegraded
	}

	// If non-critical providers are down, overall is degraded
	if hasNonCriticalDown {
		return StatusDegraded
	}

	return StatusUp
}

// startAsyncChecking starts background health checking
func (s *Service) startAsyncChecking() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(s.config.CheckInterval)
		defer ticker.Stop()

		// Run initial check
		s.Check(context.Background())

		for {
			select {
			case <-ticker.C:
				s.Check(context.Background())
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop stops the health service (for async mode)
func (s *Service) Stop() {
	if s.config.AsyncMode {
		close(s.stopCh)
		s.wg.Wait()
	}
}

// GetHealthResponse returns a formatted health response
func (s *Service) GetHealthResponse(ctx context.Context) HealthResponse {
	var results []HealthCheckResult
	var status HealthStatus

	if s.config.AsyncMode {
		results, status = s.GetCachedResults()
	} else {
		results, status = s.Check(ctx)
	}

	return HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Checks:    results,
		Details: map[string]interface{}{
			"total_checks": len(results),
			"strategy":     s.config.AggregationStrategy,
		},
	}
}

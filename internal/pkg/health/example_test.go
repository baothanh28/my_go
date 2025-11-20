package health_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"myapp/internal/pkg/health"
)

// Example_basicUsage demonstrates basic health service usage
func Example_basicUsage() {
	// Create health service
	config := health.DefaultServiceConfig()
	service := health.NewService(config)

	// Register a custom provider
	service.RegisterProvider(&mockProvider{name: "service-a"})
	service.RegisterProvider(&mockProvider{name: "service-b"})

	// Check health
	ctx := context.Background()
	results, status := service.Check(ctx)

	fmt.Printf("Overall Status: %s\n", status)
	fmt.Printf("Total Checks: %d\n", len(results))
	// Output:
	// Overall Status: UP
	// Total Checks: 2
}

// Example_asyncMode demonstrates async health checking
func Example_asyncMode() {
	// Create health service with async mode
	config := health.ServiceConfig{
		AsyncMode:      true,
		CheckInterval:  1 * time.Second,
		DefaultTimeout: 5 * time.Second,
	}
	service := health.NewService(config)
	defer service.Stop()

	// Register providers
	service.RegisterProvider(&mockProvider{name: "cache"})

	// Wait for first check
	time.Sleep(100 * time.Millisecond)

	// Get cached results (fast, non-blocking)
	results, status := service.GetCachedResults()

	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Checks: %d\n", len(results))
	// Output:
	// Status: UP
	// Checks: 1
}

// Example_httpHandler demonstrates HTTP handler integration
func Example_httpHandler() {
	// Create health service
	service := health.NewService(health.DefaultServiceConfig())
	service.RegisterProvider(&mockProvider{name: "database"})

	// Create HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := service.GetHealthResponse(r.Context())

		statusCode := http.StatusOK
		if response.Status == health.StatusDown {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	})

	// Test the handler
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	fmt.Printf("Status Code: %d\n", rec.Code)
	fmt.Printf("Content-Type: %s\n", rec.Header().Get("Content-Type"))
	// Output:
	// Status Code: 200
	// Content-Type: application/json
}

// Example_aggregationStrategies demonstrates different aggregation strategies
func Example_aggregationStrategies() {
	// Strategy: ALL (all providers must be UP)
	serviceAll := health.NewService(health.ServiceConfig{
		AggregationStrategy: health.StrategyAll,
	})
	serviceAll.RegisterProvider(&mockProvider{name: "db", status: health.StatusUp})
	serviceAll.RegisterProvider(&mockProvider{name: "cache", status: health.StatusDegraded})
	_, statusAll := serviceAll.Check(context.Background())
	fmt.Printf("ALL Strategy: %s\n", statusAll)

	// Strategy: ANY (at least one provider UP)
	serviceAny := health.NewService(health.ServiceConfig{
		AggregationStrategy: health.StrategyAny,
	})
	serviceAny.RegisterProvider(&mockProvider{name: "db", status: health.StatusUp})
	serviceAny.RegisterProvider(&mockProvider{name: "cache", status: health.StatusDown})
	_, statusAny := serviceAny.Check(context.Background())
	fmt.Printf("ANY Strategy: %s\n", statusAny)

	// Strategy: CRITICAL (only critical providers matter)
	serviceCritical := health.NewService(health.ServiceConfig{
		AggregationStrategy: health.StrategyCritical,
		CriticalProviders:   []string{"db"},
	})
	serviceCritical.RegisterProvider(&mockProvider{name: "db", status: health.StatusUp})
	serviceCritical.RegisterProvider(&mockProvider{name: "cache", status: health.StatusDown})
	_, statusCritical := serviceCritical.Check(context.Background())
	fmt.Printf("CRITICAL Strategy: %s\n", statusCritical)
	// Output:
	// ALL Strategy: DEGRADED
	// ANY Strategy: UP
	// CRITICAL Strategy: DEGRADED
}

// Example_customProvider demonstrates creating a custom health provider
func Example_customProvider() {
	// Create custom provider
	provider := &customBusinessLogicProvider{
		name:          "business-logic",
		checkInterval: 5 * time.Second,
	}

	// Create service and register
	service := health.NewService(health.DefaultServiceConfig())
	service.RegisterProvider(provider)

	// Check health
	results, _ := service.Check(context.Background())
	fmt.Printf("Provider: %s\n", results[0].Name)
	fmt.Printf("Status: %s\n", results[0].Status)
	// Output:
	// Provider: business-logic
	// Status: UP
}

// Example_httpProvider demonstrates HTTP health provider
func Example_httpProvider() {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	// Create HTTP provider
	provider := health.NewHTTPProvider(health.HTTPProviderConfig{
		Name:           "external-api",
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
		Timeout:        5 * time.Second,
	})

	// Check health
	result := provider.Check(context.Background())
	fmt.Printf("Provider: %s\n", result.Name)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Has Latency: %v\n", result.Details["latency_ms"] != nil)
	// Output:
	// Provider: external-api
	// Status: UP
	// Has Latency: true
}

// mockProvider is a simple mock for testing
type mockProvider struct {
	name   string
	status health.HealthStatus
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Check(ctx context.Context) health.HealthCheckResult {
	status := m.status
	if status == "" {
		status = health.StatusUp
	}

	return health.HealthCheckResult{
		Name:      m.name,
		Status:    status,
		CheckedAt: time.Now(),
		Details:   map[string]interface{}{"mock": true},
	}
}

// customBusinessLogicProvider demonstrates a custom provider
type customBusinessLogicProvider struct {
	name          string
	checkInterval time.Duration
}

func (p *customBusinessLogicProvider) Name() string {
	return p.name
}

func (p *customBusinessLogicProvider) Check(ctx context.Context) health.HealthCheckResult {
	result := health.HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate some business logic check
	// For example: check queue depth, pending jobs, etc.
	queueDepth := 10
	maxQueueDepth := 100

	result.Details["queue_depth"] = queueDepth
	result.Details["max_queue_depth"] = maxQueueDepth

	if queueDepth < maxQueueDepth {
		result.Status = health.StatusUp
	} else if queueDepth < maxQueueDepth*2 {
		result.Status = health.StatusDegraded
		result.Details["message"] = "queue filling up"
	} else {
		result.Status = health.StatusDown
		result.Error = "queue overflow"
	}

	return result
}

package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPProvider checks HTTP endpoint health
type HTTPProvider struct {
	name             string
	url              string
	method           string
	expectedStatus   int
	timeout          time.Duration
	degradedMS       int64
	client           *http.Client
	headers          map[string]string
	validateResponse func([]byte) error
}

// HTTPProviderConfig configures the HTTP health provider
type HTTPProviderConfig struct {
	Name             string
	URL              string
	Method           string             // Default: GET
	ExpectedStatus   int                // Default: 200
	Timeout          time.Duration      // Default: 5s
	DegradedMS       int64              // Latency threshold for degraded (default: 1000ms)
	Client           *http.Client       // Optional custom HTTP client
	Headers          map[string]string  // Optional headers
	ValidateResponse func([]byte) error // Optional response validator
}

// NewHTTPProvider creates a new HTTP health provider
func NewHTTPProvider(config HTTPProviderConfig) *HTTPProvider {
	if config.Name == "" {
		config.Name = "http"
	}
	if config.Method == "" {
		config.Method = http.MethodGet
	}
	if config.ExpectedStatus == 0 {
		config.ExpectedStatus = http.StatusOK
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.DegradedMS == 0 {
		config.DegradedMS = 1000
	}
	if config.Client == nil {
		config.Client = &http.Client{
			Timeout: config.Timeout,
		}
	}

	return &HTTPProvider{
		name:             config.Name,
		url:              config.URL,
		method:           config.Method,
		expectedStatus:   config.ExpectedStatus,
		timeout:          config.Timeout,
		degradedMS:       config.DegradedMS,
		client:           config.Client,
		headers:          config.Headers,
		validateResponse: config.ValidateResponse,
	}
}

// Name returns the provider name
func (p *HTTPProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *HTTPProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	result.Details["url"] = p.url
	result.Details["method"] = p.method

	// Create request
	req, err := http.NewRequestWithContext(ctx, p.method, p.url, nil)
	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	// Add headers
	for key, value := range p.headers {
		req.Header.Set(key, value)
	}

	// Measure latency
	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()

	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.Details["status_code"] = resp.StatusCode

	// Check status code
	if resp.StatusCode != p.expectedStatus {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("unexpected status code: got %d, expected %d", resp.StatusCode, p.expectedStatus)
		result.Details["expected_status"] = p.expectedStatus
		return result
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Status = StatusDegraded
		result.Error = fmt.Sprintf("failed to read response body: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	result.Details["response_size"] = len(body)

	// Validate response if validator provided
	if p.validateResponse != nil {
		if err := p.validateResponse(body); err != nil {
			result.Status = StatusDown
			result.Error = fmt.Sprintf("response validation failed: %v", err)
			result.Details["validation_error"] = err.Error()
			return result
		}
	}

	// Check latency threshold
	if latency.Milliseconds() > p.degradedMS {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency detected"
		return result
	}

	result.Status = StatusUp
	return result
}

// GRPCProvider checks gRPC service health (using HTTP/2 health check)
type GRPCProvider struct {
	name       string
	address    string
	timeout    time.Duration
	degradedMS int64
}

// GRPCProviderConfig configures the gRPC health provider
type GRPCProviderConfig struct {
	Name       string
	Address    string        // gRPC server address
	Timeout    time.Duration // Default: 5s
	DegradedMS int64         // Latency threshold for degraded (default: 500ms)
}

// NewGRPCProvider creates a new gRPC health provider
func NewGRPCProvider(config GRPCProviderConfig) *GRPCProvider {
	if config.Name == "" {
		config.Name = "grpc"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.DegradedMS == 0 {
		config.DegradedMS = 500
	}

	return &GRPCProvider{
		name:       config.Name,
		address:    config.Address,
		timeout:    config.Timeout,
		degradedMS: config.DegradedMS,
	}
}

// Name returns the provider name
func (p *GRPCProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *GRPCProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	result.Details["address"] = p.address

	// For now, this is a simplified check
	// In production, you would use grpc.health.v1.Health/Check RPC
	// This requires importing google.golang.org/grpc/health/grpc_health_v1

	// Simple TCP dial check as placeholder
	start := time.Now()

	// Create a basic HTTP client to check if the server is responding
	client := &http.Client{
		Timeout: p.timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s", p.address), nil)
	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()

	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("connection failed: %v", err)
		result.Details["error"] = err.Error()
		return result
	}
	defer resp.Body.Close()

	// Check latency threshold
	if latency.Milliseconds() > p.degradedMS {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency detected"
		return result
	}

	result.Status = StatusUp
	result.Details["message"] = "basic connectivity check passed"
	return result
}

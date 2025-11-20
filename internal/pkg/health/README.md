# Health Package

A comprehensive health check package for Go microservices, providing health monitoring for databases, Redis, HTTP endpoints, and custom components.

## Features

- ✅ **Multiple Providers**: Built-in support for PostgreSQL, Redis, HTTP endpoints
- ✅ **Sync & Async Modes**: Run health checks on-demand or in background
- ✅ **Aggregation Strategies**: Flexible health status aggregation (ALL, ANY, CRITICAL)
- ✅ **Low Latency**: Parallel execution with configurable timeouts
- ✅ **Thread-Safe**: Concurrent access support with proper locking
- ✅ **Extensible**: Easy to add custom health providers
- ✅ **Structured Results**: JSON-friendly health check responses
- ✅ **Degraded Status**: Detect performance degradation (high latency, connection issues)

## Installation

```bash
go get github.com/redis/go-redis/v9
```

## Quick Start

### Basic Usage

```go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"myapp/internal/pkg/health"
	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
)

func main() {
	// Create health service
	config := health.ServiceConfig{
		AsyncMode:           false,
		DefaultTimeout:      5 * time.Second,
		AggregationStrategy: health.StrategyAll,
	}
	service := health.NewService(config)

	// Register providers
	db, _ := sql.Open("postgres", "postgres://...")
	service.RegisterProvider(health.NewPostgresProvider("database", db))

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	service.RegisterProvider(health.NewRedisProvider(health.RedisProviderConfig{
		Name:   "redis",
		Client: rdb,
	}))

	// Check health
	ctx := context.Background()
	results, status := service.Check(ctx)
	fmt.Printf("Overall Status: %s\n", status)
	for _, result := range results {
		fmt.Printf("- %s: %s (latency: %v)\n", 
			result.Name, result.Status, result.Details["latency_ms"])
	}
}
```

### HTTP Handler

```go
func healthHandler(service *health.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := service.GetHealthResponse(r.Context())
		
		statusCode := http.StatusOK
		if response.Status == health.StatusDown {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == health.StatusDegraded {
			statusCode = http.StatusOK // or 429, depending on your preference
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// Register in your router
http.HandleFunc("/health", healthHandler(service))
```

### Async Mode (Background Checking)

```go
config := health.ServiceConfig{
	AsyncMode:     true,
	CheckInterval: 30 * time.Second, // Check every 30 seconds
}
service := health.NewService(config)

// Register providers...

// Get cached results (no blocking health checks)
results, status := service.GetCachedResults()

// Don't forget to stop when shutting down
defer service.Stop()
```

## Built-in Providers

### PostgreSQL Provider

```go
db, _ := sql.Open("postgres", "connection-string")
provider := health.NewPostgresProvider("my-database", db)
service.RegisterProvider(provider)
```

**Checks:**
- Database connectivity (PING)
- Connection pool statistics
- Query latency

**Status:**
- `UP`: Latency < 1s, connections available
- `DEGRADED`: High latency or pool exhausted
- `DOWN`: Connection failed or timeout

### Redis Provider

```go
rdb := redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})
provider := health.NewRedisProvider(health.RedisProviderConfig{
	Name:       "redis",
	Client:     rdb,
	DegradedMS: 100, // Mark as degraded if PING > 100ms
})
service.RegisterProvider(provider)
```

**Checks:**
- Redis PING command
- Connection pool statistics
- Server info availability

**Status:**
- `UP`: PING successful, low latency
- `DEGRADED`: High latency detected
- `DOWN`: Connection failed

### HTTP Provider

```go
provider := health.NewHTTPProvider(health.HTTPProviderConfig{
	Name:           "external-api",
	URL:            "https://api.example.com/health",
	Method:         http.MethodGet,
	ExpectedStatus: http.StatusOK,
	Timeout:        5 * time.Second,
	DegradedMS:     1000,
	Headers: map[string]string{
		"Authorization": "Bearer token",
	},
	ValidateResponse: func(body []byte) error {
		// Optional: validate response body
		return nil
	},
})
service.RegisterProvider(provider)
```

## Custom Health Providers

Implement the `HealthProvider` interface:

```go
type CustomProvider struct {
	name string
}

func (p *CustomProvider) Name() string {
	return p.name
}

func (p *CustomProvider) Check(ctx context.Context) health.HealthCheckResult {
	result := health.HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Perform your health check logic
	if everythingOK {
		result.Status = health.StatusUp
	} else {
		result.Status = health.StatusDown
		result.Error = "something went wrong"
	}

	return result
}
```

## Aggregation Strategies

### ALL (Default)

All providers must be UP for overall UP status.

```go
config := health.ServiceConfig{
	AggregationStrategy: health.StrategyAll,
}
```

- `UP`: All providers UP
- `DEGRADED`: Any provider degraded
- `DOWN`: Any provider down

### ANY

At least one provider must be UP for overall UP status.

```go
config := health.ServiceConfig{
	AggregationStrategy: health.StrategyAny,
}
```

- `UP`: At least one provider UP
- `DEGRADED`: No UP providers, but has degraded
- `DOWN`: All providers down

### CRITICAL

Only critical providers matter for overall status.

```go
config := health.ServiceConfig{
	AggregationStrategy: health.StrategyCritical,
	CriticalProviders:   []string{"database", "redis"},
}
```

- `UP`: All critical providers UP
- `DEGRADED`: Critical provider degraded OR non-critical down
- `DOWN`: Any critical provider down

## Health Response Format

```json
{
  "status": "UP",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": [
    {
      "name": "database",
      "status": "UP",
      "details": {
        "latency_ms": 5,
        "open_connections": 10,
        "in_use": 2,
        "idle": 8
      },
      "checked_at": "2024-01-15T10:30:00Z"
    },
    {
      "name": "redis",
      "status": "UP",
      "details": {
        "latency_ms": 2,
        "response": "PONG",
        "pool_hits": 1234,
        "pool_misses": 5
      },
      "checked_at": "2024-01-15T10:30:00Z"
    }
  ],
  "details": {
    "total_checks": 2,
    "strategy": "ALL"
  }
}
```

## Configuration

### Service Configuration

```go
type ServiceConfig struct {
	// Enable background health checking
	AsyncMode bool
	
	// Interval for async health checks (default: 30s)
	CheckInterval time.Duration
	
	// Default timeout for health checks (default: 5s)
	DefaultTimeout time.Duration
	
	// How to aggregate health statuses (default: ALL)
	AggregationStrategy AggregationStrategy
	
	// Providers that must be UP for overall UP status
	CriticalProviders []string
}
```

### Provider Configuration

Each provider has specific configuration options. See provider-specific documentation above.

## Best Practices

### 1. Use Async Mode in Production

```go
config := health.ServiceConfig{
	AsyncMode:     true,
	CheckInterval: 30 * time.Second,
}
```

This prevents health check endpoint from being slow or timing out when dependencies are down.

### 2. Set Appropriate Timeouts

```go
config := health.ServiceConfig{
	DefaultTimeout: 5 * time.Second, // Per provider
}
```

Ensure health checks don't block for too long.

### 3. Define Critical Providers

```go
config := health.ServiceConfig{
	AggregationStrategy: health.StrategyCritical,
	CriticalProviders:   []string{"database"},
}
```

Non-critical services (e.g., cache) failing shouldn't mark service as DOWN.

### 4. Monitor Health Endpoint

Set up monitoring to alert when health status is DOWN or DEGRADED.

### 5. Include in Kubernetes Probes

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Integration with FX (Uber's Dependency Injection)

The package includes an FX module for easy integration:

```go
package main

import (
	"myapp/internal/pkg/health"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		// Include health module
		health.Module,
		
		// Use health service in your app
		fx.Invoke(func(service *health.Service) {
			// Service is automatically configured with
			// registered database and Redis providers
		}),
	).Run()
}
```

The module automatically registers:
- Database health provider (if `*sql.DB` is provided)
- Redis health provider (if Redis client is provided)

## Thread Safety

All operations are thread-safe. The service uses `sync.RWMutex` to protect shared state and can be safely called from multiple goroutines.

## Performance

- **Parallel Execution**: All health checks run in parallel
- **Configurable Timeouts**: Each provider respects context deadlines
- **Minimal Overhead**: Async mode caches results for fast access
- **Target Latency**: < 50ms per provider (configurable)

## License

MIT


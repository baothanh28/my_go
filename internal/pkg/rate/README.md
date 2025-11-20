# Rate Limiter Package

A high-performance, flexible rate limiting package for Go with multiple strategies, storage backends, and integrations.

## Features

- **Multiple Strategies**: Token Bucket, Leaky Bucket, Fixed Window, Sliding Window
- **Storage Backends**: In-memory and Redis-based distributed rate limiting
- **High Performance**: >50k ops/s in-memory, <2ms Redis latency
- **Flexible Key Extraction**: Per-IP, per-user, per-endpoint, custom keys
- **Fail-Open/Fail-Close**: Configurable behavior when storage is unavailable
- **Integrations**: HTTP middleware, gRPC interceptors, worker hooks
- **Metrics & Logging**: Built-in observability hooks
- **Uber FX**: Native dependency injection support

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "myapp/internal/pkg/rate"
)

func main() {
    // Create storage
    storage := rate.NewMemoryStorage()
    defer storage.Close()

    // Configure rate limiter
    config := &rate.Config{
        Strategy: rate.StrategyTokenBucket,
        Rate:     10,              // 10 requests
        Burst:    20,              // burst up to 20
        Interval: 1 * time.Second, // per second
        TTL:      5 * time.Second,
        FailOpen: false,
    }

    // Create limiter
    limiter, err := rate.New(config, storage)
    if err != nil {
        panic(err)
    }
    defer limiter.Close()

    ctx := context.Background()

    // Check rate limit
    allowed, err := limiter.Allow(ctx, "user:123")
    if err != nil {
        panic(err)
    }

    if allowed {
        fmt.Println("Request allowed")
    } else {
        fmt.Println("Rate limit exceeded")
    }
}
```

### HTTP Middleware

```go
package main

import (
    "net/http"
    "time"
    
    "myapp/internal/pkg/rate"
)

func main() {
    storage := rate.NewMemoryStorage()
    
    config := &rate.Config{
        Strategy: rate.StrategyTokenBucket,
        Rate:     100,
        Burst:    200,
        Interval: 1 * time.Minute,
        TTL:      2 * time.Minute,
        FailOpen: true,
    }

    limiter, _ := rate.New(config, storage)

    // Create middleware with IP-based limiting
    middleware := rate.NewHTTPMiddleware(
        limiter,
        rate.WithKeyFunc(rate.IPKeyFunc()),
    )

    // Your handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Success"))
    })

    // Apply rate limiting
    http.Handle("/api", middleware.Middleware(handler))
    http.ListenAndServe(":8080", nil)
}
```

### gRPC Interceptor (Optional)

gRPC integration is available but requires additional dependencies. See [GRPC_INTEGRATION.md](./GRPC_INTEGRATION.md) for implementation details.

```bash
go get google.golang.org/grpc
```

### Redis Backend

```go
package main

import (
    "time"
    
    "myapp/internal/pkg/rate"
    "github.com/redis/go-redis/v9"
)

func main() {
    // Create Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // Create Redis storage
    storage := rate.NewRedisStorage(redisClient, "ratelimit")

    config := &rate.Config{
        Strategy: rate.StrategyTokenBucket,
        Rate:     100,
        Burst:    200,
        Interval: 1 * time.Minute,
        TTL:      2 * time.Minute,
        FailOpen: true,
    }

    limiter, _ := rate.New(config, storage)
    defer limiter.Close()

    // Use limiter...
}
```

## Rate Limiting Strategies

### Token Bucket

Tokens are added to a bucket at a constant rate. Each request consumes a token. If no tokens are available, the request is denied.

**Best for**: Smooth traffic with burst allowance, API rate limiting

```go
config := &rate.Config{
    Strategy: rate.StrategyTokenBucket,
    Rate:     10,   // 10 tokens per interval
    Burst:    20,   // bucket capacity
    Interval: 1 * time.Second,
    TTL:      5 * time.Second,
}
```

### Leaky Bucket

Requests "leak" from the bucket at a constant rate. New requests are added to the bucket; if the bucket overflows, requests are denied.

**Best for**: Traffic smoothing, enforcing constant output rate

```go
config := &rate.Config{
    Strategy: rate.StrategyLeakyBucket,
    Rate:     10,
    Burst:    20,
    Interval: 1 * time.Second,
    TTL:      5 * time.Second,
}
```

### Fixed Window

Counts requests in fixed time windows. Simple and memory-efficient, but can have edge-case burst issues.

**Best for**: Simple rate limiting, reducing storage overhead

```go
config := &rate.Config{
    Strategy: rate.StrategyFixedWindow,
    Rate:     100,
    Burst:    100,
    Interval: 1 * time.Minute,
    TTL:      2 * time.Minute,
}
```

### Sliding Window

Maintains a log of request timestamps. More accurate than fixed window but uses more memory.

**Best for**: Precise rate limiting, avoiding edge-case bursts

```go
config := &rate.Config{
    Strategy: rate.StrategySlidingWindow,
    Rate:     100,
    Burst:    100,
    Interval: 1 * time.Minute,
    TTL:      2 * time.Minute,
}
```

## Configuration

### Using Presets

```go
// Strict: 10 req/s, no burst
config := rate.ConfigStrict

// Moderate: 100 req/min with 2x burst
config := rate.ConfigModerate

// Lenient: 1000 req/hour with 3x burst
config := rate.ConfigLenient
```

### Custom Configuration

```go
config := &rate.LimiterConfig{
    Strategy: string(rate.StrategyTokenBucket),
    Rate:     50,
    Burst:    75,
    Interval: 30 * time.Second,
    TTL:      1 * time.Minute,
    FailOpen: true,
    Storage: rate.StorageConfig{
        Type:      "redis",
        KeyPrefix: "myapp:ratelimit",
        Redis: rate.RedisConfig{
            Addr:     "localhost:6379",
            Password: "",
            DB:       0,
            PoolSize: 10,
        },
    },
}
```

## Key Functions

### HTTP Key Functions

```go
// IP-based (default)
rate.WithKeyFunc(rate.IPKeyFunc())

// Per-endpoint
rate.WithKeyFunc(rate.PathKeyFunc())

// Custom header
rate.WithKeyFunc(rate.HeaderKeyFunc("X-API-Key"))

// Authenticated user
rate.WithKeyFunc(rate.UserKeyFunc("userID"))

// Custom
rate.WithKeyFunc(func(r *http.Request) string {
    return fmt.Sprintf("%s:%s", getUserID(r), r.URL.Path)
})
```


## Advanced Usage

### Reservations

```go
// Reserve tokens for future use
reservation, err := limiter.Reserve(ctx, "user:123")
if err != nil {
    return err
}

if reservation.OK {
    // Proceed immediately
    processRequest()
} else {
    // Wait for reservation
    if err := reservation.Wait(ctx); err != nil {
        reservation.Cancel() // Return tokens
        return err
    }
    processRequest()
}
```

### Batch Requests

```go
// Check and consume N tokens at once
allowed, err := limiter.AllowN(ctx, "batch:job", 10)
if !allowed {
    // Rate limited
}
```

### Check Without Consuming

```go
// Check if request would be allowed without consuming tokens
allowed, err := limiter.Check(ctx, "user:123")
```

### Reset Rate Limit

```go
// Reset rate limit for a specific key
err := limiter.Reset(ctx, "user:123")
```

## Uber FX Integration

```go
package main

import (
    "myapp/internal/pkg/rate"
    "go.uber.org/fx"
)

func main() {
    app := fx.New(
        rate.Module,
        rate.ProvideLimiterConfig(&rate.LimiterConfig{
            Strategy: string(rate.StrategyTokenBucket),
            Rate:     100,
            Burst:    200,
            Interval: 1 * time.Minute,
            TTL:      2 * time.Minute,
            Storage: rate.StorageConfig{
                Type: "memory",
            },
        }),
        fx.Invoke(func(limiter rate.Limiter) {
            // Use limiter
        }),
    )
    app.Run()
}
```

## Metrics & Logging

### Custom Logger

```go
type MyLogger struct{}

func (l *MyLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *MyLogger) Info(msg string, keysAndValues ...interface{}) {}
func (l *MyLogger) Warn(msg string, keysAndValues ...interface{}) {}
func (l *MyLogger) Error(msg string, keysAndValues ...interface{}) {}

limiter, _ := rate.New(config, storage, 
    rate.WithLogger(&MyLogger{}),
)
```

### Custom Metrics

```go
type MyMetrics struct{}

func (m *MyMetrics) RecordRequest(strategy rate.Strategy, allowed bool) {}
func (m *MyMetrics) RecordAllowed(strategy rate.Strategy, key string) {}
func (m *MyMetrics) RecordDenied(strategy rate.Strategy, key string, retryAfter time.Duration) {}
func (m *MyMetrics) RecordError(strategy rate.Strategy, err error) {}
func (m *MyMetrics) RecordFailOpen(strategy rate.Strategy) {}
func (m *MyMetrics) RecordLatency(strategy rate.Strategy, duration time.Duration) {}

limiter, _ := rate.New(config, storage,
    rate.WithMetrics(&MyMetrics{}),
)
```

## Performance

### Benchmarks

```
BenchmarkTokenBucketMemory-8    5000000    250 ns/op    > 4M ops/s
BenchmarkFixedWindowMemory-8    8000000    180 ns/op    > 5.5M ops/s
BenchmarkTokenBucketRedis-8      100000   1500 ns/op    > 650K ops/s
```

### Optimization Tips

1. **Use Fixed Window for high-throughput**: Fastest strategy with lowest memory
2. **Enable FailOpen for availability**: Allow requests when storage fails
3. **Use Redis for distributed systems**: Consistent limits across instances
4. **Tune TTL appropriately**: Balance memory usage vs. accuracy
5. **Use batch operations**: AllowN for bulk operations

## Error Handling

```go
allowed, err := limiter.Allow(ctx, key)
if err != nil {
    if errors.Is(err, rate.ErrStorageUnavailable) {
        // Storage backend unavailable
        // Behavior depends on FailOpen setting
    }
    if errors.Is(err, rate.ErrRateLimitExceeded) {
        // Rate limit exceeded
    }
}
```

## Testing

```bash
# Run tests
go test ./internal/pkg/rate/...

# Run benchmarks
go test -bench=. ./internal/pkg/rate/...

# Run with race detector
go test -race ./internal/pkg/rate/...
```

## Architecture

```
┌─────────────────┐
│     Limiter     │  (Public API)
└────────┬────────┘
         │
    ┌────┴────┐
    │ Executor │  (Strategy implementation)
    └────┬────┘
         │
    ┌────┴────┐
    │ Storage │  (Backend: Memory/Redis)
    └─────────┘
```

## Best Practices

1. **Choose the right strategy**: Token Bucket for most cases, Sliding Window for precision
2. **Set appropriate TTL**: At least 2x the interval to avoid race conditions
3. **Use Redis for distributed**: Consistent rate limits across multiple instances
4. **Enable metrics**: Monitor rate limit hits and storage latency
5. **Test fail scenarios**: Verify FailOpen/FailClose behavior
6. **Use proper keys**: Avoid key collisions, use namespaces

## License

See project LICENSE file.


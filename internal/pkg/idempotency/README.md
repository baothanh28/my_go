# Idempotency Package

A comprehensive idempotency package for ensuring operations are executed exactly once, even in distributed systems with retries, network issues, or job re-runs.

## Features

- ✅ **Idempotent execution** - Ensures operations run exactly once
- ✅ **Multiple storage backends** - Redis, Memory (SQL coming soon)
- ✅ **Type-safe API** - Generic support for strong typing
- ✅ **Distributed-safe** - Uses atomic operations (Redis SETNX, etc.)
- ✅ **Flexible serialization** - JSON serializer with extensible interface
- ✅ **Key generation** - UUID and hash-based key generators
- ✅ **State management** - Tracks processing, completed, and failed states
- ✅ **TTL support** - Automatic cleanup of old records
- ✅ **Dependency injection** - Fx module for easy integration

## Installation

```bash
go get myapp/internal/pkg/idempotency
```

## Quick Start

### Basic Usage

```go
import (
    "context"
    "time"
    "myapp/internal/pkg/idempotency"
)

// Create service
storage := idempotency.NewMemoryStorage()
svc := idempotency.NewService(storage, nil)

// Execute idempotently
ctx := context.Background()
key := "operation-123"
ttl := 5 * time.Minute

result, err := idempotency.ExecuteTyped(svc, ctx, key, ttl, func(ctx context.Context) (string, error) {
    // Your business logic here
    return "success", nil
})
```

### HTTP Handler Example

```go
func PaymentHandler(w http.ResponseWriter, r *http.Request) {
    // Get idempotency key from header
    idempotencyKey := r.Header.Get("Idempotency-Key")
    if idempotencyKey == "" {
        http.Error(w, "Missing idempotency key", http.StatusBadRequest)
        return
    }

    // Execute with idempotency
    result, err := idempotency.ExecuteTyped(
        idemService,
        r.Context(),
        idempotencyKey,
        5*time.Minute,
        func(ctx context.Context) (PaymentResponse, error) {
            return processPayment(ctx, req)
        },
    )

    if err != nil {
        if errors.Is(err, idempotency.ErrAlreadyProcessing) {
            http.Error(w, "Request is being processed", http.StatusConflict)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(result)
}
```

### Worker Example

```go
func ProcessMessage(ctx context.Context, msg Message) error {
    jobID := fmt.Sprintf("job-%d", msg.ID)
    
    _, err := idempotency.ExecuteTyped(
        idemService,
        ctx,
        jobID,
        time.Hour,
        func(ctx context.Context) (string, error) {
            return runJob(ctx, msg)
        },
    )
    
    return err
}
```

## Storage Backends

### Redis Storage (Recommended for Production)

```go
import (
    "github.com/redis/go-redis/v9"
    "myapp/internal/pkg/idempotency"
)

redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

storage := idempotency.NewRedisStorage(redisClient, "idempotency")
svc := idempotency.NewService(storage, nil)
```

### Memory Storage (For Testing)

```go
storage := idempotency.NewMemoryStorage()
svc := idempotency.NewService(storage, nil)
```

**⚠️ Warning**: Memory storage is NOT distributed-safe and should only be used for testing.

## Key Generation

### UUID Generator (Random Keys)

```go
keyGen := idempotency.NewUUIDKeyGenerator()
key, err := keyGen.Generate(nil) // Generates random UUID
```

### Hash Generator (Deterministic Keys)

```go
keyGen := idempotency.NewHashKeyGenerator("payment")

type Request struct {
    UserID string
    Amount int
}

key, err := keyGen.Generate(Request{
    UserID: "user-123",
    Amount: 100,
}) // Generates deterministic hash
```

## Dependency Injection (Fx)

```go
import (
    "go.uber.org/fx"
    "myapp/internal/pkg/idempotency"
)

func main() {
    fx.New(
        // Include idempotency module
        idempotency.Module,
        
        fx.Invoke(func(svc idempotency.Service) {
            // Use service
        }),
    ).Run()
}
```

For testing with memory storage:

```go
fx.New(
    idempotency.MemoryModule, // Uses memory storage
    // ... rest of your modules
).Run()
```

## API Reference

### Service Interface

```go
type Service interface {
    // Execute ensures idempotent execution (returns any)
    Execute(
        ctx context.Context,
        key string,
        ttl time.Duration,
        fn func(ctx context.Context) (any, error),
    ) (any, error)
}

// ExecuteTyped is a generic helper function for type-safe execution
func ExecuteTyped[T any](
    svc Service,
    ctx context.Context,
    key string,
    ttl time.Duration,
    fn func(ctx context.Context) (T, error),
) (T, error)

```

### Storage Interface

```go
type Storage interface {
    Load(ctx context.Context, key string) (*Record, error)
    TryMarkProcessing(ctx context.Context, key string, ttl time.Duration) (bool, error)
    SaveResult(ctx context.Context, key string, result []byte, ttl time.Duration) error
    SaveError(ctx context.Context, key string, errMsg string, ttl time.Duration) error
}
```

### Status Types

```go
const (
    StatusNone       Status = "none"       // Key doesn't exist
    StatusProcessing Status = "processing" // Currently being processed
    StatusCompleted  Status = "completed"  // Successfully completed
    StatusFailed     Status = "failed"     // Previously failed
)
```

### Errors

```go
var (
    ErrAlreadyProcessing    error // Another process is handling the key
    ErrStorageFailure       error // Storage operation failed
    ErrSerializationFailure error // Serialization failed
    ErrKeyGeneration        error // Key generation failed
    ErrPreviouslyFailed     error // Operation previously failed
)
```

## Flow Diagram

```
Client Request
     ↓
Generate/Get Idempotency Key
     ↓
Load(key) from Storage
     ↓
     ├─→ StatusCompleted? → Return cached result
     ├─→ StatusProcessing? → Return ErrAlreadyProcessing
     ├─→ StatusFailed? → Return ErrPreviouslyFailed
     └─→ StatusNone? → Continue
     ↓
TryMarkProcessing(key)
     ↓
     ├─→ Success? → Continue
     └─→ Failed? → Return ErrAlreadyProcessing
     ↓
Execute Business Logic
     ↓
     ├─→ Success? → SaveResult(key, result)
     └─→ Error? → SaveError(key, error)
     ↓
Return Result
```

## Best Practices

1. **Use meaningful keys**: Include entity type and ID in keys (e.g., `payment:user-123:txn-456`)

2. **Set appropriate TTL**: 
   - API requests: 5-15 minutes
   - Background jobs: 1-24 hours
   - Critical operations: 7 days

3. **Handle errors properly**:
   ```go
   result, err := svc.Execute(ctx, key, ttl, fn)
   if err != nil {
       if errors.Is(err, idempotency.ErrAlreadyProcessing) {
           // Return 409 Conflict
       } else if errors.Is(err, idempotency.ErrPreviouslyFailed) {
           // Return cached failure
       } else {
           // Handle other errors
       }
   }
   ```

4. **Use Redis in production**: Memory storage is not distributed-safe

5. **Monitor metrics**: Track duplication rates, cache hits, and failures

## Testing

Run tests:

```bash
go test -v ./internal/pkg/idempotency/...
```

Run with race detector:

```bash
go test -race -v ./internal/pkg/idempotency/...
```

## Performance Considerations

- **Redis Backend**: ~1-2ms latency per operation
- **Memory Backend**: <100μs latency (local only)
- **Serialization Overhead**: ~100-500μs for JSON (depends on payload size)
- **Recommended QPS**: 5,000-20,000 requests/second per instance

## Future Enhancements

- [ ] SQL storage backend (PostgreSQL, MySQL)
- [ ] Token-based idempotency
- [ ] Outbox pattern integration
- [ ] Metrics and observability
- [ ] Distributed lock fallback
- [ ] Pluggable encryption layer
- [ ] Multi-tenancy support
- [ ] MessagePack serializer for better performance

## License

Internal package for myapp project.


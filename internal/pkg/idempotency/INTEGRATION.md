# Integration Guide

This guide shows how to integrate the idempotency package into your microservices.

## Setup with Fx

### 1. Add to your service's app.go

```go
package myservice

import (
    "go.uber.org/fx"
    "myapp/internal/pkg/idempotency"
    "myapp/internal/pkg/redis"
)

var Module = fx.Module("myservice",
    // Include dependencies
    redis.Module,           // Provides Redis client
    idempotency.Module,     // Provides idempotency service
    
    fx.Provide(
        NewHandler,
        NewService,
    ),
)
```

### 2. Inject into your handler

```go
package myservice

import (
    "net/http"
    "time"
    "encoding/json"
    "errors"
    "myapp/internal/pkg/idempotency"
)

type Handler struct {
    idemSvc idempotency.Service
    bizSvc  *BusinessService
}

func NewHandler(idemSvc idempotency.Service, bizSvc *BusinessService) *Handler {
    return &Handler{
        idemSvc: idemSvc,
        bizSvc:  bizSvc,
    }
}

func (h *Handler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
    // Extract idempotency key from header
    idempotencyKey := r.Header.Get("Idempotency-Key")
    if idempotencyKey == "" {
        http.Error(w, "Missing Idempotency-Key header", http.StatusBadRequest)
        return
    }
    
    // Parse request
    var req PaymentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Execute with idempotency
    result, err := idempotency.ExecuteTyped(
        h.idemSvc,
        r.Context(),
        idempotencyKey,
        5*time.Minute,
        func(ctx context.Context) (PaymentResponse, error) {
            return h.bizSvc.ProcessPayment(ctx, req)
        },
    )
    
    // Handle errors
    if err != nil {
        if errors.Is(err, idempotency.ErrAlreadyProcessing) {
            w.WriteHeader(http.StatusConflict)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Request is currently being processed",
            })
            return
        }
        
        if errors.Is(err, idempotency.ErrPreviouslyFailed) {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Previous request failed: " + err.Error(),
            })
            return
        }
        
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return success
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}
```

## Worker Integration

### Using with Redis Streams Worker

```go
package worker

import (
    "context"
    "fmt"
    "time"
    "myapp/internal/pkg/idempotency"
    "myapp/internal/pkg/worker"
)

type NotificationWorker struct {
    idemSvc idempotency.Service
}

func NewNotificationWorker(idemSvc idempotency.Service) *NotificationWorker {
    return &NotificationWorker{idemSvc: idemSvc}
}

func (w *NotificationWorker) HandleMessage(ctx context.Context, msg *worker.Message) error {
    // Generate idempotency key from message ID
    idempotencyKey := fmt.Sprintf("notification:send:%s", msg.ID)
    
    // Execute with idempotency
    _, err := idempotency.ExecuteTyped(
        w.idemSvc,
        ctx,
        idempotencyKey,
        24*time.Hour, // Keep for 24 hours
        func(ctx context.Context) (string, error) {
            return w.sendNotification(ctx, msg)
        },
    )
    
    return err
}

func (w *NotificationWorker) sendNotification(ctx context.Context, msg *worker.Message) (string, error) {
    // Your business logic here
    return "notification-sent", nil
}
```

## Scheduler Integration

### Using with Scheduler Jobs

```go
package scheduler

import (
    "context"
    "fmt"
    "time"
    "myapp/internal/pkg/idempotency"
    "myapp/internal/pkg/scheduler"
)

type DailyReportJob struct {
    idemSvc idempotency.Service
}

func (j *DailyReportJob) Execute(ctx context.Context) error {
    // Generate idempotency key with date
    today := time.Now().Format("2006-01-02")
    idempotencyKey := fmt.Sprintf("daily-report:%s", today)
    
    // Execute with idempotency
    _, err := idempotency.ExecuteTyped(
        j.idemSvc,
        ctx,
        idempotencyKey,
        48*time.Hour, // Keep for 48 hours
        func(ctx context.Context) (string, error) {
            return j.generateReport(ctx, today)
        },
    )
    
    return err
}

func (j *DailyReportJob) generateReport(ctx context.Context, date string) (string, error) {
    // Your business logic here
    return "report-generated", nil
}
```

## Configuration

### Add to your config.yaml

```yaml
idempotency:
  redis_prefix: "myservice:idem"
  use_memory_storage: false  # Set to true only for local testing
```

### Load config in your app

```go
type Config struct {
    Idempotency *idempotency.Config `yaml:"idempotency"`
}

func provideConfig() (*Config, error) {
    var cfg Config
    // Load from yaml...
    return &cfg, nil
}
```

## Best Practices

### 1. Idempotency Key Generation

**For API Requests:**
```go
// Client should generate and send in header
// Header: Idempotency-Key: <uuid>

// Or generate from request content
import "myapp/internal/pkg/idempotency"

keyGen := idempotency.NewHashKeyGenerator("payment")
key, err := keyGen.Generate(struct{
    UserID  string
    Amount  int
    OrderID string
}{
    UserID:  req.UserID,
    Amount:  req.Amount,
    OrderID: req.OrderID,
})
```

**For Worker Messages:**
```go
// Use message ID
key := fmt.Sprintf("worker:%s:msg:%s", workerName, msg.ID)
```

**For Scheduled Jobs:**
```go
// Use job name + timestamp
key := fmt.Sprintf("job:%s:%s", jobName, time.Now().Format("2006-01-02-15"))
```

### 2. TTL Selection

- **API requests**: 5-15 minutes (prevent immediate retries)
- **Background jobs**: 1-24 hours (prevent re-queuing)
- **Scheduled jobs**: 24-48 hours (prevent duplicate runs)
- **Critical operations**: 7 days (audit trail)

### 3. Error Handling

Always handle the specific idempotency errors:

```go
result, err := idempotency.ExecuteTyped(svc, ctx, key, ttl, fn)
if err != nil {
    switch {
    case errors.Is(err, idempotency.ErrAlreadyProcessing):
        // Return 409 Conflict or retry later
        return nil, NewConflictError("request is being processed")
        
    case errors.Is(err, idempotency.ErrPreviouslyFailed):
        // Return cached failure
        return nil, NewFailedError("previous attempt failed")
        
    default:
        // Handle other errors
        return nil, err
    }
}
```

### 4. Testing

Use memory storage for unit tests:

```go
func TestMyHandler(t *testing.T) {
    storage := idempotency.NewMemoryStorage()
    serializer := idempotency.NewJSONSerializer()
    idemSvc := idempotency.NewService(storage, serializer)
    
    handler := NewHandler(idemSvc, mockBizSvc)
    
    // Test your handler...
}
```

Or use the test module:

```go
fx.New(
    fx.NopLogger,
    idempotency.MemoryModule,  // Uses memory storage
    // ... your modules
)
```

## Monitoring

### Add metrics (TODO)

```go
// Future enhancement: metrics package integration
// - idempotency_hits_total{status="completed"}
// - idempotency_hits_total{status="processing"}
// - idempotency_hits_total{status="failed"}
// - idempotency_execution_duration_seconds
```

### Add logging

```go
import "myapp/internal/pkg/logger"

func (h *Handler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
    log := logger.FromContext(r.Context())
    
    result, err := idempotency.ExecuteTyped(...)
    if err != nil {
        if errors.Is(err, idempotency.ErrAlreadyProcessing) {
            log.Warn("duplicate request detected", 
                "idempotency_key", idempotencyKey)
        }
        // ...
    }
}
```

## Migration from Old Implementation

If you're using the old `redis/idempotency.MarkIfFirst` pattern:

**Before:**
```go
marked, err := redisStore.MarkIfFirst(ctx, key, ttl)
if !marked {
    return ErrDuplicate
}
// do work
```

**After:**
```go
result, err := idempotency.ExecuteTyped(svc, ctx, key, ttl, func(ctx context.Context) (Result, error) {
    // do work
    return result, nil
})
```

Benefits:
- ✅ Stores and returns cached results
- ✅ Handles failures properly
- ✅ Type-safe execution
- ✅ Better error handling
- ✅ Distributed-safe with atomic operations

## Troubleshooting

### Issue: "ExecuteTyped requires *service implementation"

Make sure you're passing a Service created by `NewService`, not a wrapped interface.

### Issue: Type assertion failed

The ExecuteTyped function now properly handles type deserialization, but make sure your types are JSON-serializable.

### Issue: ErrAlreadyProcessing on every call

Check that your TTL is appropriate and not too long. Also verify Redis connectivity.

### Issue: Memory storage not working in distributed setup

Memory storage is NOT distributed-safe. Use Redis storage in production.


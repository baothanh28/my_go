# pgnotify

A robust, production-ready PostgreSQL LISTEN/NOTIFY package for Go with automatic reconnection, graceful shutdown, and comprehensive observability.

## Features

- 🔄 **Automatic Reconnection**: Exponential backoff reconnection with configurable attempts
- 🎯 **Multiple Channels**: Subscribe to multiple channels simultaneously
- 🔌 **Connection Management**: Automatic connection health checks and recovery
- 📊 **Observability**: Built-in hooks for logging, metrics, and monitoring
- 🛡️ **Graceful Shutdown**: Clean shutdown without goroutine leaks
- ⚡ **Concurrent Callbacks**: Non-blocking callback execution in separate goroutines
- 🔒 **Thread-Safe**: Safe for concurrent use across multiple goroutines
- 🎛️ **Configurable**: Extensive configuration options with sensible defaults
- 🧩 **Pluggable**: Abstract `ConnectionProvider` interface supports pgx, database/sql, or custom implementations

## Installation

```bash
go get github.com/yourusername/yourproject/internal/pkg/pgnotify
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/yourusername/yourproject/internal/pkg/pgnotify"
)

func main() {
    ctx := context.Background()
    dsn := "postgres://user:password@localhost:5432/dbname"
    
    // Create provider
    provider, err := pgnotify.NewPgxProvider(ctx, dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer provider.Close()
    
    // Create notifier
    notifier, err := pgnotify.NewNotifier(provider)
    if err != nil {
        log.Fatal(err)
    }
    
    // Subscribe to a channel
    sub, err := notifier.Subscribe(ctx, "events", func(ctx context.Context, n *pgnotify.Notification) error {
        log.Printf("Received: %s from %s\n", n.Payload, n.Channel)
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sub.Unsubscribe()
    
    // Start listening (blocking)
    if err := notifier.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Usage Examples

### Publishing Notifications

```go
err := notifier.Publish(ctx, "events", "Hello, World!")
if err != nil {
    log.Fatal(err)
}
```

### Multiple Subscriptions

```go
// Multiple callbacks can subscribe to the same channel
sub1, _ := notifier.Subscribe(ctx, "events", callback1)
sub2, _ := notifier.Subscribe(ctx, "events", callback2)

// Each callback will be invoked for every notification
```

### Multiple Channels

```go
channels := []string{"users", "orders", "products"}

for _, channel := range channels {
    ch := channel // Capture for closure
    notifier.Subscribe(ctx, ch, func(ctx context.Context, n *pgnotify.Notification) error {
        log.Printf("[%s] %s\n", ch, n.Payload)
        return nil
    })
}
```

### Custom Configuration

```go
notifier, err := pgnotify.NewNotifier(
    provider,
    pgnotify.WithReconnectInterval(2*time.Second),
    pgnotify.WithMaxReconnectInterval(60*time.Second),
    pgnotify.WithMaxReconnectAttempts(10),
    pgnotify.WithCallbackTimeout(5*time.Second),
    pgnotify.WithBufferSize(200),
    pgnotify.WithMaxPayloadSize(4000),
    pgnotify.WithLogger(logger),
)
```

### Observability Hooks

```go
hooks := &pgnotify.Hooks{
    OnNotification: func(n *pgnotify.Notification) {
        metrics.IncrementNotifications(n.Channel)
    },
    OnError: func(err error, channel string) {
        metrics.IncrementErrors(channel)
        log.Printf("Error on %s: %v", channel, err)
    },
    OnConnect: func() {
        log.Println("Connected to PostgreSQL")
    },
    OnDisconnect: func(err error) {
        log.Printf("Disconnected: %v", err)
    },
    OnReconnectSuccess: func(attempt int) {
        log.Printf("Reconnected after %d attempts", attempt)
    },
}

notifier, err := pgnotify.NewNotifier(
    provider,
    pgnotify.WithHooks(hooks),
)
```

### Graceful Shutdown

```go
// Create shutdown context with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Gracefully shutdown
if err := notifier.Shutdown(shutdownCtx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Use Cases

### Cache Invalidation

```go
notifier.Subscribe(ctx, "cache_invalidate", func(ctx context.Context, n *pgnotify.Notification) error {
    cacheKey := n.Payload
    cache.Delete(cacheKey)
    return nil
})

// In your application code:
db.Exec("UPDATE users SET name = $1 WHERE id = $2", name, id)
notifier.Publish(ctx, "cache_invalidate", fmt.Sprintf("user:%d", id))
```

### Real-time Notifications

```go
notifier.Subscribe(ctx, "user_notifications", func(ctx context.Context, n *pgnotify.Notification) error {
    var notification Notification
    json.Unmarshal([]byte(n.Payload), &notification)
    
    // Send to WebSocket, push notification, etc.
    websocket.Send(notification)
    return nil
})
```

### Background Job Triggering

```go
notifier.Subscribe(ctx, "trigger_job", func(ctx context.Context, n *pgnotify.Notification) error {
    jobType := n.Payload
    
    switch jobType {
    case "send_email":
        return emailService.SendPendingEmails()
    case "generate_report":
        return reportService.GenerateReport()
    }
    return nil
})

// Trigger from database trigger:
// CREATE OR REPLACE FUNCTION notify_job_trigger()
// RETURNS trigger AS $$
// BEGIN
//   PERFORM pg_notify('trigger_job', 'send_email');
//   RETURN NEW;
// END;
// $$ LANGUAGE plpgsql;
```

### Distributed Lock Coordination

```go
notifier.Subscribe(ctx, "lock_released", func(ctx context.Context, n *pgnotify.Notification) error {
    lockID := n.Payload
    
    // Try to acquire the lock
    if lockService.TryAcquire(lockID) {
        defer lockService.Release(lockID)
        // Do work...
    }
    return nil
})

// When releasing a lock:
lockService.Release(lockID)
notifier.Publish(ctx, "lock_released", lockID)
```

### Hot-reload Configuration

```go
notifier.Subscribe(ctx, "config_reload", func(ctx context.Context, n *pgnotify.Notification) error {
    configKey := n.Payload
    
    newConfig, err := configService.Load(configKey)
    if err != nil {
        return err
    }
    
    application.UpdateConfig(newConfig)
    log.Printf("Configuration reloaded: %s", configKey)
    return nil
})
```

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `ReconnectInterval` | 1s | Base delay between reconnection attempts |
| `MaxReconnectInterval` | 30s | Maximum delay between reconnection attempts |
| `MaxReconnectAttempts` | 0 (unlimited) | Maximum number of reconnection attempts |
| `ReconnectBackoffMultiplier` | 2.0 | Exponential backoff multiplier |
| `MaxPayloadSize` | 7900 bytes | Maximum payload size (PostgreSQL limit is 8000) |
| `PingInterval` | 30s | Connection health check interval |
| `CallbackTimeout` | 30s | Maximum callback execution time |
| `BufferSize` | 100 | Internal notification buffer size |
| `ShutdownTimeout` | 10s | Graceful shutdown timeout |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                          Notifier                            │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────────┐         ┌──────────────────┐          │
│  │  Subscription    │         │   Dispatcher     │          │
│  │    Manager       │◄────────┤                  │          │
│  └──────────────────┘         └──────────────────┘          │
│           │                            │                     │
│           │                            ▼                     │
│           │                   ┌──────────────────┐          │
│           │                   │   Callbacks      │          │
│           │                   │  (goroutines)    │          │
│           │                   └──────────────────┘          │
│           │                                                  │
│           ▼                                                  │
│  ┌──────────────────┐         ┌──────────────────┐          │
│  │   Connection     │         │     Metrics      │          │
│  │   Supervisor     │         │   Collector      │          │
│  └──────────────────┘         └──────────────────┘          │
│           │                                                  │
└───────────┼──────────────────────────────────────────────────┘
            │
            ▼
   ┌──────────────────┐
   │   Connection     │
   │    Provider      │
   │  (pgx/database)  │
   └──────────────────┘
            │
            ▼
    ┌──────────────┐
    │  PostgreSQL  │
    └──────────────┘
```

## Delivery Semantics

- **At-least-once delivery**: PostgreSQL LISTEN/NOTIFY provides at-least-once delivery semantics
- **No durability**: Notifications are not persisted; if no listeners are connected, the notification is lost
- **Ordering**: Notifications on the same channel are delivered in order
- **Idempotency**: For exactly-once semantics, implement idempotent callbacks with deduplication

## Best Practices

1. **Idempotent Callbacks**: Design callbacks to be idempotent since notifications may be delivered multiple times
2. **Error Handling**: Always handle errors in callbacks; they won't be propagated to PostgreSQL
3. **Timeouts**: Set appropriate callback timeouts to prevent hung callbacks
4. **Payload Size**: Keep payloads small (< 8KB); use references to larger data instead
5. **Monitoring**: Use hooks to integrate with your monitoring system
6. **Graceful Shutdown**: Always call `Shutdown()` with a timeout context
7. **Security**: Don't send sensitive data in payloads; use encryption or references

## Thread Safety

All public methods are thread-safe and can be called concurrently from multiple goroutines.

## Testing

```bash
# Run tests
go test ./internal/pkg/pgnotify/...

# Run tests with coverage
go test -cover ./internal/pkg/pgnotify/...

# Run tests with race detector
go test -race ./internal/pkg/pgnotify/...
```

## Limitations

- Maximum payload size: 8000 bytes (PostgreSQL limitation)
- At-least-once delivery semantics (not exactly-once)
- Notifications are not durable (not persisted)
- Requires a dedicated PostgreSQL connection for listening

## Contributing

Contributions are welcome! Please ensure:
- Tests pass: `go test ./...`
- Code is formatted: `gofmt -s -w .`
- Linter passes: `golangci-lint run`

## License

[Your License Here]

## Related Documentation

- [PostgreSQL LISTEN/NOTIFY Documentation](https://www.postgresql.org/docs/current/sql-notify.html)
- [pgx Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5)


package pgnotify_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"myapp/internal/pkg/pgnotify"
)

// Example_basic demonstrates basic usage of pgnotify.
func Example_basic() {
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
		fmt.Printf("Received: %s from %s\n", n.Payload, n.Channel)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sub.Unsubscribe()

	// Start listening in background
	go func() {
		if err := notifier.Start(ctx); err != nil {
			log.Printf("Notifier error: %v", err)
		}
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Publish a notification
	err = notifier.Publish(ctx, "events", "Hello, World!")
	if err != nil {
		log.Fatal(err)
	}

	// Wait for notification to be processed
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := notifier.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

// Example_withHooks demonstrates using hooks for monitoring.
func Example_withHooks() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	// Create hooks for monitoring
	hooks := &pgnotify.Hooks{
		OnNotification: func(n *pgnotify.Notification) {
			fmt.Printf("Hook: Notification received on %s\n", n.Channel)
		},
		OnError: func(err error, channel string) {
			fmt.Printf("Hook: Error on %s: %v\n", channel, err)
		},
		OnSubscribe: func(channel string) {
			fmt.Printf("Hook: Subscribed to %s\n", channel)
		},
		OnUnsubscribe: func(channel string) {
			fmt.Printf("Hook: Unsubscribed from %s\n", channel)
		},
		OnConnect: func() {
			fmt.Println("Hook: Connected to PostgreSQL")
		},
		OnDisconnect: func(err error) {
			fmt.Printf("Hook: Disconnected: %v\n", err)
		},
		OnReconnectAttempt: func(attempt int, nextRetry time.Duration) {
			fmt.Printf("Hook: Reconnect attempt %d (next in %v)\n", attempt, nextRetry)
		},
		OnReconnectSuccess: func(attempt int) {
			fmt.Printf("Hook: Reconnected after %d attempts\n", attempt)
		},
	}

	// Create notifier with hooks
	notifier, err := pgnotify.NewNotifier(
		provider,
		pgnotify.WithHooks(hooks),
		pgnotify.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, nil))),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe
	_, err = notifier.Subscribe(ctx, "events", func(ctx context.Context, n *pgnotify.Notification) error {
		fmt.Printf("Callback: %s\n", n.Payload)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start and use notifier...
}

// Example_multipleChannels demonstrates subscribing to multiple channels.
func Example_multipleChannels() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	notifier, err := pgnotify.NewNotifier(provider)
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe to multiple channels
	channels := []string{"users", "orders", "products"}

	for _, channel := range channels {
		ch := channel // Capture for closure
		_, err := notifier.Subscribe(ctx, ch, func(ctx context.Context, n *pgnotify.Notification) error {
			fmt.Printf("[%s] Received: %s\n", ch, n.Payload)
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Start listening
	go notifier.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	// Publish to different channels
	notifier.Publish(ctx, "users", "New user registered")
	notifier.Publish(ctx, "orders", "New order created")
	notifier.Publish(ctx, "products", "Product updated")

	time.Sleep(100 * time.Millisecond)
}

// Example_customConfig demonstrates using custom configuration.
func Example_customConfig() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	// Create notifier with custom configuration
	notifier, err := pgnotify.NewNotifier(
		provider,
		pgnotify.WithReconnectInterval(2*time.Second),
		pgnotify.WithMaxReconnectInterval(60*time.Second),
		pgnotify.WithMaxReconnectAttempts(10),
		pgnotify.WithCallbackTimeout(5*time.Second),
		pgnotify.WithBufferSize(200),
		pgnotify.WithMaxPayloadSize(4000),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use notifier...
	_ = notifier
}

// Example_cacheInvalidation demonstrates a cache invalidation use case.
func Example_cacheInvalidation() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	notifier, err := pgnotify.NewNotifier(provider)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate a simple cache
	cache := make(map[string]string)
	cache["user:1"] = "John Doe"

	// Subscribe to cache invalidation events
	_, err = notifier.Subscribe(ctx, "cache_invalidate", func(ctx context.Context, n *pgnotify.Notification) error {
		key := n.Payload
		delete(cache, key)
		fmt.Printf("Cache invalidated: %s\n", key)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// Start listening
	go notifier.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	// When data changes, invalidate cache
	fmt.Printf("Cache before: %v\n", cache)
	notifier.Publish(ctx, "cache_invalidate", "user:1")
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Cache after: %v\n", cache)
}

// Example_distributedLock demonstrates coordination between services.
func Example_distributedLock() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	notifier, err := pgnotify.NewNotifier(provider)
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe to lock release notifications
	_, err = notifier.Subscribe(ctx, "lock_released", func(ctx context.Context, n *pgnotify.Notification) error {
		lockID := n.Payload
		fmt.Printf("Lock released: %s - attempting to acquire\n", lockID)
		// Try to acquire the lock
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	go notifier.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// When a service releases a lock, notify others
	notifier.Publish(ctx, "lock_released", "resource:123")
}

// Example_backgroundJob demonstrates triggering background jobs.
func Example_backgroundJob() {
	ctx := context.Background()
	dsn := "postgres://user:password@localhost:5432/dbname"

	provider, err := pgnotify.NewPgxProvider(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	notifier, err := pgnotify.NewNotifier(provider)
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe to job trigger events
	_, err = notifier.Subscribe(ctx, "trigger_job", func(ctx context.Context, n *pgnotify.Notification) error {
		jobType := n.Payload
		fmt.Printf("Triggering background job: %s\n", jobType)
		// Execute job...
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	go notifier.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Trigger jobs from database triggers or application code
	notifier.Publish(ctx, "trigger_job", "send_email")
	notifier.Publish(ctx, "trigger_job", "generate_report")
}

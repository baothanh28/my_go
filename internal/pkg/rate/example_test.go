package rate_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"myapp/internal/pkg/rate"
)

func ExampleNew_tokenBucket() {
	// Create in-memory storage
	storage := rate.NewMemoryStorage()
	defer storage.Close()

	// Configure token bucket with 10 requests per second, burst of 20
	config := &rate.Config{
		Strategy: rate.StrategyTokenBucket,
		Rate:     10,
		Burst:    20,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	// Create limiter
	limiter, err := rate.New(config, storage)
	if err != nil {
		log.Fatal(err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Check rate limit for a user
	allowed, err := limiter.Allow(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}

	if allowed {
		fmt.Println("Request allowed")
	} else {
		fmt.Println("Request denied - rate limit exceeded")
	}

	// Output: Request allowed
}

func ExampleNew_fixedWindow() {
	storage := rate.NewMemoryStorage()
	defer storage.Close()

	config := &rate.Config{
		Strategy: rate.StrategyFixedWindow,
		Rate:     100,
		Burst:    100,
		Interval: 1 * time.Minute,
		TTL:      2 * time.Minute,
		FailOpen: false,
	}

	limiter, err := rate.New(config, storage)
	if err != nil {
		log.Fatal(err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Allow batch request
	allowed, err := limiter.AllowN(ctx, "api:endpoint:/users", 5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Batch allowed:", allowed)
	// Output: Batch allowed: true
}

func ExampleLimiter_Reserve() {
	storage := rate.NewMemoryStorage()
	defer storage.Close()

	config := &rate.Config{
		Strategy: rate.StrategyTokenBucket,
		Rate:     5,
		Burst:    5,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, _ := rate.New(config, storage)
	defer limiter.Close()

	ctx := context.Background()

	// Reserve a token
	reservation, err := limiter.Reserve(ctx, "task:process")
	if err != nil {
		log.Fatal(err)
	}

	if reservation.OK {
		fmt.Println("Reservation OK")
		// Process immediately
	} else {
		fmt.Printf("Reservation delayed by %v\n", reservation.Delay)
		// Wait for reservation
		if err := reservation.Wait(ctx); err != nil {
			// Timeout or cancelled
			reservation.Cancel()
			return
		}
		// Now process
	}

	// Output: Reservation OK
}

func ExampleNewHTTPMiddleware() {
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

	// Create HTTP middleware with IP-based rate limiting
	middleware := rate.NewHTTPMiddleware(
		limiter,
		rate.WithKeyFunc(rate.IPKeyFunc()),
	)

	// Create your HTTP handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Wrap with rate limiting
	http.Handle("/api", middleware.Middleware(handler))

	fmt.Println("HTTP middleware configured")
	// Output: HTTP middleware configured
}

func ExampleNewHTTPMiddleware_perEndpoint() {
	storage := rate.NewMemoryStorage()
	config := &rate.Config{
		Strategy: rate.StrategyFixedWindow,
		Rate:     1000,
		Burst:    1000,
		Interval: 1 * time.Hour,
		TTL:      2 * time.Hour,
		FailOpen: true,
	}

	limiter, _ := rate.New(config, storage)

	// Create middleware that combines IP and path for per-endpoint limits
	middleware := rate.NewHTTPMiddleware(
		limiter,
		rate.WithKeyFunc(rate.PathKeyFunc()),
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.Handle("/api/v1/users", middleware.Middleware(handler))

	fmt.Println("Per-endpoint rate limiting configured")
	// Output: Per-endpoint rate limiting configured
}

// Example for gRPC interceptor (requires google.golang.org/grpc)
// Uncomment if you have gRPC installed
//
// func ExampleUnaryServerInterceptor() {
// 	storage := rate.NewMemoryStorage()
// 	config := &rate.Config{
// 		Strategy: rate.StrategyTokenBucket,
// 		Rate:     50,
// 		Burst:    100,
// 		Interval: 1 * time.Second,
// 		TTL:      5 * time.Second,
// 		FailOpen: false,
// 	}
//
// 	limiter, _ := rate.New(config, storage)
//
// 	// Create gRPC interceptor
// 	_ = rate.UnaryServerInterceptor(limiter, rate.GRPCIPKeyFunc())
//
// 	fmt.Println("gRPC interceptor configured")
// 	// Output: gRPC interceptor configured
// }

func ExampleLimiterConfig() {
	// Use a preset configuration
	config := rate.ConfigModerate
	config.Storage.Type = "memory"

	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Strategy: %s, Rate: %d, Burst: %d\n",
		config.Strategy, config.Rate, config.Burst)
	// Output: Strategy: token_bucket, Rate: 100, Burst: 200
}

func ExampleLimiterConfig_custom() {
	// Create custom configuration
	config := &rate.LimiterConfig{
		Strategy: string(rate.StrategySlidingWindow),
		Rate:     50,
		Burst:    75,
		Interval: 30 * time.Second,
		TTL:      1 * time.Minute,
		FailOpen: true,
		Storage: rate.StorageConfig{
			Type:      "memory",
			KeyPrefix: "myapp:ratelimit",
		},
	}

	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}

	storage := rate.NewMemoryStorage()
	defer storage.Close()

	limiter, _ := rate.New(config.ToConfig(), storage)
	defer limiter.Close()

	fmt.Println("Custom limiter created")
	// Output: Custom limiter created
}

package rate

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketMemory(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyTokenBucket,
		Rate:     10,
		Burst:    20,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Test allowing requests within burst
	for i := 0; i < 20; i++ {
		allowed, err := limiter.Allow(ctx, "test-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Test denying requests after burst
	allowed, err := limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("request should be denied after burst exhausted")
	}

	// Wait and check if tokens refill
	time.Sleep(1 * time.Second)
	allowed, err = limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("request should be allowed after token refill")
	}
}

func TestFixedWindowMemory(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyFixedWindow,
		Rate:     5,
		Burst:    5,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Test allowing requests within window
	for i := 0; i < 5; i++ {
		allowed, err := limiter.Allow(ctx, "test-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Test denying requests after limit
	allowed, err := limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("request should be denied after limit reached")
	}

	// Wait for next window
	time.Sleep(1 * time.Second)
	allowed, err = limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("request should be allowed in new window")
	}
}

func TestSlidingWindowMemory(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategySlidingWindow,
		Rate:     3,
		Burst:    3,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Allow 3 requests
	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(ctx, "test-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Deny 4th request
	allowed, err := limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("request should be denied")
	}

	// Wait for sliding window to move
	time.Sleep(1100 * time.Millisecond)
	allowed, err = limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("request should be allowed after window slide")
	}
}

func TestReservation(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyTokenBucket,
		Rate:     10,
		Burst:    10,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Reserve immediately available
	res, err := limiter.Reserve(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Error("reservation should be OK")
	}
	if res.Delay != 0 {
		t.Errorf("delay should be 0, got %v", res.Delay)
	}

	// Exhaust tokens
	for i := 0; i < 9; i++ {
		limiter.Allow(ctx, "test-key")
	}

	// Reserve with delay
	res, err = limiter.Reserve(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK {
		t.Error("reservation should not be immediately OK")
	}
	if res.Delay <= 0 {
		t.Error("delay should be positive")
	}
}

func TestAllowN(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyTokenBucket,
		Rate:     10,
		Burst:    20,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Allow batch of 10
	allowed, err := limiter.AllowN(ctx, "test-key", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("batch of 10 should be allowed")
	}

	// Allow another batch of 10
	allowed, err = limiter.AllowN(ctx, "test-key", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("second batch of 10 should be allowed")
	}

	// Deny batch that exceeds remaining
	allowed, err = limiter.AllowN(ctx, "test-key", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("batch exceeding remaining should be denied")
	}
}

func TestReset(t *testing.T) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyTokenBucket,
		Rate:     5,
		Burst:    5,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, err := New(config, storage)
	if err != nil {
		t.Fatalf("failed to create limiter: %v", err)
	}
	defer limiter.Close()

	ctx := context.Background()

	// Exhaust tokens
	for i := 0; i < 5; i++ {
		limiter.Allow(ctx, "test-key")
	}

	// Should be denied
	allowed, _ := limiter.Allow(ctx, "test-key")
	if allowed {
		t.Error("should be denied after exhausting tokens")
	}

	// Reset
	err = limiter.Reset(ctx, "test-key")
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Should be allowed after reset
	allowed, err = limiter.Allow(ctx, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("should be allowed after reset")
	}
}

func BenchmarkTokenBucketMemory(b *testing.B) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyTokenBucket,
		Rate:     1000000,
		Burst:    1000000,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, _ := New(config, storage)
	defer limiter.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(ctx, "bench-key")
		}
	})
}

func BenchmarkFixedWindowMemory(b *testing.B) {
	storage := NewMemoryStorage()
	defer storage.Close()

	config := &Config{
		Strategy: StrategyFixedWindow,
		Rate:     1000000,
		Burst:    1000000,
		Interval: 1 * time.Second,
		TTL:      5 * time.Second,
		FailOpen: false,
	}

	limiter, _ := New(config, storage)
	defer limiter.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(ctx, "bench-key")
		}
	})
}

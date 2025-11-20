package rate

import (
	"context"
	"math"
	"time"
)

// LeakyBucketExecutor implements the leaky bucket algorithm
type LeakyBucketExecutor struct {
	logger  Logger
	metrics MetricsCollector
}

// NewLeakyBucketExecutor creates a new leaky bucket executor
func NewLeakyBucketExecutor(logger Logger, metrics MetricsCollector) *LeakyBucketExecutor {
	if logger == nil {
		logger = &NoOpLogger{}
	}
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}
	return &LeakyBucketExecutor{
		logger:  logger,
		metrics: metrics,
	}
}

// Execute implements the leaky bucket algorithm
// In leaky bucket, requests "leak" out at a constant rate
// New requests are added to the bucket; if bucket overflows, requests are rejected
func (e *LeakyBucketExecutor) Execute(ctx context.Context, key string, n int, cfg *Config, storage Storage) (*Result, error) {
	now := time.Now()

	// Get current state
	state, err := storage.Get(ctx, key)
	if err != nil && err != ErrStorageUnavailable {
		return nil, err
	}

	// Initialize state if not exists
	if state == nil {
		state = &State{
			Tokens:     0, // Leaky bucket starts empty
			LastUpdate: now,
		}
	}

	// Calculate how much has leaked since last update
	elapsed := now.Sub(state.LastUpdate)
	leaked := elapsed.Seconds() * float64(cfg.Rate) / cfg.Interval.Seconds()

	// Update current level (cannot go below 0)
	currentLevel := math.Max(0, state.Tokens-leaked)

	// Check if check-only operation (n=0)
	if n == 0 {
		remaining := int(math.Floor(float64(cfg.Burst) - currentLevel))
		return &Result{
			Allowed:   remaining > 0,
			Limit:     cfg.Rate,
			Remaining: remaining,
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Check if bucket has space
	if currentLevel+float64(n) <= float64(cfg.Burst) {
		// Add to bucket
		newLevel := currentLevel + float64(n)
		state.Tokens = newLevel
		state.LastUpdate = now

		// Save state
		if err := storage.Set(ctx, key, state, cfg.TTL); err != nil {
			return nil, err
		}

		remaining := int(math.Floor(float64(cfg.Burst) - newLevel))
		return &Result{
			Allowed:   true,
			Limit:     cfg.Rate,
			Remaining: remaining,
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Bucket would overflow - calculate when there would be space
	overflow := (currentLevel + float64(n)) - float64(cfg.Burst)
	retryAfter := time.Duration(overflow * cfg.Interval.Seconds() / float64(cfg.Rate) * float64(time.Second))

	remaining := int(math.Floor(float64(cfg.Burst) - currentLevel))
	return &Result{
		Allowed:    false,
		Limit:      cfg.Rate,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    now.Add(retryAfter),
	}, nil
}

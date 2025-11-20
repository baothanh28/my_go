package rate

import (
	"context"
	"math"
	"time"
)

// TokenBucketExecutor implements the token bucket algorithm
type TokenBucketExecutor struct {
	logger  Logger
	metrics MetricsCollector
}

// NewTokenBucketExecutor creates a new token bucket executor
func NewTokenBucketExecutor(logger Logger, metrics MetricsCollector) *TokenBucketExecutor {
	if logger == nil {
		logger = &NoOpLogger{}
	}
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}
	return &TokenBucketExecutor{
		logger:  logger,
		metrics: metrics,
	}
}

// Execute implements the token bucket algorithm
func (e *TokenBucketExecutor) Execute(ctx context.Context, key string, n int, cfg *Config, storage Storage) (*Result, error) {
	now := time.Now()

	// Get current state
	state, err := storage.Get(ctx, key)
	if err != nil && err != ErrStorageUnavailable {
		return nil, err
	}

	// Initialize state if not exists
	if state == nil {
		state = &State{
			Tokens:     float64(cfg.Burst),
			LastUpdate: now,
		}
	}

	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(state.LastUpdate)
	tokensToAdd := elapsed.Seconds() * float64(cfg.Rate) / cfg.Interval.Seconds()

	// Update tokens (capped at burst limit)
	newTokens := math.Min(state.Tokens+tokensToAdd, float64(cfg.Burst))

	// Check if check-only operation (n=0)
	if n == 0 {
		return &Result{
			Allowed:   newTokens >= 1,
			Limit:     cfg.Rate,
			Remaining: int(math.Floor(newTokens)),
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Check if enough tokens available
	if newTokens >= float64(n) {
		// Consume tokens
		newTokens -= float64(n)
		state.Tokens = newTokens
		state.LastUpdate = now

		// Save state
		if err := storage.Set(ctx, key, state, cfg.TTL); err != nil {
			return nil, err
		}

		return &Result{
			Allowed:   true,
			Limit:     cfg.Rate,
			Remaining: int(math.Floor(newTokens)),
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Not enough tokens - calculate retry after
	tokensNeeded := float64(n) - newTokens
	retryAfter := time.Duration(tokensNeeded * cfg.Interval.Seconds() / float64(cfg.Rate) * float64(time.Second))

	return &Result{
		Allowed:    false,
		Limit:      cfg.Rate,
		Remaining:  int(math.Floor(newTokens)),
		RetryAfter: retryAfter,
		ResetAt:    now.Add(retryAfter),
	}, nil
}

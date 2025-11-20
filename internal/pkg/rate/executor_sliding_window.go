package rate

import (
	"context"
	"time"
)

// SlidingWindowExecutor implements the sliding window log algorithm
type SlidingWindowExecutor struct {
	logger  Logger
	metrics MetricsCollector
}

// NewSlidingWindowExecutor creates a new sliding window executor
func NewSlidingWindowExecutor(logger Logger, metrics MetricsCollector) *SlidingWindowExecutor {
	if logger == nil {
		logger = &NoOpLogger{}
	}
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}
	return &SlidingWindowExecutor{
		logger:  logger,
		metrics: metrics,
	}
}

// Execute implements the sliding window log algorithm
// Maintains a log of request timestamps and slides the window
func (e *SlidingWindowExecutor) Execute(ctx context.Context, key string, n int, cfg *Config, storage Storage) (*Result, error) {
	now := time.Now()
	windowStart := now.Add(-cfg.Interval)

	// Get current state
	state, err := storage.Get(ctx, key)
	if err != nil && err != ErrStorageUnavailable {
		return nil, err
	}

	// Initialize state if not exists
	if state == nil {
		state = &State{
			Timestamps: []time.Time{},
		}
	}

	// Remove timestamps outside the window
	validTimestamps := make([]time.Time, 0, len(state.Timestamps))
	for _, ts := range state.Timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	currentCount := len(validTimestamps)

	// Check if check-only operation (n=0)
	if n == 0 {
		remaining := cfg.Burst - currentCount
		if remaining < 0 {
			remaining = 0
		}
		return &Result{
			Allowed:   remaining > 0,
			Limit:     cfg.Burst,
			Remaining: remaining,
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Check if within limit
	if currentCount+n <= cfg.Burst {
		// Add new timestamps
		for i := 0; i < n; i++ {
			validTimestamps = append(validTimestamps, now)
		}

		state.Timestamps = validTimestamps

		// Save state
		if err := storage.Set(ctx, key, state, cfg.TTL); err != nil {
			return nil, err
		}

		remaining := cfg.Burst - len(validTimestamps)
		return &Result{
			Allowed:   true,
			Limit:     cfg.Burst,
			Remaining: remaining,
			ResetAt:   now.Add(cfg.Interval),
		}, nil
	}

	// Limit exceeded - calculate retry after based on oldest timestamp
	var retryAfter time.Duration
	if len(validTimestamps) > 0 {
		oldestTimestamp := validTimestamps[0]
		retryAfter = oldestTimestamp.Add(cfg.Interval).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
	} else {
		retryAfter = cfg.Interval
	}

	remaining := cfg.Burst - currentCount
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:    false,
		Limit:      cfg.Burst,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    now.Add(retryAfter),
	}, nil
}

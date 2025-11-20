package rate

import (
	"context"
	"time"
)

// FixedWindowExecutor implements the fixed window counter algorithm
type FixedWindowExecutor struct {
	logger  Logger
	metrics MetricsCollector
}

// NewFixedWindowExecutor creates a new fixed window executor
func NewFixedWindowExecutor(logger Logger, metrics MetricsCollector) *FixedWindowExecutor {
	if logger == nil {
		logger = &NoOpLogger{}
	}
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}
	return &FixedWindowExecutor{
		logger:  logger,
		metrics: metrics,
	}
}

// Execute implements the fixed window counter algorithm
// Divides time into fixed windows and counts requests per window
func (e *FixedWindowExecutor) Execute(ctx context.Context, key string, n int, cfg *Config, storage Storage) (*Result, error) {
	now := time.Now()

	// Calculate current window start
	windowStart := now.Truncate(cfg.Interval)

	// Get current state
	state, err := storage.Get(ctx, key)
	if err != nil && err != ErrStorageUnavailable {
		return nil, err
	}

	// Initialize or reset if new window
	if state == nil || state.WindowStart.Before(windowStart) {
		state = &State{
			Counter:     0,
			WindowStart: windowStart,
		}
	}

	// Check if check-only operation (n=0)
	if n == 0 {
		remaining := cfg.Burst - int(state.Counter)
		if remaining < 0 {
			remaining = 0
		}
		return &Result{
			Allowed:   remaining > 0,
			Limit:     cfg.Burst,
			Remaining: remaining,
			ResetAt:   windowStart.Add(cfg.Interval),
		}, nil
	}

	// Check if within limit
	if state.Counter+int64(n) <= int64(cfg.Burst) {
		// Increment counter
		state.Counter += int64(n)

		// Save state
		if err := storage.Set(ctx, key, state, cfg.TTL); err != nil {
			return nil, err
		}

		remaining := cfg.Burst - int(state.Counter)
		return &Result{
			Allowed:   true,
			Limit:     cfg.Burst,
			Remaining: remaining,
			ResetAt:   windowStart.Add(cfg.Interval),
		}, nil
	}

	// Limit exceeded - retry in next window
	nextWindow := windowStart.Add(cfg.Interval)
	retryAfter := nextWindow.Sub(now)

	remaining := cfg.Burst - int(state.Counter)
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:    false,
		Limit:      cfg.Burst,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAt:    nextWindow,
	}, nil
}

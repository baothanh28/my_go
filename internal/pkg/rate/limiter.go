package rate

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrRateLimitExceeded indicates the rate limit has been exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("invalid rate limiter configuration")
	// ErrStorageUnavailable indicates the storage backend is unavailable
	ErrStorageUnavailable = errors.New("storage backend unavailable")
)

// Limiter is the main interface for rate limiting
type Limiter interface {
	// Allow checks if a request is allowed and consumes a token if it is
	Allow(ctx context.Context, key string) (bool, error)

	// AllowN checks if N requests are allowed and consumes N tokens if they are
	AllowN(ctx context.Context, key string, n int) (bool, error)

	// Check checks if a request would be allowed without consuming a token
	Check(ctx context.Context, key string) (bool, error)

	// Reserve reserves a token and returns a Reservation
	Reserve(ctx context.Context, key string) (*Reservation, error)

	// ReserveN reserves N tokens and returns a Reservation
	ReserveN(ctx context.Context, key string, n int) (*Reservation, error)

	// Reset resets the rate limit for a specific key
	Reset(ctx context.Context, key string) error

	// Close closes the limiter and releases resources
	Close() error
}

// Reservation represents a reserved rate limit token
type Reservation struct {
	// OK indicates if the reservation was successful
	OK bool

	// Delay is the time to wait before the reservation becomes valid
	Delay time.Duration

	// Tokens is the number of tokens reserved
	Tokens int

	// Limit is the rate limit configuration
	Limit *Config

	// cancel is called to cancel the reservation
	cancel func()
}

// Cancel cancels the reservation and returns the tokens
func (r *Reservation) Cancel() {
	if r.cancel != nil {
		r.cancel()
	}
}

// Wait waits until the reservation becomes valid or context is cancelled
func (r *Reservation) Wait(ctx context.Context) error {
	if r.Delay <= 0 {
		return nil
	}

	timer := time.NewTimer(r.Delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		r.Cancel()
		return ctx.Err()
	}
}

// Config holds rate limiter configuration
type Config struct {
	// Strategy is the rate limiting strategy to use
	Strategy Strategy

	// Rate is the number of tokens per interval
	Rate int

	// Burst is the maximum number of tokens that can be accumulated
	Burst int

	// Interval is the time window for rate limiting
	Interval time.Duration

	// TTL is the time-to-live for rate limit keys
	TTL time.Duration

	// FailOpen determines behavior when storage is unavailable
	// If true, allows requests when storage fails
	// If false, denies requests when storage fails
	FailOpen bool
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Rate <= 0 {
		return ErrInvalidConfig
	}
	if c.Burst < c.Rate {
		return ErrInvalidConfig
	}
	if c.Interval <= 0 {
		return ErrInvalidConfig
	}
	return nil
}

// Strategy represents the rate limiting strategy
type Strategy string

const (
	// StrategyTokenBucket uses the token bucket algorithm
	StrategyTokenBucket Strategy = "token_bucket"

	// StrategyLeakyBucket uses the leaky bucket algorithm
	StrategyLeakyBucket Strategy = "leaky_bucket"

	// StrategyFixedWindow uses the fixed window counter algorithm
	StrategyFixedWindow Strategy = "fixed_window"

	// StrategySlidingWindow uses the sliding window log algorithm
	StrategySlidingWindow Strategy = "sliding_window"
)

// Result contains the result of a rate limit check
type Result struct {
	// Allowed indicates if the request is allowed
	Allowed bool

	// Limit is the maximum number of requests allowed
	Limit int

	// Remaining is the number of requests remaining
	Remaining int

	// RetryAfter is the duration to wait before retrying
	RetryAfter time.Duration

	// ResetAt is when the rate limit resets
	ResetAt time.Time
}

// Executor executes rate limiting logic for a specific strategy
type Executor interface {
	// Execute executes the rate limiting logic
	Execute(ctx context.Context, key string, n int, cfg *Config, storage Storage) (*Result, error)
}

// Storage is the interface for rate limit storage backends
type Storage interface {
	// Get retrieves the current state for a key
	Get(ctx context.Context, key string) (*State, error)

	// Set updates the state for a key
	Set(ctx context.Context, key string, state *State, ttl time.Duration) error

	// Increment atomically increments the counter for a key
	Increment(ctx context.Context, key string, n int, ttl time.Duration) (int64, error)

	// Delete removes the state for a key
	Delete(ctx context.Context, key string) error

	// Close closes the storage backend
	Close() error

	// Ping checks if the storage backend is available
	Ping(ctx context.Context) error
}

// State represents the current state of a rate limiter for a key
type State struct {
	// Tokens is the current number of tokens available
	Tokens float64

	// LastUpdate is when the state was last updated
	LastUpdate time.Time

	// Counter is a simple counter (used by window strategies)
	Counter int64

	// WindowStart is the start of the current window
	WindowStart time.Time

	// Timestamps is a list of request timestamps (for sliding window)
	Timestamps []time.Time
}

// limiterImpl is the default implementation of Limiter
type limiterImpl struct {
	config   *Config
	storage  Storage
	executor Executor
	logger   Logger
	metrics  MetricsCollector
}

// New creates a new rate limiter
func New(config *Config, storage Storage, opts ...Option) (Limiter, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	l := &limiterImpl{
		config:  config,
		storage: storage,
		logger:  &NoOpLogger{},
		metrics: &NoOpMetrics{},
	}

	// Apply options
	for _, opt := range opts {
		opt(l)
	}

	// Create executor based on strategy
	switch config.Strategy {
	case StrategyTokenBucket:
		l.executor = NewTokenBucketExecutor(l.logger, l.metrics)
	case StrategyLeakyBucket:
		l.executor = NewLeakyBucketExecutor(l.logger, l.metrics)
	case StrategyFixedWindow:
		l.executor = NewFixedWindowExecutor(l.logger, l.metrics)
	case StrategySlidingWindow:
		l.executor = NewSlidingWindowExecutor(l.logger, l.metrics)
	default:
		return nil, ErrInvalidConfig
	}

	return l, nil
}

// Allow implements Limiter.Allow
func (l *limiterImpl) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

// AllowN implements Limiter.AllowN
func (l *limiterImpl) AllowN(ctx context.Context, key string, n int) (bool, error) {
	result, err := l.executor.Execute(ctx, key, n, l.config, l.storage)
	if err != nil {
		l.logger.Error("rate limit execution failed", "key", key, "error", err)
		l.metrics.RecordError(l.config.Strategy, err)

		// Handle fail-open/fail-close
		if errors.Is(err, ErrStorageUnavailable) && l.config.FailOpen {
			l.metrics.RecordFailOpen(l.config.Strategy)
			return true, nil
		}
		return false, err
	}

	l.metrics.RecordRequest(l.config.Strategy, result.Allowed)

	if result.Allowed {
		l.metrics.RecordAllowed(l.config.Strategy, key)
	} else {
		l.metrics.RecordDenied(l.config.Strategy, key, result.RetryAfter)
	}

	return result.Allowed, nil
}

// Check implements Limiter.Check
func (l *limiterImpl) Check(ctx context.Context, key string) (bool, error) {
	// For check, we execute with 0 tokens to avoid consuming
	result, err := l.executor.Execute(ctx, key, 0, l.config, l.storage)
	if err != nil {
		if errors.Is(err, ErrStorageUnavailable) && l.config.FailOpen {
			return true, nil
		}
		return false, err
	}
	return result.Remaining > 0, nil
}

// Reserve implements Limiter.Reserve
func (l *limiterImpl) Reserve(ctx context.Context, key string) (*Reservation, error) {
	return l.ReserveN(ctx, key, 1)
}

// ReserveN implements Limiter.ReserveN
func (l *limiterImpl) ReserveN(ctx context.Context, key string, n int) (*Reservation, error) {
	result, err := l.executor.Execute(ctx, key, n, l.config, l.storage)
	if err != nil {
		if errors.Is(err, ErrStorageUnavailable) && l.config.FailOpen {
			return &Reservation{OK: true, Tokens: n, Limit: l.config}, nil
		}
		return &Reservation{OK: false}, err
	}

	reservation := &Reservation{
		OK:     result.Allowed,
		Delay:  result.RetryAfter,
		Tokens: n,
		Limit:  l.config,
	}

	// Add cancel function if not immediately allowed
	if !result.Allowed && result.RetryAfter > 0 {
		reservation.cancel = func() {
			// Return tokens by incrementing the state
			// This is a best-effort operation
			_, _ = l.storage.Increment(context.Background(), key, n, l.config.TTL)
		}
	}

	return reservation, nil
}

// Reset implements Limiter.Reset
func (l *limiterImpl) Reset(ctx context.Context, key string) error {
	return l.storage.Delete(ctx, key)
}

// Close implements Limiter.Close
func (l *limiterImpl) Close() error {
	return l.storage.Close()
}

// Option is a functional option for configuring a Limiter
type Option func(*limiterImpl)

// WithLogger sets the logger for the limiter
func WithLogger(logger Logger) Option {
	return func(l *limiterImpl) {
		l.logger = logger
	}
}

// WithMetrics sets the metrics collector for the limiter
func WithMetrics(metrics MetricsCollector) Option {
	return func(l *limiterImpl) {
		l.metrics = metrics
	}
}

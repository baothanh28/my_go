package worker

import (
	"time"
)

// BackoffStrategy defines the backoff strategy for retries
type BackoffStrategy string

const (
	// BackoffLinear uses linear backoff (base * retry)
	BackoffLinear BackoffStrategy = "linear"

	// BackoffExponential uses exponential backoff (base * 2^retry)
	BackoffExponential BackoffStrategy = "exponential"

	// BackoffFixed uses fixed backoff (always base)
	BackoffFixed BackoffStrategy = "fixed"
)

// Config holds the configuration for the worker
type Config struct {
	// Concurrency is the number of worker goroutines
	Concurrency int

	// BackoffStrategy determines how to calculate retry delays
	BackoffStrategy BackoffStrategy

	// BaseBackoff is the base delay for retries
	BaseBackoff time.Duration

	// MaxBackoff is the maximum delay for retries
	MaxBackoff time.Duration

	// ShutdownTimeout is the timeout for graceful shutdown
	ShutdownTimeout time.Duration

	// PollInterval is the interval between polling for new tasks when queue is empty
	PollInterval time.Duration

	// ErrorBackoff is the delay after a fetch error
	ErrorBackoff time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Concurrency:     10,
		BackoffStrategy: BackoffExponential,
		BaseBackoff:     1 * time.Second,
		MaxBackoff:      5 * time.Minute,
		ShutdownTimeout: 30 * time.Second,
		PollInterval:    1 * time.Second,
		ErrorBackoff:    5 * time.Second,
	}
}

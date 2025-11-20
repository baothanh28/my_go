package pgnotify

import (
	"log/slog"
	"time"
)

// Config holds configuration for the Notifier.
type Config struct {
	// ReconnectInterval is the base delay between reconnection attempts
	ReconnectInterval time.Duration

	// MaxReconnectInterval is the maximum delay between reconnection attempts
	MaxReconnectInterval time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts.
	// Set to 0 for unlimited attempts.
	MaxReconnectAttempts int

	// ReconnectBackoffMultiplier is the multiplier for exponential backoff
	ReconnectBackoffMultiplier float64

	// MaxPayloadSize is the maximum allowed payload size in bytes
	// PostgreSQL has a hard limit of 8000 bytes
	MaxPayloadSize int

	// PingInterval is how often to check connection health
	PingInterval time.Duration

	// CallbackTimeout is the maximum time a callback can run before it's considered hung
	// Set to 0 to disable timeout
	CallbackTimeout time.Duration

	// BufferSize is the size of the internal notification buffer channel
	BufferSize int

	// Logger is the structured logger for the notifier
	Logger *slog.Logger

	// Hooks provides callbacks for monitoring and observability
	Hooks *Hooks

	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ReconnectInterval:          1 * time.Second,
		MaxReconnectInterval:       30 * time.Second,
		MaxReconnectAttempts:       0, // unlimited
		ReconnectBackoffMultiplier: 2.0,
		MaxPayloadSize:             7900, // Leave some margin below PostgreSQL's 8000 byte limit
		PingInterval:               30 * time.Second,
		CallbackTimeout:            30 * time.Second,
		BufferSize:                 100,
		Logger:                     slog.Default(),
		Hooks:                      &Hooks{},
		ShutdownTimeout:            10 * time.Second,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ReconnectInterval <= 0 {
		return ErrInvalidConfig("reconnect_interval must be positive")
	}

	if c.MaxReconnectInterval <= 0 {
		return ErrInvalidConfig("max_reconnect_interval must be positive")
	}

	if c.MaxReconnectInterval < c.ReconnectInterval {
		return ErrInvalidConfig("max_reconnect_interval must be >= reconnect_interval")
	}

	if c.ReconnectBackoffMultiplier < 1.0 {
		return ErrInvalidConfig("reconnect_backoff_multiplier must be >= 1.0")
	}

	if c.MaxPayloadSize <= 0 || c.MaxPayloadSize > 8000 {
		return ErrInvalidConfig("max_payload_size must be between 1 and 8000")
	}

	if c.PingInterval <= 0 {
		return ErrInvalidConfig("ping_interval must be positive")
	}

	if c.BufferSize <= 0 {
		return ErrInvalidConfig("buffer_size must be positive")
	}

	if c.Logger == nil {
		return ErrInvalidConfig("logger cannot be nil")
	}

	if c.Hooks == nil {
		c.Hooks = &Hooks{}
	}

	if c.ShutdownTimeout <= 0 {
		return ErrInvalidConfig("shutdown_timeout must be positive")
	}

	return nil
}

// Option is a functional option for configuring the Notifier.
type Option func(*Config)

// WithReconnectInterval sets the base reconnection interval.
func WithReconnectInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.ReconnectInterval = interval
	}
}

// WithMaxReconnectInterval sets the maximum reconnection interval.
func WithMaxReconnectInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.MaxReconnectInterval = interval
	}
}

// WithMaxReconnectAttempts sets the maximum number of reconnection attempts.
func WithMaxReconnectAttempts(attempts int) Option {
	return func(c *Config) {
		c.MaxReconnectAttempts = attempts
	}
}

// WithMaxPayloadSize sets the maximum payload size.
func WithMaxPayloadSize(size int) Option {
	return func(c *Config) {
		c.MaxPayloadSize = size
	}
}

// WithPingInterval sets the connection health check interval.
func WithPingInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.PingInterval = interval
	}
}

// WithCallbackTimeout sets the callback execution timeout.
func WithCallbackTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.CallbackTimeout = timeout
	}
}

// WithBufferSize sets the internal notification buffer size.
func WithBufferSize(size int) Option {
	return func(c *Config) {
		c.BufferSize = size
	}
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithHooks sets the observability hooks.
func WithHooks(hooks *Hooks) Option {
	return func(c *Config) {
		c.Hooks = hooks
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ShutdownTimeout = timeout
	}
}

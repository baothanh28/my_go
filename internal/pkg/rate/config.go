package rate

import (
	"fmt"
	"time"
)

// LimiterConfig holds configuration for rate limiters
type LimiterConfig struct {
	// Strategy is the rate limiting strategy to use
	Strategy string `json:"strategy" yaml:"strategy" mapstructure:"strategy"`

	// Rate is the number of requests allowed per interval
	Rate int `json:"rate" yaml:"rate" mapstructure:"rate"`

	// Burst is the maximum burst size (token capacity)
	Burst int `json:"burst" yaml:"burst" mapstructure:"burst"`

	// Interval is the time window for rate limiting
	Interval time.Duration `json:"interval" yaml:"interval" mapstructure:"interval"`

	// TTL is the time-to-live for rate limit keys
	TTL time.Duration `json:"ttl" yaml:"ttl" mapstructure:"ttl"`

	// FailOpen determines behavior when storage is unavailable
	FailOpen bool `json:"fail_open" yaml:"fail_open" mapstructure:"fail_open"`

	// Storage configuration
	Storage StorageConfig `json:"storage" yaml:"storage" mapstructure:"storage"`
}

// StorageConfig holds storage backend configuration
type StorageConfig struct {
	// Type is the storage backend type: "memory" or "redis"
	Type string `json:"type" yaml:"type" mapstructure:"type"`

	// Redis configuration (used when Type is "redis")
	Redis RedisConfig `json:"redis" yaml:"redis" mapstructure:"redis"`

	// KeyPrefix is the prefix for storage keys
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix" mapstructure:"key_prefix"`
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	// Addr is the Redis server address
	Addr string `json:"addr" yaml:"addr" mapstructure:"addr"`

	// Password is the Redis password
	Password string `json:"password" yaml:"password" mapstructure:"password"`

	// DB is the Redis database number
	DB int `json:"db" yaml:"db" mapstructure:"db"`

	// PoolSize is the maximum number of socket connections
	PoolSize int `json:"pool_size" yaml:"pool_size" mapstructure:"pool_size"`
}

// Validate validates the limiter configuration
func (c *LimiterConfig) Validate() error {
	if c.Rate <= 0 {
		return fmt.Errorf("rate must be positive")
	}

	if c.Burst < c.Rate {
		c.Burst = c.Rate // Auto-correct burst to at least rate
	}

	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	if c.TTL <= 0 {
		c.TTL = c.Interval * 2 // Default TTL to 2x interval
	}

	strategy := Strategy(c.Strategy)
	switch strategy {
	case StrategyTokenBucket, StrategyLeakyBucket, StrategyFixedWindow, StrategySlidingWindow:
		// Valid strategy
	default:
		return fmt.Errorf("invalid strategy: %s", c.Strategy)
	}

	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("invalid storage config: %w", err)
	}

	return nil
}

// ToConfig converts LimiterConfig to Config
func (c *LimiterConfig) ToConfig() *Config {
	return &Config{
		Strategy: Strategy(c.Strategy),
		Rate:     c.Rate,
		Burst:    c.Burst,
		Interval: c.Interval,
		TTL:      c.TTL,
		FailOpen: c.FailOpen,
	}
}

// Validate validates the storage configuration
func (c *StorageConfig) Validate() error {
	switch c.Type {
	case "memory", "redis":
		// Valid types
	case "":
		c.Type = "memory" // Default to memory
	default:
		return fmt.Errorf("invalid storage type: %s", c.Type)
	}

	if c.Type == "redis" {
		if c.Redis.Addr == "" {
			return fmt.Errorf("redis address is required")
		}
	}

	if c.KeyPrefix == "" {
		c.KeyPrefix = "ratelimit" // Default prefix
	}

	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *LimiterConfig {
	return &LimiterConfig{
		Strategy: string(StrategyTokenBucket),
		Rate:     100,
		Burst:    200,
		Interval: 1 * time.Minute,
		TTL:      2 * time.Minute,
		FailOpen: false,
		Storage: StorageConfig{
			Type:      "memory",
			KeyPrefix: "ratelimit",
		},
	}
}

// Presets for common rate limiting scenarios
var (
	// ConfigStrict: 10 requests per second, no burst
	ConfigStrict = &LimiterConfig{
		Strategy: string(StrategyTokenBucket),
		Rate:     10,
		Burst:    10,
		Interval: 1 * time.Second,
		TTL:      2 * time.Second,
		FailOpen: false,
	}

	// ConfigModerate: 100 requests per minute with 2x burst
	ConfigModerate = &LimiterConfig{
		Strategy: string(StrategyTokenBucket),
		Rate:     100,
		Burst:    200,
		Interval: 1 * time.Minute,
		TTL:      2 * time.Minute,
		FailOpen: false,
	}

	// ConfigLenient: 1000 requests per hour with 3x burst
	ConfigLenient = &LimiterConfig{
		Strategy: string(StrategyTokenBucket),
		Rate:     1000,
		Burst:    3000,
		Interval: 1 * time.Hour,
		TTL:      2 * time.Hour,
		FailOpen: true,
	}
)

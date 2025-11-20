package scheduler

import "time"

// SchedulerConfig holds all configuration for the scheduler.
type SchedulerConfig struct {
	// Scheduler settings
	TickInterval  time.Duration `json:"tick_interval" yaml:"tick_interval"`
	MaxConcurrent int           `json:"max_concurrent" yaml:"max_concurrent"`

	// Lock settings
	LockTTL             time.Duration `json:"lock_ttl" yaml:"lock_ttl"`
	LockRefreshInterval time.Duration `json:"lock_refresh_interval" yaml:"lock_refresh_interval"`

	// Backend settings
	BackendType string `json:"backend_type" yaml:"backend_type"` // "redis" or "memory"

	// Redis settings (if backend_type is "redis")
	RedisAddr     string `json:"redis_addr" yaml:"redis_addr"`
	RedisPassword string `json:"redis_password" yaml:"redis_password"`
	RedisDB       int    `json:"redis_db" yaml:"redis_db"`
}

// DefaultSchedulerConfig returns the default scheduler configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		TickInterval:        5 * time.Second,
		MaxConcurrent:       10,
		LockTTL:             30 * time.Second,
		LockRefreshInterval: 10 * time.Second,
		BackendType:         "memory",
		RedisAddr:           "localhost:6379",
		RedisPassword:       "",
		RedisDB:             0,
	}
}

// Validate validates the scheduler configuration.
func (c *SchedulerConfig) Validate() error {
	if c.TickInterval <= 0 {
		return ErrInvalidTimeout
	}

	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 10
	}

	if c.LockTTL <= 0 {
		c.LockTTL = 30 * time.Second
	}

	if c.LockRefreshInterval <= 0 {
		c.LockRefreshInterval = 10 * time.Second
	}

	if c.LockRefreshInterval >= c.LockTTL {
		c.LockRefreshInterval = c.LockTTL / 3
	}

	if c.BackendType != "redis" && c.BackendType != "memory" {
		c.BackendType = "memory"
	}

	return nil
}

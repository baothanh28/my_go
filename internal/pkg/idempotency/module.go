package idempotency

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Config holds idempotency service configuration
type Config struct {
	// Prefix for Redis keys
	RedisPrefix string `yaml:"redis_prefix" default:"idempotency"`

	// UseMemoryStorage forces in-memory storage (for testing only)
	UseMemoryStorage bool `yaml:"use_memory_storage" default:"false"`
}

// Module provides idempotency service with dependencies
var Module = fx.Module("idempotency",
	fx.Provide(
		NewJSONSerializer,
		NewUUIDKeyGenerator,
		provideStorage,
		NewService,
	),
)

// provideStorage creates the appropriate storage based on configuration
func provideStorage(config *Config, client *redis.Client) Storage {
	if config != nil && config.UseMemoryStorage {
		return NewMemoryStorage()
	}

	prefix := "idempotency"
	if config != nil && config.RedisPrefix != "" {
		prefix = config.RedisPrefix
	}

	return NewRedisStorage(client, prefix)
}

// MemoryModule provides idempotency service with memory storage (for testing)
var MemoryModule = fx.Module("idempotency-memory",
	fx.Provide(
		NewJSONSerializer,
		NewUUIDKeyGenerator,
		NewMemoryStorage,
		NewService,
	),
)

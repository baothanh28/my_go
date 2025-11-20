package idempotency

import (
	"context"
	"time"

	"myapp/internal/pkg/redis/keys"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// RedisStore implements idempotency.Store interface using Redis
type RedisStore struct {
	rdb *redisv9.Client
}

// NewStore creates a new Redis-based idempotency store
func NewStore(rdb *redisv9.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

// Module wires idempotency store
var Module = fx.Module("redis-idempotency",
	fx.Provide(NewStore),
)

// MarkIfFirst implements idempotency.Store interface
// Sets an id key if not exists with TTL, returns true if set
func (s *RedisStore) MarkIfFirst(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, "1", ttl).Result()
}

// MarkDeliveredIfFirst implements idempotency.Store interface
// A convenience API using the standard delivered:<id> key
func (s *RedisStore) MarkDeliveredIfFirst(ctx context.Context, notificationID string, ttl time.Duration) (bool, error) {
	return s.MarkIfFirst(ctx, keys.DeliveredKey(notificationID), ttl)
}

package redis

import (
	"context"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

type Locker struct {
	rdb *redisv9.Client
}

func NewLocker(rdb *redisv9.Client) *Locker {
	return &Locker{rdb: rdb}
}

// TryLock attempts to acquire a lock key with TTL
func (l *Locker) TryLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return l.rdb.SetNX(ctx, key, value, ttl).Result()
}

func (l *Locker) Unlock(ctx context.Context, key string) error {
	// Simple delete; for stronger guarantees use value match with Lua script
	return l.rdb.Del(ctx, key).Err()
}

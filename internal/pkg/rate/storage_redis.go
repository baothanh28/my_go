package rate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements Storage interface using Redis
type RedisStorage struct {
	client *redis.Client
	prefix string
}

// NewRedisStorage creates a new Redis storage
func NewRedisStorage(client *redis.Client, prefix string) *RedisStorage {
	if prefix == "" {
		prefix = "ratelimit"
	}
	return &RedisStorage{
		client: client,
		prefix: prefix,
	}
}

// Get retrieves the current state for a key
func (s *RedisStorage) Get(ctx context.Context, key string) (*State, error) {
	fullKey := s.makeKey(key)

	data, err := s.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// Set updates the state for a key
func (s *RedisStorage) Set(ctx context.Context, key string, state *State, ttl time.Duration) error {
	fullKey := s.makeKey(key)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if ttl <= 0 {
		ttl = 24 * time.Hour // Default TTL
	}

	if err := s.client.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}

	return nil
}

// Increment atomically increments the counter for a key using Lua script
func (s *RedisStorage) Increment(ctx context.Context, key string, n int, ttl time.Duration) (int64, error) {
	fullKey := s.makeKey(key)

	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	// Lua script for atomic increment with TTL
	script := redis.NewScript(`
		local current = redis.call('GET', KEYS[1])
		if current == false then
			redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
			return tonumber(ARGV[1])
		else
			local new_val = redis.call('INCRBY', KEYS[1], ARGV[1])
			redis.call('EXPIRE', KEYS[1], ARGV[2])
			return new_val
		end
	`)

	result, err := script.Run(ctx, s.client, []string{fullKey}, n, int(ttl.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}

	return result, nil
}

// Delete removes the state for a key
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	fullKey := s.makeKey(key)

	if err := s.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}

	return nil
}

// Close closes the storage backend
func (s *RedisStorage) Close() error {
	// Don't close the client as it might be shared
	return nil
}

// Ping checks if the storage backend is available
func (s *RedisStorage) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}
	return nil
}

// makeKey creates the full Redis key with prefix
func (s *RedisStorage) makeKey(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}

// TokenBucketRedisStorage implements token bucket using Redis Lua script for atomic operations
type TokenBucketRedisStorage struct {
	client *redis.Client
	prefix string
}

// NewTokenBucketRedisStorage creates a new token bucket Redis storage with optimized Lua scripts
func NewTokenBucketRedisStorage(client *redis.Client, prefix string) *TokenBucketRedisStorage {
	if prefix == "" {
		prefix = "ratelimit:tb"
	}
	return &TokenBucketRedisStorage{
		client: client,
		prefix: prefix,
	}
}

// AllowN checks and consumes tokens atomically using Lua script
func (s *TokenBucketRedisStorage) AllowN(ctx context.Context, key string, n int, rate int, burst int, interval time.Duration, ttl time.Duration) (bool, int, error) {
	fullKey := fmt.Sprintf("%s:%s", s.prefix, key)

	// Lua script for atomic token bucket check and consume
	script := redis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local n = tonumber(ARGV[2])
		local rate = tonumber(ARGV[3])
		local burst = tonumber(ARGV[4])
		local interval = tonumber(ARGV[5])
		local ttl = tonumber(ARGV[6])
		
		local data = redis.call('HMGET', key, 'tokens', 'last_update')
		local tokens = tonumber(data[1])
		local last_update = tonumber(data[2])
		
		-- Initialize if not exists
		if tokens == nil then
			tokens = burst
			last_update = now
		end
		
		-- Calculate tokens to add
		local elapsed = now - last_update
		local tokens_to_add = (elapsed * rate) / interval
		tokens = math.min(tokens + tokens_to_add, burst)
		
		-- Check if enough tokens
		if tokens >= n then
			tokens = tokens - n
			redis.call('HMSET', key, 'tokens', tokens, 'last_update', now)
			redis.call('EXPIRE', key, ttl)
			return {1, math.floor(tokens)}
		else
			return {0, math.floor(tokens)}
		end
	`)

	now := time.Now().Unix()
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	result, err := script.Run(ctx, s.client, []string{fullKey}, now, n, rate, burst, int(interval.Seconds()), int(ttl.Seconds())).Int64Slice()
	if err != nil {
		return false, 0, fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}

	allowed := result[0] == 1
	remaining := int(result[1])

	return allowed, remaining, nil
}

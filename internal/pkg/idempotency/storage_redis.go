package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStorage implements Storage using Redis
type redisStorage struct {
	client *redis.Client
	prefix string
}

// NewRedisStorage creates a new Redis-based storage
func NewRedisStorage(client *redis.Client, prefix string) Storage {
	if prefix == "" {
		prefix = "idempotency"
	}
	return &redisStorage{
		client: client,
		prefix: prefix,
	}
}

// makeKey creates a prefixed key
func (s *redisStorage) makeKey(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}

// Load retrieves a record by key
func (s *redisStorage) Load(ctx context.Context, key string) (*Record, error) {
	redisKey := s.makeKey(key)
	data, err := s.client.Get(ctx, redisKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Key doesn't exist
		}
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &record, nil
}

// TryMarkProcessing atomically marks a key as processing
func (s *redisStorage) TryMarkProcessing(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	redisKey := s.makeKey(key)

	record := Record{
		Key:       key,
		Status:    StatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("failed to marshal record: %w", err)
	}

	// Use SETNX to atomically set only if key doesn't exist
	success, err := s.client.SetNX(ctx, redisKey, data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx failed: %w", err)
	}

	return success, nil
}

// SaveResult saves the successful result
func (s *redisStorage) SaveResult(ctx context.Context, key string, result []byte, ttl time.Duration) error {
	redisKey := s.makeKey(key)

	record := Record{
		Key:       key,
		Status:    StatusCompleted,
		Result:    result,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Use SET to overwrite the processing state
	if err := s.client.Set(ctx, redisKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

// SaveError saves the error state
func (s *redisStorage) SaveError(ctx context.Context, key string, errMsg string, ttl time.Duration) error {
	redisKey := s.makeKey(key)

	record := Record{
		Key:       key,
		Status:    StatusFailed,
		ErrorMsg:  errMsg,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Use SET to overwrite the processing state
	if err := s.client.Set(ctx, redisKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

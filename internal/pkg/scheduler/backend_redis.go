package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisJobPrefix  = "scheduler:job:"
	redisLockPrefix = "scheduler:lock:"
	redisJobsSet    = "scheduler:jobs"
)

// RedisBackend implements BackendProvider using Redis.
type RedisBackend struct {
	client *redis.Client
}

// NewRedisBackend creates a new Redis backend.
func NewRedisBackend(client *redis.Client) *RedisBackend {
	return &RedisBackend{
		client: client,
	}
}

func (r *RedisBackend) SaveJob(ctx context.Context, job *Job) error {
	jobKey := redisJobPrefix + job.Name

	// Update timestamps
	job.Metadata.UpdatedAt = time.Now()

	// Check if job exists to set created timestamp
	exists, err := r.client.Exists(ctx, jobKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check job existence: %w", err)
	}
	if exists == 0 {
		job.Metadata.CreatedAt = time.Now()
	}

	// Serialize job metadata (handler cannot be serialized)
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Save to Redis
	pipe := r.client.Pipeline()
	pipe.Set(ctx, jobKey, data, 0)
	pipe.SAdd(ctx, redisJobsSet, job.Name)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}

	return nil
}

func (r *RedisBackend) LoadJobs(ctx context.Context) ([]*Job, error) {
	jobNames, err := r.client.SMembers(ctx, redisJobsSet).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to load job names: %w", err)
	}

	jobs := make([]*Job, 0, len(jobNames))

	for _, name := range jobNames {
		job, err := r.LoadJob(ctx, name)
		if err != nil {
			// Skip jobs that couldn't be loaded
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (r *RedisBackend) LoadJob(ctx context.Context, jobName string) (*Job, error) {
	jobKey := redisJobPrefix + jobName

	data, err := r.client.Get(ctx, jobKey).Result()
	if err == redis.Nil {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load job: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

func (r *RedisBackend) UpdateMetadata(ctx context.Context, jobName string, metadata *JobMetadata) error {
	job, err := r.LoadJob(ctx, jobName)
	if err != nil {
		return err
	}

	metadata.UpdatedAt = time.Now()
	job.Metadata = *metadata

	return r.SaveJob(ctx, job)
}

func (r *RedisBackend) DeleteJob(ctx context.Context, jobName string) error {
	jobKey := redisJobPrefix + jobName

	pipe := r.client.Pipeline()
	pipe.Del(ctx, jobKey)
	pipe.SRem(ctx, redisJobsSet, jobName)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}

func (r *RedisBackend) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) (bool, error) {
	fullKey := redisLockPrefix + lockKey

	// Use SET NX (set if not exists) with expiration
	success, err := r.client.SetNX(ctx, fullKey, owner, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return success, nil
}

func (r *RedisBackend) ReleaseLock(ctx context.Context, lockKey string, owner string) error {
	fullKey := redisLockPrefix + lockKey

	// Use Lua script to ensure atomic check-and-delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, r.client, []string{fullKey}, owner).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if result.(int64) == 0 {
		return ErrLockNotHeld
	}

	return nil
}

func (r *RedisBackend) RefreshLock(ctx context.Context, lockKey string, ttl time.Duration, owner string) error {
	fullKey := redisLockPrefix + lockKey

	// Use Lua script to ensure atomic check-and-expire
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, r.client, []string{fullKey}, owner, int(ttl.Seconds())).Result()
	if err != nil {
		return fmt.Errorf("failed to refresh lock: %w", err)
	}

	if result.(int64) == 0 {
		return ErrLockNotHeld
	}

	return nil
}

func (r *RedisBackend) GetJobsDueForExecution(ctx context.Context, now time.Time) ([]*Job, error) {
	jobs, err := r.LoadJobs(ctx)
	if err != nil {
		return nil, err
	}

	var dueJobs []*Job

	for _, job := range jobs {
		if job.Metadata.Status == JobStatusPaused ||
			job.Metadata.Status == JobStatusCancelled {
			continue
		}

		if !job.Metadata.NextRunAt.IsZero() &&
			(job.Metadata.NextRunAt.Before(now) || job.Metadata.NextRunAt.Equal(now)) {
			dueJobs = append(dueJobs, job)
		}
	}

	return dueJobs, nil
}

func (r *RedisBackend) Close() error {
	return r.client.Close()
}

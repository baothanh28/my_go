package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"myapp/internal/pkg/logger"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisProviderConfig holds configuration for the Redis provider
type RedisProviderConfig struct {
	// Stream is the Redis stream name
	Stream string

	// Group is the consumer group name
	Group string

	// Consumer is the consumer name (typically instance ID)
	Consumer string

	// Count is the number of messages to fetch per batch
	Count int64

	// Block is the duration to block waiting for new messages
	Block time.Duration

	// ClaimMinIdle is the minimum idle time before claiming messages from other consumers
	ClaimMinIdle time.Duration

	// ClaimCount is the number of pending messages to claim per batch
	ClaimCount int64

	// EnableAutoClaim enables automatic claiming of stale messages
	EnableAutoClaim bool

	// DLQStream is the dead letter queue stream name
	DLQStream string

	// MaxLen is the maximum length of the stream (0 for unlimited)
	MaxLen int64
}

// DefaultRedisProviderConfig returns a config with sensible defaults
func DefaultRedisProviderConfig(stream, group, consumer string) RedisProviderConfig {
	return RedisProviderConfig{
		Stream:          stream,
		Group:           group,
		Consumer:        consumer,
		Count:           1,
		Block:           1 * time.Second,
		ClaimMinIdle:    5 * time.Minute,
		ClaimCount:      10,
		EnableAutoClaim: true,
		DLQStream:       stream + ":dlq",
		MaxLen:          10000,
	}
}

// RedisProvider implements the Provider interface using Redis Streams
type RedisProvider struct {
	client *redisv9.Client
	config RedisProviderConfig
	logger *logger.Logger
}

// NewRedisProvider creates a new Redis provider
func NewRedisProvider(client *redisv9.Client, config RedisProviderConfig, log *logger.Logger) (*RedisProvider, error) {
	provider := &RedisProvider{
		client: client,
		config: config,
		logger: log,
	}

	// Ensure consumer group exists
	if err := provider.ensureGroup(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer group: %w", err)
	}

	// Ensure DLQ stream exists
	if err := provider.ensureDLQGroup(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure DLQ: %w", err)
	}

	log.Info("Redis provider initialized",
		zap.String("stream", config.Stream),
		zap.String("group", config.Group),
		zap.String("consumer", config.Consumer),
	)

	return provider, nil
}

// ensureGroup ensures the consumer group exists
func (p *RedisProvider) ensureGroup(ctx context.Context) error {
	err := p.client.XGroupCreateMkStream(ctx, p.config.Stream, p.config.Group, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// ensureDLQGroup ensures the DLQ consumer group exists
func (p *RedisProvider) ensureDLQGroup(ctx context.Context) error {
	err := p.client.XGroupCreateMkStream(ctx, p.config.DLQStream, p.config.Group, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// Fetch retrieves the next task from the Redis stream
func (p *RedisProvider) Fetch(ctx context.Context) (*Task, error) {
	// Try to claim stale messages first (if enabled)
	if p.config.EnableAutoClaim {
		task, err := p.claimStaleMessage(ctx)
		if err != nil {
			p.logger.Warn("Failed to claim stale message", zap.Error(err))
		}
		if task != nil {
			return task, nil
		}
	}

	// Read new messages
	streams, err := p.client.XReadGroup(ctx, &redisv9.XReadGroupArgs{
		Group:    p.config.Group,
		Consumer: p.config.Consumer,
		Streams:  []string{p.config.Stream, ">"},
		Count:    p.config.Count,
		Block:    p.config.Block,
	}).Result()

	if err != nil {
		if err == redisv9.Nil {
			// No messages available
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	// Convert first message to task
	msg := streams[0].Messages[0]
	return p.messageToTask(msg)
}

// claimStaleMessage attempts to claim a stale message from another consumer
func (p *RedisProvider) claimStaleMessage(ctx context.Context) (*Task, error) {
	msgs, _, err := p.client.XAutoClaim(ctx, &redisv9.XAutoClaimArgs{
		Stream:   p.config.Stream,
		Group:    p.config.Group,
		Consumer: p.config.Consumer,
		MinIdle:  p.config.ClaimMinIdle,
		Start:    "0",
		Count:    p.config.ClaimCount,
	}).Result()

	if err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		return nil, nil
	}

	// Return first claimed message
	return p.messageToTask(msgs[0])
}

// messageToTask converts a Redis stream message to a Task
func (p *RedisProvider) messageToTask(msg redisv9.XMessage) (*Task, error) {
	task := &Task{
		ID:       msg.ID,
		Metadata: make(map[string]string),
	}

	// Extract task fields from message values
	if payload, ok := msg.Values["payload"].(string); ok {
		task.Payload = []byte(payload)
	}

	if createdAt, ok := msg.Values["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			task.CreatedAt = t
		}
	}

	if retry, ok := msg.Values["retry"].(string); ok {
		if r, err := strconv.Atoi(retry); err == nil {
			task.Retry = r
		}
	}

	if maxRetry, ok := msg.Values["max_retry"].(string); ok {
		if mr, err := strconv.Atoi(maxRetry); err == nil {
			task.MaxRetry = mr
		}
	}

	if timeout, ok := msg.Values["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			task.Timeout = d
		}
	}

	// Extract metadata
	if metadataStr, ok := msg.Values["metadata"].(string); ok {
		if err := json.Unmarshal([]byte(metadataStr), &task.Metadata); err != nil {
			p.logger.Warn("Failed to unmarshal metadata", zap.Error(err))
		}
	}

	// Add individual metadata fields (for backward compatibility)
	for key, val := range msg.Values {
		if key != "payload" && key != "created_at" && key != "retry" && key != "max_retry" && key != "timeout" && key != "metadata" {
			if strVal, ok := val.(string); ok {
				task.Metadata[key] = strVal
			}
		}
	}

	return task, nil
}

// Ack acknowledges successful processing of a task
func (p *RedisProvider) Ack(ctx context.Context, task *Task) error {
	_, err := p.client.XAck(ctx, p.config.Stream, p.config.Group, task.ID).Result()
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}

	// Delete the message from the stream
	_, delErr := p.client.XDel(ctx, p.config.Stream, task.ID).Result()
	if delErr != nil {
		p.logger.Warn("Failed to delete acked message", zap.String("task_id", task.ID), zap.Error(delErr))
	}

	return nil
}

// Nack negatively acknowledges a task
func (p *RedisProvider) Nack(ctx context.Context, task *Task, requeue bool) error {
	// Always ack the original message first
	_, err := p.client.XAck(ctx, p.config.Stream, p.config.Group, task.ID).Result()
	if err != nil {
		p.logger.Warn("Failed to ack message before nack", zap.String("task_id", task.ID), zap.Error(err))
	}

	// Delete from original stream
	_, delErr := p.client.XDel(ctx, p.config.Stream, task.ID).Result()
	if delErr != nil {
		p.logger.Warn("Failed to delete nacked message", zap.String("task_id", task.ID), zap.Error(delErr))
	}

	if requeue {
		// Add back to the stream for retry
		return p.requeue(ctx, task)
	} else {
		// Send to DLQ
		return p.sendToDLQ(ctx, task)
	}
}

// requeue adds a task back to the stream for retry
func (p *RedisProvider) requeue(ctx context.Context, task *Task) error {
	values := p.taskToValues(task)

	// Calculate delay if scheduled
	if !task.ScheduledAt.IsZero() && task.ScheduledAt.After(time.Now()) {
		// For delayed tasks, we could use a separate delayed stream or store delay in metadata
		values["scheduled_at"] = task.ScheduledAt.Format(time.RFC3339)
	}

	_, err := p.client.XAdd(ctx, &redisv9.XAddArgs{
		Stream: p.config.Stream,
		MaxLen: p.config.MaxLen,
		Approx: true,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to requeue task: %w", err)
	}

	p.logger.Info("Task requeued", zap.String("task_id", task.ID), zap.Int("retry", task.Retry))
	return nil
}

// sendToDLQ sends a task to the dead letter queue
func (p *RedisProvider) sendToDLQ(ctx context.Context, task *Task) error {
	values := p.taskToValues(task)
	values["dlq_timestamp"] = time.Now().Format(time.RFC3339)

	_, err := p.client.XAdd(ctx, &redisv9.XAddArgs{
		Stream: p.config.DLQStream,
		MaxLen: p.config.MaxLen,
		Approx: true,
		Values: values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to send task to DLQ: %w", err)
	}

	p.logger.Warn("Task sent to DLQ", zap.String("task_id", task.ID), zap.Int("retry", task.Retry))
	return nil
}

// taskToValues converts a Task to Redis stream values
func (p *RedisProvider) taskToValues(task *Task) map[string]interface{} {
	values := make(map[string]interface{})

	values["payload"] = string(task.Payload)
	values["created_at"] = task.CreatedAt.Format(time.RFC3339)
	values["retry"] = strconv.Itoa(task.Retry)
	values["max_retry"] = strconv.Itoa(task.MaxRetry)

	if task.Timeout > 0 {
		values["timeout"] = task.Timeout.String()
	}

	// Serialize metadata as JSON
	if len(task.Metadata) > 0 {
		if metadataBytes, err := json.Marshal(task.Metadata); err == nil {
			values["metadata"] = string(metadataBytes)
		}

		// Also add individual fields for easy querying
		for key, val := range task.Metadata {
			values[key] = val
		}
	}

	return values
}

// Close cleans up the provider resources
func (p *RedisProvider) Close() error {
	// Redis client is shared, so we don't close it here
	p.logger.Info("Redis provider closed")
	return nil
}

// EnqueueTask is a helper method to enqueue a new task
func (p *RedisProvider) EnqueueTask(ctx context.Context, task *Task) (string, error) {
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	values := p.taskToValues(task)

	id, err := p.client.XAdd(ctx, &redisv9.XAddArgs{
		Stream: p.config.Stream,
		MaxLen: p.config.MaxLen,
		Approx: true,
		Values: values,
	}).Result()

	if err != nil {
		return "", fmt.Errorf("failed to enqueue task: %w", err)
	}

	p.logger.Info("Task enqueued", zap.String("task_id", id))
	return id, nil
}

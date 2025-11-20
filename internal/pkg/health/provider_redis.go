package health

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisProvider checks Redis health
type RedisProvider struct {
	name       string
	client     redis.UniversalClient
	degradedMS int64
}

// RedisProviderConfig configures the Redis health provider
type RedisProviderConfig struct {
	Name       string
	Client     redis.UniversalClient
	DegradedMS int64 // Latency threshold for degraded status (default: 100ms)
}

// NewRedisProvider creates a new Redis health provider
func NewRedisProvider(config RedisProviderConfig) *RedisProvider {
	if config.Name == "" {
		config.Name = "redis"
	}
	if config.DegradedMS == 0 {
		config.DegradedMS = 100
	}

	return &RedisProvider{
		name:       config.Name,
		client:     config.Client,
		degradedMS: config.DegradedMS,
	}
}

// Name returns the provider name
func (p *RedisProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *RedisProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Measure PING latency
	start := time.Now()
	pong, err := p.client.Ping(ctx).Result()
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()

	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to ping redis: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	result.Details["response"] = pong

	// Get pool stats if available
	if client, ok := p.client.(*redis.Client); ok {
		stats := client.PoolStats()
		result.Details["pool_hits"] = stats.Hits
		result.Details["pool_misses"] = stats.Misses
		result.Details["pool_timeouts"] = stats.Timeouts
		result.Details["total_conns"] = stats.TotalConns
		result.Details["idle_conns"] = stats.IdleConns
		result.Details["stale_conns"] = stats.StaleConns
	}

	// Check latency threshold
	if latency.Milliseconds() > p.degradedMS {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency detected"
		return result
	}

	// Try to get server info
	infoCmd := p.client.Info(ctx, "server")
	if infoCmd.Err() == nil {
		info := infoCmd.Val()
		result.Details["server_info_available"] = true
		// Parse basic info (optional, can be extended)
		if len(info) > 0 {
			result.Details["connected"] = true
		}
	}

	result.Status = StatusUp
	return result
}

// RedisClusterProvider checks Redis Cluster health
type RedisClusterProvider struct {
	name       string
	client     *redis.ClusterClient
	degradedMS int64
}

// RedisClusterProviderConfig configures the Redis Cluster health provider
type RedisClusterProviderConfig struct {
	Name       string
	Client     *redis.ClusterClient
	DegradedMS int64
}

// NewRedisClusterProvider creates a new Redis Cluster health provider
func NewRedisClusterProvider(config RedisClusterProviderConfig) *RedisClusterProvider {
	if config.Name == "" {
		config.Name = "redis-cluster"
	}
	if config.DegradedMS == 0 {
		config.DegradedMS = 100
	}

	return &RedisClusterProvider{
		name:       config.Name,
		client:     config.Client,
		degradedMS: config.DegradedMS,
	}
}

// Name returns the provider name
func (p *RedisClusterProvider) Name() string {
	return p.name
}

// Check performs the health check
func (p *RedisClusterProvider) Check(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{
		Name:      p.name,
		CheckedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Measure PING latency
	start := time.Now()
	err := p.client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	latency := time.Since(start)

	result.Details["latency_ms"] = latency.Milliseconds()

	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to ping redis cluster: %v", err)
		result.Details["error"] = err.Error()
		return result
	}

	// Get cluster info
	clusterInfo, err := p.client.ClusterInfo(ctx).Result()
	if err == nil {
		result.Details["cluster_info_available"] = true
	}

	// Get cluster nodes
	nodes, err := p.client.ClusterNodes(ctx).Result()
	if err == nil {
		result.Details["cluster_nodes_count"] = len(nodes)
	}

	// Get pool stats
	stats := p.client.PoolStats()
	result.Details["pool_hits"] = stats.Hits
	result.Details["pool_misses"] = stats.Misses
	result.Details["pool_timeouts"] = stats.Timeouts
	result.Details["total_conns"] = stats.TotalConns
	result.Details["idle_conns"] = stats.IdleConns

	// Check latency threshold
	if latency.Milliseconds() > p.degradedMS {
		result.Status = StatusDegraded
		result.Details["message"] = "high latency detected"
		return result
	}

	// Check if cluster info indicates issues
	if clusterInfo != "" && err == nil {
		// Basic check - can be enhanced with more sophisticated parsing
		result.Details["cluster_state"] = "available"
	}

	result.Status = StatusUp
	return result
}

package dlq

import (
	"context"

	"myapp/internal/pkg/redis"

	redisv9 "github.com/redis/go-redis/v9"

	"go.uber.org/fx"
)

type DLQ struct {
	stream *redis.StreamClient
}

func New(rdb *redisv9.Client) *DLQ {
	return &DLQ{stream: redis.NewStreamClient(rdb)}
}

var Module = fx.Module("redis-dlq",
	fx.Provide(New),
)

func (d *DLQ) Push(ctx context.Context, dlqStream string, maxLen int64, values map[string]interface{}) (string, error) {
	return d.stream.XAdd(ctx, redis.XAddArgs{
		Stream: dlqStream,
		MaxLen: maxLen,
		Values: values,
	})
}

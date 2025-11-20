package redis

import (
	"context"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

type StreamClient struct {
	rdb *redisv9.Client
}

func NewStreamClient(rdb *redisv9.Client) *StreamClient {
	return &StreamClient{rdb: rdb}
}

type XAddArgs struct {
	Stream string
	MaxLen int64 // approximate
	Values map[string]interface{}
}

func (s *StreamClient) XAdd(ctx context.Context, args XAddArgs) (string, error) {
	options := &redisv9.XAddArgs{
		Stream: args.Stream,
		MaxLen: args.MaxLen,
		Approx: true,
		Values: args.Values,
	}
	return s.rdb.XAdd(ctx, options).Result()
}

func (s *StreamClient) EnsureGroup(ctx context.Context, stream, group string) error {
	// Create stream and group if not exists
	err := s.rdb.XGroupCreateMkStream(ctx, stream, group, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

type ReadGroupArgs struct {
	Group    string
	Consumer string
	Streams  []string
	Count    int64
	Block    time.Duration
	NoAck    bool
}

func (s *StreamClient) XReadGroup(ctx context.Context, args ReadGroupArgs) ([]redisv9.XStream, error) {
	cmd := s.rdb.XReadGroup(ctx, &redisv9.XReadGroupArgs{
		Group:    args.Group,
		Consumer: args.Consumer,
		Streams:  args.Streams,
		Count:    args.Count,
		Block:    args.Block,
		NoAck:    args.NoAck,
	})
	return cmd.Result()
}

func (s *StreamClient) XAck(ctx context.Context, stream, group string, ids ...string) (int64, error) {
	return s.rdb.XAck(ctx, stream, group, ids...).Result()
}

func (s *StreamClient) XAutoClaim(ctx context.Context, stream, group, consumer string, minIdle time.Duration, count int64, start string) ([]redisv9.XMessage, string, error) {
	msgs, nextStart, err := s.rdb.XAutoClaim(ctx, &redisv9.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    start,
		Count:    count,
	}).Result()
	return msgs, nextStart, err
}

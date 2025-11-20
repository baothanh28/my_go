package redis

import (
	"context"
	"crypto/tls"
	"time"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/logger"

	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module exports the redis module for FX
var Module = fx.Module("redis",
	fx.Provide(NewRedisClient),
	fx.Invoke(registerHooks),
)

// NewRedisClient constructs a shared Redis client
func NewRedisClient(cfg *config.Config, log *logger.Logger) (*redisv9.Client, error) {
	opts := &redisv9.Options{
		Addr:         cfg.Redis.Addr,
		Username:     cfg.Redis.Username,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSec) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSec) * time.Second,
	}
	if cfg.Redis.TLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	client := redisv9.NewClient(opts)

	// Simple ping test
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	log.Info("Redis client initialized")
	return client, nil
}

func registerHooks(lc fx.Lifecycle, rdb *redisv9.Client, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Redis module started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Closing Redis client")
			return rdb.Close()
		},
	})
}

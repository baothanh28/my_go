package pgnotify

import (
	"context"
	"log/slog"
)

// ProvideNotifier provides a Notifier instance with default configuration.
func ProvideNotifier(provider ConnectionProvider, logger *slog.Logger) (Notifier, error) {
	return NewNotifier(
		provider,
		WithLogger(logger),
	)
}

// ProvidePgxProvider provides a PgxProvider instance.
func ProvidePgxProvider(ctx context.Context, dsn string) (*PgxProvider, error) {
	return NewPgxProvider(ctx, dsn)
}

// ProvideNotifierWithConfig provides a Notifier instance with custom configuration.
func ProvideNotifierWithConfig(provider ConnectionProvider, config *Config) (Notifier, error) {
	return NewNotifier(
		provider,
		WithLogger(config.Logger),
		WithReconnectInterval(config.ReconnectInterval),
		WithMaxReconnectInterval(config.MaxReconnectInterval),
		WithMaxReconnectAttempts(config.MaxReconnectAttempts),
		WithMaxPayloadSize(config.MaxPayloadSize),
		WithPingInterval(config.PingInterval),
		WithCallbackTimeout(config.CallbackTimeout),
		WithBufferSize(config.BufferSize),
		WithHooks(config.Hooks),
		WithShutdownTimeout(config.ShutdownTimeout),
	)
}

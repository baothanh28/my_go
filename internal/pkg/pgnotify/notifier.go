package pgnotify

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// notifier is the main implementation of the Notifier interface.
type notifier struct {
	config     *Config
	logger     *slog.Logger
	provider   ConnectionProvider
	subMgr     *subscriptionManager
	dispatcher *dispatcher
	metrics    *metricsCollector

	// State management
	mu        sync.RWMutex
	started   atomic.Bool
	stopped   atomic.Bool
	connected atomic.Bool

	// Context management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewNotifier creates a new Notifier with the given connection provider and options.
func NewNotifier(provider ConnectionProvider, opts ...Option) (Notifier, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	subMgr := newSubscriptionManager()
	metrics := newMetricsCollector()
	dispatcher := newDispatcher(config, subMgr, metrics)

	return &notifier{
		config:     config,
		logger:     config.Logger,
		provider:   provider,
		subMgr:     subMgr,
		dispatcher: dispatcher,
		metrics:    metrics,
	}, nil
}

// Publish sends a NOTIFY command to the specified channel with the given payload.
func (n *notifier) Publish(ctx context.Context, channel string, payload string) error {
	if channel == "" {
		return ErrChannelEmpty
	}

	if len(payload) > n.config.MaxPayloadSize {
		return ErrPayloadTooLarge
	}

	if !n.provider.IsConnected() {
		return ErrNotConnected
	}

	err := n.provider.Notify(ctx, channel, payload)
	if err != nil {
		n.logger.Error("failed to publish",
			slog.String("channel", channel),
			slog.String("error", err.Error()))
		return ErrPublish(channel, err)
	}

	n.logger.Debug("published notification",
		slog.String("channel", channel),
		slog.Int("payload_size", len(payload)))

	return nil
}

// Subscribe registers a callback for notifications on the specified channel.
func (n *notifier) Subscribe(ctx context.Context, channel string, callback CallbackFunc) (Subscription, error) {
	if channel == "" {
		return nil, ErrChannelEmpty
	}

	if callback == nil {
		return nil, ErrCallbackNil
	}

	// Check if we need to send LISTEN command (first subscription for this channel)
	needsListen := !n.subMgr.HasChannel(channel)

	// Add subscription to manager
	sub := n.subMgr.Add(channel, callback, n)

	// Send LISTEN if needed and if connected
	if needsListen && n.provider.IsConnected() {
		err := n.provider.Listen(ctx, channel)
		if err != nil {
			n.subMgr.Remove(channel, sub)
			n.logger.Error("failed to listen",
				slog.String("channel", channel),
				slog.String("error", err.Error()))
			return nil, ErrSubscribe(channel, err)
		}
	}

	n.logger.Info("subscribed to channel",
		slog.String("channel", channel))

	// Call hook if provided
	if n.config.Hooks.OnSubscribe != nil {
		n.dispatcher.safeCallHook(func() {
			n.config.Hooks.OnSubscribe(channel)
		})
	}

	return sub, nil
}

// unsubscribe removes a subscription for the given channel.
func (n *notifier) unsubscribe(channel string) error {
	// Check if this was the last subscription for this channel
	if !n.subMgr.HasChannel(channel) {
		// Send UNLISTEN if connected
		if n.provider.IsConnected() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := n.provider.Unlisten(ctx, channel)
			if err != nil {
				n.logger.Error("failed to unlisten",
					slog.String("channel", channel),
					slog.String("error", err.Error()))
				return ErrUnsubscribe(channel, err)
			}
		}

		n.logger.Info("unsubscribed from channel",
			slog.String("channel", channel))

		// Call hook if provided
		if n.config.Hooks.OnUnsubscribe != nil {
			n.dispatcher.safeCallHook(func() {
				n.config.Hooks.OnUnsubscribe(channel)
			})
		}
	}

	return nil
}

// Start begins listening for notifications. This is a blocking call.
func (n *notifier) Start(ctx context.Context) error {
	if n.started.Load() {
		return ErrAlreadyStarted
	}

	if n.stopped.Load() {
		return ErrAlreadyStopped
	}

	n.started.Store(true)
	n.ctx, n.cancel = context.WithCancel(ctx)

	n.logger.Info("starting notifier")

	// Start connection supervisor
	n.wg.Add(1)
	go n.connectionSupervisor()

	// Start main listener loop
	n.wg.Add(1)
	go n.listenerLoop()

	// Wait for context cancellation
	<-n.ctx.Done()

	// Wait for goroutines to finish
	n.wg.Wait()

	n.logger.Info("notifier stopped")
	return nil
}

// listenerLoop is the main loop that receives notifications from PostgreSQL.
func (n *notifier) listenerLoop() {
	defer n.wg.Done()

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		// Skip if not connected
		if !n.provider.IsConnected() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Wait for notification
		notification, err := n.provider.WaitForNotification(n.ctx)
		if err != nil {
			if n.ctx.Err() != nil {
				return // Context cancelled
			}

			n.logger.Debug("notification wait error",
				slog.String("error", err.Error()))
			continue
		}

		if notification != nil {
			n.metrics.IncrementNotifications()
			n.dispatcher.Dispatch(n.ctx, notification)
		}
	}
}

// connectionSupervisor monitors connection health and handles reconnection.
func (n *notifier) connectionSupervisor() {
	defer n.wg.Done()

	ticker := time.NewTicker(n.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.checkConnection()
		}
	}
}

// checkConnection checks if the connection is healthy and reconnects if needed.
func (n *notifier) checkConnection() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if connected
	if !n.provider.IsConnected() {
		n.handleDisconnection(nil)
		n.reconnect()
		return
	}

	// Ping connection
	err := n.provider.Ping(ctx)
	if err != nil {
		n.logger.Warn("connection ping failed",
			slog.String("error", err.Error()))
		n.handleDisconnection(err)
		n.reconnect()
	}
}

// handleDisconnection handles connection loss.
func (n *notifier) handleDisconnection(err error) {
	if n.connected.Swap(false) {
		n.metrics.SetConnected(false)

		n.logger.Warn("connection lost")

		// Call hook if provided
		if n.config.Hooks.OnDisconnect != nil {
			n.dispatcher.safeCallHook(func() {
				n.config.Hooks.OnDisconnect(err)
			})
		}
	}
}

// reconnect attempts to reconnect to PostgreSQL with exponential backoff.
func (n *notifier) reconnect() {
	attempt := 0
	backoff := n.config.ReconnectInterval

	for {
		// Check if context is cancelled
		if n.ctx.Err() != nil {
			return
		}

		// Check max attempts
		if n.config.MaxReconnectAttempts > 0 && attempt >= n.config.MaxReconnectAttempts {
			n.logger.Error("max reconnect attempts reached",
				slog.Int("attempts", attempt))

			// Call hook if provided
			if n.config.Hooks.OnReconnectFailed != nil {
				n.dispatcher.safeCallHook(func() {
					n.config.Hooks.OnReconnectFailed(attempt, ErrReconnectFailed)
				})
			}
			return
		}

		attempt++

		n.logger.Info("attempting to reconnect",
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff))

		// Call hook if provided
		if n.config.Hooks.OnReconnectAttempt != nil {
			n.dispatcher.safeCallHook(func() {
				n.config.Hooks.OnReconnectAttempt(attempt, backoff)
			})
		}

		// Wait before reconnecting
		select {
		case <-n.ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Attempt reconnection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := n.provider.Reconnect(ctx)
		cancel()

		if err != nil {
			n.metrics.IncrementReconnects()
			n.logger.Error("reconnect failed",
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()))

			// Increase backoff exponentially
			backoff = time.Duration(float64(backoff) * n.config.ReconnectBackoffMultiplier)
			if backoff > n.config.MaxReconnectInterval {
				backoff = n.config.MaxReconnectInterval
			}
			continue
		}

		// Reconnection successful
		n.connected.Store(true)
		n.metrics.SetConnected(true)

		n.logger.Info("reconnected successfully",
			slog.Int("attempt", attempt))

		// Call hook if provided
		if n.config.Hooks.OnConnect != nil {
			n.dispatcher.safeCallHook(func() {
				n.config.Hooks.OnConnect()
			})
		}

		if n.config.Hooks.OnReconnectSuccess != nil {
			n.dispatcher.safeCallHook(func() {
				n.config.Hooks.OnReconnectSuccess(attempt)
			})
		}

		// Re-register all LISTEN commands
		n.reregisterListeners()

		return
	}
}

// reregisterListeners re-registers all LISTEN commands after reconnection.
func (n *notifier) reregisterListeners() {
	channels := n.subMgr.Channels()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, channel := range channels {
		err := n.provider.Listen(ctx, channel)
		if err != nil {
			n.logger.Error("failed to re-register listener",
				slog.String("channel", channel),
				slog.String("error", err.Error()))
		} else {
			n.logger.Debug("re-registered listener",
				slog.String("channel", channel))
		}
	}
}

// Shutdown gracefully stops the notifier.
func (n *notifier) Shutdown(ctx context.Context) error {
	if n.stopped.Swap(true) {
		return ErrAlreadyStopped
	}

	n.logger.Info("shutting down notifier")

	// Cancel context to stop all goroutines
	if n.cancel != nil {
		n.cancel()
	}

	// Wait for graceful shutdown or timeout
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		n.dispatcher.Wait()
		close(done)
	}()

	select {
	case <-done:
		n.logger.Info("graceful shutdown completed")
	case <-ctx.Done():
		n.logger.Warn("shutdown timeout exceeded")
		return ErrShutdownTimeout
	}

	// Close connection
	if err := n.provider.Close(); err != nil {
		n.logger.Error("failed to close connection",
			slog.String("error", err.Error()))
		return err
	}

	return nil
}

// IsHealthy returns true if the notifier has an active PostgreSQL connection.
func (n *notifier) IsHealthy() bool {
	return n.provider.IsConnected()
}

// GetStatistics returns runtime statistics.
func (n *notifier) GetStatistics() Statistics {
	return n.metrics.GetStatistics(n.subMgr.Count())
}

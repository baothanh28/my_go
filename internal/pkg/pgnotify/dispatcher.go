package pgnotify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// dispatcher handles incoming notifications and dispatches them to registered callbacks.
type dispatcher struct {
	config  *Config
	logger  *slog.Logger
	subMgr  *subscriptionManager
	wg      sync.WaitGroup
	metrics *metricsCollector
}

// newDispatcher creates a new dispatcher.
func newDispatcher(config *Config, subMgr *subscriptionManager, metrics *metricsCollector) *dispatcher {
	return &dispatcher{
		config:  config,
		logger:  config.Logger,
		subMgr:  subMgr,
		metrics: metrics,
	}
}

// Dispatch dispatches a notification to all subscribed callbacks.
func (d *dispatcher) Dispatch(ctx context.Context, notification *Notification) {
	// Call hook if provided
	if d.config.Hooks.OnNotification != nil {
		d.safeCallHook(func() {
			d.config.Hooks.OnNotification(notification)
		})
	}

	// Get all subscriptions for this channel
	subs := d.subMgr.Get(notification.Channel)

	if len(subs) == 0 {
		d.logger.Debug("no subscriptions for channel",
			slog.String("channel", notification.Channel))
		return
	}

	d.logger.Debug("dispatching notification",
		slog.String("channel", notification.Channel),
		slog.Int("subscribers", len(subs)))

	// Dispatch to each subscription in a separate goroutine
	for _, sub := range subs {
		d.wg.Add(1)
		go d.dispatchToSubscription(ctx, sub, notification)
	}
}

// dispatchToSubscription dispatches a notification to a single subscription.
func (d *dispatcher) dispatchToSubscription(ctx context.Context, sub *subscription, notification *Notification) {
	defer d.wg.Done()
	defer d.recoverPanic(notification.Channel)

	// Create context with timeout if configured
	callbackCtx := ctx
	var cancel context.CancelFunc

	if d.config.CallbackTimeout > 0 {
		callbackCtx, cancel = context.WithTimeout(ctx, d.config.CallbackTimeout)
		defer cancel()
	}

	// Execute callback
	start := time.Now()
	err := sub.Invoke(callbackCtx, notification)
	duration := time.Since(start)

	if err != nil {
		d.metrics.IncrementErrors()
		d.logger.Error("callback error",
			slog.String("channel", notification.Channel),
			slog.String("error", err.Error()),
			slog.Duration("duration", duration))

		// Call error hook if provided
		if d.config.Hooks.OnError != nil {
			d.safeCallHook(func() {
				d.config.Hooks.OnError(ErrCallback(notification.Channel, err), notification.Channel)
			})
		}
	} else {
		d.logger.Debug("callback completed",
			slog.String("channel", notification.Channel),
			slog.Duration("duration", duration))
	}
}

// recoverPanic recovers from panics in callback execution.
func (d *dispatcher) recoverPanic(channel string) {
	if r := recover(); r != nil {
		d.metrics.IncrementErrors()

		err := fmt.Errorf("panic in callback: %v", r)
		d.logger.Error("callback panic",
			slog.String("channel", channel),
			slog.String("panic", fmt.Sprintf("%v", r)))

		// Call error hook if provided
		if d.config.Hooks.OnError != nil {
			d.safeCallHook(func() {
				d.config.Hooks.OnError(ErrCallback(channel, err), channel)
			})
		}
	}
}

// safeCallHook safely calls a hook function, recovering from panics.
func (d *dispatcher) safeCallHook(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("hook panic", slog.String("panic", fmt.Sprintf("%v", r)))
		}
	}()
	fn()
}

// Wait waits for all dispatched callbacks to complete.
func (d *dispatcher) Wait() {
	d.wg.Wait()
}

// metricsCollector collects runtime metrics.
type metricsCollector struct {
	mu                 sync.RWMutex
	totalNotifications int64
	totalErrors        int64
	totalReconnects    int64
	lastNotification   time.Time
	lastError          time.Time
	connectedAt        time.Time
	isConnected        bool
}

// newMetricsCollector creates a new metrics collector.
func newMetricsCollector() *metricsCollector {
	return &metricsCollector{}
}

// IncrementNotifications increments the notification counter.
func (m *metricsCollector) IncrementNotifications() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalNotifications++
	m.lastNotification = time.Now()
}

// IncrementErrors increments the error counter.
func (m *metricsCollector) IncrementErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalErrors++
	m.lastError = time.Now()
}

// IncrementReconnects increments the reconnect counter.
func (m *metricsCollector) IncrementReconnects() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalReconnects++
}

// SetConnected updates the connection status.
func (m *metricsCollector) SetConnected(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if connected && !m.isConnected {
		m.connectedAt = time.Now()
	}
	m.isConnected = connected
}

// GetStatistics returns current statistics.
func (m *metricsCollector) GetStatistics(activeSubscriptions int) Statistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Statistics{
		TotalNotifications:  m.totalNotifications,
		TotalErrors:         m.totalErrors,
		TotalReconnects:     m.totalReconnects,
		ActiveSubscriptions: activeSubscriptions,
		IsConnected:         m.isConnected,
		LastNotificationAt:  m.lastNotification,
		LastErrorAt:         m.lastError,
		ConnectedAt:         m.connectedAt,
	}
}

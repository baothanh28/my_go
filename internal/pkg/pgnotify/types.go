package pgnotify

import (
	"context"
	"time"
)

// Notification represents a PostgreSQL NOTIFY event.
type Notification struct {
	// Channel is the channel name where the notification was sent
	Channel string

	// Payload is the notification payload (max 8KB in PostgreSQL)
	Payload string

	// ReceivedAt is the timestamp when the notification was received
	ReceivedAt time.Time
}

// CallbackFunc is the function signature for notification callbacks.
// The callback is executed in a separate goroutine and should handle errors internally.
type CallbackFunc func(ctx context.Context, notification *Notification) error

// Subscription represents an active subscription to a PostgreSQL channel.
type Subscription interface {
	// Channel returns the channel name this subscription is listening to
	Channel() string

	// Unsubscribe removes this subscription and sends UNLISTEN to PostgreSQL
	Unsubscribe() error
}

// Notifier is the main interface for PostgreSQL LISTEN/NOTIFY functionality.
type Notifier interface {
	// Publish sends a NOTIFY command to the specified channel with the given payload.
	// The payload must not exceed 8KB (PostgreSQL limitation).
	Publish(ctx context.Context, channel string, payload string) error

	// Subscribe registers a callback for notifications on the specified channel.
	// It sends a LISTEN command to PostgreSQL and returns a Subscription handle.
	Subscribe(ctx context.Context, channel string, callback CallbackFunc) (Subscription, error)

	// Start begins listening for notifications. This is a blocking call.
	// It should be run in a goroutine.
	Start(ctx context.Context) error

	// Shutdown gracefully stops the notifier, unsubscribes from all channels,
	// and closes the PostgreSQL connection.
	Shutdown(ctx context.Context) error

	// IsHealthy returns true if the notifier has an active PostgreSQL connection.
	IsHealthy() bool
}

// Hooks provides callbacks for observability and monitoring.
type Hooks struct {
	// OnNotification is called when a notification is received (before dispatching to callback)
	OnNotification func(notification *Notification)

	// OnError is called when an error occurs during notification processing
	OnError func(err error, channel string)

	// OnSubscribe is called when a new subscription is created
	OnSubscribe func(channel string)

	// OnUnsubscribe is called when a subscription is removed
	OnUnsubscribe func(channel string)

	// OnConnect is called when connection is established/re-established
	OnConnect func()

	// OnDisconnect is called when connection is lost
	OnDisconnect func(err error)

	// OnReconnectAttempt is called before each reconnection attempt
	OnReconnectAttempt func(attempt int, nextRetry time.Duration)

	// OnReconnectSuccess is called after successful reconnection
	OnReconnectSuccess func(attempt int)

	// OnReconnectFailed is called when all reconnection attempts fail
	OnReconnectFailed func(attempts int, err error)
}

// ConnectionProvider abstracts the PostgreSQL connection interface.
// This allows using pgx, database/sql, or other implementations.
type ConnectionProvider interface {
	// Listen sends a LISTEN command to PostgreSQL
	Listen(ctx context.Context, channel string) error

	// Unlisten sends an UNLISTEN command to PostgreSQL
	Unlisten(ctx context.Context, channel string) error

	// Notify sends a NOTIFY command to PostgreSQL
	Notify(ctx context.Context, channel string, payload string) error

	// WaitForNotification waits for a notification from PostgreSQL.
	// It returns the notification or an error (including context cancellation).
	WaitForNotification(ctx context.Context) (*Notification, error)

	// Ping checks if the connection is still alive
	Ping(ctx context.Context) error

	// Close closes the connection
	Close() error

	// IsConnected returns true if the connection is active
	IsConnected() bool

	// Reconnect attempts to reconnect to PostgreSQL
	Reconnect(ctx context.Context) error
}

// Statistics holds runtime statistics for the notifier.
type Statistics struct {
	// TotalNotifications is the total number of notifications received
	TotalNotifications int64

	// TotalErrors is the total number of errors encountered
	TotalErrors int64

	// TotalReconnects is the total number of reconnection attempts
	TotalReconnects int64

	// ActiveSubscriptions is the current number of active subscriptions
	ActiveSubscriptions int

	// IsConnected indicates if currently connected to PostgreSQL
	IsConnected bool

	// LastNotificationAt is the timestamp of the last notification received
	LastNotificationAt time.Time

	// LastErrorAt is the timestamp of the last error
	LastErrorAt time.Time

	// ConnectedAt is the timestamp when connection was established
	ConnectedAt time.Time
}

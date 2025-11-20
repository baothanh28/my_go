package pgnotify

import (
	"errors"
	"fmt"
)

var (
	// ErrNotConnected is returned when attempting operations on a disconnected notifier
	ErrNotConnected = errors.New("pgnotify: not connected to PostgreSQL")

	// ErrAlreadyStarted is returned when attempting to start an already running notifier
	ErrAlreadyStarted = errors.New("pgnotify: notifier already started")

	// ErrAlreadyStopped is returned when attempting to stop an already stopped notifier
	ErrAlreadyStopped = errors.New("pgnotify: notifier already stopped")

	// ErrChannelEmpty is returned when attempting to subscribe to an empty channel name
	ErrChannelEmpty = errors.New("pgnotify: channel name cannot be empty")

	// ErrPayloadTooLarge is returned when payload exceeds maximum size
	ErrPayloadTooLarge = errors.New("pgnotify: payload exceeds maximum size")

	// ErrReconnectFailed is returned when all reconnection attempts fail
	ErrReconnectFailed = errors.New("pgnotify: reconnection failed after maximum attempts")

	// ErrShutdownTimeout is returned when graceful shutdown times out
	ErrShutdownTimeout = errors.New("pgnotify: shutdown timeout exceeded")

	// ErrCallbackNil is returned when attempting to subscribe with a nil callback
	ErrCallbackNil = errors.New("pgnotify: callback function cannot be nil")
)

// ErrInvalidConfig represents a configuration validation error.
func ErrInvalidConfig(reason string) error {
	return fmt.Errorf("pgnotify: invalid config: %s", reason)
}

// ErrPublish wraps errors that occur during publishing.
func ErrPublish(channel string, err error) error {
	return fmt.Errorf("pgnotify: failed to publish to channel %q: %w", channel, err)
}

// ErrSubscribe wraps errors that occur during subscription.
func ErrSubscribe(channel string, err error) error {
	return fmt.Errorf("pgnotify: failed to subscribe to channel %q: %w", channel, err)
}

// ErrUnsubscribe wraps errors that occur during unsubscription.
func ErrUnsubscribe(channel string, err error) error {
	return fmt.Errorf("pgnotify: failed to unsubscribe from channel %q: %w", channel, err)
}

// ErrCallback wraps errors that occur during callback execution.
func ErrCallback(channel string, err error) error {
	return fmt.Errorf("pgnotify: callback error for channel %q: %w", channel, err)
}

// ErrConnection wraps connection-related errors.
func ErrConnection(operation string, err error) error {
	return fmt.Errorf("pgnotify: connection %s failed: %w", operation, err)
}

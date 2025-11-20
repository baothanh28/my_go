package pgnotify

import (
	"context"
	"sync"
)

// subscription represents an active subscription to a PostgreSQL channel.
type subscription struct {
	channel  string
	callback CallbackFunc
	notifier *notifier
	mu       sync.Mutex
	active   bool
}

// newSubscription creates a new subscription.
func newSubscription(channel string, callback CallbackFunc, notifier *notifier) *subscription {
	return &subscription{
		channel:  channel,
		callback: callback,
		notifier: notifier,
		active:   true,
	}
}

// Channel returns the channel name this subscription is listening to.
func (s *subscription) Channel() string {
	return s.channel
}

// Unsubscribe removes this subscription and sends UNLISTEN to PostgreSQL.
func (s *subscription) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil // Already unsubscribed
	}

	s.active = false
	return s.notifier.unsubscribe(s.channel)
}

// IsActive returns true if the subscription is still active.
func (s *subscription) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Invoke executes the callback function.
func (s *subscription) Invoke(ctx context.Context, notification *Notification) error {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return nil // Skip if unsubscribed
	}
	callback := s.callback
	s.mu.Unlock()

	return callback(ctx, notification)
}

// subscriptionManager manages all active subscriptions.
type subscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[string][]*subscription
}

// newSubscriptionManager creates a new subscription manager.
func newSubscriptionManager() *subscriptionManager {
	return &subscriptionManager{
		subscriptions: make(map[string][]*subscription),
	}
}

// Add adds a new subscription for the given channel.
func (sm *subscriptionManager) Add(channel string, callback CallbackFunc, notifier *notifier) *subscription {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub := newSubscription(channel, callback, notifier)
	sm.subscriptions[channel] = append(sm.subscriptions[channel], sub)

	return sub
}

// Remove removes a subscription for the given channel.
func (sm *subscriptionManager) Remove(channel string, sub *subscription) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := sm.subscriptions[channel]
	for i, s := range subs {
		if s == sub {
			// Remove by swapping with last element
			subs[i] = subs[len(subs)-1]
			sm.subscriptions[channel] = subs[:len(subs)-1]
			break
		}
	}

	// Clean up empty channel entries
	if len(sm.subscriptions[channel]) == 0 {
		delete(sm.subscriptions, channel)
	}
}

// Get returns all subscriptions for a given channel.
func (sm *subscriptionManager) Get(channel string) []*subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs := sm.subscriptions[channel]
	// Return a copy to avoid race conditions
	result := make([]*subscription, len(subs))
	copy(result, subs)
	return result
}

// GetAll returns all subscriptions across all channels.
func (sm *subscriptionManager) GetAll() map[string][]*subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Return a deep copy
	result := make(map[string][]*subscription)
	for channel, subs := range sm.subscriptions {
		result[channel] = make([]*subscription, len(subs))
		copy(result[channel], subs)
	}
	return result
}

// Channels returns all channels that have active subscriptions.
func (sm *subscriptionManager) Channels() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	channels := make([]string, 0, len(sm.subscriptions))
	for channel := range sm.subscriptions {
		channels = append(channels, channel)
	}
	return channels
}

// Count returns the total number of active subscriptions.
func (sm *subscriptionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, subs := range sm.subscriptions {
		count += len(subs)
	}
	return count
}

// Clear removes all subscriptions.
func (sm *subscriptionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.subscriptions = make(map[string][]*subscription)
}

// HasChannel returns true if there are active subscriptions for the given channel.
func (sm *subscriptionManager) HasChannel(channel string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs, exists := sm.subscriptions[channel]
	return exists && len(subs) > 0
}

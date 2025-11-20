package rate

import (
	"context"
	"sync"
	"time"
)

// MemoryStorage implements Storage interface using in-memory map
type MemoryStorage struct {
	mu   sync.RWMutex
	data map[string]*storageEntry
	done chan struct{}
	wg   sync.WaitGroup
}

type storageEntry struct {
	state     *State
	expiresAt time.Time
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	s := &MemoryStorage{
		data: make(map[string]*storageEntry),
		done: make(chan struct{}),
	}

	// Start cleanup goroutine
	s.wg.Add(1)
	go s.cleanupLoop()

	return s
}

// Get retrieves the current state for a key
func (s *MemoryStorage) Get(ctx context.Context, key string) (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.data[key]
	if !exists {
		return nil, nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	// Return a copy to prevent external modifications
	state := &State{
		Tokens:      entry.state.Tokens,
		LastUpdate:  entry.state.LastUpdate,
		Counter:     entry.state.Counter,
		WindowStart: entry.state.WindowStart,
	}

	if entry.state.Timestamps != nil {
		state.Timestamps = make([]time.Time, len(entry.state.Timestamps))
		copy(state.Timestamps, entry.state.Timestamps)
	}

	return state, nil
}

// Set updates the state for a key
func (s *MemoryStorage) Set(ctx context.Context, key string, state *State, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a copy to prevent external modifications
	stateCopy := &State{
		Tokens:      state.Tokens,
		LastUpdate:  state.LastUpdate,
		Counter:     state.Counter,
		WindowStart: state.WindowStart,
	}

	if state.Timestamps != nil {
		stateCopy.Timestamps = make([]time.Time, len(state.Timestamps))
		copy(stateCopy.Timestamps, state.Timestamps)
	}

	expiresAt := time.Now().Add(ttl)
	if ttl <= 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default 24 hour expiry
	}

	s.data[key] = &storageEntry{
		state:     stateCopy,
		expiresAt: expiresAt,
	}

	return nil
}

// Increment atomically increments the counter for a key
func (s *MemoryStorage) Increment(ctx context.Context, key string, n int, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if !exists || time.Now().After(entry.expiresAt) {
		// Create new entry
		expiresAt := time.Now().Add(ttl)
		if ttl <= 0 {
			expiresAt = time.Now().Add(24 * time.Hour)
		}

		s.data[key] = &storageEntry{
			state: &State{
				Counter:    int64(n),
				LastUpdate: time.Now(),
			},
			expiresAt: expiresAt,
		}
		return int64(n), nil
	}

	// Increment existing entry
	entry.state.Counter += int64(n)
	entry.state.LastUpdate = time.Now()

	// Extend TTL
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}

	return entry.state.Counter, nil
}

// Delete removes the state for a key
func (s *MemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Close closes the storage backend
func (s *MemoryStorage) Close() error {
	close(s.done)
	s.wg.Wait()
	return nil
}

// Ping checks if the storage backend is available
func (s *MemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// cleanupLoop periodically removes expired entries
func (s *MemoryStorage) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.done:
			return
		}
	}
}

// cleanup removes expired entries
func (s *MemoryStorage) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.data {
		if now.After(entry.expiresAt) {
			delete(s.data, key)
		}
	}
}

// Len returns the number of entries (for testing)
func (s *MemoryStorage) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

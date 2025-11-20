package idempotency

import (
	"context"
	"sync"
	"time"
)

// memoryRecord wraps a record with expiry information
type memoryRecord struct {
	record    *Record
	expiresAt time.Time
}

// memoryStorage implements Storage using in-memory storage
// WARNING: This is NOT distributed-safe and should only be used for testing
type memoryStorage struct {
	mu      sync.RWMutex
	records map[string]*memoryRecord
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() Storage {
	storage := &memoryStorage{
		records: make(map[string]*memoryRecord),
	}
	// Start background cleanup goroutine
	go storage.cleanup()
	return storage
}

// cleanup periodically removes expired records
func (s *memoryStorage) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, mr := range s.records {
			if now.After(mr.expiresAt) {
				delete(s.records, key)
			}
		}
		s.mu.Unlock()
	}
}

// Load retrieves a record by key
func (s *memoryStorage) Load(ctx context.Context, key string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mr, exists := s.records[key]
	if !exists {
		return nil, nil
	}

	// Check expiry
	if time.Now().After(mr.expiresAt) {
		return nil, nil
	}

	// Return a copy to prevent external modifications
	recordCopy := *mr.record
	return &recordCopy, nil
}

// TryMarkProcessing atomically marks a key as processing
func (s *memoryStorage) TryMarkProcessing(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if key already exists and is not expired
	if mr, exists := s.records[key]; exists {
		if time.Now().Before(mr.expiresAt) {
			return false, nil // Key already exists
		}
	}

	// Mark as processing
	record := &Record{
		Key:       key,
		Status:    StatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	s.records[key] = &memoryRecord{
		record:    record,
		expiresAt: time.Now().Add(ttl),
	}

	return true, nil
}

// SaveResult saves the successful result
func (s *memoryStorage) SaveResult(ctx context.Context, key string, result []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := &Record{
		Key:       key,
		Status:    StatusCompleted,
		Result:    result,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	s.records[key] = &memoryRecord{
		record:    record,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// SaveError saves the error state
func (s *memoryStorage) SaveError(ctx context.Context, key string, errMsg string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := &Record{
		Key:       key,
		Status:    StatusFailed,
		ErrorMsg:  errMsg,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TTL:       ttl,
	}

	s.records[key] = &memoryRecord{
		record:    record,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

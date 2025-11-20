package idempotency

import (
	"context"
	"fmt"
	"time"
)

// Status represents the state of an idempotency record
type Status string

const (
	StatusNone       Status = "none"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// Record represents an idempotency record with state and result
type Record struct {
	Key       string
	Status    Status
	Result    []byte
	ErrorMsg  string
	CreatedAt time.Time
	UpdatedAt time.Time
	TTL       time.Duration
}

// Storage defines the interface for idempotency storage backends
type Storage interface {
	// Load retrieves a record by key, returns nil if not exists
	Load(ctx context.Context, key string) (*Record, error)

	// TryMarkProcessing uses atomic operations (Redis SETNX, SQL INSERT) to ensure
	// only one thread/process can mark a key as processing
	TryMarkProcessing(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// SaveResult saves the successful result as completed
	SaveResult(ctx context.Context, key string, result []byte, ttl time.Duration) error

	// SaveError saves the error state as failed
	SaveError(ctx context.Context, key string, errMsg string, ttl time.Duration) error
}

// Serializer defines the interface for serializing/deserializing results
type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// KeyGenerator defines the interface for generating idempotency keys
type KeyGenerator interface {
	Generate(input any) (string, error)
}

// Service defines the main idempotency service interface
type Service interface {
	// Execute ensures idempotent execution of fn for the given key
	Execute(
		ctx context.Context,
		key string,
		ttl time.Duration,
		fn func(ctx context.Context) (any, error),
	) (any, error)
}

// TypedService provides a generic wrapper for type-safe execution
type TypedService[T any] struct {
	svc Service
}

// NewTypedService creates a type-safe wrapper around a Service
func NewTypedService[T any](svc Service) *TypedService[T] {
	return &TypedService[T]{svc: svc}
}

// Execute is a type-safe version that preserves type information
func (ts *TypedService[T]) Execute(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	var zero T

	result, err := ts.svc.Execute(ctx, key, ttl, func(ctx context.Context) (any, error) {
		return fn(ctx)
	})

	if err != nil {
		return zero, err
	}

	typed, ok := result.(T)
	if !ok {
		return zero, fmt.Errorf("%w: type assertion failed", ErrSerializationFailure)
	}

	return typed, nil
}

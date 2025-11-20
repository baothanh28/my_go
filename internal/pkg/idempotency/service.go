package idempotency

import (
	"context"
	"fmt"
	"time"
)

// service implements the Service interface
type service struct {
	storage    Storage
	serializer Serializer
}

// NewService creates a new idempotency service
func NewService(storage Storage, serializer Serializer) Service {
	if serializer == nil {
		serializer = NewJSONSerializer()
	}
	return &service{
		storage:    storage,
		serializer: serializer,
	}
}

// Execute ensures idempotent execution of fn for the given key
func (s *service) Execute(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func(ctx context.Context) (any, error),
) (any, error) {
	// Step 1: Load existing record
	record, err := s.storage.Load(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load record: %v", ErrStorageFailure, err)
	}

	// Step 2: Handle existing states
	if record != nil {
		switch record.Status {
		case StatusCompleted:
			// Return cached result
			var result any
			if err := s.serializer.Unmarshal(record.Result, &result); err != nil {
				return nil, fmt.Errorf("%w: failed to unmarshal cached result: %v", ErrSerializationFailure, err)
			}
			return result, nil

		case StatusProcessing:
			// Another process is handling this
			return nil, ErrAlreadyProcessing

		case StatusFailed:
			// Previous attempt failed
			return nil, fmt.Errorf("%w: %s", ErrPreviouslyFailed, record.ErrorMsg)
		}
	}

	// Step 3: Try to mark as processing
	marked, err := s.storage.TryMarkProcessing(ctx, key, ttl)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to mark processing: %v", ErrStorageFailure, err)
	}
	if !marked {
		// Another process won the race
		return nil, ErrAlreadyProcessing
	}

	// Step 4: Execute the function
	result, execErr := fn(ctx)

	// Step 5: Save the result or error
	if execErr != nil {
		if saveErr := s.storage.SaveError(ctx, key, execErr.Error(), ttl); saveErr != nil {
			return nil, fmt.Errorf("%w: failed to save error state: %v (original error: %v)", ErrStorageFailure, saveErr, execErr)
		}
		return nil, execErr
	}

	// Serialize the result
	resultBytes, err := s.serializer.Marshal(result)
	if err != nil {
		saveErr := s.storage.SaveError(ctx, key, fmt.Sprintf("serialization failed: %v", err), ttl)
		if saveErr != nil {
			return nil, fmt.Errorf("%w: failed to serialize result: %v (save error: %v)", ErrSerializationFailure, err, saveErr)
		}
		return nil, fmt.Errorf("%w: %v", ErrSerializationFailure, err)
	}

	// Save the successful result
	if err := s.storage.SaveResult(ctx, key, resultBytes, ttl); err != nil {
		return nil, fmt.Errorf("%w: failed to save result: %v", ErrStorageFailure, err)
	}

	return result, nil
}

// ExecuteTyped is a generic helper function for type-safe execution
func ExecuteTyped[T any](
	svc Service,
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	var zero T

	// Get the underlying service implementation to access storage directly
	serviceImpl, ok := svc.(*service)
	if !ok {
		return zero, fmt.Errorf("ExecuteTyped requires *service implementation")
	}

	// Step 1: Load existing record
	record, err := serviceImpl.storage.Load(ctx, key)
	if err != nil {
		return zero, fmt.Errorf("%w: failed to load record: %v", ErrStorageFailure, err)
	}

	// Step 2: Handle existing states
	if record != nil {
		switch record.Status {
		case StatusCompleted:
			// Return cached result with proper deserialization
			var result T
			if err := serviceImpl.serializer.Unmarshal(record.Result, &result); err != nil {
				return zero, fmt.Errorf("%w: failed to unmarshal cached result: %v", ErrSerializationFailure, err)
			}
			return result, nil

		case StatusProcessing:
			// Another process is handling this
			return zero, ErrAlreadyProcessing

		case StatusFailed:
			// Previous attempt failed
			return zero, fmt.Errorf("%w: %s", ErrPreviouslyFailed, record.ErrorMsg)
		}
	}

	// Step 3: Try to mark as processing
	marked, err := serviceImpl.storage.TryMarkProcessing(ctx, key, ttl)
	if err != nil {
		return zero, fmt.Errorf("%w: failed to mark processing: %v", ErrStorageFailure, err)
	}
	if !marked {
		// Another process won the race
		return zero, ErrAlreadyProcessing
	}

	// Step 4: Execute the function
	result, execErr := fn(ctx)

	// Step 5: Save the result or error
	if execErr != nil {
		if saveErr := serviceImpl.storage.SaveError(ctx, key, execErr.Error(), ttl); saveErr != nil {
			return zero, fmt.Errorf("%w: failed to save error state: %v (original error: %v)", ErrStorageFailure, saveErr, execErr)
		}
		return zero, execErr
	}

	// Serialize the result
	resultBytes, err := serviceImpl.serializer.Marshal(result)
	if err != nil {
		saveErr := serviceImpl.storage.SaveError(ctx, key, fmt.Sprintf("serialization failed: %v", err), ttl)
		if saveErr != nil {
			return zero, fmt.Errorf("%w: failed to serialize result: %v (save error: %v)", ErrSerializationFailure, err, saveErr)
		}
		return zero, fmt.Errorf("%w: %v", ErrSerializationFailure, err)
	}

	// Save the successful result
	if err := serviceImpl.storage.SaveResult(ctx, key, resultBytes, ttl); err != nil {
		return zero, fmt.Errorf("%w: failed to save result: %v", ErrStorageFailure, err)
	}

	return result, nil
}

package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Execute_Success(t *testing.T) {
	storage := NewMemoryStorage()
	serializer := NewJSONSerializer()
	svc := NewService(storage, serializer)

	ctx := context.Background()
	key := "test-key-1"
	ttl := 5 * time.Minute

	callCount := 0
	fn := func(ctx context.Context) (any, error) {
		callCount++
		return "success-result", nil
	}

	// First execution - should execute the function
	result1, err := svc.Execute(ctx, key, ttl, fn)
	require.NoError(t, err)
	assert.Equal(t, "success-result", result1)
	assert.Equal(t, 1, callCount)

	// Second execution - should return cached result
	result2, err := svc.Execute(ctx, key, ttl, fn)
	require.NoError(t, err)
	assert.Equal(t, "success-result", result2)
	assert.Equal(t, 1, callCount) // Function should not be called again
}

func TestService_Execute_Error(t *testing.T) {
	storage := NewMemoryStorage()
	serializer := NewJSONSerializer()
	svc := NewService(storage, serializer)

	ctx := context.Background()
	key := "test-key-2"
	ttl := 5 * time.Minute

	expectedErr := errors.New("operation failed")
	fn := func(ctx context.Context) (any, error) {
		return nil, expectedErr
	}

	// First execution - should fail
	result1, err := svc.Execute(ctx, key, ttl, fn)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result1)

	// Second execution - should return previous failure
	result2, err := svc.Execute(ctx, key, ttl, fn)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPreviouslyFailed)
	assert.Nil(t, result2)
}

func TestService_ExecuteTyped_Success(t *testing.T) {
	storage := NewMemoryStorage()
	serializer := NewJSONSerializer()
	svc := NewService(storage, serializer)

	ctx := context.Background()
	key := "test-key-3"
	ttl := 5 * time.Minute

	type Response struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	callCount := 0
	fn := func(ctx context.Context) (Response, error) {
		callCount++
		return Response{ID: 123, Name: "test"}, nil
	}

	// First execution
	result1, err := ExecuteTyped(svc, ctx, key, ttl, fn)
	require.NoError(t, err)
	assert.Equal(t, 123, result1.ID)
	assert.Equal(t, "test", result1.Name)
	assert.Equal(t, 1, callCount)

	// Second execution - should return cached result
	result2, err := ExecuteTyped(svc, ctx, key, ttl, fn)
	require.NoError(t, err)
	assert.Equal(t, 123, result2.ID)
	assert.Equal(t, "test", result2.Name)
	assert.Equal(t, 1, callCount) // Function should not be called again
}

func TestService_Execute_Concurrent(t *testing.T) {
	storage := NewMemoryStorage()
	serializer := NewJSONSerializer()
	svc := NewService(storage, serializer)

	ctx := context.Background()
	key := "test-key-4"
	ttl := 5 * time.Minute

	callCount := 0
	fn := func(ctx context.Context) (any, error) {
		callCount++
		time.Sleep(100 * time.Millisecond) // Simulate work
		return "result", nil
	}

	// Execute concurrently
	done := make(chan struct{})
	var err1, err2 error
	var result1, result2 any

	go func() {
		result1, err1 = svc.Execute(ctx, key, ttl, fn)
		done <- struct{}{}
	}()

	go func() {
		time.Sleep(10 * time.Millisecond) // Slight delay to ensure race
		result2, err2 = svc.Execute(ctx, key, ttl, fn)
		done <- struct{}{}
	}()

	<-done
	<-done

	// One should succeed, the other should get ErrAlreadyProcessing
	if err1 == nil {
		assert.Equal(t, "result", result1)
		assert.ErrorIs(t, err2, ErrAlreadyProcessing)
	} else {
		assert.ErrorIs(t, err1, ErrAlreadyProcessing)
		assert.Equal(t, "result", result2)
		assert.NoError(t, err2)
	}

	// Function should be called exactly once
	assert.Equal(t, 1, callCount)
}

func TestStorage_Load_NotFound(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	record, err := storage.Load(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, record)
}

func TestStorage_TryMarkProcessing_Twice(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	key := "test-key-5"
	ttl := 5 * time.Minute

	// First mark should succeed
	marked1, err := storage.TryMarkProcessing(ctx, key, ttl)
	require.NoError(t, err)
	assert.True(t, marked1)

	// Second mark should fail
	marked2, err := storage.TryMarkProcessing(ctx, key, ttl)
	require.NoError(t, err)
	assert.False(t, marked2)
}

func TestMemoryStorage_Expiry(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	key := "test-key-6"
	ttl := 100 * time.Millisecond

	// Mark as processing with short TTL
	marked, err := storage.TryMarkProcessing(ctx, key, ttl)
	require.NoError(t, err)
	assert.True(t, marked)

	// Should exist immediately
	record, err := storage.Load(ctx, key)
	require.NoError(t, err)
	assert.NotNil(t, record)

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	record, err = storage.Load(ctx, key)
	require.NoError(t, err)
	assert.Nil(t, record)
}

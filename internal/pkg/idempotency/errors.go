package idempotency

import "errors"

var (
	// ErrAlreadyProcessing indicates another process is currently handling the key
	ErrAlreadyProcessing = errors.New("idempotency: key is already being processed")

	// ErrStorageFailure indicates a storage operation failed
	ErrStorageFailure = errors.New("idempotency: storage operation failed")

	// ErrSerializationFailure indicates serialization/deserialization failed
	ErrSerializationFailure = errors.New("idempotency: serialization failed")

	// ErrKeyGeneration indicates key generation failed
	ErrKeyGeneration = errors.New("idempotency: key generation failed")

	// ErrPreviouslyFailed indicates the operation previously failed
	ErrPreviouslyFailed = errors.New("idempotency: operation previously failed")
)

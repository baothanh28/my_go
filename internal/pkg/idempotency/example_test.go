package idempotency_test

import (
	"context"
	"fmt"
	"time"

	"myapp/internal/pkg/idempotency"
)

// Example_httpHandler demonstrates idempotency in HTTP handler
func Example_httpHandler() {
	// Setup
	storage := idempotency.NewMemoryStorage()
	serializer := idempotency.NewJSONSerializer()
	svc := idempotency.NewService(storage, serializer)

	// Simulate HTTP request with idempotency key
	ctx := context.Background()
	idempotencyKey := "payment-abc-123"
	ttl := 5 * time.Minute

	type PaymentResponse struct {
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
	}

	// Business logic function
	processPayment := func(ctx context.Context) (PaymentResponse, error) {
		// Simulate payment processing
		return PaymentResponse{
			TransactionID: "txn-456",
			Status:        "success",
		}, nil
	}

	// Execute with idempotency
	result, err := idempotency.ExecuteTyped(svc, ctx, idempotencyKey, ttl, processPayment)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Payment processed: %s\n", result.TransactionID)
	// Output: Payment processed: txn-456
}

// Example_worker demonstrates idempotency in background worker
func Example_worker() {
	// Setup
	storage := idempotency.NewMemoryStorage()
	svc := idempotency.NewService(storage, nil)

	// Simulate worker processing message
	ctx := context.Background()
	messageID := "msg-789"
	jobID := fmt.Sprintf("job-%s", messageID)
	ttl := 1 * time.Hour

	// Business logic function
	runJob := func(ctx context.Context) (string, error) {
		// Simulate job processing
		return "job-completed", nil
	}

	// Execute with idempotency
	result, err := idempotency.ExecuteTyped(svc, ctx, jobID, ttl, runJob)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Job result: %s\n", result)
	// Output: Job result: job-completed
}

// Example_keyGenerator demonstrates key generation
func Example_keyGenerator() {
	// Hash-based key generator for deterministic keys
	hashGen := idempotency.NewHashKeyGenerator("payment")

	type PaymentRequest struct {
		UserID string `json:"user_id"`
		Amount int    `json:"amount"`
	}

	req := PaymentRequest{
		UserID: "user-123",
		Amount: 100,
	}

	key, err := hashGen.Generate(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated key prefix: %s\n", key[:8])
	// Output will be consistent hash
}

// Example_uuidKeyGenerator demonstrates UUID key generation
func Example_uuidKeyGenerator() {
	// UUID-based key generator for random keys
	uuidGen := idempotency.NewUUIDKeyGenerator()

	key, err := uuidGen.Generate(nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated UUID key length: %d\n", len(key))
	// Output: Generated UUID key length: 36
}

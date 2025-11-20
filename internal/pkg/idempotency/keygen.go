package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// uuidKeyGenerator generates random UUID keys
type uuidKeyGenerator struct{}

// NewUUIDKeyGenerator creates a UUID-based key generator
func NewUUIDKeyGenerator() KeyGenerator {
	return &uuidKeyGenerator{}
}

// Generate creates a new random UUID key
func (g *uuidKeyGenerator) Generate(input any) (string, error) {
	return uuid.New().String(), nil
}

// hashKeyGenerator generates deterministic hash-based keys from input
type hashKeyGenerator struct {
	prefix string
}

// NewHashKeyGenerator creates a hash-based key generator with optional prefix
func NewHashKeyGenerator(prefix string) KeyGenerator {
	return &hashKeyGenerator{prefix: prefix}
}

// Generate creates a deterministic hash key from the input
func (g *hashKeyGenerator) Generate(input any) (string, error) {
	// Serialize input to JSON for consistent hashing
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("%w: failed to marshal input: %v", ErrKeyGeneration, err)
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	if g.prefix != "" {
		return fmt.Sprintf("%s:%s", g.prefix, hashStr), nil
	}
	return hashStr, nil
}

package idempotency

import (
	"encoding/json"
)

// jsonSerializer implements Serializer using JSON encoding
type jsonSerializer struct{}

// NewJSONSerializer creates a new JSON-based serializer
func NewJSONSerializer() Serializer {
	return &jsonSerializer{}
}

// Marshal serializes a value to JSON bytes
func (s *jsonSerializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal deserializes JSON bytes to a value
func (s *jsonSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

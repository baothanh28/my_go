package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
)

// ConfigManager manages configuration loading and access
type ConfigManager interface {
	Load() error
	Reload() error
	Get(key string) any
	Unmarshal(target any) error
	Watch(callback func()) error
}

// manager implements ConfigManager
type manager struct {
	mu          sync.RWMutex
	providers   []Provider
	data        map[string]any
	config      *Config
	validator   *validator.Validate
	watchActive bool
	watchStop   chan struct{}
}

// New creates a new config manager with the given providers
func New(opts ...Option) ConfigManager {
	m := &manager{
		providers: make([]Provider, 0),
		data:      make(map[string]any),
		validator: validator.New(),
		watchStop: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Option is a functional option for configuring the manager
type Option func(*manager)

// WithProvider adds a provider to the manager
func WithProvider(provider Provider) Option {
	return func(m *manager) {
		m.providers = append(m.providers, provider)
	}
}

// Load loads configuration from all providers in priority order
// Priority: env > file > default (last provider wins)
func (m *manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	merged := make(map[string]any)

	// Load from all providers in order (later providers override earlier ones)
	for _, provider := range m.providers {
		data, err := provider.Load()
		if err != nil {
			// Log warning but continue with other providers
			continue
		}
		merged = mergeMaps(merged, data)
	}

	m.data = merged

	// Unmarshal into Config struct
	var cfg Config
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(merged); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	// Validate
	if err := m.validator.Struct(&cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	m.config = &cfg
	return nil
}

// Reload reloads configuration from all providers
func (m *manager) Reload() error {
	return m.Load()
}

// Get retrieves a configuration value by key (dot-separated path)
func (m *manager) Get(key string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return getNestedValue(m.data, key)
}

// getNestedValue retrieves a value from a nested map using dot-separated keys
func getNestedValue(m map[string]any, key string) any {
	keys := splitKey(key)
	current := any(m)

	for _, k := range keys {
		if m, ok := current.(map[string]any); ok {
			if val, exists := m[k]; exists {
				current = val
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return current
}

// splitKey splits a dot-separated key into parts
func splitKey(key string) []string {
	return strings.Split(key, ".")
}

// Unmarshal unmarshals the configuration into the target struct
func (m *manager) Unmarshal(target any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           target,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	return decoder.Decode(m.data)
}

// Watch watches for configuration file changes and reloads
// This is a simplified implementation - in production, you'd use fsnotify
func (m *manager) Watch(callback func()) error {
	m.mu.Lock()
	if m.watchActive {
		m.mu.Unlock()
		return fmt.Errorf("watch is already active")
	}
	m.watchActive = true
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if files changed (simplified - in production use fsnotify)
				if err := m.Reload(); err == nil {
					if callback != nil {
						callback()
					}
				}
			case <-m.watchStop:
				return
			}
		}
	}()

	return nil
}

// GetConfig returns the unmarshaled Config struct
func (m *manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

package config

import (
	"os"
	"path/filepath"
	"time"
)

var (
	// globalConfig is the global config instance
	globalConfig  *Config
	globalManager ConfigManager
)

// NewConfig creates a new config manager with default providers
// serviceDir should be the absolute or relative directory path of the service
// (e.g., "internal/service/auth" or absolute path)
func NewConfig(serviceDir string) ConfigManager {
	// Resolve service directory to absolute path
	var servicePath string
	if serviceDir != "" {
		// If it's already absolute, use it
		if filepath.IsAbs(serviceDir) {
			servicePath = serviceDir
		} else {
			// Try to resolve relative to current working directory
			wd, err := os.Getwd()
			if err == nil {
				servicePath = filepath.Join(wd, serviceDir)
			} else {
				// Fallback: use as-is
				servicePath = serviceDir
			}
		}
	}

	// Create providers in priority order (last one wins)
	providers := []Provider{
		NewDefaultProvider(getDefaultConfig()),
		NewFileProvider(servicePath),
		NewEnvProvider("APP_"),
	}

	opts := make([]Option, 0, len(providers))
	for _, p := range providers {
		opts = append(opts, WithProvider(p))
	}

	return New(opts...)
}

// getDefaultConfig returns default configuration values
func getDefaultConfig() map[string]any {
	return map[string]any{
		"server": map[string]any{
			"host":             "0.0.0.0",
			"port":             8080,
			"read_timeout":     10,
			"write_timeout":    10,
			"shutdown_timeout": 10,
		},
		"database": map[string]any{
			"host":              "localhost",
			"port":              5432,
			"user":              "postgres",
			"password":          "postgres",
			"dbname":            "myapp",
			"sslmode":           "disable",
			"max_open_conns":    25,
			"max_idle_conns":    5,
			"conn_max_lifetime": 300,
		},
		"jwt": map[string]any{
			"secret":      "your-super-secret-jwt-key-change-this-in-production",
			"expire_hour": 24,
		},
		"logger": map[string]any{
			"level":       "info",
			"format":      "json",
			"output_path": "stdout",
		},
		"redis": map[string]any{
			"addr":              "localhost:6379",
			"username":          "",
			"password":          "",
			"db":                0,
			"pool_size":         10,
			"min_idle_conns":    5,
			"dial_timeout_sec":  5,
			"read_timeout_sec":  3,
			"write_timeout_sec": 3,
			"tls":               false,
		},
	}
}

// LoadGlobalConfig loads the global config instance
// This should be called once at application startup
func LoadGlobalConfig(serviceDir string) (*Config, error) {
	if globalManager == nil {
		globalManager = NewConfig(serviceDir)
	}

	if err := globalManager.Load(); err != nil {
		return nil, err
	}

	// Get the config struct
	if mgr, ok := globalManager.(*manager); ok {
		globalConfig = mgr.GetConfig()
		return globalConfig, nil
	}

	// Fallback: unmarshal manually
	var cfg Config
	if err := globalManager.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	globalConfig = &cfg
	return globalConfig, nil
}

// GetGlobalConfig returns the global config instance
// Returns nil if LoadGlobalConfig hasn't been called yet
func GetGlobalConfig() *Config {
	return globalConfig
}

// GetGlobalConfigManager returns the global config manager instance
// Returns nil if LoadGlobalConfig hasn't been called yet
func GetGlobalConfigManager() ConfigManager {
	return globalManager
}

// Helper functions for easy access
// GetString retrieves a string value by key
func GetString(key string) string {
	if globalManager == nil {
		return ""
	}
	val := globalManager.Get(key)
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// GetInt retrieves an int value by key
func GetInt(key string) int {
	if globalManager == nil {
		return 0
	}
	val := globalManager.Get(key)
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// GetDuration retrieves a duration value by key
func GetDuration(key string) time.Duration {
	if globalManager == nil {
		return 0
	}
	val := globalManager.Get(key)
	if dur, ok := val.(time.Duration); ok {
		return dur
	}
	if str, ok := val.(string); ok {
		if dur, err := time.ParseDuration(str); err == nil {
			return dur
		}
	}
	return 0
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Provider defines the interface for configuration providers
type Provider interface {
	Load() (map[string]any, error)
	Name() string
}

// DefaultProvider provides default configuration values
type DefaultProvider struct {
	defaults map[string]any
}

// NewDefaultProvider creates a new default provider
func NewDefaultProvider(defaults map[string]any) *DefaultProvider {
	return &DefaultProvider{defaults: defaults}
}

// Name returns the provider name
func (p *DefaultProvider) Name() string {
	return "default"
}

// Load returns the default configuration
func (p *DefaultProvider) Load() (map[string]any, error) {
	if p.defaults == nil {
		return make(map[string]any), nil
	}
	return p.defaults, nil
}

// FileProvider loads configuration from files (YAML, JSON, .env)
type FileProvider struct {
	paths      []string
	env        string
	serviceDir string
}

// NewFileProvider creates a new file provider
// It searches for config files in the following order:
// 1. Global base: config/config.yaml
// 2. Global env: config/config.<env>.yaml
// 3. Service base: <serviceDir>/config/config.yaml
// 4. Service env: <serviceDir>/config/config.<env>.yaml
func NewFileProvider(serviceDir string) *FileProvider {
	env := getEnv()
	return &FileProvider{
		paths:      []string{},
		env:        env,
		serviceDir: serviceDir,
	}
}

// Name returns the provider name
func (p *FileProvider) Name() string {
	return "file"
}

// Load loads configuration from files
func (p *FileProvider) Load() (map[string]any, error) {
	result := make(map[string]any)

	// Find global config directory
	globalConfigDir := findGlobalConfigDir()

	// Load global base config
	if globalConfigDir != "" {
		globalBase := filepath.Join(globalConfigDir, "config.yaml")
		if data, err := loadFile(globalBase); err == nil {
			result = mergeMaps(result, data)
		}

		// Load global env config
		if p.env != "" && p.env != "development" {
			globalEnv := filepath.Join(globalConfigDir, fmt.Sprintf("config.%s.yaml", p.env))
			if data, err := loadFile(globalEnv); err == nil {
				result = mergeMaps(result, data)
			}
		}
	}

	// Load service base config
	if p.serviceDir != "" {
		serviceBase := filepath.Join(p.serviceDir, "config", "config.yaml")
		if data, err := loadFile(serviceBase); err == nil {
			result = mergeMaps(result, data)
		}

		// Load service env config
		if p.env != "" && p.env != "development" {
			serviceEnv := filepath.Join(p.serviceDir, "config", fmt.Sprintf("config.%s.yaml", p.env))
			if data, err := loadFile(serviceEnv); err == nil {
				result = mergeMaps(result, data)
			}
		}
	}

	return result, nil
}

// loadFile loads a single config file
func loadFile(path string) (map[string]any, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v.AllSettings(), nil
}

// EnvProvider loads configuration from environment variables
type EnvProvider struct {
	prefix string
}

// NewEnvProvider creates a new environment variable provider
// Environment variables should be prefixed with APP_ (e.g., APP_SERVER_PORT)
// Nested keys use double underscore (e.g., APP_DATABASE__HOST)
func NewEnvProvider(prefix string) *EnvProvider {
	if prefix == "" {
		prefix = "APP_"
	}
	return &EnvProvider{prefix: prefix}
}

// Name returns the provider name
func (p *EnvProvider) Name() string {
	return "env"
}

// Load loads configuration from environment variables
func (p *EnvProvider) Load() (map[string]any, error) {
	result := make(map[string]any)

	// Get all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check if it starts with prefix
		if !strings.HasPrefix(key, p.prefix) {
			continue
		}

		// Remove prefix and convert to nested map
		configKey := strings.TrimPrefix(key, p.prefix)
		configKey = strings.ToLower(configKey)

		// Convert double underscore to nested structure
		keys := strings.Split(configKey, "__")
		setNestedValue(result, keys, value)
	}

	return result, nil
}

// setNestedValue sets a value in a nested map structure
func setNestedValue(m map[string]any, keys []string, value any) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}

	key := keys[0]
	if _, exists := m[key]; !exists {
		m[key] = make(map[string]any)
	}

	if subMap, ok := m[key].(map[string]any); ok {
		setNestedValue(subMap, keys[1:], value)
	}
}

// mergeMaps merges two maps, with b taking precedence over a
func mergeMaps(a, b map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy a
	for k, v := range a {
		result[k] = v
	}

	// Merge b
	for k, v := range b {
		if existing, exists := result[k]; exists {
			if existingMap, ok := existing.(map[string]any); ok {
				if newMap, ok := v.(map[string]any); ok {
					result[k] = mergeMaps(existingMap, newMap)
					continue
				}
			}
		}
		result[k] = v
	}

	return result
}

// getEnv gets the environment name from environment variables
func getEnv() string {
	envVars := []string{"APP_ENV", "GO_ENV", "ENV"}
	for _, envVar := range envVars {
		if env := os.Getenv(envVar); env != "" {
			return env
		}
	}
	return "development"
}

// findGlobalConfigDir finds the global config directory by searching upward from current directory
func findGlobalConfigDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := wd
	for {
		configPath := filepath.Join(dir, "config", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return filepath.Join(dir, "config")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

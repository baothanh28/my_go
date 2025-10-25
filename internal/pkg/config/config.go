package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	SSLMode         string `mapstructure:"sslmode"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	ExpireHour int    `mapstructure:"expire_hour"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// NewConfig creates and returns a new Config instance
// It loads configuration from file, environment variables, and defaults
func NewConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Merge configuration layers (lowest precedence to highest):
	// 1) repository root config/config.yaml
	// 2) service-local ../../config/config.yaml (when running from service/*/cmd)
	// 3) root environment config/config.<env>.yaml
	// 4) service environment ../../config/config.<env>.yaml
	// 5) environment variables (highest precedence)
	if err := mergeConfigLayers(v); err != nil {
		return nil, err
	}

	// Enable environment variable overrides
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("APP")

	// Unmarshal config into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// mergeConfigLayers reads and merges multiple config files in order
func mergeConfigLayers(v *viper.Viper) error {
	v.SetConfigType("yaml")

	env := getEnvironment()

	// Discover configs upward from CWD
	baseFiles, envFiles := discoverConfigFiles(env)

	var globalBase, serviceBase, globalEnv, serviceEnv string
	if len(baseFiles) > 0 {
		globalBase = baseFiles[0]
		serviceBase = baseFiles[len(baseFiles)-1]
	}
	if len(envFiles) > 0 {
		globalEnv = envFiles[0]
		serviceEnv = envFiles[len(envFiles)-1]
	}

	// Load in simple order: global -> service (service overrides)
	for _, path := range []string{globalBase, globalEnv, serviceBase, serviceEnv} {
		if path == "" {
			continue
		}
		v.SetConfigFile(path)
		if err := v.MergeInConfig(); err != nil {
			return fmt.Errorf("failed to merge config file %s: %w", path, err)
		}
	}

	return nil
}

// discoverConfigFiles walks up from the working directory and returns all
// config/config.yaml and config/config.<env>.yaml files found, ordered from
// highest ancestor (root-most) to current directory (most specific).
func discoverConfigFiles(env string) (baseFiles []string, envFiles []string) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, nil
	}

	// Accumulate from CWD upward
	var bases []string
	var envs []string
	dir := wd
	for i := 0; i < 8; i++ { // search up to 8 levels which covers this repo layout
		base := filepath.Join(dir, "config", "config.yaml")
		envp := filepath.Join(dir, "config", "config."+env+".yaml")
		if fileExists(base) {
			bases = append(bases, base)
		}
		if fileExists(envp) {
			envs = append(envs, envp)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Reverse to get highest ancestor first
	for i := len(bases) - 1; i >= 0; i-- {
		baseFiles = append(baseFiles, bases[i])
	}
	for i := len(envs) - 1; i >= 0; i-- {
		envFiles = append(envFiles, envs[i])
	}
	return
}

func getEnvironment() string {
	// Prefer APP_ENV, then GO_ENV, then ENV; default to "development"
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	if v := os.Getenv("GO_ENV"); v != "" {
		return v
	}
	if v := os.Getenv("ENV"); v != "" {
		return v
	}
	return "development"
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 10)
	v.SetDefault("server.write_timeout", 10)
	v.SetDefault("server.shutdown_timeout", 10)

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "myapp")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 300)

	// JWT defaults
	v.SetDefault("jwt.secret", "your-secret-key-change-this")
	v.SetDefault("jwt.expire_hour", 24)

	// Logger defaults
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.output_path", "stdout")
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", cfg.Database.Port)
	}

	if cfg.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	return nil
}

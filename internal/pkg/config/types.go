package config

// Config holds the application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server" validate:"required"`
	Database DatabaseConfig `mapstructure:"database" validate:"required"`
	JWT      JWTConfig      `mapstructure:"jwt" validate:"required"`
	Logger   LoggerConfig   `mapstructure:"logger" validate:"required"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host            string `mapstructure:"host" validate:"required"`
	Port            int    `mapstructure:"port" validate:"required,gt=0,lte=65535"`
	ReadTimeout     int    `mapstructure:"read_timeout" validate:"gte=0"`
	WriteTimeout    int    `mapstructure:"write_timeout" validate:"gte=0"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout" validate:"gte=0"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string `mapstructure:"host" validate:"required"`
	Port            int    `mapstructure:"port" validate:"required,gt=0,lte=65535"`
	User            string `mapstructure:"user" validate:"required"`
	Password        string `mapstructure:"password" validate:"required"`
	DBName          string `mapstructure:"dbname" validate:"required"`
	SSLMode         string `mapstructure:"sslmode" validate:"required,oneof=disable require verify-ca verify-full"`
	MaxOpenConns    int    `mapstructure:"max_open_conns" validate:"gte=1"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns" validate:"gte=0"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime" validate:"gte=0"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string `mapstructure:"secret" validate:"required"`
	ExpireHour int    `mapstructure:"expire_hour" validate:"gte=1"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format     string `mapstructure:"format" validate:"required,oneof=json console"`
	OutputPath string `mapstructure:"output_path" validate:"required"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr            string `mapstructure:"addr"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DB              int    `mapstructure:"db" validate:"gte=0"`
	PoolSize        int    `mapstructure:"pool_size" validate:"gte=1"`
	MinIdleConns    int    `mapstructure:"min_idle_conns" validate:"gte=0"`
	DialTimeoutSec  int    `mapstructure:"dial_timeout_sec" validate:"gte=0"`
	ReadTimeoutSec  int    `mapstructure:"read_timeout_sec" validate:"gte=0"`
	WriteTimeoutSec int    `mapstructure:"write_timeout_sec" validate:"gte=0"`
	TLS             bool   `mapstructure:"tls"`
}

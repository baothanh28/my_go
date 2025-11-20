package config

import (
	"myapp/internal/pkg/config"
)

// ServiceConfig embeds the common application config for the notification service
type ServiceConfig struct {
	*config.Config
	Notification NotificationServiceConfig `mapstructure:"notification"`
}

// NotificationServiceConfig holds notification-specific configuration
type NotificationServiceConfig struct {
	// Poller configuration
	Poller PollerConfig `mapstructure:"poller"`

	// Worker configuration
	WorkerConcurrency int `mapstructure:"worker_concurrency" default:"10"`

	// Retry configuration
	MaxRetries      int `mapstructure:"max_retries" default:"3"`
	RetryBackoffSec int `mapstructure:"retry_backoff_sec" default:"60"`

	// Sender configuration
	Senders SenderConfig `mapstructure:"senders"`
}

// PollerConfig holds poller-specific configuration
type PollerConfig struct {
	Enabled                  bool `mapstructure:"enabled" default:"true"`
	PollIntervalSec          int  `mapstructure:"poll_interval_sec" default:"5"`
	BatchSize                int  `mapstructure:"batch_size" default:"1000"`
	MaxQueueSize             int  `mapstructure:"max_queue_size" default:"2000"`
	BackoffOnEmptySec        int  `mapstructure:"backoff_on_empty_sec" default:"30"`
	ProcessingTimeoutMinutes int  `mapstructure:"processing_timeout_minutes" default:"5"`
}

// SenderConfig holds configuration for notification senders
type SenderConfig struct {
	Expo  ExpoConfig  `mapstructure:"expo"`
	FCM   FCMConfig   `mapstructure:"fcm"`
	APNS  APNSConfig  `mapstructure:"apns"`
	Email EmailConfig `mapstructure:"email"`
}

// ExpoConfig holds Expo push notification configuration
type ExpoConfig struct {
	Enabled     bool   `mapstructure:"enabled" default:"true"`
	APIURL      string `mapstructure:"api_url" default:"https://exp.host/--/api/v2/push/send"`
	AccessToken string `mapstructure:"access_token"`
	TimeoutSec  int    `mapstructure:"timeout_sec" default:"30"`
	MaxRetries  int    `mapstructure:"max_retries" default:"3"`
}

// FCMConfig holds Firebase Cloud Messaging configuration
type FCMConfig struct {
	Enabled         bool   `mapstructure:"enabled" default:"false"`
	ProjectID       string `mapstructure:"project_id"`
	CredentialsFile string `mapstructure:"credentials_file"`
	TimeoutSec      int    `mapstructure:"timeout_sec" default:"30"`
	MaxRetries      int    `mapstructure:"max_retries" default:"3"`
}

// APNSConfig holds Apple Push Notification Service configuration
type APNSConfig struct {
	Enabled    bool   `mapstructure:"enabled" default:"false"`
	KeyID      string `mapstructure:"key_id"`
	TeamID     string `mapstructure:"team_id"`
	BundleID   string `mapstructure:"bundle_id"`
	KeyFile    string `mapstructure:"key_file"`
	Production bool   `mapstructure:"production" default:"false"`
	TimeoutSec int    `mapstructure:"timeout_sec" default:"30"`
	MaxRetries int    `mapstructure:"max_retries" default:"3"`
}

// EmailConfig holds email sender configuration
type EmailConfig struct {
	Enabled    bool   `mapstructure:"enabled" default:"false"`
	SMTPHost   string `mapstructure:"smtp_host"`
	SMTPPort   int    `mapstructure:"smtp_port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	FromEmail  string `mapstructure:"from_email"`
	TimeoutSec int    `mapstructure:"timeout_sec" default:"30"`
	MaxRetries int    `mapstructure:"max_retries" default:"3"`
}

// NewServiceConfig constructs the notification service config from the common config
func NewServiceConfig(cfg *config.Config) (*ServiceConfig, error) {
	serviceCfg := &ServiceConfig{
		Config: cfg,
	}

	// Get the config manager to access raw config
	mgr := config.GetGlobalConfigManager()
	if mgr != nil {
		if err := mgr.Unmarshal(serviceCfg); err != nil {
			return nil, err
		}
	}

	return serviceCfg, nil
}

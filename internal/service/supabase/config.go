package supabase

import "myapp/internal/pkg/config"

// ServiceConfig embeds the common application config for the supabase service.
type ServiceConfig struct {
	*config.Config
	Supabase SupabaseConfig `mapstructure:"supabase"`
}

// NewServiceConfig constructs the service config from the common config.
func NewServiceConfig(cfg *config.Config) *ServiceConfig {
	return &ServiceConfig{Config: cfg}
}

// SupabaseConfig holds Supabase project configuration
type SupabaseConfig struct {
	URL     string `mapstructure:"url"`
	AnonKey string `mapstructure:"anon_key"`
}

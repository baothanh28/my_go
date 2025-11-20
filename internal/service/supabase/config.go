package supabase

import "myapp/internal/pkg/config"

// ServiceConfig embeds the common application config for the supabase service.
type ServiceConfig struct {
	*config.Config
	Supabase SupabaseConfig `mapstructure:"supabase"`
}

// NewServiceConfig constructs the service config from the common config.
func NewServiceConfig(cfg *config.Config) (*ServiceConfig, error) {
	sc := &ServiceConfig{Config: cfg}

	// Unmarshal supabase-specific config
	// Get the config manager to access raw config
	mgr := config.GetGlobalConfigManager()
	if mgr != nil {
		if err := mgr.Unmarshal(sc); err != nil {
			return nil, err
		}
	}

	return sc, nil
}

// SupabaseConfig holds Supabase project configuration
type SupabaseConfig struct {
	URL     string `mapstructure:"url"`
	AnonKey string `mapstructure:"anon_key"`
}

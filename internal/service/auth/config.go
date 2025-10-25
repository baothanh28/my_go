package auth

import "myapp/internal/pkg/config"

// ServiceConfig embeds the common application config for the auth service.
type ServiceConfig struct {
	*config.Config
}

// NewServiceConfig constructs the auth service config from the common config.
func NewServiceConfig(cfg *config.Config) *ServiceConfig {
	return &ServiceConfig{Config: cfg}
}

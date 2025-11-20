package config

import (
	"path/filepath"
	"runtime"

	"go.uber.org/fx"
)

// Module provides the config module for FX dependency injection
var Module = fx.Module("config",
	fx.Provide(NewConfigProvider),
)

// ConfigProviderParams defines parameters for config provider
type ConfigProviderParams struct {
	fx.In
	// ServiceDir can be injected if needed, otherwise auto-detected
	ServiceDir string `optional:"true" name:"serviceDir"`
}

// NewConfigProvider creates a config provider for FX
func NewConfigProvider(params ConfigProviderParams) (*Config, error) {
	serviceDir := params.ServiceDir

	// Auto-detect service directory if not provided
	if serviceDir == "" {
		serviceDir = detectServiceDir()
	}

	return LoadGlobalConfig(serviceDir)
}

// detectServiceDir attempts to detect the service directory from the call stack
func detectServiceDir() string {
	// Get the caller's file path (skip 3 frames: detectServiceDir -> NewConfigProvider -> FX -> caller)
	_, filename, _, ok := runtime.Caller(3)
	if !ok {
		return ""
	}

	// Find the service directory
	// Look for patterns like: .../internal/service/<service>/...
	dir := filepath.Dir(filename)

	// Walk up to find "service" directory
	for {
		base := filepath.Base(dir)
		if base == "service" {
			// Found service directory, check if parent is "internal"
			parent := filepath.Dir(dir)
			if filepath.Base(parent) == "internal" {
				// The next directory after "service" should be the service name
				// But we're currently at "service", so we need to check the filename's parent
				// The filename is in: internal/service/<service>/...
				// So we need to get the service name from the path relative to "service"
				serviceDir := filepath.Dir(filename)
				// Find the directory that is a direct child of "service"
				for {
					if filepath.Dir(serviceDir) == dir {
						// Found it! serviceDir is the service directory
						if absPath, err := filepath.Abs(serviceDir); err == nil {
							return absPath
						}
						return serviceDir
					}
					parent := filepath.Dir(serviceDir)
					if parent == serviceDir {
						break
					}
					serviceDir = parent
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// WithServiceDir is an FX option to specify the service directory
func WithServiceDir(serviceDir string) fx.Option {
	return fx.Supply(
		fx.Annotate(
			serviceDir,
			fx.ResultTags(`name:"serviceDir"`),
		),
	)
}

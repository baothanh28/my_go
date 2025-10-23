package config

import "go.uber.org/fx"

// Module exports the config module for FX
var Module = fx.Module("config",
	fx.Provide(NewConfig),
)


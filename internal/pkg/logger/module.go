package logger

import "go.uber.org/fx"

// Module exports the logger module for FX
var Module = fx.Module("logger",
	fx.Provide(NewLogger),
)


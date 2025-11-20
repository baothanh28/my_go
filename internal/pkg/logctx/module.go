package logctx

import (
	"context"

	"go.uber.org/fx"
)

type traceKeyType struct{}
type correlationKeyType struct{}

var traceKey = traceKeyType{}
var correlationKey = correlationKeyType{}

// Module placeholder (no providers needed, helpers only)
var Module = fx.Module("logctx")

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceKey, traceID)
}

func TraceID(ctx context.Context) (string, bool) {
	v := ctx.Value(traceKey)
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationKey, correlationID)
}

func CorrelationID(ctx context.Context) (string, bool) {
	v := ctx.Value(correlationKey)
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

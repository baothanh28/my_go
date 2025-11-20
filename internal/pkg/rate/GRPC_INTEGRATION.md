# gRPC Integration for Rate Limiter

This document describes how to integrate the rate limiter with gRPC services.

## Prerequisites

Install gRPC dependencies:

```bash
go get google.golang.org/grpc
go get google.golang.org/grpc/codes
go get google.golang.org/grpc/metadata
go get google.golang.org/grpc/peer
go get google.golang.org/grpc/status
```

## Implementation

Create a file `middleware_grpc.go` in your project:

```go
package yourpackage

import (
    "context"
    "fmt"
    
    "myapp/internal/pkg/rate"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/peer"
    "google.golang.org/grpc/status"
)

// GRPCKeyFunc extracts the rate limit key from a gRPC context
type GRPCKeyFunc func(ctx context.Context, method string) string

// UnaryServerInterceptor returns a gRPC unary server interceptor for rate limiting
func UnaryServerInterceptor(limiter rate.Limiter, keyFunc GRPCKeyFunc) grpc.UnaryServerInterceptor {
    if keyFunc == nil {
        keyFunc = DefaultGRPCKeyFunc
    }

    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        key := keyFunc(ctx, info.FullMethod)

        allowed, err := limiter.Allow(ctx, key)
        if err != nil {
            return nil, status.Error(codes.Internal, "rate limiter error")
        }

        if !allowed {
            if reservation, err := limiter.Reserve(ctx, key); err == nil && !reservation.OK {
                md := metadata.Pairs("retry-after", fmt.Sprintf("%d", int64(reservation.Delay.Seconds())))
                grpc.SetHeader(ctx, md)
            }
            return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
        }

        return handler(ctx, req)
    }
}

// StreamServerInterceptor returns a gRPC stream server interceptor for rate limiting
func StreamServerInterceptor(limiter rate.Limiter, keyFunc GRPCKeyFunc) grpc.StreamServerInterceptor {
    if keyFunc == nil {
        keyFunc = DefaultGRPCKeyFunc
    }

    return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
        ctx := ss.Context()
        key := keyFunc(ctx, info.FullMethod)

        allowed, err := limiter.Allow(ctx, key)
        if err != nil {
            return status.Error(codes.Internal, "rate limiter error")
        }

        if !allowed {
            if reservation, err := limiter.Reserve(ctx, key); err == nil && !reservation.OK {
                md := metadata.Pairs("retry-after", fmt.Sprintf("%d", int64(reservation.Delay.Seconds())))
                grpc.SetHeader(ctx, md)
            }
            return status.Error(codes.ResourceExhausted, "rate limit exceeded")
        }

        return handler(srv, ss)
    }
}

// DefaultGRPCKeyFunc extracts the peer address as the rate limit key
func DefaultGRPCKeyFunc(ctx context.Context, method string) string {
    if p, ok := peer.FromContext(ctx); ok {
        return p.Addr.String()
    }
    return "unknown"
}

// GRPCIPKeyFunc creates a key function that uses the peer IP
func GRPCIPKeyFunc() GRPCKeyFunc {
    return DefaultGRPCKeyFunc
}

// GRPCMethodKeyFunc creates a key function that combines IP and method
func GRPCMethodKeyFunc() GRPCKeyFunc {
    return func(ctx context.Context, method string) string {
        ip := "unknown"
        if p, ok := peer.FromContext(ctx); ok {
            ip = p.Addr.String()
        }
        return fmt.Sprintf("%s:%s", ip, method)
    }
}

// GRPCMetadataKeyFunc creates a key function that uses metadata
func GRPCMetadataKeyFunc(key string) GRPCKeyFunc {
    return func(ctx context.Context, method string) string {
        if md, ok := metadata.FromIncomingContext(ctx); ok {
            if values := md.Get(key); len(values) > 0 {
                return values[0]
            }
        }
        return DefaultGRPCKeyFunc(ctx, method)
    }
}

// GRPCUserKeyFunc creates a key function for authenticated users
func GRPCUserKeyFunc(contextKey interface{}) GRPCKeyFunc {
    return func(ctx context.Context, method string) string {
        if userID := ctx.Value(contextKey); userID != nil {
            return fmt.Sprintf("user:%v", userID)
        }
        return DefaultGRPCKeyFunc(ctx, method)
    }
}
```

## Usage

```go
package main

import (
    "time"
    
    "myapp/internal/pkg/rate"
    "yourpackage"
    "google.golang.org/grpc"
)

func main() {
    storage := rate.NewMemoryStorage()
    
    config := &rate.Config{
        Strategy: rate.StrategyTokenBucket,
        Rate:     50,
        Burst:    100,
        Interval: 1 * time.Second,
        TTL:      5 * time.Second,
        FailOpen: false,
    }

    limiter, _ := rate.New(config, storage)

    // Create gRPC server with rate limiting
    server := grpc.NewServer(
        grpc.UnaryInterceptor(
            yourpackage.UnaryServerInterceptor(limiter, yourpackage.GRPCIPKeyFunc()),
        ),
        grpc.StreamInterceptor(
            yourpackage.StreamServerInterceptor(limiter, yourpackage.GRPCIPKeyFunc()),
        ),
    )

    // Register your services...
}
```

## Key Functions

- **GRPCIPKeyFunc()**: Rate limit by client IP address
- **GRPCMethodKeyFunc()**: Rate limit by IP + method combination
- **GRPCMetadataKeyFunc(key)**: Rate limit by metadata value
- **GRPCUserKeyFunc(contextKey)**: Rate limit by authenticated user

## Error Handling

When rate limit is exceeded, clients will receive:
- Status code: `ResourceExhausted`
- Metadata header: `retry-after` with seconds to wait


package rate

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// HTTPMiddleware creates an HTTP middleware for rate limiting
type HTTPMiddleware struct {
	limiter   Limiter
	keyFunc   KeyFunc
	onLimited OnLimitedFunc
	skipFunc  SkipFunc
}

// KeyFunc extracts the rate limit key from an HTTP request
type KeyFunc func(*http.Request) string

// OnLimitedFunc is called when a request is rate limited
type OnLimitedFunc func(w http.ResponseWriter, r *http.Request, result *Result)

// SkipFunc determines if rate limiting should be skipped for a request
type SkipFunc func(*http.Request) bool

// HTTPMiddlewareOption is a functional option for HTTPMiddleware
type HTTPMiddlewareOption func(*HTTPMiddleware)

// NewHTTPMiddleware creates a new HTTP rate limiting middleware
func NewHTTPMiddleware(limiter Limiter, opts ...HTTPMiddlewareOption) *HTTPMiddleware {
	m := &HTTPMiddleware{
		limiter:   limiter,
		keyFunc:   DefaultKeyFunc,
		onLimited: DefaultOnLimitedFunc,
		skipFunc:  nil,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithKeyFunc sets the key extraction function
func WithKeyFunc(fn KeyFunc) HTTPMiddlewareOption {
	return func(m *HTTPMiddleware) {
		m.keyFunc = fn
	}
}

// WithOnLimited sets the rate limit exceeded handler
func WithOnLimited(fn OnLimitedFunc) HTTPMiddlewareOption {
	return func(m *HTTPMiddleware) {
		m.onLimited = fn
	}
}

// WithSkipFunc sets the skip function
func WithSkipFunc(fn SkipFunc) HTTPMiddlewareOption {
	return func(m *HTTPMiddleware) {
		m.skipFunc = fn
	}
}

// Middleware returns the HTTP middleware handler
func (m *HTTPMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if should skip
		if m.skipFunc != nil && m.skipFunc(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract key
		key := m.keyFunc(r)

		// Check rate limit
		start := time.Now()
		reservation, err := m.limiter.Reserve(r.Context(), key)
		duration := time.Since(start)

		if err != nil {
			// On error, fail based on configuration
			// The limiter's fail-open/fail-close should handle this
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !reservation.OK {
			// Rate limited
			result := &Result{
				Allowed:    false,
				RetryAfter: reservation.Delay,
			}
			m.onLimited(w, r, result)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(reservation.Limit.Rate))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(reservation.Tokens))
		if reservation.Delay > 0 {
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(reservation.Delay).Unix(), 10))
		}

		// Record latency if metrics available
		if m, ok := m.limiter.(*limiterImpl); ok && m.metrics != nil {
			m.metrics.RecordLatency(m.config.Strategy, duration)
		}

		next.ServeHTTP(w, r)
	})
}

// DefaultKeyFunc extracts the client IP as the rate limit key
func DefaultKeyFunc(r *http.Request) string {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// IPKeyFunc creates a key function that uses the client IP
func IPKeyFunc() KeyFunc {
	return DefaultKeyFunc
}

// PathKeyFunc creates a key function that combines IP and path
func PathKeyFunc() KeyFunc {
	return func(r *http.Request) string {
		ip := DefaultKeyFunc(r)
		return fmt.Sprintf("%s:%s", ip, r.URL.Path)
	}
}

// HeaderKeyFunc creates a key function that uses a specific header
func HeaderKeyFunc(header string) KeyFunc {
	return func(r *http.Request) string {
		value := r.Header.Get(header)
		if value == "" {
			return DefaultKeyFunc(r)
		}
		return value
	}
}

// UserKeyFunc creates a key function for authenticated users
// Assumes user ID is stored in request context
func UserKeyFunc(contextKey interface{}) KeyFunc {
	return func(r *http.Request) string {
		if userID := r.Context().Value(contextKey); userID != nil {
			return fmt.Sprintf("user:%v", userID)
		}
		return DefaultKeyFunc(r)
	}
}

// DefaultOnLimitedFunc is the default rate limit exceeded handler
func DefaultOnLimitedFunc(w http.ResponseWriter, r *http.Request, result *Result) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Retry-After", strconv.FormatInt(int64(result.RetryAfter.Seconds()), 10))
	w.Header().Set("Retry-After", strconv.FormatInt(int64(result.RetryAfter.Seconds()), 10))

	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after":%d}`, int64(result.RetryAfter.Seconds()))
}

package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type Policy struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
	MaxAttempts int
}

func ExponentialBackoff(base, max time.Duration, jitter bool, maxAttempts int) Policy {
	return Policy{
		BaseDelay:   base,
		MaxDelay:    max,
		Jitter:      jitter,
		MaxAttempts: maxAttempts,
	}
}

func (p Policy) nextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	// 2^(attempt-1) * base
	factor := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(p.BaseDelay) * factor)
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	if p.Jitter {
		j := rand.Float64()*0.4 + 0.8 // [0.8, 1.2)
		delay = time.Duration(float64(delay) * j)
	}
	return delay
}

// Do runs fn with retry upon error while isRetryable(err) is true
func Do[T any](ctx context.Context, policy Policy, fn func(context.Context) (T, error), isRetryable func(error) bool) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; policy.MaxAttempts == 0 || attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}
		res, err := fn(ctx)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if isRetryable != nil && !isRetryable(err) {
			break
		}
		delay := policy.nextDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, lastErr
}

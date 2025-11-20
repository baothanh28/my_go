package errorsx

import "errors"

var (
	// Retryable indicates the operation may succeed if retried
	Retryable = errors.New("retryable")
	// Permanent indicates the operation will not succeed upon retry
	Permanent = errors.New("permanent")
)

// WrapRetryable wraps an error as retryable
func WrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(Retryable, err)
}

// WrapPermanent wraps an error as permanent
func WrapPermanent(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(Permanent, err)
}

func IsRetryable(err error) bool {
	return errors.Is(err, Retryable)
}

func IsPermanent(err error) bool {
	return errors.Is(err, Permanent)
}

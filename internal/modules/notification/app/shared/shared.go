package shared

// NonRetryableError wraps an error as non-retryable for worker retry semantics.
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

func (e *NonRetryableError) NonRetryable() bool {
	return true
}

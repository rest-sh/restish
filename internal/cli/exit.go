package cli

import "fmt"

// ExitCodeError is returned when a command completed and the process should
// exit with a specific non-zero code. If Cause is nil the response body has
// already been written to stdout (HTTP status errors, SIGINT 130), so the
// caller should NOT print an additional error message. If Cause is non-nil, the
// caller should print it before exiting.
type ExitCodeError struct {
	Code  int
	Cause error
}

func (e *ExitCodeError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

// Unwrap returns the underlying cause, if any.
func (e *ExitCodeError) Unwrap() error {
	return e.Cause
}

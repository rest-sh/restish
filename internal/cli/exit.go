package cli

import (
	"errors"
	"fmt"
	"strings"
)

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

// UsageError marks command-line invocation problems that should exit with 2.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	if e == nil || e.Err == nil {
		return "usage error"
	}
	return e.Err.Error()
}

func (e *UsageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newUsageError(err error) error {
	if err == nil {
		return nil
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return err
	}
	return &UsageError{Err: err}
}

func usageExitError(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return err
	}
	if isUsageError(err) {
		return &ExitCodeError{Code: 2, Cause: err}
	}
	return err
}

func isUsageError(err error) bool {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return true
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unknown command "):
		return true
	case strings.HasPrefix(msg, "unknown flag: "):
		return true
	case strings.HasPrefix(msg, "unknown shorthand flag: "):
		return true
	case strings.HasPrefix(msg, "unsupported shell "):
		return true
	case strings.HasPrefix(msg, "unknown shell command "):
		return true
	case strings.HasPrefix(msg, "requires at least "):
		return true
	case strings.HasPrefix(msg, "requires at most "):
		return true
	case strings.HasPrefix(msg, "requires a minimum of "):
		return true
	case strings.HasPrefix(msg, "accepts ") && strings.Contains(msg, " arg(s)"):
		return true
	case strings.Contains(msg, "invalid number of arguments"):
		return true
	default:
		return false
	}
}

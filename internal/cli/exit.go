package cli

import "fmt"

// ExitCodeError is returned when a command completed but should exit with a
// specific non-zero code. The response body has already been written to stdout
// before this error is returned, so the caller should NOT print an additional
// error message — just call os.Exit with Code.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

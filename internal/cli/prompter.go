package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"golang.org/x/term"
)

// Prompter is the interface for interactive user prompts. All methods check ctx
// before blocking on I/O, so callers can respect cancellation.
type Prompter interface {
	// Prompt shows label and reads a line of visible text input.
	Prompt(ctx context.Context, label string) (string, error)
	// Secret shows label and reads a secret value without echoing it.
	Secret(ctx context.Context, label string) (string, error)
	// Confirm shows label and reads a yes/no answer. Returns true only for
	// explicit "y" or "yes"; empty input on a non-TTY returns false.
	Confirm(ctx context.Context, label string) (bool, error)
}

var promptOpenTTY = func() (*os.File, error) {
	return os.Open("/dev/tty")
}

// Prompt implements Prompter by reading a visible input line from the user.
func (c *CLI) Prompt(ctx context.Context, label string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	src, cleanup := c.promptSource()
	defer cleanup()
	return readPromptValue(label, src, c.Stderr, false)
}

// Secret implements Prompter by reading a password/token without echo.
func (c *CLI) Secret(ctx context.Context, label string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	src, cleanup := c.promptSource()
	defer cleanup()
	return readPromptValue(label, src, c.Stderr, true)
}

// Confirm implements Prompter by reading a yes/no confirmation.
//
// Rules:
//   - "y" or "yes" (case-insensitive) → true
//   - "n", "no", or any other non-empty input → false
//   - Empty input (Enter) → true only when stdin is an interactive TTY
//   - EOF → false (safe default for piped/scripted invocations)
func (c *CLI) Confirm(ctx context.Context, label string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	fmt.Fprint(c.Stderr, label)

	src, cleanup := c.promptSource()
	defer cleanup()
	reader := bufio.NewReader(src)
	line, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("confirm: %w", err)
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" {
		if f, ok := src.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			return true, nil
		}
		return false, nil
	}
	return answer == "y" || answer == "yes", nil
}

// promptSource returns the I/O reader to use for interactive prompts.
// Priority:
//  1. hooks.PassReader (set in tests to avoid TTY dependency)
//  2. /dev/tty when c.Stdin is not a terminal (stdin may be piped but we still
//     need to reach the user's keyboard for interactive prompts)
//  3. c.Stdin as a last resort
func (c *CLI) promptSource() (io.Reader, func()) {
	if c.hooks.PassReader != nil {
		return c.hooks.PassReader, func() {}
	}
	if !output.IsTerminalReader(c.Stdin) {
		if f, err := promptOpenTTY(); err == nil {
			return f, func() { _ = f.Close() }
		}
	}
	return c.Stdin, func() {}
}

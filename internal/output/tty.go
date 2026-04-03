package output

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// IsTerminal reports whether w is a real terminal (TTY).
func IsTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return false
}

// ColorEnabled reports whether ANSI color output should be used for w.
// Rules (in priority order):
//  1. NOCOLOR or NO_COLOR env var → off
//  2. COLOR env var → on
//  3. w is a TTY → on; otherwise off
func ColorEnabled(w io.Writer) bool {
	if os.Getenv("NOCOLOR") != "" || os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("COLOR") != "" {
		return true
	}
	return IsTerminal(w)
}

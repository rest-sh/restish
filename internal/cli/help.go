package cli

import (
	"strings"
	"sync"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// setupMarkdownHelp installs a custom HelpFunc on root that passes each
// command's Long description through Glamour before delegating to Cobra's
// default help renderer. On non-TTY stdout it keeps help plain text while still
// applying Restish-specific help text selection such as generated API
// description truncation.
//
// A mutex protects the temporary cmd.Long substitution so that concurrent
// help calls from different goroutines do not race on the shared cobra.Command.
func (c *CLI) setupMarkdownHelp(root *cobra.Command) {
	var mu sync.Mutex
	original := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		long := commandHelpLong(cmd)
		if long == "" {
			original(cmd, args)
			return
		}
		if output.IsTerminal(c.Stdout) {
			rendered, err := renderMarkdown(long, c)
			if err != nil {
				original(cmd, args)
				return
			}
			long = rendered
		}
		// Swap cmd.Long with the rendered version for Cobra's template pipeline,
		// then restore it synchronously (no defer) so the window is as small as
		// possible and protected by a mutex for concurrent safety.
		mu.Lock()
		orig := cmd.Long
		cmd.Long = long
		original(cmd, args)
		cmd.Long = orig
		mu.Unlock()
	})
}

func commandHelpLong(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if cmd.Annotations != nil {
		if commandHelpAllRequested(cmd) {
			if long := cmd.Annotations[generatedAPIHelpFullAnnotation]; long != "" {
				return long
			}
		}
		if long := cmd.Annotations[generatedAPIHelpShortAnnotation]; long != "" {
			return long
		}
	}
	return cmd.Long
}

// renderMarkdown passes s through Glamour and returns the rendered string.
// Word-wrap width is taken from the terminal attached to c.Stdout, falling
// back to 80 columns.
func renderMarkdown(s string, c *CLI) (string, error) {
	width := 80
	if f, ok := c.Stdout.(interface{ Fd() uintptr }); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	r, err := output.NewMarkdownRenderer(width)
	if err != nil {
		return "", err
	}

	rendered, err := r.Render(s)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(rendered, "\n"), nil
}

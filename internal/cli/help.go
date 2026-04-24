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
// default help renderer. On non-TTY stdout the original help function is used
// unchanged so that piped help output stays plain text.
//
// A mutex protects the temporary cmd.Long substitution so that concurrent
// help calls from different goroutines do not race on the shared cobra.Command.
func (c *CLI) setupMarkdownHelp(root *cobra.Command) {
	if !output.IsTerminal(c.Stdout) {
		return
	}

	var mu sync.Mutex
	original := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.Long == "" {
			original(cmd, args)
			return
		}
		rendered, err := renderMarkdown(cmd.Long, c)
		if err != nil {
			original(cmd, args)
			return
		}
		// Swap cmd.Long with the rendered version for Cobra's template pipeline,
		// then restore it synchronously (no defer) so the window is as small as
		// possible and protected by a mutex for concurrent safety.
		mu.Lock()
		orig := cmd.Long
		cmd.Long = rendered
		original(cmd, args)
		cmd.Long = orig
		mu.Unlock()
	})
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

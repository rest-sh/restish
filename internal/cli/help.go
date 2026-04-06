package cli

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// setupMarkdownHelp installs a custom HelpFunc on root that passes each
// command's Long description through Glamour before delegating to Cobra's
// default help renderer. On non-TTY stdout the original help function is used
// unchanged so that piped help output stays plain text.
//
// The substitution is temporary (deferred restore) so that command objects,
// which are reused across invocations, are never left with rendered ANSI in
// their Long field.
func (c *CLI) setupMarkdownHelp(root *cobra.Command) {
	if !output.IsTerminal(c.Stdout) {
		return
	}

	original := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.Long != "" {
			if rendered, err := renderMarkdown(cmd.Long, c); err == nil {
				orig := cmd.Long
				cmd.Long = rendered
				defer func() { cmd.Long = orig }()
			}
		}
		original(cmd, args)
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

	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}

	rendered, err := r.Render(s)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(rendered, "\n"), nil
}

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
		if long != "" && c.stdoutIsTerminal() {
			rendered, err := renderMarkdown(long, c)
			if err == nil {
				long = rendered
			}
		}
		// Swap cmd.Long with the rendered version for Cobra's template pipeline,
		// then restore it synchronously (no defer) so the window is as small as
		// possible and protected by a mutex for concurrent safety.
		mu.Lock()
		orig := cmd.Long
		cmd.Long = long
		if output.ColorEnabled(c.Stdout) {
			var buf strings.Builder
			origOut := cmd.OutOrStdout()
			cmd.SetOut(&buf)
			original(cmd, args)
			cmd.SetOut(origOut)
			cmd.Long = orig
			mu.Unlock()
			_, _ = c.Stdout.Write([]byte(colorizeHelpText(buf.String(), humanTextStyleFor(c.Stdout))))
			return
		}
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

func colorizeHelpText(text string, style humanTextStyle) string {
	if !style.color {
		return text
	}
	var out strings.Builder
	lines := strings.SplitAfter(text, "\n")
	for _, raw := range lines {
		line := strings.TrimSuffix(raw, "\n")
		newline := raw[len(line):]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out.WriteString(raw)
		case isHelpHeading(trimmed):
			out.WriteString(colorizeWholeLine(line, style.heading))
			out.WriteString(newline)
		default:
			out.WriteString(raw)
		}
	}
	return out.String()
}

func isHelpHeading(trimmed string) bool {
	if strings.HasSuffix(trimmed, ":") {
		return true
	}
	switch trimmed {
	case rootGroupHTTP,
		rootGroupConfig,
		rootGroupPlugin,
		rootGroupAPI,
		rootGroupUtility,
		rootGroupHelp,
		flagGroupRequest,
		flagGroupOutput,
		flagGroupAuth,
		flagGroupTLS,
		flagGroupPaging,
		flagGroupCache,
		flagGroupGeneral,
		flagGroupUngrouped,
		"Generic HTTP Commands",
		"Configuration and Setup",
		"Plugin Commands",
		"Registered APIs",
		"Utilities",
		"Help":
		return true
	default:
		return false
	}
}

func colorizeWholeLine(line string, fn func(string) string) string {
	return fn(line)
}

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
)

type diagnosticRole string

const (
	diagnosticInfo  diagnosticRole = "diagnostic_info"
	diagnosticWarn  diagnosticRole = "diagnostic_warn"
	diagnosticError diagnosticRole = "diagnostic_error"
	diagnosticHint  diagnosticRole = "diagnostic_hint"
)

func (c *CLI) infof(format string, args ...any) {
	writeDiagnostic(c.Stderr, diagnosticInfo, "info", format, args...)
}

func (c *CLI) warnf(format string, args ...any) {
	writeDiagnostic(c.Stderr, diagnosticWarn, "warning", format, args...)
}

func (c *CLI) hintf(format string, args ...any) {
	writeDiagnostic(c.Stderr, diagnosticHint, "hint", format, args...)
}

func (c *CLI) tipf(format string, args ...any) {
	writeDiagnostic(c.Stderr, diagnosticHint, "tip", format, args...)
}

func writeDiagnostic(w io.Writer, role diagnosticRole, label, format string, args ...any) {
	prefix := label + ":"
	prefix = colorDiagnosticPrefix(w, role, prefix)
	fmt.Fprintf(w, "%s %s\n", prefix, fmt.Sprintf(format, args...))
}

func colorDiagnosticPrefix(w io.Writer, role diagnosticRole, prefix string) string {
	if !output.ColorEnabled(w) {
		return prefix
	}
	return output.StyleText(string(role), prefix)
}

func diagnosticPrefixWriter(w io.Writer) io.Writer {
	return diagnosticWriter{w: w}
}

type diagnosticWriter struct {
	w io.Writer
}

func (w diagnosticWriter) Write(p []byte) (int, error) {
	text := string(p)
	for _, def := range []struct {
		prefix string
		role   diagnosticRole
	}{
		{"info:", diagnosticInfo},
		{"warning:", diagnosticWarn},
		{"error:", diagnosticError},
		{"hint:", diagnosticHint},
		{"tip:", diagnosticHint},
	} {
		if strings.HasPrefix(text, def.prefix) {
			text = colorDiagnosticPrefix(w.w, def.role, def.prefix) + text[len(def.prefix):]
			break
		}
	}
	if _, err := io.WriteString(w.w, text); err != nil {
		return 0, err
	}
	return len(p), nil
}

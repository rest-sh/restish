package cli

import (
	"io"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
)

type humanTextStyle struct {
	color bool
}

func humanTextStyleFor(w io.Writer) humanTextStyle {
	return humanTextStyle{color: output.ColorEnabled(w)}
}

func (s humanTextStyle) ok(text string) string {
	return s.style("status_2xx", text)
}

func (s humanTextStyle) info(text string) string {
	return s.style("diagnostic_info", text)
}

func (s humanTextStyle) warn(text string) string {
	return s.style("diagnostic_warn", text)
}

func (s humanTextStyle) error(text string) string {
	return s.style("diagnostic_error", text)
}

func (s humanTextStyle) hint(text string) string {
	return s.style("diagnostic_hint", text)
}

func (s humanTextStyle) key(text string) string {
	return s.style("key", text)
}

func (s humanTextStyle) heading(text string) string {
	return s.style("heading", text)
}

func (s humanTextStyle) authStatus(text string) string {
	lower := strings.ToLower(text)
	switch {
	case lower == "configured":
		return s.ok(text)
	case lower == "none" || lower == "empty" || lower == "missing" || lower == "unavailable":
		return s.warn(text)
	case strings.HasPrefix(lower, "configured ("):
		return s.warn(text)
	default:
		return text
	}
}

func (s humanTextStyle) httpStatus(statusCode int, text string) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return s.ok(text)
	case statusCode >= 300 && statusCode < 400:
		return s.warn(text)
	case statusCode >= 400:
		return s.error(text)
	default:
		return text
	}
}

func (s humanTextStyle) style(tokenName, text string) string {
	if !s.color {
		return text
	}
	return output.StyleText(tokenName, text)
}

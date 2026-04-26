package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"golang.org/x/term"
)

func readPromptValue(prompt string, src io.Reader, stderr io.Writer, hidden bool) (string, error) {
	fmt.Fprint(stderr, prompt)

	if hidden {
		if f, ok := src.(*os.File); ok && output.IsTerminalReader(src) {
			value, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(stderr)
			return string(value), err
		}
	}

	value, err := readPromptLine(src)
	if err == nil {
		return strings.TrimRight(value, "\r\n"), nil
	}
	if err != io.EOF {
		return "", err
	}
	if value != "" {
		return strings.TrimRight(value, "\r\n"), nil
	}
	if hidden {
		return "", fmt.Errorf("unexpected EOF reading password")
	}
	return "", fmt.Errorf("unexpected EOF reading prompt")
}

func readPromptLine(src io.Reader) (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			b.WriteByte(buf[0])
			if buf[0] == '\n' {
				return b.String(), nil
			}
		}
		if err != nil {
			if err == io.EOF && b.Len() > 0 {
				return b.String(), io.EOF
			}
			return b.String(), err
		}
	}
}

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func readPromptValue(prompt string, src io.Reader, stderr io.Writer, hidden bool) (string, error) {
	fmt.Fprint(stderr, prompt)

	if hidden {
		if f, ok := src.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			value, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(stderr)
			return string(value), err
		}
	}

	scanner := bufio.NewScanner(src)
	if scanner.Scan() {
		return strings.TrimRight(scanner.Text(), "\r\n"), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if hidden {
		return "", fmt.Errorf("unexpected EOF reading password")
	}
	return "", fmt.Errorf("unexpected EOF reading prompt")
}

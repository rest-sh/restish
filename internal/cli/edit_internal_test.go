package cli

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommandLinePreservesEmptyQuotedArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "double quotes",
			in:   `emacsclient -a "" -c`,
			want: []string{"emacsclient", "-a", "", "-c"},
		},
		{
			name: "single quotes",
			in:   `cmd '' tail`,
			want: []string{"cmd", "", "tail"},
		},
		{
			name: "mixed quoted and unquoted",
			in:   `cmd prefix""suffix ""`,
			want: []string{"cmd", "prefixsuffix", ""},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := splitCommandLine(tt.in)
			if err != nil {
				t.Fatalf("splitCommandLine(%q): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitCommandLine(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// TestConfirmEditEOFReturnsFalse verifies that piped or closed stdin treats
// an EOF (or empty input) as "no", preventing accidental destructive confirms.
func TestConfirmEditEOFReturnsFalse(t *testing.T) {
	c := &CLI{
		Stdin:  io.NopCloser(strings.NewReader("")),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	ok, err := c.confirmEdit()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false (no) on EOF stdin, got true")
	}
}

// TestConfirmEditYesInputReturnsTrue verifies that "y\n" on non-TTY stdin
// still returns true.
func TestConfirmEditYesInputReturnsTrue(t *testing.T) {
	c := &CLI{
		Stdin:  strings.NewReader("y\n"),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	ok, err := c.confirmEdit()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for 'y' input, got false")
	}
}

// TestConfirmEditNoInputReturnsFalse verifies that "n\n" input returns false.
func TestConfirmEditNoInputReturnsFalse(t *testing.T) {
	c := &CLI{
		Stdin:  strings.NewReader("n\n"),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	ok, err := c.confirmEdit()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for 'n' input, got true")
	}
}

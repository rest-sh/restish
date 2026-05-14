package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/output"
)

func TestColorEnabled_NOCOLOREnv(t *testing.T) {
	t.Setenv("NOCOLOR", "1")
	t.Setenv("COLOR", "")
	if output.ColorEnabled(&bytes.Buffer{}) {
		t.Error("expected color off when NOCOLOR is set")
	}
}

func TestColorEnabled_NO_COLOREnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLOR", "")
	if output.ColorEnabled(&bytes.Buffer{}) {
		t.Error("expected color off when NO_COLOR is set")
	}
}

func TestColorEnabled_COLOREnv(t *testing.T) {
	t.Setenv("NOCOLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLOR", "1")
	// A bytes.Buffer is not a TTY, but COLOR forces it on.
	if !output.ColorEnabled(&bytes.Buffer{}) {
		t.Error("expected color on when COLOR is set")
	}
}

func TestColorEnabled_NonTTYDefault(t *testing.T) {
	t.Setenv("NOCOLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLOR", "")
	if output.ColorEnabled(&bytes.Buffer{}) {
		t.Error("expected color off for non-TTY writer with no env overrides")
	}
}

func TestIsTerminal_Buffer(t *testing.T) {
	if output.IsTerminal(&bytes.Buffer{}) {
		t.Error("expected false for bytes.Buffer")
	}
}

// TestAutoFormatter_WithColor exercises the chroma highlight path.
// We can't easily verify the exact ANSI codes, so we check that the body
// content is still present and the call doesn't error.
func TestAutoFormatter_WithColor(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    map[string]any{"colored": true},
	}

	var buf bytes.Buffer
	f := &output.AutoFormatter{}
	if err := f.Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error with color enabled: %v", err)
	}

	// Strip ANSI escape sequences (ESC [ ... m) for the content check.
	stripped := stripANSI(buf.String())

	if strings.Contains(stripped, "HTTP/1.1") {
		t.Errorf("auto formatter included status in colored output: %q", stripped)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stripped)), new(any)); err != nil {
		t.Errorf("body output not valid JSON after stripping ANSI: %v\nbody: %s", err, stripped)
	}
}

// stripANSI removes ANSI CSI escape sequences (e.g. color codes) from s.
// Simple implementation sufficient for testing purposes.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm' (or any final byte in 0x40–0x7E range).
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
				i++
			}
			i++ // skip the final byte
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

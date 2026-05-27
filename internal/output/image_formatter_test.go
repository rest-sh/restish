package output_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/output"
)

// makePNG creates a minimal valid PNG with the given dimensions and fills it
// with a solid colour.
func makePNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("makePNG: %v", err)
	}
	return buf.Bytes()
}

func imageResp(raw []byte) *output.Response {
	return &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Headers: map[string][]string{
			"Content-Type": {"image/png"},
		},
		Body: string(raw), // content registry returns string for unknown types
		Raw:  raw,
	}
}

// --- ImageFormatter: non-color (non-TTY) ---

func TestImageFormatter_NonColor_WritesRawBytes(t *testing.T) {
	data := makePNG(t, 4, 4, color.RGBA{255, 0, 0, 255})
	resp := imageResp(data)

	var buf bytes.Buffer
	f := output.DefaultFormatters()["image"]
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("expected raw image bytes on non-TTY")
	}
}

func TestImageFormatter_NonColor_EmptyBody(t *testing.T) {
	resp := &output.Response{Headers: map[string][]string{}}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil Raw, got %d bytes", buf.Len())
	}
}

// clearGraphicsEnv forces the half-block path by clearing all env vars that
// would select a richer terminal graphics protocol.
func clearGraphicsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TERM_PROGRAM", "")
}

// --- ImageFormatter: half-block (color=true, no special TERM env) ---

func TestImageFormatter_HalfBlock_ProducesOutput(t *testing.T) {
	clearGraphicsEnv(t)

	// 8×8 red PNG: should produce half-block output with ANSI sequences.
	data := makePNG(t, 8, 8, color.RGBA{255, 0, 0, 255})
	resp := imageResp(data)

	var buf bytes.Buffer
	f := output.DefaultFormatters()["image"]
	if err := f.Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if len(got) == 0 {
		t.Fatal("expected non-empty output for half-block rendering")
	}
	// Should contain the half-block character.
	if !strings.Contains(got, "▀") {
		t.Errorf("expected ▀ in half-block output; got:\n%s", got)
	}
	// Should contain ANSI reset at end of each row.
	if !strings.Contains(got, "\x1b[0m") {
		t.Errorf("expected ANSI reset sequence in output")
	}
}

func TestImageFormatter_HalfBlock_InvalidImage_FallsBackToRaw(t *testing.T) {
	clearGraphicsEnv(t)

	data := []byte("not an image")
	resp := &output.Response{
		Headers: map[string][]string{"Content-Type": {"image/png"}},
		Raw:     data,
	}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("expected raw bytes fallback for invalid image")
	}
}

// --- renderKitty via TERM env ---

func TestImageFormatter_Kitty_EscapeSequences(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-kitty")

	data := makePNG(t, 4, 4, color.RGBA{0, 255, 0, 255})
	resp := imageResp(data)

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	// APC opener.
	if !strings.Contains(got, "\x1b_G") {
		t.Errorf("expected Kitty APC sequence (ESC_G) in output")
	}
	// APC closer.
	if !strings.Contains(got, "\x1b\\") {
		t.Errorf("expected Kitty APC terminator (ESC\\) in output")
	}
	// a=T, f=100.
	if !strings.Contains(got, "a=T") || !strings.Contains(got, "f=100") {
		t.Errorf("expected Kitty a=T,f=100 parameters in output; got: %q", got[:min(80, len(got))])
	}
}

// --- renderITerm2 via TERM_PROGRAM env ---

func TestImageFormatter_ITerm2_EscapeSequences(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	// Ensure KITTY env is unset so Kitty doesn't win the protocol check.
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "")

	data := makePNG(t, 4, 4, color.RGBA{0, 0, 255, 255})
	resp := imageResp(data)

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	// OSC 1337 opener.
	if !strings.Contains(got, "\x1b]1337;File=") {
		t.Errorf("expected iTerm2 OSC 1337 sequence in output; got: %q", got[:min(80, len(got))])
	}
	// BEL terminator.
	if !strings.Contains(got, "\a") {
		t.Errorf("expected BEL (\\a) terminator in iTerm2 output")
	}
}

func TestImageFormatter_WezTerm_EscapeSequences(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "")

	data := makePNG(t, 2, 2, color.RGBA{128, 128, 128, 255})
	resp := imageResp(data)

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b]1337;File=") {
		t.Errorf("expected iTerm2 protocol for WezTerm")
	}
}

// --- RSH_IMAGE_PROTOCOL env var override ---

func TestImageFormatter_RSH_IMAGE_PROTOCOL_Kitty(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("RSH_IMAGE_PROTOCOL", "kitty")

	data := makePNG(t, 4, 4, color.RGBA{255, 255, 0, 255})
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, imageResp(data), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b_G") {
		t.Errorf("RSH_IMAGE_PROTOCOL=kitty should produce Kitty APC sequences")
	}
}

func TestImageFormatter_RSH_IMAGE_PROTOCOL_ITerm2(t *testing.T) {
	clearGraphicsEnv(t)
	t.Setenv("RSH_IMAGE_PROTOCOL", "iterm2")

	data := makePNG(t, 4, 4, color.RGBA{0, 255, 255, 255})
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, imageResp(data), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b]1337;File=") {
		t.Errorf("RSH_IMAGE_PROTOCOL=iterm2 should produce OSC 1337 sequence")
	}
}

func TestImageFormatter_RSH_IMAGE_PROTOCOL_HalfBlock(t *testing.T) {
	// Even in a Kitty terminal, RSH_IMAGE_PROTOCOL=halfblock should override.
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("RSH_IMAGE_PROTOCOL", "halfblock")

	data := makePNG(t, 4, 4, color.RGBA{255, 0, 255, 255})
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["image"].Format(&buf, imageResp(data), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "\x1b_G") {
		t.Errorf("RSH_IMAGE_PROTOCOL=halfblock should not produce Kitty sequences")
	}
	if !strings.Contains(got, "▀") {
		t.Errorf("RSH_IMAGE_PROTOCOL=halfblock should produce half-block characters")
	}
}

// --- DefaultFormatters includes "image" ---

func TestDefaultFormatters_IncludesImage(t *testing.T) {
	fmts := output.DefaultFormatters()
	if _, ok := fmts["image"]; !ok {
		t.Error("DefaultFormatters() missing \"image\" entry")
	}
}

// min is defined here for Go versions before 1.21 that lack the builtin.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

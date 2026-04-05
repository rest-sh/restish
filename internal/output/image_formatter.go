package output

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// imageProtocol identifies which terminal graphics protocol to use.
type imageProtocol int

const (
	protoHalfBlock imageProtocol = iota // Unicode half-block (▀) with 24-bit ANSI colour
	protoKitty                          // Kitty Graphics Protocol (APC sequences)
	protoITerm2                         // iTerm2 Inline Images (OSC 1337)
)

// ImageFormatter renders image/* responses inline on a TTY using the best
// available terminal graphics protocol.
//
// Protocol is selected in order:
//  1. $RSH_IMAGE_PROTOCOL env var (kitty | iterm2 | halfblock)
//  2. Terminal env vars ($KITTY_WINDOW_ID / $TERM=xterm-kitty → Kitty;
//     $TERM_PROGRAM ∈ {iTerm.app, WezTerm, Hyper} → iTerm2)
//  3. Unicode half-block fallback
//
// When color is false (non-TTY), raw bytes are written unchanged.
type ImageFormatter struct{}

// Format renders resp.Raw as a terminal image.
func (f *ImageFormatter) Format(w io.Writer, resp *Response, color bool) error {
	if len(resp.Raw) == 0 {
		return nil
	}
	// Non-TTY: behave like RawFormatter so piping/redirecting still works.
	if !color {
		_, err := w.Write(resp.Raw)
		return err
	}

	switch detectImageProtocol() {
	case protoKitty:
		return renderKitty(w, resp.Raw)
	case protoITerm2:
		return renderITerm2(w, resp.Raw, resp.Headers["Content-Type"])
	default:
		img, _, err := image.Decode(bytes.NewReader(resp.Raw))
		if err != nil {
			// Unknown format: fall back to raw bytes so the caller can redirect.
			_, err = w.Write(resp.Raw)
			return err
		}
		return renderHalfBlock(w, img, terminalWidth(w))
	}
}

// detectImageProtocol inspects $RSH_IMAGE_PROTOCOL first, then terminal env
// vars. No terminal queries are performed.
func detectImageProtocol() imageProtocol {
	switch strings.ToLower(os.Getenv("RSH_IMAGE_PROTOCOL")) {
	case "kitty":
		return protoKitty
	case "iterm2":
		return protoITerm2
	case "halfblock":
		return protoHalfBlock
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" {
		return protoKitty
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "Hyper":
		return protoITerm2
	}
	return protoHalfBlock
}

// renderKitty transmits image data using the Kitty Graphics Protocol. The raw
// bytes are base64-encoded and sent in ≤4096-byte chunks inside APC sequences.
// f=100 tells Kitty to decode the payload itself (PNG/JPEG/GIF pass through as-is).
// q=2 suppresses per-chunk acknowledgement responses from the terminal.
func renderKitty(w io.Writer, data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	const chunkMax = 4096
	total := len(encoded)

	for i := 0; i < total; i += chunkMax {
		end := i + chunkMax
		if end > total {
			end = total
		}
		chunk := encoded[i:end]

		// m=1 → more chunks follow; m=0 → last (or only) chunk.
		m := 1
		if end >= total {
			m = 0
		}

		var seq string
		if i == 0 {
			seq = fmt.Sprintf("\x1b_Ga=T,f=100,q=2,m=%d;%s\x1b\\", m, chunk)
		} else {
			seq = fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", m, chunk)
		}
		if _, err := io.WriteString(w, seq); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

// renderITerm2 transmits image data using the iTerm2 Inline Images Protocol
// (OSC 1337). The entire payload is base64-encoded into a single sequence.
// WezTerm and Hyper also support this protocol.
//
// name= and size= are set so iTerm2's security dialog shows a meaningful
// filename (e.g. "image.jpeg") rather than "Unnamed file of size 0 bytes".
func renderITerm2(w io.Writer, data []byte, contentType string) error {
	// Derive a filename from the MIME subtype: "image/jpeg" → "image.jpeg".
	name := "image"
	if ct, _, _ := strings.Cut(contentType, ";"); ct != "" {
		if _, sub, ok := strings.Cut(strings.TrimSpace(ct), "/"); ok && sub != "" {
			name = "image." + sub
		}
	}
	nameB64 := base64.StdEncoding.EncodeToString([]byte(name))

	encoded := base64.StdEncoding.EncodeToString(data)
	_, err := fmt.Fprintf(w,
		"\x1b]1337;File=inline=1;name=%s;size=%d;width=auto;height=auto;preserveAspectRatio=1:%s\a\n",
		nameB64, len(data), encoded)
	return err
}

// renderHalfBlock renders img as Unicode half-block characters with 24-bit
// ANSI colour. Each character cell represents two vertically stacked pixels:
// the upper pixel maps to the foreground colour of ▀ (U+2580) and the lower
// pixel maps to its background colour. The image is scaled with
// nearest-neighbour interpolation to fit width terminal columns.
func renderHalfBlock(w io.Writer, img image.Image, width int) error {
	b := img.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW == 0 || srcH == 0 {
		return nil
	}

	// Cap at source width to avoid upscaling artefacts.
	renderW := width
	if renderW > srcW {
		renderW = srcW
	}

	scale := float64(renderW) / float64(srcW)
	renderH := int(float64(srcH) * scale)
	// Must be even so every row maps to exactly two pixel rows.
	if renderH%2 != 0 {
		renderH++
	}
	if renderH == 0 {
		renderH = 2
	}

	scaled := resizeNearest(img, renderW, renderH)
	rb := scaled.Bounds()

	var sb strings.Builder
	for y := rb.Min.Y; y < rb.Max.Y; y += 2 {
		for x := rb.Min.X; x < rb.Max.X; x++ {
			r1, g1, b1, _ := scaled.At(x, y).RGBA()
			ur, ug, ub := uint8(r1>>8), uint8(g1>>8), uint8(b1>>8)

			var lr, lg, lb uint8
			if y+1 < rb.Max.Y {
				r2, g2, b2, _ := scaled.At(x, y+1).RGBA()
				lr, lg, lb = uint8(r2>>8), uint8(g2>>8), uint8(b2>>8)
			}

			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				ur, ug, ub, lr, lg, lb)
		}
		sb.WriteString("\x1b[0m\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// resizeNearest scales src to (w, h) using nearest-neighbour interpolation.
func resizeNearest(src image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	srcW := sb.Dx()
	srcH := sb.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := x*srcW/w + sb.Min.X
			srcY := y*srcH/h + sb.Min.Y
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// terminalWidth returns the number of columns in the terminal attached to w,
// falling back to $COLUMNS, then 80.
func terminalWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			return width
		}
	}
	if col := os.Getenv("COLUMNS"); col != "" {
		if n, err := strconv.Atoi(col); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

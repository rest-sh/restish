# Image Rendering

## Summary

Restish v2 renders `image/*` responses directly in the terminal when stdout is a
TTY. The best available protocol is chosen automatically based on environment
variables. Fallback to Unicode half-block rendering ensures something useful
appears on any colour-capable terminal.

## Problem

REST APIs increasingly return image payloads alongside JSON: generated avatars,
QR codes, captchas, thumbnails, map tiles. When a user runs

```
restish get https://api.example.com/users/42/avatar
```

they want to see the image in-place, not binary noise or a base64 dump. On
non-TTY (pipe or redirect), raw bytes should still flow unchanged so `> out.png`
just works.

## Design

### Core, not a plugin

Image rendering ships in the core binary for three reasons:

1. **Content-type-aware dispatch requires a core change anyway.** The existing
   `Select()` function picks a formatter from `{name, tty}` only. Supporting
   automatic image rendering requires consulting the response `Content-Type` at
   selection time. That logic lives inside the CLI, not the plugin layer.

2. **No Cgo and no heavy dependencies.** PNG, JPEG, and GIF decoders are in the
   Go standard library. The terminal protocols (Kitty, iTerm2) are base64 +
   escape sequences. The half-block fallback needs only integer arithmetic and
   ANSI codes. Nothing here requires native libraries.

3. **It should just work.** Downloading an avatar and having it render without
   installing any plugin is the right default experience.

Plugin formatters remain the right hook for *additional* media types (video,
SVG rasterization, custom image codecs) because those can't reasonably ship in
every binary. See the extensibility section below.

### Content-type-aware formatter dispatch

`formatResponse` in `internal/cli/http.go` checks the response `Content-Type`
before calling `Select()`. When stdout is a TTY, no explicit `-o` format was
given, and no filter other than the default `@` is active, it routes `image/*`
responses directly to the registered `"image"` formatter, bypassing the usual
`readable` default:

```
TTY + fmtName=="" + filterExpr=="@" + Content-Type starts with "image/"
    → fmts["image"].Format(...)
```

Non-TTY output is unaffected: `Select()` returns `RawFormatter` as before, and
binary bytes flow through unchanged.

Explicit `-o image` forces image rendering regardless of content type.

### Protocol auto-detection

The formatter inspects environment variables at render time, no terminal queries:

| Priority | Protocol                    | Condition                                       |
|----------|-----------------------------|-------------------------------------------------|
| 1        | `$RSH_IMAGE_PROTOCOL`       | `kitty`, `iterm2`, or `halfblock`               |
| 2        | Kitty Graphics Protocol     | `$KITTY_WINDOW_ID` set, or `$TERM=xterm-kitty`  |
| 3        | iTerm2 Inline Images (OSC 1337) | `$TERM_PROGRAM` is `iTerm.app`, `WezTerm`, or `Hyper` |
| 4        | Unicode half-block          | always available (fallback)                     |

**Kitty** (`ESC_G…ESC\`): image data is base64-encoded and sent in 4096-byte
chunks using APC sequences with `f=100` (PNG/JPEG/GIF forwarded as-is; the
terminal decodes). `q=2` suppresses acknowledgement noise.

**iTerm2/WezTerm** (`ESC]1337;File=…BEL`): the entire raw payload is
base64-encoded and embedded in a single OSC 1337 sequence. Width and height are
left as `auto` so the terminal scales to fit.

**Half-block**: the image is decoded with `image.Decode` (PNG/JPEG/GIF via
standard library), scaled with nearest-neighbour to fit the terminal width
(capped at the original image width to avoid upscaling), then rendered one
character cell per two pixel rows using `▀` (U+2580). Each cell's foreground
colour is the upper pixel and background colour is the lower pixel, both using
ANSI 24-bit sequences. Terminal width comes from `golang.org/x/term` (already a
dependency) with a `$COLUMNS` fallback and a default of 80.

### Supported image formats

| Format | Kitty/iTerm2 | Half-block |
|--------|-------------|------------|
| PNG    | yes          | yes (stdlib `image/png`)  |
| JPEG   | yes          | yes (stdlib `image/jpeg`) |
| GIF    | yes          | yes (stdlib `image/gif`)  |
| WebP, AVIF, HEIC, … | yes (terminal decodes) | no — decodes as raw bytes fallback |

For Kitty and iTerm2, unsupported formats are forwarded anyway; whether they
render depends on the terminal. For the half-block path, `image.Decode` returns
an error for unknown formats and the formatter falls back to writing the raw
bytes, which is the same as `RawFormatter`.

### Plugin extensibility for future media types

The `"image"` formatter is registered in `DefaultFormatters()` like any other.
A plugin can shadow it by declaring `"image"` in its `formatter_names` manifest
field — the last-registered entry wins, which is the plugin's.

For entirely new media type families (video, audio, documents), the recommended
path is:

1. Declare the media type pattern in a new manifest field (e.g.
   `media_types: ["video/*"]`) — this is not yet implemented; it is reserved for
   a future plugin protocol extension.
2. Register a formatter name (e.g. `"video"`) and teach `formatResponse` to
   dispatch on `video/*` content types.

Until that extension lands, users can force plugin rendering with `-o myplugin`
for any content type.

## Alternatives Considered

### Implement via a hook plugin formatter

A hook plugin can already register formatter names, but it cannot participate in
content-type-aware auto-dispatch without changes to `formatResponse`. Those same
changes are needed for the built-in path. Since the protocols are pure escape
sequences and the decoders are stdlib, the complexity savings from out-of-process
rendering are negative — more IPC, no less code.

### Use `github.com/kenshaw/rasterm` or similar

`rasterm` is a clean pure-Go library that covers all four protocols. It would
reduce the implementation by ~150 lines. However, adding a new module dependency
for ~150 lines of straightforward escape-sequence generation did not seem worth
the tradeoff given we already have `golang.org/x/term` for terminal sizing.

### Query the terminal for capability (DA2 / `XTSMGRAPHICS`)

Active terminal probing would give accurate capability detection, especially for
Sixel support. It requires setting raw mode, writing an escape sequence, reading
the response with a timeout, and restoring the terminal — all before the first
HTTP request. The env-var approach covers the most common terminals (Kitty,
iTerm2, WezTerm) without any timing risk or complexity, and the half-block
fallback is acceptable on everything else.

### Sixel support

Sixel would extend coverage to terminals like `mlterm` and `foot`. The protocol
requires palette quantisation and run-length encoding, which is non-trivial to
implement correctly. Given that Kitty and iTerm2 cover most developer terminals
today, Sixel is deferred. A future plugin could add it.

## Notes

Implementation lives in:

- `internal/output/image_formatter.go` — `ImageFormatter`, protocol detection,
  Kitty/iTerm2/half-block renderers
- `internal/output/format.go` — `"image"` entry in `DefaultFormatters()`
- `internal/cli/http.go` — content-type-aware dispatch in `formatResponse()`
- `internal/output/image_formatter_test.go` — unit tests

The `image.Decode` call registers decoders via blank imports of
`image/png`, `image/jpeg`, and `image/gif` in `image_formatter.go`. These add
negligible binary size and are already pulled transitively by other packages in
the module.

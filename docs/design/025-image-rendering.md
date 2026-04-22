# Image Rendering

## Summary

Restish v2 renders `image/*` responses directly in the terminal when stdout is a
TTY. The best available protocol is chosen automatically based on environment
variables and terminal heuristics. Fallback to Unicode half-block rendering
ensures something useful appears on any colour-capable terminal.

On non-TTY output, raw bytes still flow unchanged so `> out.png` works exactly
as users expect.

## Goals

- make common image responses immediately useful in interactive terminals
- preserve exact raw bytes for non-interactive output
- choose the best available terminal image protocol without complex probing
- keep image rendering inside the core output system rather than making users
  install a plugin for basic behavior

## Non-Goals

- rasterizing every possible media type in the core binary
- probing terminals with a complicated handshake before every request
- forcing plugin formatters to solve core content-type dispatch problems

## Why It Lives In Core

Image rendering ships in the core binary for three reasons:

1. content-type-aware dispatch requires a core decision anyway
2. the required codecs and protocols are lightweight
3. it should "just work" for a common API experience

Plugin formatters remain the right hook for:

- additional media families
- richer image codecs
- alternate rendering styles

But the default experience for `image/*` should not depend on optional
installation.

## Dispatch Rules

Image rendering is a content-type-aware dispatch decision.

When:

- stdout is a TTY
- no explicit `-o` format was given, and
- no non-default filter is active that changes the logical output away from the
  original image payload

then `image/*` responses should route to the registered `image` formatter
instead of the usual `readable` default.

Non-TTY output is unaffected: raw bytes flow through unchanged.

Explicit `-o image` forces image rendering regardless of content type.

## Protocol Selection

The formatter inspects environment variables at render time rather than actively
querying the terminal.

Priority order:

1. `$RSH_IMAGE_PROTOCOL`
2. Kitty graphics protocol detection
3. iTerm2-style inline image detection
4. half-block fallback

This keeps first-byte latency low and avoids fragile terminal probing logic.

## Supported Protocols

### Kitty Graphics Protocol

Selected when:

- `$RSH_IMAGE_PROTOCOL=kitty`, or
- `$KITTY_WINDOW_ID` is set, or
- `$TERM=xterm-kitty`

Image data is base64-encoded and sent in chunks using Kitty APC sequences.

### iTerm2 Inline Images

Selected when:

- `$RSH_IMAGE_PROTOCOL=iterm2`, or
- `$TERM_PROGRAM` indicates a compatible terminal such as `iTerm.app`,
  `WezTerm`, or `Hyper`

The raw payload is base64-encoded and embedded in an OSC 1337 sequence.

### Half-Block Fallback

Selected when:

- `$RSH_IMAGE_PROTOCOL=halfblock`, or
- no richer protocol is detected

The image is decoded locally, scaled to terminal width, and rendered using the
Unicode upper half block (`▀`) with ANSI 24-bit foreground/background colors.

This is the universal fallback path when inline image protocols are unavailable.

## Image Format Support

For Kitty and iTerm2-style protocols, the formatter can forward common image
payloads as-is and let the terminal decode them.

For the half-block path, Restish needs local decode support. The core design
expects at least:

- PNG
- JPEG
- GIF

If local decoding fails in the half-block path, the formatter should fall back
gracefully rather than pretending it rendered the image successfully.

## Width And Scaling

Half-block rendering needs a terminal width estimate.

The design order is:

1. terminal size query through the TTY helper
2. `$COLUMNS`
3. default width such as 80 columns

The formatter should avoid upscaling tiny images unnecessarily. Its job is to
render usefully, not to invent artificial enlargement.

## Interaction With Output Planning

Image rendering is part of the output system, not a separate feature.

Important planner rules:

- TTY + original `image/*` payload -> image formatter is eligible
- redirected output -> raw bytes win
- filtered or transformed image responses no longer count as "original image
  payload" for auto-dispatch

This keeps image rendering aligned with the same "raw versus normalized value"
distinction used elsewhere in the output model.

## Error And Fallback Behavior

If the chosen image protocol path fails, Restish should degrade predictably:

- protocol override asks for a supported path -> try that path
- if local half-block decode fails -> fall back toward raw behavior rather than
  emitting corrupted bytes to the terminal as if they were an image
- explicit `-o image` with unusable image data should surface an error rather
  than silently pretending success

The design goal is graceful fallback, not silent corruption.

## Examples

Interactive image rendering:

```bash
restish get https://api.example.com/users/42/avatar
```

Raw file download:

```bash
restish get https://api.example.com/users/42/avatar > avatar.png
```

Force half-block rendering:

```bash
RSH_IMAGE_PROTOCOL=halfblock restish get https://api.example.com/users/42/avatar
```

## Alternatives Considered

### Implement Via A Hook Plugin Formatter

A plugin formatter can render image bytes, but the core still has to decide when
to auto-dispatch based on content type. Since that dispatch logic already lives
in the core, basic image rendering belongs there too.

### Query The Terminal For Capability

Would be more precise in some cases, but adds timing, raw-mode, and complexity
cost that is hard to justify given the fallback path.

### Sixel Support In Core

Useful for some terminals, but deferred because it increases protocol and
rendering complexity substantially. A future plugin or future core expansion can
add it deliberately.

## Relationship To Other Designs

- Design 009 defines the normalized/raw output split image rendering depends on.
- Design 017 defines terminal-facing CLI behavior and diagnostics.
- Design 028 defines output-family planning and auto-dispatch interactions.

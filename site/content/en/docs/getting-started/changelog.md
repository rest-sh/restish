---
title: Changelog
linkTitle: Changelog
weight: 50
description: What changed in Restish v2 compared to v1 — new features, breaking changes, and migration notes.
---

This page summarizes the major changes in Restish v2. For a migration checklist
start at [Upgrade From v1](../upgrade-from-v1/).

---

## New In v2

### Configuration

- **Single config file** — `restish.json` replaces the separate `apis.json` and
  `config.json` from v1. Config is stored under the platform default config
  directory.
- **`restish api add`** — one-shot API registration with shorthand expressions,
  no editor required.
- **`restish api set`** — programmatic shorthand updates to any config field
  (`set`, `append`, `delete` via `undefined`).
- **JSONC support** — comments and trailing commas are preserved through all
  programmatic config edits.
- **Auto-migration** — v1 config is automatically migrated on first run with a
  `.bak.v1` backup.

### Auth

- **Device-code OAuth** — RFC 8628 device authorization flow added; falls back
  to manual code entry when no browser is available.
- **Refresh token preservation** — if a token refresh response omits a new
  refresh token, the cached token is kept instead of being overwritten.
- **Secure OAuth callback** — stray browser preflight requests (favicon, extra
  paths) no longer abort an in-flight OAuth callback.
- **Unified Prompter interface** — plugins and the CLI share the same prompt /
  secret / confirm surface for consistent interactive auth flows.

### Requests And HTTP

- **HEAD and OPTIONS** — added as generic HTTP verbs alongside `get`, `post`,
  `put`, `patch`, `delete`.
- **`--rsh-server` flag** — override the base server for a single request
  without editing config.
- **SSRF protection** — Link-header-supplied spec URLs are now same-origin
  guarded; cross-origin follows require `--allow-cross-origin-spec`.
- **Rate-limit retry** — respects `Retry-After` headers.

### Output And Filtering

- **Table formatter** — `--rsh-output table` renders arrays of objects as a
  Unicode box-drawing table with optional `--rsh-sort-by` and column
  selection.
- **Image rendering** — inline image display in Kitty, iTerm2, and halfblock
  terminals via `RSH_IMAGE_PROTOCOL` or auto-detection.
- **`--rsh-raw` / `-r`** — raw scalar output mode; strings are printed without
  quotes, arrays one value per line.
- **jq filter support** — filter expressions not starting with a shorthand root
  are passed to jq automatically.

### Plugins

- **Hook timeout configuration** — per-hook timeouts via `hook_timeouts` in the
  plugin manifest; defaults are 30 s general, 5 min for `auth`.
- **`plugin debug`** — decode and inspect raw CBOR plugin traffic from the
  terminal.
- **`allowed_plugins` allowlist** — restrict which plugins are loaded at startup.
- **TLS signer plugin type** — out-of-process client certificate signing for
  hardware keys and HSMs.

### Developer Experience

- **Shell setup for fish** — `restish setup fish` adds a wrapper function.
- **Built-in name collision guard** — registering an API with a name that
  conflicts with a built-in command (`api`, `get`, `post`, etc.) now errors.
- **Verbose TLS details** — `-vv` dumps TLS version, cipher suite, and peer
  certificate chain.

---

## Breaking Changes From v1

### Config Shape

| v1 | v2 |
|----|----|
| `apis.json` + `config.json` | `restish.json` |
| `auth.name` | `auth.type` |
| profile `base` | profile `base_url` |
| API `base` | API `base_url` |
| `restish api configure <name>` | `restish api configure <name> <url>` |

### Commands And Flags

- v0-style slug aliases are removed permanently; use the current operation name
  or add `x-cli-aliases` to the spec.
- `restish setup fish` now writes a function wrapper rather than a `noglob`
  alias (fish does not support `noglob`).

### Plugins

- The v2 plugin wire protocol (CBOR over stdin/stdout) is incompatible with v1.
  Rebuild or replace v1 plugins for v2.
- Plugin manifests now use `restish_api_version: 2` as the API version field.

### Module Path

The Go module path changed from `github.com/danielgtaylor/restish/v2` to
`github.com/rest-sh/restish/v2`. Go SDK users must update their import paths.

---

## Deprecations

- None for v2.0. If you are using `restish` as a Go library (embedded), note
  that the embedded-library surface is secondary; the CLI is the primary
  supported interface.

---

## Migration Notes

See [Upgrade From v1](../upgrade-from-v1/) for the step-by-step migration
checklist, command mapping table, and plugin migration guidance.

If you hit a bug or regression not listed here, please report it at
https://github.com/rest-sh/restish/issues.

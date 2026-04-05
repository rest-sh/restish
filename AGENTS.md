# AGENTS.md

## Commands

```bash
# Build the main CLI
go build ./cmd/restish

# Build plugins
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/cli/...

# Update golden files for output formatter regression tests
go test -update ./internal/output/...

# Format code
go fmt ./...
```

## Architecture

Restish v2 is a CLI for interacting with REST-ish HTTP APIs. It generates commands from OpenAPI 3.x specs and supports generic HTTP verbs, content negotiation, authentication, pagination, caching, filtering, and plugins.

### Central `CLI` Struct

The core design is a `CLI` struct in `internal/cli/cli.go` that owns all state — I/O handles, config, content registry, spec loaders, link parsers, formatters, and plugins. This replaces v1's global state, making the CLI testable and embeddable. Tests instantiate `CLI` directly with `bytes.Buffer` for I/O and `httptest.Server` for HTTP.

**Entry point**: `cmd/restish/main.go` creates a `CLI` and calls `Run(os.Args)`.

### Key Internal Packages

| Package                | Role                                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------- |
| `internal/cli/`        | Root command, global flags, API command generation from specs, plugin orchestration               |
| `internal/config/`     | JSONC config (`~/.config/restish/restish.json`); no Viper, just `encoding/json` + `tidwall/jsonc` |
| `internal/spec/`       | OpenAPI spec discovery, loading via `libopenapi`, caching                                         |
| `internal/auth/`       | `Handler` interface; Basic, OAuth2 client credentials, OAuth2 authcode+PKCE implementations       |
| `internal/request/`    | HTTP transport, TLS, retries, RFC 7234 caching                                                    |
| `internal/output/`     | Response normalization, formatters (JSON, readable, table, gron, CBOR, raw)                       |
| `internal/content/`    | Registry for content types (JSON, YAML, CBOR, etc.) and encodings (gzip, brotli)                  |
| `internal/filter/`     | Response filtering via shorthand (`danielgtaylor/shorthand`) and jq (`itchyny/gojq`)              |
| `internal/hypermedia/` | Automatic pagination link parsers: RFC 5988, HAL, Siren, JSON:API, TSJ                            |
| `internal/input/`      | CLI shorthand parsing for structured request bodies                                               |
| `internal/plugin/`     | Plugin discovery, manifest loading, hook dispatch, TLS signer coordination                        |
| `plugin/`              | Public package with `WriteMessage`/`ReadMessage` CBOR framing helpers for plugin authors          |

### Plugin System

Plugins are executables named `restish-<name>` on PATH or in `~/.config/restish/plugins/`. Invoked with `--rsh-plugin-manifest` to declare capabilities. Transport is **length-prefixed CBOR** (4-byte big-endian length + CBOR payload) over stdin/stdout.

Three plugin types:

**Hook plugins** (short-lived): Restish writes one CBOR message, reads one reply, plugin exits. Hooks: `auth`, `request-middleware`, `response-middleware`, `loader`, `formatter`. 30-second timeout. Implementation: `internal/plugin/hook.go`.

**Command plugins** (long-lived): Plugin declares `command` hook; Restish discovers subcommands via `--rsh-plugin-commands`. Bidirectional CBOR conversation where plugin can delegate HTTP calls, formatting, and I/O back to Restish. Plugin sends `done` to exit. Implementation: `internal/cli/command_plugin.go`.

**TLS signer plugins** (persistent): Configured per-profile with `tls_signer`. Plugin provides leaf cert on `ready`, then handles `Sign(...)` delegations during TLS handshakes for hardware-backed mTLS. Implementation: `internal/plugin/tls_signer.go`.

### Handler Interfaces

Key extension points registered on the `CLI` struct:

- **Auth**: `Handler` — `Parameters() []Param`, `OnRequest(req, params) error`
- **Formatters**: `Format(w io.Writer, resp *Response, color bool) error`
- **Spec loaders**: `Detect(contentType, body) bool`, `Load(body) (*APISpec, error)`, `Priority() int`
- **Link parsers**: `Parse(body []byte) ([]Link, error)`

### Testing Patterns

- Unit tests alongside source files (e.g., `auth_test.go` next to `auth.go`)
- Integration tests use `CLI` struct with `bytes.Buffer` I/O and `httptest.Server`
- Golden file tests for output formatters in `testdata/*.golden`; regenerate with `-update` flag
- Table-driven tests throughout

### Design Documentation

`docs/design/` contains architectural design documents covering each subsystem in detail. Read these before making changes to core systems.

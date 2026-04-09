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

| Package                | Role                                                                                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/cli/`        | Root command, global flags, API command generation from specs, plugin orchestration                                                               |
| `internal/config/`     | JSONC config (`~/.config/restish/restish.json`); no Viper, just `encoding/json` + `tidwall/jsonc`                                                 |
| `internal/spec/`       | OpenAPI spec discovery, loading via `libopenapi`, caching                                                                                         |
| `internal/auth/`       | `Handler` interface; Basic, OAuth2 client credentials, OAuth2 authcode+PKCE implementations                                                       |
| `internal/request/`    | HTTP transport, TLS, retries, RFC 7234 caching                                                                                                    |
| `internal/output/`     | Response normalization, formatters (JSON, readable, table, gron, CBOR, raw)                                                                       |
| `internal/content/`    | Registry for content types (JSON, YAML, CBOR, etc.) and encodings (gzip, brotli)                                                                  |
| `internal/filter/`     | Response filtering via shorthand (`danielgtaylor/shorthand`) and jq (`itchyny/gojq`)                                                              |
| `internal/hypermedia/` | Automatic pagination link parsers: RFC 5988, HAL, Siren, JSON:API, TSJ                                                                            |
| `internal/input/`      | CLI shorthand parsing for structured request bodies                                                                                               |
| `internal/plugin/`     | Plugin discovery, manifest loading, hook dispatch, TLS signer coordination                                                                        |
| `plugin/`              | Public package with `WriteMessage`/`ReadMessage` (one-shot) and `NewDecoder`/`(*Decoder).ReadMessage` (streaming) CBOR helpers for plugin authors |

### Plugin System

Plugins are executables named `restish-<name>` on PATH or in `~/.config/restish/plugins/`. Invoked with `--rsh-plugin-manifest` to declare capabilities. Transport is **plain CBOR** (self-delimiting CBOR data items, no length prefix) over stdin/stdout.

Three plugin types:

**Hook plugins** (short-lived): Restish writes one CBOR data item to stdin, reads one reply from stdout, plugin exits. Hooks: `auth`, `request-middleware`, `response-middleware`, `loader`, `formatter`. 30-second timeout. Implementation: `internal/plugin/hook.go`.

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

## Common Pitfalls

### Subprocess lifecycle

Every `exec.Cmd` that has been `Start()`ed must be `Wait()`ed, or the process becomes a zombie. The standard pattern for error paths is a `cleanup` closure that closes stdin/stdout pipes, calls `Process.Kill()`, then `Wait()`. See `TLSCertificateFromPlugin` in `internal/plugin/tls_signer.go` for the canonical example.

### Goroutines that read from a subprocess pipe with a timeout

Spawning a goroutine to read from a pipe and then `select`-ing with a timeout leaves the goroutine leaked if the timeout fires — the goroutine stays blocked on the read forever. Fix: store the pipe as `io.ReadCloser`, close it in the timeout branch, then drain the result channel before returning. See `readTLSSignerMessage` in `internal/plugin/tls_signer.go`.

### Goroutines that block on `io.Reader` (especially stdin)

A goroutine reading from `c.Stdin` cannot be interrupted by closing an unrelated pipe. Use the two-goroutine pattern: an inner goroutine does the blocking `Read` and sends results to a buffered channel; an outer goroutine `select`s on that channel and a `done` channel. The outer goroutine exits immediately on `done`; the inner goroutine exits as soon as it can send to the channel (which drains the `done` case). See `streamPluginStdin` in `internal/cli/command_plugin.go`.

### Plugin API version field

`CurrentPluginAPIVersion` in `internal/plugin/plugin.go` must be incremented whenever the plugin wire protocol changes in a backward-incompatible way. `LoadManifest` will then warn when an older plugin is loaded.

### CBOR byte decoding

CBOR implementations may decode a byte string as `[]byte`, `string`, or `[]any` of integers depending on the decoder configuration. Always use `plugin.MsgBytes(v)` from `internal/plugin/bytes.go` rather than a direct type assertion.

### Config fields that are parsed but unused

`config.Load` uses `json.Decoder` with `DisallowUnknownFields()`. Adding a field to the config struct but not implementing its behaviour is an easy source of confusion: users set it and nothing happens. Either implement it or don't add the field.

### Concurrent writes to `bytes.Buffer` in tests

The test `CLI` uses `bytes.Buffer` for stdout and stderr. If a subprocess's stderr is wired to the same buffer that the test reads (via `proc.Stderr = cmd.ErrOrStderr()`), and the main goroutine also writes to the buffer, a data race results. Run `go test -race ./...` and check the output; the known pre-existing races are in `TestBulkPluginWorkflow` and `TestMCPRequiresAPIName`.

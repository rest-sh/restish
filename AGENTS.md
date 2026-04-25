# AGENTS.md

## Commands

```bash
# Build the main CLI
go build ./cmd/restish

# Build plugins
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/cli/...

# Update golden files for output formatter regression tests
go test -update ./internal/output/...
```

## Architecture

Restish is a CLI for interacting with REST-ish HTTP APIs. It generates commands from OpenAPI 3.x specs and supports generic HTTP verbs, content negotiation, authentication, pagination, caching, filtering, and plugins.

### Central `CLI` Struct

The core design is a `CLI` struct in `internal/cli/cli.go` that owns all state — I/O handles, config, content registry, spec loaders, link parsers, formatters, and plugins. Tests instantiate `CLI` directly with `bytes.Buffer` for I/O and `httptest.Server` for HTTP.

**Entry point**: `cmd/restish/main.go` creates a `CLI` and calls `Run(os.Args)`.

## Design Documentation

`docs/design/` contains architectural design documents covering each subsystem in detail. Read these before making changes to core systems. Before writing a significant new feature or change, write a design doc and get feedback. See `docs/design/README.md` for a list of design docs and what they cover.

## Documentation Site

`site/` contains the source for the documentation site at https://rest.sh/. This is user-facing documentation and should be updated with new features and changes.

## Skills

Invoke these skills when relevant:

- `rsh-review` for code review feedback on code changes
- `rsh-docs` for writing and maintaining documentation
- `rsh-simplify` for approved simplification and dead-code cleanup work

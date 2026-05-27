# Restish

Restish is a CLI for working with REST-ish HTTP APIs. It can make generic HTTP
requests, discover OpenAPI descriptions, generate API-aware commands, manage
profiles and auth, render structured output, follow pagination links, and run
plugins.

This branch contains the v2 codebase. The user-facing documentation source
lives in [`site/`](site/), and the architecture notes live in
[`docs/design/`](docs/design/).

## Install

Use Homebrew for the easiest managed install on macOS:

```bash
brew install rest-sh/tap/restish
restish --version
```

Check `restish --version` after installation. Users who need the legacy v1 line
can install `rest-sh/tap/restish@1`.

Build from this repository when developing Restish itself:

```bash
go build ./cmd/restish
./restish --help
./restish api.rest.sh/
```

Or use mise:

```bash
mise use -g restish@latest
restish --version
```

Install the latest tagged v2 release from source with Go:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

## Documentation

- [Getting Started](site/content/en/docs/getting-started/_index.md)
- [Install](site/content/en/docs/getting-started/install.md)
- [Tour](site/content/en/docs/getting-started/tour.md)
- [Upgrade From v1](site/content/en/docs/getting-started/upgrade-from-v1.md)
- [Plugin Docs](site/content/en/docs/plugins/_index.md)
- [Design Docs](docs/design/README.md)

## Development

Run the normal development loop with:

```bash
go test ./...
```

Build the CLI and bundled plugins with:

```bash
go build ./cmd/restish
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

Run the full integration suite before releases or larger CLI/plugin changes:

```bash
go test -tags=integration ./...
```

## License

Restish is released under the MIT License. See [`LICENSE.md`](LICENSE.md).

# Restish

[![CI](https://github.com/rest-sh/restish/actions/workflows/ci.yml/badge.svg)](https://github.com/rest-sh/restish/actions/workflows/ci.yml)
[![Docs Site](https://github.com/rest-sh/restish/actions/workflows/docs-site.yml/badge.svg)](https://github.com/rest-sh/restish/actions/workflows/docs-site.yml)
[![Release](https://img.shields.io/github/v/release/rest-sh/restish)](https://github.com/rest-sh/restish/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/rest-sh/restish/v2.svg)](https://pkg.go.dev/github.com/rest-sh/restish/v2)
[![License](https://img.shields.io/github/license/rest-sh/restish)](LICENSE.md)

Restish is a CLI for working with REST-ish HTTP APIs. It can make generic HTTP
requests, discover OpenAPI descriptions, generate API-aware commands, manage
profiles and auth, render structured output, follow pagination links, and run
plugins.

Restish v2 is the active release line. Users who need the legacy v1 line can
install `rest-sh/tap/restish@1`; see the
[v1 upgrade guide](https://rest.sh/docs/getting-started/upgrade-from-v1/).

## Try It

Make a direct request with no setup beyond installing the CLI:

```bash
restish api.rest.sh/types
```

Then format, filter, and page through real API responses:

```bash
restish api.rest.sh/images -o table --rsh-columns name,format,self
restish api.rest.sh/example -f 'body.basics.{name,url,profiles}'
```

Example table output:

```text
name                       format  self
Dragonfly macro            jpeg    /images/jpeg
Origami under blacklight   webp    /images/webp
Andy Warhol mural in Miami gif     /images/gif
```

## Install

Use Homebrew for the easiest managed install on macOS:

```bash
brew install restish
restish --version
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

Check `restish --version` after installation. See the
[install guide](https://rest.sh/docs/getting-started/install/) for shell setup,
upgrades, and platform notes.

## Why Restish?

- Use one-off HTTP commands with good defaults for headers, content types,
  retries, caching, TLS, and output.
- Connect OpenAPI descriptions to get generated API commands, help, profiles,
  auth, and shell completion.
- Filter normalized responses with shorthand or jq, then render JSON, YAML,
  tables, CSV, NDJSON, lines, gron, images, and raw downloads.
- Follow pagination links and stream API data without writing boilerplate loops.
- Extend the CLI with plugins for bulk workflows, CSV, MCP, TLS signing, and
  custom API behavior.

## Documentation

- [Documentation Home](https://rest.sh/docs/)
- [Tour of Restish](https://rest.sh/docs/getting-started/tour/)
- [Install](https://rest.sh/docs/getting-started/install/)
- [Connect to an API](https://rest.sh/docs/getting-started/connect-to-an-api/)
- [Authentication](https://rest.sh/docs/guides/authentication/)
- [Plugins](https://rest.sh/docs/plugins/)
- [Upgrade From v1](https://rest.sh/docs/getting-started/upgrade-from-v1/)

## Plugins

Bundled plugins live in this repository and can be built alongside the main
CLI:

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

Start with [Install and Use Plugins](https://rest.sh/docs/plugins/install-and-use/)
or [Plugin Quickstart](https://rest.sh/docs/plugins/quickstart/).

## Development

Build from this repository when developing Restish itself:

```bash
go build ./cmd/restish
./restish --help
./restish api.rest.sh/types
```

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

Development docs:

- [Contributing](https://rest.sh/docs/contributing/)
- [Development Setup](https://rest.sh/docs/contributing/development-setup/)
- [Release Packaging](https://rest.sh/docs/contributing/release-packaging/)
- [Design Docs](docs/design/README.md)
- [Docs Source](site/)

## License

Restish is released under the MIT License. See [`LICENSE.md`](LICENSE.md).

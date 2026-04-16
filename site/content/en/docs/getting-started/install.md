---
title: Install
linkTitle: Install
weight: 10
description: Install Restish with your preferred package manager, release binary, or source build.
---

The recommended install path for most users is Homebrew, but Restish is also
easy to install with other package managers, GitHub releases, or `go install`.

## Homebrew

```bash
brew install restish
```

That should install the `restish` binary onto your `PATH`. This is the best
path for most macOS users and a good default on Linux systems that already use
Homebrew.

## mise

If you use [mise](https://mise.jdx.dev/), install the latest release with:

```bash
mise use -g restish@latest
```

This is a good choice if you already manage CLIs and runtimes through mise.

## Nixpkgs

If you use Nix, install the package from Nixpkgs:

```bash
nix-env --install --attr nixpkgs.restish
```

This is the most natural option when the rest of your machine is already
managed with Nix.

## GitHub Releases

If you prefer manual installs, download the appropriate binary from the
[GitHub releases](https://github.com/rest-sh/restish/releases) page and place
it somewhere on your `PATH`, such as `/usr/local/bin/restish`.

This is a good fallback when you do not want to use a package manager.

## `go install`

If you already have a Go toolchain and just want the CLI:

```bash
go install github.com/rest-sh/restish@latest
```

This is convenient for contributors and Go-heavy environments.

## Verify The Install

Make sure the CLI starts cleanly:

```bash
restish --help
```

You should see top-level help text and a successful exit.

If you want a second quick check:

```bash
restish https://api.rest.sh/
```

Example output:

```readable
HTTP/2.0 200 OK
Content-Type: application/cbor

{
  message: "Welcome to the Restish example API"
  self: "https://api.rest.sh/"
}
```

If you see a formatted response like that, the install is working.

## Next Step

Once the binary is runnable, go straight to [First Request](../first-request/).

## Optional Shell Setup

If you plan to use Restish interactively, follow [Shell Setup](../shell-setup/)
next for `noglob`-style input handling and completion.

## Build From Source

If you prefer to build from source or you are contributing, you can still build
from the repo root:

```bash
go build ./cmd/restish
```

That produces a `restish` binary you can run from the current directory or move
onto your `PATH`.

If you want the plugin binaries too, build them separately:

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

## What You Need For Source Builds

Building from source assumes:

- a working Go toolchain
- the Restish repository checked out locally

If you are contributing, this is also the best path because it keeps the docs,
design records, and current code all in the same workspace.

After a source build, verify with:

```bash
restish --help
```

If the binary is not on your `PATH` yet, run it directly from the repo root:

```bash
./restish --help
```

You should see top-level help text and a successful exit.

## Which Install Method Should You Choose

- Use Homebrew if you want the fastest mainstream install path.
- Use mise or Nixpkgs if those already manage the rest of your tools.
- Use GitHub releases if you want a manual binary install.
- Use `go install` or a source build if you are contributing or already work in
  Go.

## Related Guides

- [Getting Started](../)
- [Quickstart](../quickstart/)
- [First Request](../first-request/)
- [Shell Setup](../shell-setup/)
- [Development Setup](/docs/contributing/development-setup/)

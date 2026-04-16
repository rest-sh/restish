---
title: Install
linkTitle: Install
weight: 10
description: Install Restish and verify that the CLI works on your machine.
---

The recommended install path is Homebrew.

## Homebrew

```bash
brew install restish
```

That should install the `restish` binary onto your `PATH`.

## Verify The Install

Make sure the CLI starts cleanly:

```bash
restish --help
```

If you want a second quick check:

```bash
restish https://api.rest.sh/
```

You should see a successful response body on stdout.

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

## Related Guides

- [Getting Started](../)
- [First Request](../first-request/)
- [Shell Setup](../shell-setup/)
- [Development Setup](/docs/contributing/development-setup/)

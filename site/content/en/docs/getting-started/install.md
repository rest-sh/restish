---
title: Install
linkTitle: Install
weight: 10
description: Install Restish and verify that the CLI works on your machine.
---

# Install Restish

Restish v2 packaging is still being finalized, so the most reliable path today
is building from source.

## Build From Source

From the repo root:

```bash
go build ./cmd/restish
```

That produces a `restish` binary you can run from the current directory or move
onto your `PATH`.

If you want the plugin binaries too, build them separately:

```bash
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

## What You Need

Building from source assumes:

- a working Go toolchain
- the Restish repository checked out locally

If you are contributing, this is also the best path because it keeps the docs,
design records, and current code all in the same workspace.

## Verify Your Install

After building or installing, make sure the CLI starts cleanly:

```bash
restish --help
restish version
```

If the binary is not on your `PATH` yet, run it directly from the repo root:

```bash
./restish --help
```

You should see top-level help text and a successful exit.

## Optional Next Setup

Once the binary is runnable:

1. follow [Shell Setup](../shell-setup/) if you use Restish interactively
2. make a [First Request](../first-request/)

## Packaging Direction

The intended long-term install story is:

- Homebrew for macOS and Linux users
- direct binary downloads for supported platforms
- an optional Go-based install path for users who prefer Go toolchains

This page can be expanded with exact commands as release artifacts settle.

## Related Guides

- [Shell Setup](../shell-setup/)
- [First Request](../first-request/)
- [Development Setup](../contributing/development-setup/)

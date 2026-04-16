---
title: Development Setup
linkTitle: Development Setup
weight: 10
description: Build, test, and iterate on Restish from a local checkout.
---

Use this page when you want to work on Restish itself rather than just install
the released CLI.

## Prerequisites

You need:

- a working Go toolchain
- a local checkout of the Restish repository

All commands below assume you are running from the repository root.

## Build The Main CLI

Build the main binary with:

```bash
go build ./cmd/restish
```

That produces a `restish` binary in the current directory.

## Build Plugin Binaries

Several plugins live in this repository and can be built directly from source:

```bash
go build ./cmd/restish-bulk
go build ./cmd/restish-csv
go build ./cmd/restish-mcp
go build ./cmd/restish-pkcs11
```

These are useful when you are working on plugin behavior, integration tests, or
local end-to-end workflows.

## Run Tests

Run the full test suite with:

```bash
go test ./...
```

During focused work it is often faster to run one package at a time:

```bash
go test ./internal/cli/...
```

## Update Golden Files

Some output tests use golden files. When an intentional formatter change needs
new expected output, update those files with:

```bash
go test -update ./internal/output/...
```

Review the resulting diffs carefully before committing them.

## Typical Workflow

A practical local loop usually looks like this:

1. build the binary or plugin you are changing
2. run the targeted package tests while iterating
3. run `go test ./...` before sending the change out for review

If your change affects user-facing behavior, update the documentation in
`site/` at the same time.

## Related Pages

- [Design Records](/docs/contributing/design-records/)
- [Install](/docs/getting-started/install/)
- [Plugin Quickstart](/docs/plugins/quickstart/)

---
title: Built-In Example Plugins
linkTitle: Example Plugins
weight: 50
description: Reference for the first-party plugin binaries and test fixtures in this repository.
---

This repository includes several plugin implementations that are useful both as
real features and as reference code for plugin authors.

## First-Party Plugin Binaries

- `cmd/restish-csv/main.go`: formatter hook plugin that renders array-shaped
  results as CSV
- `cmd/restish-mcp/main.go`: command plugin that exposes registered APIs over
  MCP
- `cmd/restish-pkcs11/main.go`: TLS signer plugin for PKCS#11-backed client-key
  signing
- `cmd/restish-bulk/main.go`: command plugin for Git-like bulk resource
  management

## Small Test Fixtures

- `internal/cli/testdata/cmdplugin/main.go`: very small command-plugin fixture
- `internal/cli/testdata/hookplugin/main.go`: small hook-plugin fixture used in
  tests

## How To Use This Page

If you are writing:

- a formatter plugin, start with `restish-csv`
- a command plugin, start with `restish-mcp` or the tiny cmdplugin fixture
- a TLS signer plugin, start with `restish-pkcs11`

These examples are useful because they show the real host/plugin boundary
rather than only isolated snippets.

## Related Pages

- [Plugin Quickstart](./quickstart/)
- [Hook Plugins](./hook-plugins/)
- [Command Plugins](./command-plugins/)
- [TLS Signer Plugins](./tls-signer-plugins/)

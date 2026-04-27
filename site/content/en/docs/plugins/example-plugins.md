---
title: Built-In Example Plugins
linkTitle: Example Plugins
weight: 20
description: Map first-party plugin binaries and fixtures to user and author goals.
---

The repository includes plugin binaries that are both useful tools and reference
implementations.

## Operator-Facing Plugins

| Plugin | Path | Use it for |
| --- | --- | --- |
| `restish-csv` | `cmd/restish-csv/main.go` | Render array responses as CSV. |
| `restish-bulk` | `cmd/restish-bulk/main.go` | Manage API collections as local files. |
| `restish-mcp` | `cmd/restish-mcp/main.go` | Serve registered APIs as MCP tools. |
| `restish-pkcs11` | `cmd/restish-pkcs11/main.go` | Sign mTLS handshakes with PKCS#11-backed keys. |

## Author Fixtures

| Fixture | Path | Use it for |
| --- | --- | --- |
| Tiny command plugin | `internal/cli/testdata/cmdplugin/main.go` | Minimal command protocol behavior. |
| Tiny hook plugin | `internal/cli/testdata/hookplugin/main.go` | Minimal hook behavior in tests. |

## Related Pages

- [Install and Use Plugins](../install-and-use/)
- [Hook Plugins](../hook-plugins/)
- [Command Plugins](../command-plugins/)
- [TLS Signer Plugins](../tls-signer-plugins/)

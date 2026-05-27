---
title: Built-In Example Plugins
linkTitle: Example Plugins
weight: 20
description: Map first-party plugin binaries and fixtures to user and author goals.
---

The repository includes plugin binaries that are both useful tools and reference
implementations. Operators can use these to try real plugin behavior. Authors
can read them to see the smallest practical shape for each plugin category.

If you are only trying to install and run a plugin, start with
[Install and Use Plugins](../install-and-use/). If you are writing one, read
the relevant author guide after finding the closest example here.

## Operator-Facing Plugins

| Plugin | Path | Use it for |
| --- | --- | --- |
| `restish-csv` | `cmd/restish-csv/main.go` | Render array responses as CSV. |
| `restish-bulk` | `cmd/restish-bulk/main.go` | Manage API collections as local files. |
| `restish-mcp` | `cmd/restish-mcp/main.go` | Serve registered APIs as MCP tools. |
| `restish-pkcs11` | `cmd/restish-pkcs11/main.go` | Sign mTLS handshakes with PKCS#11-backed keys. |

These binaries are built like normal Go commands. Once installed where Restish
can discover them, they participate in the same plugin lifecycle as third-party
plugins.

## Author Fixtures

| Fixture | Path | Use it for |
| --- | --- | --- |
| Multipurpose test plugin | `internal/cli/testdata/testplugin/main.go` | Minimal command, hook, and TLS signer behavior in tests. |

Fixtures are intentionally small and test-oriented. Use them to understand the
protocol shape, then use the first-party binaries above for production-style
examples.

## Related Pages

- [Install and Use Plugins](../install-and-use/)
- [Hook Plugins](../hook-plugins/)
- [Command Plugins](../command-plugins/)
- [TLS Signer Plugins](../tls-signer-plugins/)

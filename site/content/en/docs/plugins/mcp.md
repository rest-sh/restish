---
title: Serve APIs Over MCP
linkTitle: MCP
weight: 105
description: Expose registered Restish APIs as MCP tools through the restish-mcp command plugin.
aliases:
  - /docs/plugins/mcp/
---

`restish-mcp` is a command plugin that exposes registered OpenAPI operations as
MCP tools. Use it when an MCP client should call APIs through Restish profiles,
auth, TLS, retries, and output normalization.

## Prerequisites

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish plugin list
```

The API must be registered and have a usable spec.

## Serve Tools

```bash
restish mcp serve example
```

The plugin reads the registered API spec, turns operations into MCP tools, and
delegates HTTP execution back to Restish.

The generated help below is the exact command reference, including write-tool
opt-in, operation allowlists, timeouts, and result-size limits.

Tool arguments follow the OpenAPI parameter shape. Query arrays are sent as
repeated query keys when the spec uses the usual `form` plus `explode: true`
style, and header arrays are comma-joined. Object parameters and unsupported
array styles are rejected with a tool error so the client does not send
ambiguous values.

## Generated Plugin Help

<!-- BEGIN GENERATED: restish-docgen mcp-help -->
Generated from the compiled `restish-mcp` plugin binary.

### `restish mcp --help`

```text
Expose registered APIs as MCP tools via Restish-authenticated HTTP delegation.

Use `restish mcp serve <api...>` from an MCP client command configuration. Restish loads each registered API's OpenAPI operations and forwards tool calls through the same auth, profile, TLS, and request pipeline as the CLI.

Usage:
  restish mcp serve <api...>

Commands:
  serve    Serve registered APIs over stdio
```

### `restish mcp serve --help`

```text
Serve registered APIs over the Model Context Protocol.

By default, Restish exposes read-oriented tools and hides write operations. Use `--allow-write-tools` only for MCP clients and models you trust to make `POST`, `PUT`, `PATCH`, and `DELETE` calls against the selected APIs.

Usage:
  restish mcp serve [flags] <api...>

Flags:
  --operations string        Comma-separated operationId allowlist
  --max-result-bytes int     Maximum tool result payload size
  --request-timeout int      Per-tool HTTP request timeout in seconds (0 disables)
  --read-only                Expose only GET/HEAD operations
  --allow-write-tools        Expose POST, PUT, PATCH, and DELETE operations as MCP tools
```
<!-- END GENERATED -->

## Hide Operations

Use OpenAPI hints when some operations should not be exposed to MCP clients:

```yaml
x-mcp-ignore: true
```

Use this for destructive, admin-only, or confusing operations.

## Good Fit

MCP works well for APIs with clear operation IDs, descriptions, schemas, and
safe auth profiles. It is a poor fit for APIs where operations are destructive
without confirmation or where the spec hides important side effects.

OpenAPI parameters that use `content` keep their declared schema in MCP tools.
For JSON parameter content, pass the native object, array, or scalar value and
Restish serializes it into the outgoing HTTP parameter.

## Troubleshooting

- Run `restish api sync <name>` after spec changes.
- Confirm `restish <name> --help` shows generated operations.
- Use `restish plugin debug` when plugin startup or messages fail.
- Pass `--allow-write-tools` only when missing tools are write operations you
  intentionally want to expose.
- Hide operations in the spec rather than relying on MCP clients to avoid them.

## Related Pages

- [OpenAPI Reference](/docs/reference/openapi-cli-integration/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Troubleshooting](/docs/guides/troubleshooting/)

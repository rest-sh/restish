---
title: Serve APIs Over MCP
linkTitle: MCP
weight: 105
description: Expose registered Restish APIs as MCP tools through the restish-mcp command plugin.
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

By default, MCP exposes read-like operations and hides `POST`, `PUT`, `PATCH`,
and `DELETE`. Use the opt-in flag only when the connected MCP client is allowed
to make write calls:

```bash
restish mcp serve example --allow-write-tools
```

If write operations are hidden, startup diagnostics say how many were skipped.
Each delegated tool call also has a timeout; use `--request-timeout <seconds>`
to tune it or `--request-timeout 0` to disable the timeout.

Tool arguments follow the OpenAPI parameter shape. Query arrays are sent as
repeated query keys when the spec uses the usual `form` plus `explode: true`
style, and header arrays are comma-joined. Object parameters and unsupported
array styles are rejected with a tool error so the client does not send
ambiguous values.

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
- [Troubleshooting](../troubleshooting/)

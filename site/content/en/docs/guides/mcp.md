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
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
restish plugin list
restish mcp --help
```

The API must be registered and have a usable spec.

## Serve Tools

```bash
restish mcp serve example
```

The plugin reads the registered API spec, turns operations into MCP tools, and
delegates HTTP execution back to Restish.

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
- Hide operations in the spec rather than relying on MCP clients to avoid them.

## Related Pages

- [OpenAPI and CLI Integration](../openapi-cli-integration/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Troubleshooting](../troubleshooting/)

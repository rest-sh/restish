---
title: Command Plugins
linkTitle: Command Plugins
weight: 40
description: Author top-level Restish workflows that exchange messages with the host.
---

Command plugins add root commands such as `bulk` and `mcp`. They can perform
multi-step workflows while delegating HTTP, config, prompts, and output back to
Restish.

## When To Use One

Use a command plugin when a feature needs:

- a top-level command
- multiple HTTP requests
- progress messages
- prompts or confirmations
- access to registered APIs and profiles

Use a hook plugin for one request/response/auth/formatting task.

## Delegated HTTP

Command plugins should usually ask Restish to make requests:

```json
{
  "type": "http-request",
  "request": {
    "method": "GET",
    "uri": "https://api.rest.sh/items"
  }
}
```

That preserves host profiles, auth, TLS signer behavior, retries, cache,
pagination, and output normalization.

## Lifecycle

1. The plugin declares commands during startup discovery.
2. Restish starts the plugin when the user runs the contributed command.
3. The plugin sends requests such as `http-request`, `api-spec`, `prompt`, or `response`.
4. The plugin sends `done` with an exit code.

## Real Examples

- `restish-bulk` manages API collections as local files.
- `restish-mcp` exposes registered API operations as MCP tools.

## Related Pages

- [Plugin Messages](/docs/reference/plugin-messages/)
- [Bulk Management](/docs/guides/bulk-management/)
- [MCP](/docs/guides/mcp/)

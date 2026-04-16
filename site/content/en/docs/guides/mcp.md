---
title: Serve APIs Over MCP
linkTitle: MCP
weight: 88
description: Expose registered Restish APIs as MCP tools so an MCP client can call OpenAPI operations through Restish.
---

`restish mcp` exposes registered APIs as MCP tools over stdio.

Use it when you want an MCP client to call real API operations while still
letting Restish handle:

- API registration
- auth and profiles
- TLS and mTLS
- retries and cache behavior

## What It Needs

Before `restish mcp` is useful, register at least one API with a usable spec:

```bash
restish api configure example https://api.rest.sh
restish api sync example
```

The plugin builds its tools from cached OpenAPI operations.

## Basic Usage

Serve one API over stdio:

```bash
restish mcp example
```

Serve multiple APIs:

```bash
restish mcp example github
```

When serving more than one API, tool names are namespaced to avoid collisions.

## Useful Flags

Restrict the exposed operations:

```bash
restish mcp example --operations listImages,getImage
```

Expose only safe read-only operations:

```bash
restish mcp example --read-only
```

Cap large tool results:

```bash
restish mcp example --max-result-bytes 32768
```

## How Tool Calls Work

When an MCP client calls a tool, the plugin translates the tool input into an
HTTP request and delegates execution back to Restish.

That means a tool call still uses the same Restish runtime behavior as a normal
CLI request:

- registered base URLs
- profile selection
- auth injection
- middleware hooks
- TLS settings

In practice, `restish mcp` is not a second API client. It is a protocol bridge
on top of the existing Restish request path.

## Good Fit vs Bad Fit

Use `restish mcp` when:

- you already have a Restish API registration
- the API has clear OpenAPI `operationId` values
- you want MCP tools that inherit the same auth and transport config as the CLI

Avoid it when:

- the API does not have a usable spec yet
- you need custom tool semantics unrelated to API operations
- you want raw HTTP behavior instead of Restish's normalized request pipeline

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [API Commands](/docs/concepts/api-commands/)

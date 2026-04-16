---
title: Plugins
linkTitle: Plugins
weight: 60
description: Extend Restish with hook plugins, command plugins, and TLS signer plugins.
---

Plugins are a first-class part of Restish v2. They let you add auth flows,
middleware, formatters, custom commands, spec loaders, and advanced TLS
integrations without forking the CLI.

Some plugins are primarily for plugin authors, while others expose
user-facing workflows such as `restish bulk` and `restish mcp`.

## Start Here

- [Plugin Quickstart](./quickstart/) for the shortest path to a working plugin.
- [Hook Plugins](./hook-plugins/) if you want one-shot request or response
  integration points.
- [Command Plugins](./command-plugins/) if you need a top-level command with a
  longer-lived conversation.
- [TLS Signer Plugins](./tls-signer-plugins/) for hardware-backed or external
  mutual TLS signing.

## Choosing a Plugin Type

- Choose a hook plugin for focused request/response behavior.
- Choose a command plugin for new product surfaces such as `restish mcp`.
- Choose a TLS signer plugin only when the private key must stay outside the
  Restish process.

## User-Facing Built-Ins

- [Bulk Management](/docs/guides/bulk-management/) for API collections checked
  out to disk.
- [MCP](/docs/guides/mcp/) for exposing registered APIs as MCP tools.

---
title: Plugins
linkTitle: Plugins
weight: 60
description: Install, operate, debug, and author Restish plugins.
---

Plugins are a first-class extension model in Restish v2. They add auth flows,
middleware, loaders, formatters, command workflows, and TLS signing while
keeping HTTP execution and output behavior anchored in the host CLI.

## Operator Track

Start here when you want to use an existing plugin:

- [Install and Use Plugins](./install-and-use/) for installation, listing, removal, and debugging.
- [Built-In Example Plugins](./example-plugins/) to find first-party plugin binaries.
- [TLS Signer Plugins](./tls-signer-plugins/) for hardware-backed or external mTLS signing.
- [Bulk Management](/docs/guides/bulk-management/) for `restish-bulk`.
- [MCP](/docs/guides/mcp/) for `restish-mcp`.

## Author Track

Start here when you are writing a plugin:

- [Plugin Quickstart](./quickstart/) for a minimal working plugin.
- [Hook Plugins](./hook-plugins/) for auth, middleware, loaders, and formatters.
- [Command Plugins](./command-plugins/) for top-level workflows.
- [Plugin Manifest](/docs/reference/plugin-manifest/) and [Plugin Messages](/docs/reference/plugin-messages/) for protocol reference.

## Choosing A Type

- Choose a hook plugin for focused request, response, auth, loader, or formatter behavior.
- Choose a command plugin for workflows that need their own command and multiple host interactions.
- Choose a TLS signer plugin when the private key must remain outside Restish.

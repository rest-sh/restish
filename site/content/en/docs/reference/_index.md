---
title: Reference
linkTitle: Reference
weight: 50
description: Factual lookup for Restish commands, flags, config, output, content types, profiles, and plugins.
---

Reference pages answer exact questions. For workflows, start with guides; for a
single task, use recipes.

## Command And Flag Reference

- [Commands](./commands/) maps the top-level command surface.
- [HTTP Commands](./http-commands/) covers generic verbs and bare URL GET.
- [Global Flags](./global-flags/) lists shared request, output, auth, TLS, pagination, cache, retry, and config flags.
- [API Management](./api-management/) covers `restish api ...`.
- [API Auth Inspect](./auth-header-command/), [Cache](./cache-command/), [Cert](./cert-command/), [Edit](./edit-command/), [Links](./links-command/), [Setup](./setup-command/), [Theme](./theme-command/), [Plugin](./plugin-command/), and [Bulk](./bulk-command/) cover focused commands.

## Data And Configuration Reference

- [Example API](./example-api/) lists live docs fixtures.
- [Config](./config/) and [Profiles](./profiles/) describe persistent settings.
- [Environment Variables](./environment-variables/) lists env overrides.
- [Content Types](./content-types/) covers request encoding and response decoding.
- [Output Defaults](./output-defaults/) and [Output Formats](./output-formats/) explain rendering.
- [Shorthand](./shorthand/) and [Query Syntax](./query-syntax/) cover input and filters.

## Plugin Reference

- [Plugins](./plugins/) for discovery and operating model.
- [Plugin Manifest](./plugin-manifest/) for manifest fields.
- [Plugin Messages](./plugin-messages/) for host/plugin protocol families.

## Generated Help Audit

Before release, compare command reference pages with:

```bash
restish --help
restish api --help
restish plugin --help
restish setup --help
restish api content-types
```

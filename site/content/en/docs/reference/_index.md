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
- [HTTP Commands](./http-commands/) covers generic verbs and bare URL method inference.
- [Global Flags](./global-flags/) lists shared request, output, auth, TLS, pagination, cache, retry, and config flags.
- [API Management](./api-management/) covers `restish api ...`.
- [Config Command](./config-command/), [Cache Command](./cache-command/), [Doctor](./doctor-command/), [Shell](./shell-command/), and [Utilities](./utility-commands/) cover support commands.
- [OpenAPI](./openapi-cli-integration/) covers generated command behavior and Restish OpenAPI extensions.
- [Edit](./edit-command/), [Plugin](./plugin-command/), and [Bulk](./bulk-command/) cover larger focused commands.

## Data And Configuration Reference

- [Example API](./example-api/) lists live docs fixtures.
- [Config](./config/), [Profiles](./profiles/), and [Auth](./auth/) describe persistent settings and credentials.
- [Environment Variables](./environment-variables/) lists env overrides.
- [Content Types](./content-types/) covers request encoding and response decoding.
- [Output Defaults](./output-defaults/) and [Output Formats](./output-formats/) explain rendering.
- [Shorthand](./shorthand/) and [Query Syntax](./query-syntax/) cover input and filters.
- [Embedding](./embedding/) covers custom Go CLIs built on Restish.

## Plugin Reference

- [Plugin Command](./plugin-command/) for operator command syntax.
- [Plugin Manifest](./plugin-manifest/) for manifest fields.
- [Plugin Messages](./plugin-messages/) for host/plugin protocol families.

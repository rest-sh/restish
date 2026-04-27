---
title: Documentation
linkTitle: Documentation
menu:
  main:
    weight: 10
---

Restish is a CLI for REST-ish HTTP APIs. Start with a direct request, then move
repeated work into API-aware commands, profiles, filters, pagination, and
plugins.

## Start Here

- New to Restish: [Install](./getting-started/install/) -> [Quickstart](./getting-started/quickstart/) -> [Requests](./guides/requests/)
- Already have an OpenAPI-described API: [Connect to an API](./getting-started/connect-to-an-api/) -> [OpenAPI and CLI Integration](./guides/openapi-cli-integration/)
- Repeating headers, tokens, or environment URLs: [Set Up Profiles](./getting-started/set-up-profiles/) -> [Authentication](./guides/authentication/)
- Extending Restish: [Install and Use Plugins](./plugins/install-and-use/) -> [Plugin Quickstart](./plugins/quickstart/)

## Common Workflows

- [Make requests](./guides/requests/) with generic HTTP verbs or generated commands.
- [Send request bodies](./guides/input/) with shorthand, stdin, forms, and multipart uploads.
- [Shape output](./guides/output/) with formats, filters, tables, raw bytes, and files.
- [Follow pagination and links](./guides/pagination/) across collections.
- [Stream events](./guides/streaming/) from SSE and NDJSON endpoints.
- [Troubleshoot behavior](./guides/troubleshooting/) with symptom-driven fixes.

## Popular Reference

- [Example API](./reference/example-api/) lists the live `api.rest.sh` endpoints used throughout the docs.
- [Commands](./reference/commands/) maps the top-level command surface.
- [Global Flags](./reference/global-flags/) explains request, output, auth, TLS, pagination, cache, and retry flags.
- [Config](./reference/config/) and [Profiles](./reference/profiles/) document persistent settings.
- [Shorthand](./reference/shorthand/) and [Query Syntax](./reference/query-syntax/) cover structured input and filtering.

## Sections

- **Getting Started** gets first-time users to a useful request quickly.
- **Concepts** explains the mental model behind the workflows.
- **Guides** cover multi-step daily work.
- **Recipes** give command-first answers to narrow tasks.
- **Reference** is factual lookup for commands, config, formats, and protocols.
- **Plugins** separates operator docs from author docs.
- **Contributing** preserves maintainer workflow and design intent.

---
title: Core Concepts
linkTitle: Core Concepts
weight: 20
description: Understand the model behind generic requests, API commands, profiles, normalized responses, and plugins.
---

Concept pages explain why Restish behaves consistently across direct URLs,
generated API commands, profiles, output formats, pagination, and plugins.

Use this section when a command works but the behavior feels surprising. The
concept pages give you the mental model that makes features compose: a direct
URL and a generated command both become requests; responses are normalized
before filtering; profiles layer defaults instead of replacing flags.

## Start Here

- [How Restish Works](./how-restish-works/) for the end-to-end request model.
- [API Commands](./api-commands/) for generic URLs versus generated operations.
- [Profiles](./profiles/) for layered request defaults.
- [Normalized Responses](./normalized-responses/) for `headers`, `links`, and `body` filter roots.
- [Plugins](./plugins/) for the extension model.

## Use Concepts When

- you understand one command but want the larger mental model
- a guide mentions behavior that spans several commands
- you are deciding between generic requests, API-aware commands, profiles, and plugins

You do not need to read concepts front-to-back. Follow links from tutorials and
guides when you want to understand why a command behaves the way it does.

## Related Pages

- [Requests](/docs/guides/requests/)
- [Output](/docs/guides/output/)
- [Design Records](/docs/contributing/design-records/)

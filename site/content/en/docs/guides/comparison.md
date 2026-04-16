---
title: Restish vs curl and HTTPie
linkTitle: Comparison
weight: 12
description: Understand where Restish fits compared with lower-level HTTP CLIs and why API-aware commands change the daily workflow.
---

Restish is not trying to replace every use of `curl`. It is trying to make the
common API workflow faster and more repeatable once you move past one-off raw
HTTP calls.

## Where `curl` Still Wins

Use `curl` when you want:

- the lowest-level wire control
- a tool that already exists on nearly every machine
- ad hoc shell one-liners with no API-aware setup at all

`curl` is still the baseline tool for raw HTTP debugging.

## Where HTTPie Still Wins

Use HTTPie when you want:

- a friendly one-off HTTP client
- a command style centered on immediate request construction
- a lighter mental model than API registration and generated commands

HTTPie is a strong fit when you care about human-friendly ad hoc requests but
not about turning an API description into a daily command surface.

## Where Restish Is Better

Restish becomes more valuable when one or more of these are true:

- you talk to the same API repeatedly
- the API has a usable OpenAPI description
- you want generated command names instead of manually rebuilding URLs
- you want profiles for environment, auth, and headers
- you want filtering, pagination, links, retries, caching, and output formats
  to feel like one system instead of separate tools

The main difference is that Restish has two modes of value:

1. a better direct HTTP client
2. a generated CLI for named APIs

That second mode is the differentiator.

## The Before And After

With a lower-level HTTP client, repeated use often looks like:

```bash
curl -H 'Authorization: Bearer ...' \
  'https://api.example.com/items?per_page=100'
```

With Restish before API setup:

```bash
restish -H 'Authorization: Bearer ...' \
  -q per_page=100 \
  https://api.example.com/items
```

With Restish after API setup and profiles:

```bash
restish api configure myapi https://api.example.com
restish -p prod myapi list-items
```

That is the product shift Restish is designed around: repeated API work should
look like using the API's CLI, not like rebuilding the same request shape from
scratch.

## Quick Decision Rule

- Use `curl` for raw wire-level work.
- Use HTTPie for friendly ad hoc HTTP.
- Use Restish when API work is becoming repeatable enough to deserve names,
  profiles, generated commands, and richer response handling.

## Related Pages

- [Quickstart](/docs/getting-started/quickstart/)
- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [Requests](/docs/guides/requests/)

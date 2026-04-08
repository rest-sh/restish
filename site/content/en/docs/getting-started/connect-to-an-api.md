---
title: Connect to an API
linkTitle: Connect to an API
weight: 40
description: Discover or register an API so Restish can generate commands from its description.
---

# Connect to an API

Restish becomes much more powerful when it can work from an API description,
such as an OpenAPI document.

## Two Main Paths

- point Restish at a URL and let it discover the description
- register an API in config so the generated commands are always available

## Quick Start: Register An API

The easiest durable workflow is to register a short API name:

```bash
restish api configure petstore https://api.example.com
```

That creates or updates a config entry for `petstore` and attempts to discover
its spec immediately.

If a spec is found, Restish can generate commands under the API name. If not,
you can still use the registration and later force a spec refresh with:

```bash
restish api sync petstore
```

After that, check what Restish knows:

```bash
restish api list
restish api show petstore
restish petstore --help
```

## Why Use API Commands

API commands can provide:

- stable top-level names for the API
- generated operations and parameters
- richer help output
- shell completion for discovered operations

## A Practical Flow

For a new API, the usual sequence is:

1. Register the API with `restish api configure <name> <url>`.
2. Run `restish <name> --help` to inspect the generated command group.
3. Run a specific generated command, or fall back to generic requests against
   the saved API name.
4. Use `restish api sync <name>` whenever the spec or server behavior changes.

## What Discovery Tries

When Restish needs a spec, it works through a small ordered strategy:

1. a cached spec for the registered API
2. `spec_url` from config, if set
3. link headers discovered from a `GET` on the API base URL
4. well-known paths such as `/openapi.json` and `/openapi.yaml`
5. the base URL response body itself

Restish runs the network probes in parallel and uses the first spec that parses
successfully.

## What Happens If No Spec Is Found

Registration still helps even if discovery does not find a spec right away:

- the API short name is saved in config
- profiles can still provide base URLs, auth, headers, and query defaults
- you can point `spec_url` at the correct document later
- you can retry discovery with `restish api sync <name>`

## Explicit Config Example

If you already know exactly where the spec lives, make that explicit in config:

```json
{
  "apis": {
    "petstore": {
      "base_url": "https://api.example.com",
      "spec_url": "https://api.example.com/openapi.json"
    }
  }
}
```

That is the most predictable setup when the API does not advertise its spec in
conventional places.

## Generated Commands Are Built From Cached Specs

Restish does not need to rediscover the spec every time you open the CLI. Once
it has a cached spec for an API, it can rebuild the generated command tree from
that local copy.

That makes startup faster and keeps API-aware commands available even when you
are offline.

## What Generated Commands Look Like

Once an API is registered and its spec is available, operations from the spec
show up as ordinary CLI commands under the API short name.

For example, an OpenAPI operation might turn into something like:

```bash
restish petstore pet <pet-id> --include owner
```

instead of requiring you to hand-write the full URL and query parameters every
time.

Required path or query parameters usually become positional arguments, while
optional parameters become flags. Request bodies still use the same shorthand
and stdin input model as generic commands.

## Useful Commands

- `restish api list`
- `restish api show <name>`
- `restish api set <name> <key> <value>`
- `restish api sync <name>`
- `restish api delete <name>`
- `restish <api> --help`

## Related Guides

- [First Request](../first-request/)
- [Requests](../guides/requests/)
- [API Commands](../concepts/api-commands/)

## Source Material

- [`docs/design/006-spec-discovery-and-loading.md`](/Users/daniel/src/restish2/docs/design/006-spec-discovery-and-loading.md)
- [`docs/design/007-api-command-generation.md`](/Users/daniel/src/restish2/docs/design/007-api-command-generation.md)

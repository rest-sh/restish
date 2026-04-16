---
title: Connect to an API
linkTitle: Connect to an API
weight: 40
description: Register an API, discover its OpenAPI document, and switch from raw URLs to generated commands.
---

Restish becomes much more powerful when it can work from an API description,
such as an OpenAPI document.

This is the moment where Restish stops being only a generic HTTP client and
starts behaving like an API-specific CLI.

## Two Main Paths

- point Restish at a URL and let it discover the description
- register an API in config so the generated commands are always available

## Quick Start: Register An API

The easiest durable workflow is to register a short API name:

```bash
restish api configure example https://api.rest.sh
```

That creates or updates a config entry for `example` and attempts to discover
its spec immediately.

If a spec is found, Restish can generate commands under the API name. If not,
you can still use the registration and later force a spec refresh with:

```bash
restish api sync example
```

After that, check what Restish knows:

```bash
restish api list
restish api show example
restish example --help
```

Example output:

```text
Configured APIs:
  example
```

And the generated help should now show operations such as:

```text
Commands generated from the example API spec

Available Commands:
  get-image
  list-images
```

If you see generated subcommands under `example`, you are ready to stop typing
full URLs for common operations.

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

That flow is worth learning early because it is one of Restish's biggest
advantages over lower-level HTTP tools.

## Before And After

Before registration, you make direct URL-based requests:

```bash
restish https://api.rest.sh/images/jpeg
```

After registration, the same API becomes easier to discover and easier to
remember:

```bash
restish example list-images
restish example get-image jpeg
```

That is usually the point where Restish stops feeling like "a nicer curl" and
starts feeling like "the CLI for this API".

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

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json"
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
are offline. For the docs example API, the discovered spec is
`https://api.rest.sh/openapi.json`.

## What Generated Commands Look Like

Once an API is registered and its spec is available, operations from the spec
show up as ordinary CLI commands under the API short name.

For the docs example API, generated commands look like:

```bash
restish example list-images
restish example get-image jpeg
```

You can also keep using URL-style requests against the same registered API:

```bash
restish example/images
```

That fallback is important. Registration is still useful even before the API
description is perfect because the API short name can carry your base URL,
profiles, auth, and other defaults.

Required path or query parameters usually become positional arguments, while
optional parameters become flags. Request bodies still use the same shorthand
and stdin input model as generic commands.

## A Good Mental Model

Think about Restish in two layers:

- generic commands are for immediate ad hoc calls
- API commands are for repeatable, discoverable daily use

Most users end up using both.

## Useful Commands

- `restish api list`
- `restish api show <name>`
- `restish api set <name> <key> <value>`
- `restish api sync <name>`
- `restish api delete <name>`
- `restish <api> --help`

## Related Guides

- [Quickstart](../quickstart/)
- [First Request](../first-request/)
- [Shell Setup](../shell-setup/)
- [Set Up Profiles](../set-up-profiles/)
- [Requests](../guides/requests/)
- [API Commands](../concepts/api-commands/)

## Source Material

- [Design Records](/docs/contributing/design-records/)

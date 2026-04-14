---
title: API Setup and Discovery
linkTitle: API Setup and Discovery
weight: 15
description: Register APIs, discover their specs, and work with generated Restish commands.
---

Restish gets much more powerful when it can attach a short API name to a base
URL and a discovered OpenAPI description.

That setup unlocks:

- generated commands
- richer help text
- shell completion
- profile-aware requests without repeating full URLs

## Fastest Path

Register an API:

```bash
restish api configure example https://api.rest.sh
```

Then inspect what Restish learned:

```bash
restish api list
restish example --help
```

## How Discovery Works

When Restish needs a spec, it tries this ordered strategy:

1. cached spec for the API name
2. explicit `spec_url` from config
3. `Link` headers from a `GET` on the base URL
4. well-known paths such as `/openapi.json` and `/openapi.yaml`
5. the base URL response body itself

Network probes run in parallel, and the first successfully parsed spec wins.

## Make Discovery Predictable

If the server does not publish its spec in conventional places, set `spec_url`
or `spec_files` explicitly:

```json
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json"
    }
  }
}
```

Use `spec_files` when you want to point at local files or merge multiple spec
documents in order.

## Generated Command Shape

Generated commands are grouped under the API short name. They behave like
ordinary CLI commands rather than a separate codegen mode.

For example:

```bash
restish example get-image jpeg
```

## When To Re-Sync

Run this when the upstream spec changed:

```bash
restish api sync example
```

## When Discovery Fails

Even without a discovered spec, an API registration is still useful because it
can hold `base_url`, profiles, auth settings, TLS options, and pagination
defaults. You can still make generic requests against the saved API name and
add the spec location later.

## Learn More

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [Commands Reference](/docs/reference/commands/)
- [API Management Reference](/docs/reference/api-management/)

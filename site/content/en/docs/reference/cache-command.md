---
title: Cache Commands
linkTitle: Cache Commands
weight: 14
description: Reference for inspecting and clearing the Restish HTTP response cache.
---

Restish exposes two cache subcommands:

- `restish cache info`
- `restish cache clear [api]`

Use these when you want to inspect cache size, confirm the active cache
directory, or force fresh reads after a spec or server response changes.

## Examples

```bash
restish cache info
restish cache clear
restish cache clear example
```

## What Each Command Does

- `cache info`: prints the cache directory, current size, and entry count
- `cache clear`: removes all cached responses
- `cache clear <api>`: removes cached responses associated with one configured API

## When To Use It

Use `cache info` when:

- you want to confirm which cache directory Restish is using
- you need a quick sanity check on cache growth
- you are debugging whether a repeated request is being served locally

Use `cache clear` when:

- an API spec changed and you want a fresh load
- cached responses are hiding a server-side change
- you are reproducing a bug and need a clean starting point

## Related Flags And Env Vars

- `--rsh-no-cache`: bypass cache reads and writes for one request
- `RSH_CACHE_DIR`: move the cache to a different directory

## Related Pages

- [Retries and Caching](/docs/guides/retries-and-caching/)
- [API Management](../api-management/)
- [Environment Variables](../environment-variables/)

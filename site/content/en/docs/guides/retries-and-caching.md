---
title: Retries and Caching
linkTitle: Retries and Caching
weight: 80
description: Control retries, timeouts, HTTP cache behavior, and conditional request examples.
aliases:
  - /docs/recipes/retry-until-a-flaky-endpoint-succeeds/
  - /docs/recipes/limit-a-slow-request-with-a-timeout/
---

Restish retries conservative transient failures and uses a disk-backed HTTP
cache for cacheable responses.

## Retry Behavior

Restish retries network errors plus `408`, `429`, `500`, `502`, `503`, and
`504` responses by default. Automatic retries are limited to `GET` and `HEAD`,
where retrying is normally safe. `--rsh-retry N` controls the attempt count;
use `--rsh-retry-unsafe` only when you want to retry POST, PUT, PATCH, or
DELETE, because that can repeat side effects if the server processed the first
attempt. Restish honors `Retry-After` and `X-Retry-In` when present, capped at
5 minutes by default. Use
`--rsh-retry-max-wait 30s` for one command, or set an API-level
`retry_max_wait` duration in `restish.json`, when a service needs a shorter
rate-limit wait.

Use the flaky fixture to see recovery:

{{< restish-example >}}
restish 'api.rest.sh/flaky?failures=1&key=docs-retry' --rsh-retry 2
{{< /restish-example >}}

Use unique keys while experimenting so a previous successful run does not make
the fixture look less flaky than it is.

Opt into replaying an unsafe method only when the endpoint is idempotent enough
for your use case:

```bash
restish post https://api.vendor.test/jobs name:demo --rsh-retry 2 --rsh-retry-unsafe
```

Disable retries for strict single-attempt debugging:

{{< restish-example >}}
restish 'api.rest.sh/flaky?failures=1&key=docs-once' --rsh-retry 0
{{< /restish-example >}}

## Timeouts

{{< restish-example >}}
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
{{< /restish-example >}}

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 3s
```

The first command should fail quickly because the server waits longer than the
client allows. The second gives the fixture enough time to respond. Use
timeouts in scripts and CI so slow services fail predictably.

## Status Fixtures

{{< restish-example >}}
restish api.rest.sh/status/429 --rsh-retry 2 --rsh-ignore-status-code
{{< /restish-example >}}

```bash
restish api.rest.sh/status/503 --rsh-retry 2 --rsh-ignore-status-code
```

`--rsh-ignore-status-code` lets you inspect error bodies while keeping the CLI
exit code from stopping a shell pipeline.

## Cache Behavior

Bypass cache reads and writes for one command:

{{< restish-example >}}
restish api.rest.sh/cache --rsh-no-cache
{{< /restish-example >}}

Use verbose output to see cache diagnostics:

{{< restish-example >}}
restish api.rest.sh/cached/60 -v
{{< /restish-example >}}

Credentialed API-profile requests are cached inside that API/profile namespace.
Direct URL requests that put credentials in headers or query parameters bypass
the cache because Restish has no profile namespace to separate entries.
Cache entries are written atomically and cache eviction is coordinated across
Restish processes that share the same cache directory.

Manage cache files:

```bash
restish cache info
restish cache clear
restish cache clear example
```

`cache clear` removes HTTP response cache entries only. Use
`restish api auth logout` when you need to delete cached OAuth/auth tokens.

## Conditional Requests

Use ETag fixtures when testing conditional behavior:

{{< restish-example >}}
restish api.rest.sh/etag/docs
{{< /restish-example >}}

```bash
restish -H 'If-None-Match: "docs"' api.rest.sh/etag/docs --rsh-ignore-status-code
```

## Related Pages

- [Commands](/docs/reference/commands/)
- [Global Flags](/docs/reference/global-flags/)
- [Command Behavior](../command-behavior/)
- [Troubleshooting](../troubleshooting/)

---
title: Retries and Caching
linkTitle: Retries and Caching
weight: 80
description: Control retries, timeouts, HTTP cache behavior, and conditional request examples.
---

Restish retries conservative transient failures and uses a disk-backed HTTP
cache for cacheable responses.

## Retry Behavior

Restish retries network errors, `408`, `429`, and `5xx` responses by default.
Automatic retries are limited to `GET` and `HEAD`, where retrying is normally
safe. Passing an explicit non-negative retry count such as `--rsh-retry 2`
opts a command into retrying other methods too, which can repeat side effects
if the server processed the first attempt. Restish honors `Retry-After` and
`X-Retry-In` when present.

Use the flaky fixture to see recovery:

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-retry' --rsh-retry 2
```

Disable retries for strict single-attempt debugging:

```bash
restish 'https://api.rest.sh/flaky?failures=1&key=docs-once' --rsh-retry 0
```

## Timeouts

```bash
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 3s
```

Use timeouts in scripts and CI so slow services fail predictably.

## Status Fixtures

```bash
restish https://api.rest.sh/status/429 --rsh-retry 2 --rsh-ignore-status-code
restish https://api.rest.sh/status/503 --rsh-retry 2 --rsh-ignore-status-code
```

`--rsh-ignore-status-code` lets you inspect error bodies while keeping the CLI
exit code from stopping a shell pipeline.

## Cache Behavior

Bypass cache reads and writes for one command:

```bash
restish https://api.rest.sh/cache --rsh-no-cache
```

Use verbose output to see cache diagnostics:

```bash
restish https://api.rest.sh/cached/60 -v
```

Manage cache files:

```bash
restish cache info
restish cache clear
restish cache clear example
```

## Conditional Requests

Use ETag fixtures when testing conditional behavior:

```bash
restish https://api.rest.sh/etag/docs
restish -H 'If-None-Match: "docs"' https://api.rest.sh/etag/docs --rsh-ignore-status-code
```

## Related Pages

- [Cache Command](/docs/reference/cache-command/)
- [Global Flags](/docs/reference/global-flags/)
- [Command Behavior](../command-behavior/)
- [Troubleshooting](../troubleshooting/)

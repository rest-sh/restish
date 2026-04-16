---
title: Retries and Caching
linkTitle: Retries and Caching
weight: 80
description: Learn how Restish handles retries, rate limits, and local HTTP caching.
---

Restish includes retry behavior and local HTTP caching to make repeated API work
faster and more resilient.

## Retry Behavior

Retries are conservative by default:

- network errors are retried
- `5xx` responses are retried
- `4xx` responses are not retried
- `Retry-After` is honored when present

The default retry count is 2. Override it with `--rsh-retry`:

```bash
restish https://api.example.com/items --rsh-retry 5
restish https://api.example.com/items --rsh-retry 0
```

Use `0` to disable retries entirely.

## Timeouts

Use `--rsh-timeout` when you want to bound how long one request can run:

```bash
restish https://api.example.com/items --rsh-timeout 15s
restish https://api.example.com/items --rsh-timeout 500ms
```

This is useful when:

- you are scripting against slow or unreliable services
- you want CI checks to fail quickly
- you are debugging retry and latency behavior

Related shell default:

```bash
export RSH_TIMEOUT=15s
```

## Cache Behavior

Restish uses a disk-backed HTTP response cache. Cacheable responses can be
reused across repeated requests, which makes routine API exploration and
read-heavy workflows faster.

By default, cached responses live under Restish's cache directory. You can
override that location with `RSH_CACHE_DIR`.

By default, cache use participates in the normal request path. To bypass the
cache for one invocation:

```bash
restish https://api.example.com/items --rsh-no-cache
```

That disables both cache reads and cache writes for that request.

## How Cache And Retry Work Together

The transport is layered so cache hits return immediately, while only real
network requests participate in retry behavior.

That means:

- cached responses are not retried
- retry logic only applies to live requests
- request flow stays predictable

## Inspect And Clear The Cache

Restish exposes cache management commands directly:

```bash
restish cache info
restish cache clear
restish cache clear myapi
```

Use `cache info` to inspect directory, size, and entry count. Use `cache clear`
to remove everything or just the entries associated with one configured API.

## Practical Guidance

Leave retries enabled when:

- you are talking to remote APIs over the public internet
- rate limiting or transient failures are common
- idempotent reads are the main workflow

Consider `--rsh-no-cache` when:

- you must force a fresh read
- you are debugging server behavior
- you are working with responses that should not be reused

Consider `--rsh-retry 0` when:

- you need strict single-attempt behavior
- you are debugging request timing or failure modes

## Related Guides

- [Requests](../requests/)
- [Pagination and Links](../pagination/)
- [Environment Variables](/docs/reference/environment-variables/)

Source material:

- [Design Records](/docs/contributing/design-records/)

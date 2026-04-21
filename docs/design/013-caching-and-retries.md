# Caching And Retries

## Summary

Restish v2 handles response caching and retries in the transport stack. Caching
reduces repeated network work for cacheable responses, while retries make common
transient failures less disruptive.

These behaviors are layered so they cooperate without obscuring request flow.

## Problem

API clients benefit from both caching and retries, but the two features can
interact in confusing ways if they are not clearly ordered.

The design needed to:

- support ordinary HTTP cache semantics for real responses
- keep cache behavior bounded on disk
- retry only meaningful failures
- avoid retrying responses that were served from cache
- preserve normal request/response behavior for callers

## Design

Restish builds its HTTP transport in layers:

```text
httpcache transport -> retry transport -> base http transport
```

This ordering is important.

The retry layer sits below the cache so only real server requests are retried.
Cache hits return immediately without replaying retry logic.

The response cache is a disk-backed, size-bounded cache with LRU-style eviction.
Entries are grouped under per-host directories so cache management can operate
on one API host or the entire cache tree.

The size bound is part of the public design, not just an implementation detail.
If a size-related config value exists, it should affect actual eviction policy.

Retry behavior is intentionally conservative:

- network errors are retried
- `5xx` responses are retried
- `408` and `429` are retry candidates when policy allows
- `4xx` responses are returned immediately
- request bodies are only retried when they can be recreated safely
- backoff uses exponential delay with jitter
- `Retry-After` is honored when provided

This keeps retries focused on failures that are likely to succeed on a later
attempt while avoiding silent replay of requests that cannot be reproduced
correctly.

## Retry State Hygiene

Retries must not accidentally return stale response objects from earlier
attempts when a later retry decision fails. Each retry attempt should treat its
response/error pair as attempt-local state.

## Examples

A request with caching enabled and retries configured conceptually behaves like:

```text
request -> cache lookup
  cache hit: return cached response
  cache miss: send real request through retry layer
```

For a transient server failure sequence like:

```text
GET /items -> 502
GET /items -> 503
GET /items -> 200
```

Restish retries the first two responses and returns the final success.

For a client error like:

```text
GET /items -> 404
```

Restish does not retry, because the error is unlikely to be transient.

The disk cache is organized roughly like:

```text
<cache-dir>/<hostname>/<sha256(url)>.cache
```

which allows cache clearing by host as well as global cache management.

Cache inspection and management are exposed directly in the CLI:

```bash
restish cache info
restish cache clear
restish cache clear myapi
```

## Alternatives Considered

### Retry above the cache layer

This would make cache hits participate in retry logic, which is unnecessary and
blurs the distinction between cached and live requests.

### Retry all non-2xx responses

That would waste time on client-side errors and produce surprising behavior for
users. Restricting retries to network failures and `5xx` responses is a better
default.

### Use an unbounded disk cache

That would be simpler initially, but it risks unbounded growth on developer
machines and CI environments. Size limits and eviction are worth the added
implementation detail.

## Notes

The current implementation reflects this design directly:

- `internal/request/do.go` assembles the transport stack
- `internal/request/retry.go` implements retry policy and backoff behavior
- `internal/cache/cache.go` provides the disk-backed bounded cache

One detail worth preserving is that retryability depends on whether the request
body can be replayed. Restish intentionally refuses to blindly retry requests
whose bodies cannot be recreated, which is safer than assuming all failures are
idempotent in practice.

# Caching And Retries

## Summary

Restish v2 handles response caching and retries in the transport stack. Caching
reduces repeated network work for cacheable responses, while retries make common
transient failures less disruptive.

These behaviors are layered so they cooperate without obscuring request flow.

## Goals

- support ordinary HTTP cache semantics for real responses
- keep cache behavior bounded on disk
- retry only meaningful transient failures
- avoid retrying responses that were served from cache
- preserve consistent request semantics across generic commands and generated
  commands

## Non-Goals

- turning retries into blind replay of every failed request
- making the cache an opaque black box with unbounded growth
- hiding whether a response was live or cache-derived in ways that break
  debugging

## Transport Layering

Restish builds its HTTP transport in layers:

```text
httpcache transport -> retry transport -> base http transport
```

This ordering is important.

The retry layer sits below the cache so only real server requests are retried.
Cache hits return immediately without replaying retry logic.

## Cache Model

The response cache is a disk-backed, size-bounded cache with LRU-style
eviction. Entries are grouped under per-host directories so cache management can
operate on one API host or the entire cache tree.

Cache entry writes are atomic: data is written to a temporary file in the
target directory, synced, and then renamed into place. Readers should see either
the previous complete entry or the new complete entry, never a partially written
entry. LRU eviction uses an advisory sibling lock in addition to the in-process
eviction mutex so multiple Restish processes can share a cache directory without
racing each other during cleanup.

Credentialed requests may use the cache only when Restish can partition entries
by API/profile namespace. API-aware requests set the namespace to
`<api>:<profile>`, so a response cached for one profile is not reused by another
profile on the same host. Direct URL requests that carry credential headers or
credential-looking query parameters still bypass the cache because there is no
stable profile namespace to key on.

The size bound is part of the public design, not just an implementation detail.
If a size-related config value exists, it should affect actual eviction policy.

The disk cache is organized roughly like:

```text
<cache-dir>/<hostname>/<sha256(url)>.cache
```

This layout supports both:

- global cache management
- targeted per-host clearing

## Retry Policy

Retry behavior is intentionally conservative:

- network errors are retried
- only explicit transient statuses are retried: `408`, `429`, `500`, `502`,
  `503`, and `504`
- `4xx` responses are otherwise returned immediately
- non-transient `5xx` statuses such as `501` and `505` are returned immediately
- request bodies are only retried when they can be recreated safely
- backoff uses exponential delay with jitter
- `Retry-After` and `X-Retry-In` are honored when provided, capped by
  `--rsh-retry-max-wait` or the API's `retry_max_wait` setting

This keeps retries focused on failures that are likely to succeed on a later
attempt while avoiding silent replay of requests that cannot be reproduced
correctly.

## Replay Safety

Retryability depends on whether the request body can be replayed safely.

The design rule is:

- if the body can be reconstructed, retry may proceed
- if the body cannot be reconstructed, stop and return the last real failure

Restish intentionally refuses to blindly retry requests whose bodies cannot be
recreated, which is safer than assuming all failures are idempotent in practice.
`--rsh-retry` only controls the retry attempt count. When a user explicitly
enables retries for unsafe methods with `--rsh-retry-unsafe`, Restish warns once
per CLI session because POST, PUT, PATCH, and DELETE retries can double-process
server-side side effects.

## Retry Algorithm

The conceptual retry loop is:

1. build or clone a retryable request attempt
2. send the request through the base transport
3. evaluate the result:
   - success or non-retryable failure -> return
   - retryable failure -> continue if limits allow
4. compute wait delay from:
   - `Retry-After` when applicable
   - `X-Retry-In` when applicable
   - otherwise exponential backoff with jitter
5. wait unless the context is canceled
6. rebuild the next attempt with a fresh body reader if needed
7. repeat until retry budget is exhausted

This algorithm should be explicit in the implementation, not spread across
early-return edge cases that can accidentally leak stale state.

## Retry State Hygiene

Retries must not accidentally return stale response objects from earlier
attempts when a later retry decision fails. Each retry attempt should treat its
response/error pair as attempt-local state.

This is a correctness rule, not just an implementation cleanup preference.

## Cache/Retry Interaction

The cache and retry layers should cooperate like this:

- cache hit -> no retry logic
- cache miss -> normal retry policy on live request
- live successful response -> cache may store it according to HTTP cache rules
- live failure -> retry policy evaluates it; cache should not mask it as a hit

This preserves a clear distinction between:

- a real live server interaction
- a cache-served response

## Observability

Verbose output should be able to surface:

- cache hits and misses
- retry attempts
- backoff delays
- whether `Retry-After` was honored

These diagnostics are important because cache and retry behavior are otherwise
easy to misinterpret when debugging API calls.

## Cache Management Commands

Cache inspection and management are exposed directly in the CLI:

```bash
restish cache info
restish cache clear
restish cache clear myapi
```

Those commands are part of the operator model for working with a disk-backed
cache, not an afterthought.

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

## Alternatives Considered

### Retry Above The Cache Layer

Would make cache hits participate in retry logic unnecessarily.

### Retry All Non-2xx Responses

Too noisy and too surprising for users.

### Use An Unbounded Disk Cache

Too risky for developer machines and CI environments.

## Relationship To Other Designs

- Design 029 defines where the transport stack is assembled in the request
  pipeline.
- Design 017 defines the diagnostics surface that should expose cache and retry
  behavior.
- Design 030 defines the expectation that silent fallbacks and stale-state bugs
  are unacceptable in core infrastructure.

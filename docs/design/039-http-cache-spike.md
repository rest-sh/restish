# HTTP Cache Spike

## Status

Accepted for implementation, May 2026.

## Product Frame

Problem:
Restish has an HTTP response cache that is useful for repeated API work, but
recent findings showed gaps around immediate staleness and variant selection.
The current short-term fixes avoid serving known-bad entries by refusing to
store `Cache-Control: max-age=0` and `Vary` responses. That is correct, but it
also leaves performance and protocol behavior on the table.

User:
Daily CLI users and API integrators who expect Restish to be fast without
silently serving the wrong representation.

Job:
Repeat GET-style requests during exploration, scripting, generated-command use,
and docs examples while keeping response freshness, auth boundaries, and cache
management predictable.

Outcome:
Restish should keep the operator-facing cache model (`cache info`, scoped
`cache clear`, size-bounded disk storage, API/profile namespaces) while moving
closer to correct private-client HTTP cache behavior.

## Current Implementation

Restish currently layers caching as:

```text
httpcache transport -> retry transport -> base transport
```

The protocol layer is `github.com/gregjones/httpcache`, backed by
`internal/cache.DiskCache`, which implements the small `Get`/`Set`/`Delete`
cache interface. Restish wraps the disk cache before handing it to the transport
so responses with credential-looking headers, `Vary`, or zero max-age are not
stored.

This preserves safety, but it is also a signal that too much HTTP cache
semantics now live in defensive wrappers around an older transport.

## Candidate: `github.com/sandrolain/httpcache`

`github.com/sandrolain/httpcache` is a maintained fork of
`github.com/gregjones/httpcache`. Its public surface is intentionally close to
the existing dependency: the same cache interface is available, and
`NewTransport(c)` still accepts a Restish-compatible backend.

Useful capabilities for Restish:

- automatic stale response validation with `ETag`/`Last-Modified`
- `Cache-Control: no-store` handling
- private-cache mode by default
- optional `Vary` separation
- request-header cache-key dimensions for explicitly configured headers
- cache hit/revalidation marker headers that can support future verbose
  diagnostics

Spike notes:

- `v1.4.0` requires Go `1.25.3`; Restish currently declares Go `1.25.0`.
- A scratch test using `NewTransport(NewMemoryCache())` passed:
  - `Cache-Control: max-age=0` + `ETag` revalidates and serves the cached body
    after a `304 Not Modified`.
  - `Cache-Control: no-store` is not reused.
  - `Vary: Accept` with `EnableVarySeparation = true` keeps JSON and XML
    variants separate when the response includes a valid `Date` header.
  - `CacheKeyHeaders = []string{"Accept"}` also separates known request-header
    dimensions.
- Restish must keep its own sensitive-response guard. The candidate is a
  private HTTP cache, not a Restish secret policy engine; responses containing
  `Set-Cookie` or API-key-like headers should still be refused.

## Recommendation

Prefer a narrow dependency swap spike over writing a new HTTP cache protocol
layer from scratch.

The candidate appears to give Restish the missing option-1 behavior for F68/F73:
conditional revalidation and variant separation, while preserving the existing
disk backend and cache command model. That is much smaller than building and
maintaining a custom RFC 9111 cache transport.

Do not land the swap as a drive-by dependency update. It should be a focused
change with the matrix below as acceptance tests.

## Proposed Implementation Plan

1. Replace `github.com/gregjones/httpcache` with
   `github.com/sandrolain/httpcache`.
2. Keep `internal/cache.DiskCache` as the backend so `cache info`, scoped
   clearing, cache namespaces, file modes, atomic writes, and size eviction
   remain Restish-owned.
3. Configure the new transport with:
   - `EnableVarySeparation = true`
   - private-cache mode (`IsPublicCache = false`)
   - marker headers enabled unless they interfere with output normalization
4. Keep or adapt the Restish storage-policy wrapper so Restish still refuses to
   store:
   - credential-bearing response headers
   - `Set-Cookie`
   - any response Restish classifies as sensitive
5. Remove Restish's temporary `Vary` and `max-age=0` parsing from the wrapper;
   those are HTTP cache semantics owned by the transport library.
6. Bump the module Go directive from `1.25.0` to `1.25.3`.

## Acceptance Tests

Add request/cache transport tests for:

- fresh `Cache-Control: max-age=N` hit
- `max-age=0` with `ETag`, expecting conditional revalidation and cached body
  reuse on `304`
- `max-age=0` with `Last-Modified`, expecting conditional revalidation and
  cached body reuse on `304`
- `Cache-Control: no-store`, expecting two origin hits
- `Vary: Accept`, expecting separate cached variants
- `Vary: *`, expecting no reusable cache hit
- `Set-Cookie`, expecting no stored response
- response with an API-key-like header, expecting no stored response
- unaffiliated Authorization-bearing request, expecting cache bypass
- API/profile Authorization-bearing request, expecting namespace isolation
- cache namespace separation for two APIs/profiles sharing a URL
- `cache clear`, `cache clear <api>`, and `cache info`
- size eviction still removes older entries
- retry layering remains below cache: cache hit does not enter retry logic;
  cache miss still does

## Risks And Open Questions

- The candidate's Go version requirement is slightly ahead of Restish's current
  module directive.
- `Vary` correctness depends on stored responses having enough metadata for
  freshness calculations. Real HTTP responses should include `Date`, but tests
  need to model that explicitly.
- Cache marker headers (`X-From-Cache`, `X-Revalidated`, etc.) are useful for
  verbose diagnostics, but Restish should not expose them as if they came from
  the origin unless that behavior is intentionally documented.
- `CacheKeyHeaders` can include sensitive header values in backend keys. Restish
  should avoid using it for secrets unless key hashing/encryption is added or
  the values are already partitioned by API/profile namespace.

## Rejected Alternative: Keep Wrapper-Only Fixes

Keeping the current dependency and only adding more `Set`-time filters is safe
for v2 release risk, but it cannot implement correct `Vary` selection because
the cache backend sees only the cache key and serialized response bytes. It does
not receive the current request headers needed to select a variant.

That path is acceptable as a temporary safety patch, but it is not the right
long-term cache design.

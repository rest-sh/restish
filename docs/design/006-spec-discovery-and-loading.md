# Spec Discovery And Loading

## Summary

Restish v2 discovers API specs by trying a small ordered set of sources, then
hands the fetched content to registered loaders. Successful results are cached
on disk so generated API commands can be rebuilt without paying the network
cost every time.

## Problem

Restish needs API descriptions in order to generate commands and expose richer
API-aware behavior, but real APIs do not all publish specs in the same place or
with the same headers.

The discovery system needed to balance a few goals:

- work with explicit configuration when the user already knows the spec URL
- find specs automatically when the API advertises them conventionally
- stay fast enough for routine CLI use
- allow additional spec formats in the future without redesigning discovery
- avoid forcing the command tree to depend on live network access

## Design

The design splits the problem into discovery, loading, and caching.

Discovery answers "where should we try to fetch a spec from?" The current
ordered strategy is:

1. cached spec for the registered API name
2. explicit `spec_url` from config
3. Link headers discovered from a GET on the API base URL
4. well-known paths such as `/openapi.json` and `/openapi.yaml`
5. the base URL response body itself

Network probes run in parallel and the first successfully parsed spec wins.
This keeps the common case responsive while still checking several conventional
locations.

Loading answers "does this content look like a format Restish understands, and
can it be parsed into an internal API spec?" That work is delegated to
registered loaders selected by detection plus priority. Built-in v2 behavior
starts with an OpenAPI loader, but the model is intentionally extensible.

Caching answers "can we reuse a previously fetched spec safely?" Restish stores
the raw fetched document and content type in a CBOR cache keyed by API name. A
cache entry is considered valid only when:

- it has not expired
- it matches the running Restish version

On cache hits, Restish re-parses the raw cached document through the loader
pipeline instead of trusting a previously serialized in-memory representation.
That keeps the cache format simple and lets parser behavior evolve with the
binary.

## Alternatives Considered

### Require explicit spec URLs for all APIs

This is simple, but it gives up too much convenience. Restish should be able to
meet APIs halfway when they publish a spec in a conventional location.

### Always fetch the network on startup

That would make generated commands more dynamic, but it would also slow startup,
make offline use worse, and make the command tree depend on network conditions.

### Cache parsed internal structures instead of raw spec documents

That might save parse time, but it creates tighter coupling between cache data
and internal implementation details. Caching raw documents is more stable and
easier to reason about.

## Notes

The current implementation maps closely to this design:

- `internal/spec/discover.go` implements discovery order, parallel probes, and
  cache TTL handling
- `internal/spec/spec.go` defines the loader abstraction and selection rules
- `internal/spec/openapi.go` provides the built-in OpenAPI loader
- `internal/spec/cache.go` stores cached specs as CBOR entries keyed by API
  name

One detail worth preserving is that cache validity is tied to the Restish
version. That makes it safe to change parsing behavior or internal assumptions
without trying to maintain compatibility with stale cached interpretations.

# Conditional Operation Security

## Status

Accepted for the v2 development line.

## Problem

OpenAPI security can express alternatives and conjunctions, but it cannot make
one alternative conditional on the concrete value of a path parameter. Some
transparent APIs need exactly that behavior. Publishing all sets as ordinary
alternatives loses the condition, so a client can request too little authority
and discover the missing requirement only after the server rejects the request.

## Decision

Restish supports the additive operation extension
`x-restish-security-alternatives`. It is an ordered list of rules:

```yaml
x-restish-security-alternatives:
  - when:
      pathParameter: path
      prefix: .github/workflows/
    alternatives: [1]
```

`alternatives` contains zero-based indexes into that operation's standard
OpenAPI `security` array. After the concrete request path is known, Restish
extracts and percent-decodes the named path parameter. The first matching rule
replaces the candidate list with the indexed alternatives. When no rule matches,
standard OpenAPI selection is unchanged.

The extension only narrows standard alternatives; it cannot create a scheme,
scope, or conjunction. Invalid parameter names, empty predicates, repeated
indexes, and out-of-range indexes fail specification loading. Authority remains
visible to ordinary OpenAPI tooling and the extension cannot silently widen it.

## Compatibility

The extension is optional. Existing specifications and clients continue to see
the complete standard OpenAPI security alternatives. Inspection, help, and
configuration retain the complete static view because no concrete path value
exists at those stages. Generated commands and generic requests both apply the
predicate immediately before operation authentication planning.

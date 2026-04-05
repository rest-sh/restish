# TLS And Cert Handling

## Summary

Restish v2 centralizes TLS behavior in request option processing. The same TLS
option model supports ordinary HTTPS verification, custom CA bundles, and mTLS
via client certificate files.

The CLI also exposes a `cert` command for inspecting a server's presented
certificate chain.

## Problem

HTTP clients need secure defaults, but API tooling also needs escape hatches for
real-world environments:

- internal PKI and custom CA roots
- mutual TLS using certificate/key files
- hardware-backed or external signing flows
- debugging certificate chains and expiry

The design needed to keep those capabilities available without teaching every
request path about TLS details independently.

## Design

TLS behavior is derived from request options and assembled into a `tls.Config`
before the HTTP transport is created.

The current model supports:

- `--rsh-insecure` to skip certificate verification
- minimum TLS version selection
- custom CA bundles
- client certificate plus private key files for mTLS

File-based mTLS is treated as part of ordinary request option resolution. When
client certificate support is configured, the request transport is given the
client identity before any request is sent.

The separate `cert` command reuses the same TLS option model to connect to a
server, perform a handshake, and render the peer certificate chain. That keeps
certificate inspection aligned with the same trust and CA settings the request
path uses.

## Examples

Using a custom CA bundle:

```bash
restish get --rsh-ca-cert ./corp-ca.pem https://api.example.com/items
```

Using mTLS with certificate files:

```bash
restish get \
  --rsh-client-cert ./client.pem \
  --rsh-client-key ./client.key \
  --rsh-ca-cert ./ca.pem \
  https://api.example.com/items
```

Inspecting a server certificate chain:

```bash
restish cert --rsh-ca-cert ./ca.pem https://api.example.com
```

which renders details like:

- subject
- issuer
- validity window
- DNS names

and can warn with a non-zero exit code when expiry is near:

```bash
restish cert --warn-days 14 https://api.example.com
```

## Alternatives Considered

### Scatter TLS handling across commands

That would make the request path harder to reason about and would risk
inconsistent behavior between regular requests and certificate inspection.

### Treat certificate inspection as a completely separate tool

A separate tool would work, but it would duplicate option handling and make it
harder to inspect the exact trust context Restish itself would use.

## Notes

The current implementation reflects this design directly:

- `internal/request/tls.go` builds TLS config from request options
- `internal/cli/cert.go` implements the certificate inspection command

One detail worth preserving is that the `cert` command uses the same TLS option
resolution path as normal requests. That keeps trust decisions, CA overrides,
and mTLS behavior consistent between inspection and actual API calls.

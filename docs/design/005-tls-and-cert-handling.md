# TLS And Cert Handling

## Summary

Restish v2 centralizes TLS behavior in request option processing. The same TLS
option model supports ordinary HTTPS verification, custom CA bundles, file-based
mTLS, and external TLS signer integration.

The CLI also exposes a `cert` command for inspecting a server's presented
certificate chain using the same trust-resolution path as normal requests.

## Goals

- secure defaults for ordinary HTTPS
- explicit escape hatches for internal PKI and specialized mTLS setups
- one TLS-resolution path shared by requests and certificate inspection
- compatibility with external signer plugins
- cancellation-aware certificate inspection and request setup

## Non-Goals

- scattering TLS handling across unrelated commands
- silently falling back from mTLS or TLS-signer failure to a weaker mode
- making certificate inspection a separate trust model from actual requests

## TLS Option Model

TLS behavior is derived from request options and assembled into a `tls.Config`
before the HTTP transport is created.

The current model should support:

- `--rsh-insecure` to skip certificate verification
- minimum TLS version selection
- custom CA bundles
- client certificate plus private key files for mTLS
- external TLS signer plugins for non-exportable private keys

The request planner is responsible for deciding which of those inputs apply to a
given request. The transport builder is responsible for turning that decision
into concrete TLS settings.

## File-Based mTLS

File-based mTLS is part of ordinary request option resolution.

When client certificate support is configured, Restish should:

- load and validate the certificate/key pair before request execution
- attach the client identity to the transport
- fail clearly if the certificate and key are unusable

There is no silent fallback to anonymous TLS if the user explicitly requested
client authentication.

## Custom CA Bundles

Custom CA support exists for internal PKI and development environments.

CA overrides should:

- augment or replace trust roots in a deliberate way
- apply consistently to normal requests and the `cert` command
- be visible in verbose diagnostics when helpful

## `--rsh-insecure`

Skipping certificate verification is a deliberate unsafe override. It should be:

- explicit
- local to the invocation unless persisted intentionally
- visible in diagnostics when troubleshooting

This flag exists as an operator escape hatch, not a silent fallback mode.

## External TLS Signers

TLS signer plugins are part of the TLS design, not an unrelated plugin feature.

The decision tree is:

- if client cert/key files are configured, use file-based mTLS
- if a TLS signer is configured, use the signer-backed certificate path
- if both are configured, the product should define a clear precedence or treat
  the configuration as invalid

The signer protocol itself is defined in design 021; this document defines that
signer selection belongs to the same profile-driven TLS option model as every
other TLS feature.

## `cert` Command

The `cert` command reuses the same TLS option model to connect to a server,
perform a handshake, and render the peer certificate chain.

That keeps certificate inspection aligned with the same trust and CA settings
the request path uses.

The command should surface at least:

- subject
- issuer
- validity window
- SANs / DNS names
- warning state such as "expiring soon"

## Cancellation

TLS operations should honor the command context. This includes:

- slow request handshakes
- `cert` command handshakes
- plugin-backed TLS signer work

Ignoring command context during TLS setup is a design bug because it makes Ctrl-C
unreliable and breaks embedding expectations.

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

and warning on upcoming expiry:

```bash
restish cert --warn-days 14 https://api.example.com
```

## Alternatives Considered

### Scatter TLS Handling Across Commands

Too inconsistent and harder to reason about.

### Separate Certificate Inspection Tool

Would duplicate trust-resolution behavior and produce confusing mismatches.

## Relationship To Other Designs

- Design 002 defines where TLS-related profile config lives.
- Design 021 defines TLS signer plugin lifecycle and protocol.
- Design 029 defines where TLS configuration is applied in request execution.
- Design 030 defines the safety posture for insecure overrides and signer
  teardown.

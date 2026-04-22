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

## Assembly Algorithm

Conceptually, TLS assembly proceeds in this order:

1. start from secure default verification settings
2. apply minimum TLS version if configured
3. apply CA-root customization
4. resolve client-auth mode:
   - none
   - file-based client certificate
   - TLS signer plugin
5. build the resulting `tls.Config`
6. attach it to the transport used for the request or cert inspection

The important design rule is that TLS behavior is assembled once from explicit
inputs, not improvised later during request execution.

More concretely, the transport builder should derive a single resolved TLS plan
containing at least:

- verification mode
- minimum TLS version
- root CA pool source
- client-auth source
- optional server-name override if the CLI supports one in the future

The resulting `tls.Config` should be built from that resolved plan rather than
from raw flags scattered across multiple layers.

## Precedence And Conflict Rules

TLS inputs can come from defaults, config, profiles, environment variables, and
CLI flags. By the time TLS assembly begins, those sources should already have
been merged into one effective runtime configuration.

Within that effective configuration, TLS-specific conflict rules should be
explicit:

1. explicit CLI flags beat profile-derived values
2. `--rsh-insecure` only affects server verification and does not implicitly
   disable client authentication
3. file-based mTLS and TLS-signer mTLS are mutually exclusive unless the product
   explicitly defines one as higher precedence
4. invalid combinations should fail during request planning, before network I/O

Failing early matters because TLS configuration problems are easier to diagnose
before a connection attempt adds unrelated handshake noise.

## File-Based mTLS

File-based mTLS is part of ordinary request option resolution.

When client certificate support is configured, Restish should:

- load and validate the certificate/key pair before request execution
- attach the client identity to the transport
- fail clearly if the certificate and key are unusable

There is no silent fallback to anonymous TLS if the user explicitly requested
client authentication.

Certificate and key material should be loaded once per resolved runtime config
when practical, not re-read for every request in a command that issues multiple
requests. This reduces repeated filesystem work and ensures failures happen
deterministically before the request loop begins.

## Custom CA Bundles

Custom CA support exists for internal PKI and development environments.

CA overrides should:

- augment or replace trust roots in a deliberate way
- apply consistently to normal requests and the `cert` command
- be visible in verbose diagnostics when helpful

Whether the implementation augments or replaces the system pool in a given mode
should be explicit and stable rather than accidental.

The preferred model is:

- ordinary custom CA configuration augments the platform trust store
- an explicit "trust only this bundle" mode would need to be separately defined

If platform root loading fails, the runtime should surface that local error
clearly rather than silently falling back to a weaker trust base.

## `--rsh-insecure`

Skipping certificate verification is a deliberate unsafe override. It should be:

- explicit
- local to the invocation unless persisted intentionally
- visible in diagnostics when troubleshooting

This flag exists as an operator escape hatch, not a silent fallback mode.

Verbose diagnostics should make it obvious when insecure verification is active,
because request failures and certificate output otherwise become harder to
interpret.

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

Signer-backed TLS should still require an explicit client certificate identity.
The plugin may supply signing capability, certificate metadata, or both, but
the host must treat the final result as one resolved client-auth configuration
instead of a late transport patch.

## Certificate Inspection

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

The inspection command is therefore not a separate TLS tool; it is another
consumer of the shared TLS configuration model.

The `cert` command should define a stable target-resolution contract:

1. resolve scheme, host, and port from the target argument
2. reject unsupported non-TLS schemes early
3. build the same TLS plan used for requests
4. perform the handshake without issuing an HTTP application request
5. render the observed peer chain and local warning state

This makes the command useful both as a debugging tool and as a way to confirm
that profile-driven TLS settings are wired as expected.

## Cancellation

TLS operations should honor the command context. This includes:

- slow request handshakes
- `cert` command handshakes
- plugin-backed TLS signer work

Ignoring command context during TLS setup is a design bug because it makes Ctrl-C
unreliable and breaks embedding expectations.

## Failure Model

TLS failures should remain attributable to the actual failing stage where
possible:

- CA loading
- certificate/key parsing
- signer startup
- handshake failure
- hostname verification

This matters because TLS issues are already hard enough to debug without vague
"request failed" behavior.

The product should avoid flattening all TLS problems into one generic exit path
internally, even if several cases eventually map to the same process exit code.
Users need the stderr explanation to preserve which stage failed.

## Cert Warning And Exit Behavior

The `cert` command exposes operator-oriented checks, so its warning model should
be explicit.

Recommended behavior:

- successful inspection with no warning threshold breach exits `0`
- successful inspection with an expiry warning may still exit `0` unless the
  product later adds an explicit "fail on warning" mode
- handshake or trust failure exits as a local CLI failure, not as an HTTP error

This keeps certificate inspection distinct from ordinary request status-code
handling.

## Connection Reuse And Isolation

Resolved TLS configuration is part of the transport identity. Requests that
differ in any TLS-relevant field should not accidentally share a transport or
connection pool entry.

At minimum, transport reuse boundaries should consider:

- insecure versus verified mode
- root CA configuration
- client-auth configuration
- signer-backed versus file-backed identity

This matters for both correctness and trust isolation.

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

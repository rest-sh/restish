# TLS Signer Plugins

## Summary

TLS signer plugins support mTLS setups where Restish must present a client
certificate but cannot hold the private key material directly. Instead,
Restish delegates signing operations to a long-lived external process.

## Problem

Some mTLS environments use:

- hardware tokens
- PKCS#11-backed keys
- externally managed signing services

In those cases, the normal `client_cert` and `client_key` file model is not
enough. Restish still needs a `tls.Certificate` for the Go TLS stack, but the
private key operations have to stay outside the process boundary.

## Design

TLS signer plugins are discovered through the normal plugin manifest path, but
they are treated as a distinct plugin type via the `tls-signer` hook.

The profile-level config model is:

- `tls_signer`: plugin name or executable name
- `tls_signer_params`: plugin-specific key/value parameters

When a request needs mTLS and a TLS signer is configured, Restish resolves the
plugin path and starts a persistent signer process. The startup handshake is:

1. Restish starts the plugin executable.
2. Restish sends an `init` message containing plugin parameters.
3. The plugin replies with a `ready` message containing the leaf certificate.
4. Restish parses the certificate and constructs a `tls.Certificate` whose
   `PrivateKey` is a proxy object that calls back into the plugin.

During the TLS handshake, Go eventually calls `Sign(...)` on that proxy. The
proxy then:

1. sends a `sign` message with the digest and hash identifier
2. waits for a reply containing either `signature` or `error`
3. returns the signature bytes back to the TLS stack

This gives Restish the certificate material needed to authenticate the client
without ever requiring access to the underlying private key.

## Why It Is Separate

TLS signer plugins could have been folded into the general command-plugin
protocol, but they have a very different operational profile:

- they exist to satisfy the Go TLS stack, not to add user-facing commands
- they need a stable request/reply signer object, not workflow orchestration
- they operate on key material boundaries, which deserves a smaller contract

Documenting them separately makes the security and lifecycle assumptions much
clearer.

## Failure Model

If the plugin cannot start, does not return `ready`, returns malformed
certificate data, or dies before a later signing operation, Restish surfaces a
clear error and lets the TLS handshake fail cleanly.

That failure path is important: external signers are optional infrastructure,
not something Restish should silently fall back from.

## Alternatives Considered

### Only support PEM client keys on disk

That keeps the implementation smaller, but it excludes hardware-backed and
externally managed mTLS environments that are common in enterprise use.

### Shell out for every sign request without a persistent process

That would simplify state management, but it would make the protocol slower and
more brittle. A persistent signer fits repeated signing during a TLS session
better.

### Reuse the command-plugin protocol directly

Possible, but unnecessarily broad. The TLS signer contract is much smaller and
easier to reason about when it stands on its own.

## Notes

The current implementation lives in
[`internal/plugin/tls_signer.go`](/Users/daniel/src/restish2/internal/plugin/tls_signer.go),
with request integration in
[`internal/cli/hooks.go`](/Users/daniel/src/restish2/internal/cli/hooks.go)
and profile-level coverage in
[`internal/cli/tls_signer_test.go`](/Users/daniel/src/restish2/internal/cli/tls_signer_test.go).

One detail worth preserving is that TLS signer selection happens through the
same profile-driven request options model as the rest of Restish's transport
configuration. It is an extension of the mTLS story, not a separate config
system.

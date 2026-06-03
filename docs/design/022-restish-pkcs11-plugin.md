# `restish-pkcs11` Plugin

## Summary

`restish-pkcs11` is the concrete TLS-signer plugin for PKCS#11 devices such as
YubiKey-backed PIV tokens. It lets Restish perform mTLS client authentication
while keeping the private key inside the PKCS#11 provider.

This plugin is the main real-world validation of the generic TLS-signer design.

## Goals

- allow Restish to use non-exportable private keys for mTLS
- support common PKCS#11 deployment patterns without bloating Restish core
- make token and certificate selection explicit and non-ambiguous
- keep unattended and interactive PIN workflows possible
- map the generic TLS-signer protocol onto `crypto11` cleanly

## Non-Goals

- auto-selecting a "best guess" token when several could match
- exposing arbitrary PKCS#11 configuration complexity directly through the host
- weakening signer failure into an implicit fallback to file-based or anonymous
  TLS

## Position In The Architecture

This plugin is a concrete implementation of design 021.

The host responsibilities are:

- select the plugin from profile config
- launch and manage signer lifetime
- ask for a certificate and later signatures

The plugin responsibilities are:

- resolve PKCS#11 module and token config
- establish a `crypto11.Context`
- select exactly one signer-backed certificate
- perform sign operations when requested

## Manifest

The plugin advertises a simple manifest:

- `name: pkcs11`
- `hooks: ["tls-signer"]`

It does not add commands, formatters, or loaders.

## Startup Flow

On startup the plugin expects the standard TLS-signer `init` message and
interprets the `params` object as PKCS#11-specific configuration.

The startup flow is:

1. parse and validate configuration
2. resolve the module path
3. resolve token selection
4. resolve PIN policy
5. create the `crypto11.Context`
6. find matching paired certificates
7. require exactly one usable match
8. return the leaf certificate DER bytes in the `ready` message
9. enter the long-lived signing loop

If any of those stages fails, startup fails immediately and clearly.

## Configuration Model

The plugin accepts a few aliases so profile config can stay ergonomic:

- `module` or `path` for the PKCS#11 shared library
- `token_label` or `label`
- `token_serial` or `serial`
- `slot`
- `pin`
- `pin_env`
- `login_not_supported`

The configuration surface is intentionally narrower than raw PKCS#11 because the
goal is a stable signer plugin, not a complete PKCS#11 management tool.

## Token Selection

At most one token selector may be provided:

- token label
- token serial
- slot number

If no selector is provided, the plugin enumerates present tokens and
auto-selects the slot only when exactly one token is available. Otherwise, the
plugin refuses ambiguous selection so it does not accidentally choose the wrong
certificate when multiple tokens or slots are available.

If more than one selector is supplied, startup should fail.

## Module Path Resolution

Module path resolution is ordered:

1. explicit `module` or `path`
2. `PKCS11_MODULE_PATH`
3. a small OS-specific list of common OpenSC library paths

If none of those resolve, startup fails with a clear error instead of guessing
more broadly.

This preserves operator control and keeps debugging manageable across different
platforms and distributions.

## PIN Resolution

PIN lookup is ordered:

1. explicit `pin`
2. environment variable named by `pin_env`
3. `PKCS11_PIN`
4. an interactive prompt on `/dev/tty` or `CONIN$`

If `login_not_supported` is true, the plugin skips the PIN requirement.

This model keeps unattended execution possible through config or environment
variables while still allowing an interactive fallback for local use.

The design intentionally allows explicit `pin` even though storing PINs in
config is not ideal, because some unattended environments need that option.
Operator guidance should still prefer env vars or terminal prompting where
possible.

## Certificate And Signer Loading

After parsing config, the plugin creates a `crypto11.Context` and calls
`FindAllPairedCertificates()`.

It currently requires exactly one matching certificate:

- zero matches is an error
- more than one match is an error

That is a conservative design choice. It avoids inventing extra selection
heuristics inside the plugin and keeps failure modes obvious.

Once it has the matching certificate, it verifies that the private key exposes
`crypto11.Signer`, returns the leaf certificate DER bytes in the `ready`
message, and then enters the long-lived signing loop.

## Signing Behavior

For each `sign` request from Restish, the plugin reads:

- `digest`
- `hash`
- optional `padding`
- optional `salt_length`

It maps those onto `crypto.SignerOpts`, including RSA-PSS when
`padding == "pss"`, then calls the PKCS#11-backed signer.

Replies are intentionally small:

- `{"signature": ...}` on success
- `{"error": "..."}` on failure

The plugin should not retain per-sign mutable session state beyond what the
underlying PKCS#11 context requires.

## Error Model

Startup errors should identify the failing stage when practical:

- module path resolution
- token selection
- PIN lookup
- token login
- certificate matching
- signer capability validation

Sign-time errors should also be explicit, especially for cases such as:

- token removed
- wrong PIN / login failure
- unsupported signer options
- device/session busy conditions

The host may surface plugin stderr as part of these failures, so the plugin
should emit diagnostics that are useful but do not leak secrets.

## Shutdown

The plugin should treat stdin EOF or host process exit as the normal shutdown
signal.

Shutdown behavior should include:

- closing the `crypto11.Context`
- releasing token sessions cleanly
- exiting promptly so the host does not need to kill the signer in the common
  case

Graceful shutdown matters because leaked signer processes can leave hardware
token sessions busy for later requests.

## Security Properties

The key security property of the plugin is:

- Restish receives the certificate but not the private key
- all private-key operations remain inside the PKCS#11 provider

This is stronger operationally than file-based key export and is the reason the
plugin exists at all.

## Alternatives Considered

### Auto-Select The First Available Certificate

Convenient, but too risky.

### Require All Secrets In Config

Would simplify startup but would make local interactive use clumsy and
encourage storing PINs where they do not need to live.

### Push PKCS#11 Specifics Into Restish Core

Would enlarge the main binary, add CGO pressure, and blur the core/plugin
boundary.

## Relationship To Other Designs

- Design 005 defines how TLS-signer selection fits into request TLS behavior.
- Design 021 defines the generic TLS-signer lifecycle and host contract.
- Design 030 defines the expectations around sensitive diagnostics and cleanup.

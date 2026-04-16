# `restish-pkcs11` Plugin

## Summary

`restish-pkcs11` is the concrete TLS-signer plugin for PKCS#11 devices such as
YubiKey-backed PIV tokens. It lets Restish perform mTLS client authentication
while keeping the private key inside the PKCS#11 provider.

This plugin is the main real-world validation of the generic TLS-signer design.

## Problem

The generic TLS-signer protocol explains how Restish can delegate signing, but
contributors still need a concrete design for the most important current
implementation:

- selecting a PKCS#11 module and token
- sourcing a PIN safely
- loading exactly one certificate/key pair
- mapping Restish signer requests onto `crypto11`

Without that concrete record, the TLS-signer design stays abstract and it is
harder to see which tradeoffs the shipped plugin actually makes.

## Design

The plugin advertises a simple manifest:

- `name: pkcs11`
- `hooks: ["tls-signer"]`

On startup it expects the standard TLS-signer `init` message and interprets the
`params` object as PKCS#11-specific configuration.

### Configuration Model

The plugin accepts a few aliases so profile config can stay ergonomic:

- `module` or `path` for the PKCS#11 shared library
- `token_label` or `label`
- `token_serial` or `serial`
- `slot`
- `pin`
- `pin_env`
- `login_not_supported`

Exactly one token selector must be provided:

- token label
- token serial
- slot number

That constraint is intentional. The plugin refuses ambiguous selection so it
does not accidentally choose the wrong certificate when multiple tokens or
slots are available.

### Module Path Resolution

Module path resolution is ordered:

1. explicit `module` or `path`
2. `PKCS11_MODULE_PATH`
3. a small OS-specific list of common OpenSC library paths

If none of those resolve, startup fails with a clear error instead of guessing
more broadly.

### PIN Resolution

PIN lookup is also ordered:

1. explicit `pin`
2. environment variable named by `pin_env`
3. `PKCS11_PIN`
4. an interactive prompt on `/dev/tty` or `CONIN$`

If `login_not_supported` is true, the plugin skips the PIN requirement.

This model keeps unattended execution possible through config or environment
variables while still allowing an interactive fallback for local use.

### Certificate And Signer Loading

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

### Signing Behavior

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

## Alternatives Considered

### Auto-select the first available certificate

Convenient, but too risky. The current "exactly one match" rule is safer and
easier to debug.

### Require all secrets in config

That would simplify startup, but it would make local interactive use clumsy and
encourage storing PINs where they do not need to live.

### Push PKCS#11 specifics into Restish core

That would make the main binary larger and couple platform-specific token logic
to the request stack. It would also pull PKCS#11's CGO requirements into the
main Restish binary. Keeping it in a plugin preserves the cleaner core
boundary and isolates the CGO-dependent code to a separately shipped
executable.

## Notes

The implementation lives in
[`cmd/restish-pkcs11/main.go`](../../cmd/restish-pkcs11/main.go)
and
[`cmd/restish-pkcs11/config.go`](../../cmd/restish-pkcs11/config.go),
with focused coverage in
[`cmd/restish-pkcs11/config_test.go`](../../cmd/restish-pkcs11/config_test.go)
and
[`cmd/restish-pkcs11/main_test.go`](../../cmd/restish-pkcs11/main_test.go).

One detail worth preserving is that the plugin narrows ambiguity aggressively.
That makes it slightly stricter to configure, but much safer for hardware-backed
credentials.

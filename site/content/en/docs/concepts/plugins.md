---
title: Plugins
linkTitle: Plugins
weight: 50
description: Understand how plugins extend Restish without replacing the core request model.
---

Plugins are extension points around the Restish runtime. They can add auth,
request and response middleware, spec loaders, formatters, top-level commands,
and TLS signing.

Plugins are trusted local executables and run at your own risk. Restish checks
their manifest and declared capabilities, but it does not sandbox plugin code.

## Operator vs Author

Operators install, configure, run, verify, and debug plugins. They should start
with [Install and Use Plugins](/docs/plugins/install-and-use/).

Authors implement plugin protocols and should start with [Plugin Quickstart](/docs/plugins/quickstart/).

Keeping these paths separate matters: a user should not need protocol internals
to use `restish-csv`, `restish-bulk`, `restish-mcp`, or `restish-pkcs11`.

## Plugin Types

- Hook plugins are short-lived integrations for auth, middleware, loaders, and formatters.
- Command plugins add top-level workflows such as `restish bulk` and `restish mcp`.
- TLS signer plugins keep private keys outside the Restish process for mTLS.

## Design Principle

Plugins should delegate HTTP, auth, TLS, retries, cache, and output back to
Restish whenever they want normal user behavior. That keeps plugin workflows
from becoming separate tools with surprising config and security rules.

## Related Pages

- [Plugins](/docs/plugins/)
- [Hook Plugins](/docs/plugins/hook-plugins/)
- [Command Plugins](/docs/plugins/command-plugins/)
- [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/)
- [Plugin Reference](/docs/reference/plugins/)

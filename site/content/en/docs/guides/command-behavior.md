---
title: Command Behavior
linkTitle: Command Behavior
weight: 85
description: Understand Restish exit codes, diagnostics, and stdout versus stderr behavior.
---

Restish is designed to work both interactively and in scripts. That means its
output channels and exit codes are deliberate parts of the interface.

## Exit Codes

HTTP status families map to CLI exit codes:

- `2xx -> 0`
- `3xx -> 3`
- `4xx -> 4`
- `5xx -> 5`

## Stdout vs Stderr

Restish keeps normal command output on stdout and diagnostics on stderr.

Stdout is for response bodies and machine-readable output. Stderr is for
prompts, warnings, and verbose request and response logs.

## Verbose Mode

Use `-v` when you need request and response visibility:

```bash
restish https://api.rest.sh/images -v
```

Use `-vv` when you also want more TLS detail.

## Ignore Status Codes

If you care more about capturing the body than about HTTP-derived exit codes,
use:

```bash
restish https://api.rest.sh/images --rsh-ignore-status-code
```

## Silent Mode

If you want only the exit code:

```bash
restish https://api.rest.sh/images --rsh-silent
```

## Bare URL Shortcut

A bare URL or API-relative target is treated as `GET`.

```bash
restish https://api.rest.sh/
restish myapi/items
```

## Related Pages

- [Global Flags Reference](/docs/reference/global-flags/)
- [Requests Guide](/docs/guides/requests/)

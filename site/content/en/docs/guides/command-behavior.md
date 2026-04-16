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

This is deliberate: HTTP status is part of the command contract, not just text
printed to the terminal.

## Stdout vs Stderr

Restish keeps normal command output on stdout and diagnostics on stderr.

Stdout is for response bodies and machine-readable output. Stderr is for
prompts, warnings, and verbose request and response logs.

That separation is what makes filtered output, redirects, and shell pipelines
work predictably.

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

Use this when the body matters more than the exit code, such as when you are
capturing a structured error response for debugging.

## Silent Mode

If you want only the exit code:

```bash
restish https://api.rest.sh/images --rsh-silent
```

This is useful in probes, CI checks, and wrapper scripts where only success or
failure matters.

## Bare URL Shortcut

A bare URL or API-relative target is treated as `GET`.

```bash
restish https://api.rest.sh/
restish myapi/items
```

## Related Pages

- [Global Flags Reference](/docs/reference/global-flags/)
- [Output Defaults](/docs/reference/output-defaults/)
- [Requests Guide](/docs/guides/requests/)

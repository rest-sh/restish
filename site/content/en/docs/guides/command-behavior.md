---
title: Command Behavior
linkTitle: Command Behavior
weight: 85
description: Understand exit codes, redirects, diagnostics, stdout, stderr, and script-friendly behavior.
---

Restish is designed for terminals and scripts. Output channels, exit codes, and
verbose diagnostics are part of the interface.

## Exit Codes

HTTP status families map to CLI exit codes:

| HTTP status | Exit code |
| --- | --- |
| `2xx` | `0` |
| `3xx` | `3` |
| `4xx` | `4` |
| `5xx` | `5` |
| Network, parse, config, or command errors | `1` |

Inspect an error body without failing the shell command:

```bash
restish https://api.rest.sh/status/404 --rsh-ignore-status-code
```

## Stdout And Stderr

Response output goes to stdout. Diagnostics, verbose request/response metadata,
progress, and warnings go to stderr.

```bash
restish -v https://api.rest.sh/images/jpeg > dragonfly.jpg 2> headers.txt
```

## Verbose Mode

```bash
restish -v https://api.rest.sh/headers
restish -vv https://api.rest.sh/headers
```

`-v` shows request and response headers plus the resolved config path, profile,
auth state, input source, request body media type, response decode media type,
filter language, output format, plugin invocations, and a compact pipeline
summary when those details apply. `-vv` adds more TLS detail.

## Redirects

Use redirect fixtures to inspect behavior:

```bash
restish https://api.rest.sh/redirect/2 -v
restish 'https://api.rest.sh/redirect-to?url=/get&status_code=307' -v
```

When auth or custom headers are involved, use verbose mode to confirm what is
sent after redirects.

## Timeouts

```bash
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

Timeouts are useful in scripts where a hanging request is worse than a clear
failure.

## Silent Mode

Use `-S` when only the exit code matters:

```bash
restish -S https://api.rest.sh/status/204
```

## Related Pages

- [Global Flags](/docs/reference/global-flags/)
- [Retries and Caching](../retries-and-caching/)
- [Troubleshooting](../troubleshooting/)
- [Output Defaults](/docs/reference/output-defaults/)
